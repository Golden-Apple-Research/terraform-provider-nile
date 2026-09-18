// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// NewWorkspaceComputeUsageDataSource constructs the data source for the
// compute usage metrics of a workspace.
func NewWorkspaceComputeUsageDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_compute_usage",
		workspaceComputeUsageSchema,
		readWorkspaceComputeUsage,
	)
}

// chartPointModel is one point of the usage chart.
type chartPointModel struct {
	// X is the point on the x axis.
	X types.Int64 `tfsdk:"x"`
	// Y is the point on the y axis.
	Y types.Float64 `tfsdk:"y"`
}

// instanceUsageModel is the usage of a single compute instance.
type instanceUsageModel struct {
	// Name is the instance name.
	Name types.String `tfsdk:"name"`
	// Size is the instance compute size.
	Size types.String `tfsdk:"size"`
	// TotalVCPUHours is the total vCPU hours consumed by the instance.
	TotalVCPUHours types.Float64 `tfsdk:"total_vcpu_hours"`
	// Start is the start of the instance usage window.
	Start types.String `tfsdk:"start"`
	// End is the end of the instance usage window.
	End types.String `tfsdk:"end"`
}

// databaseUsageModel is the usage of one database within a period.
type databaseUsageModel struct {
	// Name is the database name.
	Name types.String `tfsdk:"name"`
	// TotalVCPUHours is the total vCPU hours consumed by the database.
	TotalVCPUHours types.Float64 `tfsdk:"total_vcpu_hours"`
	// Start is the start of the database usage window.
	Start types.String `tfsdk:"start"`
	// End is the end of the database usage window.
	End types.String `tfsdk:"end"`
	// Instances are the per-instance usage entries of the database.
	Instances []instanceUsageModel `tfsdk:"instances"`
}

// computeUsagePeriodModel is one compute usage period of a workspace.
type computeUsagePeriodModel struct {
	// TotalVCPUHours is the total vCPU hours consumed in the period.
	TotalVCPUHours types.Float64 `tfsdk:"total_vcpu_hours"`
	// Start is the start of the period.
	Start types.String `tfsdk:"start"`
	// End is the end of the period.
	End types.String `tfsdk:"end"`
	// MaxCPUCount is the maximum CPU count in the chart data.
	MaxCPUCount types.Int64 `tfsdk:"max_cpu_count"`
	// ChartPoints are the chart data points of the period.
	ChartPoints []chartPointModel `tfsdk:"chart_points"`
	// Databases are the per-database usage entries of the period.
	Databases []databaseUsageModel `tfsdk:"databases"`
	// Raw is the redacted JSON payload of the period.
	Raw types.String `tfsdk:"raw_json"`
}

// workspaceComputeUsageDataSourceModel is the state of the
// nile_workspace_compute_usage data source.
type workspaceComputeUsageDataSourceModel struct {
	// WorkspaceSlug is the workspace whose usage is read.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Start is the optional start of the usage window.
	Start types.String `tfsdk:"start"`
	// End is the optional end of the usage window.
	End types.String `tfsdk:"end"`
	// ID is the stable identifier of this data source instance.
	ID types.String `tfsdk:"id"`
	// Periods are the compute usage periods returned by the API.
	Periods []computeUsagePeriodModel `tfsdk:"periods"`
}

// workspaceComputeUsageSchema builds the schema of the workspace compute
// usage data source.
func workspaceComputeUsageSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the compute usage of a workspace via " +
			"`GET /workspaces/{workspaceSlug}/metrics/compute`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose compute usage is read.",
			},
			"start": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					isRFC3339Validator{},
				},
				MarkdownDescription: "RFC3339 timestamp marking the start of the usage window (`start` query parameter).",
			},
			"end": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					isRFC3339Validator{},
				},
				MarkdownDescription: "RFC3339 timestamp marking the end of the usage window (`end` query parameter).",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"periods": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Compute usage periods returned by the API.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"total_vcpu_hours": schema.Float64Attribute{Computed: true, MarkdownDescription: "Total vCPU hours in the period."},
					"start":            schema.StringAttribute{Computed: true, MarkdownDescription: "Start of the period."},
					"end":              schema.StringAttribute{Computed: true, MarkdownDescription: "End of the period."},
					"max_cpu_count":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum CPU count in the chart data."},
					"chart_points": schema.ListNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Chart data points of the period.",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"x": schema.Int64Attribute{Computed: true, MarkdownDescription: "Point on the x axis."},
							"y": schema.Float64Attribute{Computed: true, MarkdownDescription: "Point on the y axis."},
						}},
					},
					"databases": schema.ListNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Per-database usage in the period.",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Database name."},
							"total_vcpu_hours": schema.Float64Attribute{Computed: true, MarkdownDescription: "Total vCPU hours of the database."},
							"start":            schema.StringAttribute{Computed: true, MarkdownDescription: "Start of the database usage window."},
							"end":              schema.StringAttribute{Computed: true, MarkdownDescription: "End of the database usage window."},
							"instances": schema.ListNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Per-instance usage of the database.",
								NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
									"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Instance name."},
									"size":             schema.StringAttribute{Computed: true, MarkdownDescription: "Instance compute size."},
									"total_vcpu_hours": schema.Float64Attribute{Computed: true, MarkdownDescription: "Total vCPU hours of the instance."},
									"start":            schema.StringAttribute{Computed: true, MarkdownDescription: "Start of the instance usage window."},
									"end":              schema.StringAttribute{Computed: true, MarkdownDescription: "End of the instance usage window."},
								}},
							},
						}},
					},
					"raw_json": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Redacted JSON payload of the period."},
				}},
			},
		},
	}
}

// readWorkspaceComputeUsage fetches the compute usage of a workspace and
// populates the data source state from it.
func readWorkspaceComputeUsage(ctx context.Context, client *nileapi.Client, data *workspaceComputeUsageDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	periods, err := client.ListWorkspaceComputeUsage(ctx, workspaceSlug, data.Start.ValueString(), data.End.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing workspace compute usage",
			fmt.Sprintf("Could not list compute usage of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Periods = make([]computeUsagePeriodModel, 0, len(periods))
	for _, p := range periods {
		period := computeUsagePeriodModel{
			TotalVCPUHours: types.Float64Value(p.TotalVCPUHours),
			Start:          stringOrNull(p.Start),
			End:            stringOrNull(p.End),
			Raw:            redactedRawJSON(p.Raw),
		}
		if p.ChartData != nil {
			period.MaxCPUCount = types.Int64Value(p.ChartData.MaxCPUCount)
			period.ChartPoints = make([]chartPointModel, 0, len(p.ChartData.Points))
			for _, pt := range p.ChartData.Points {
				period.ChartPoints = append(period.ChartPoints, chartPointModel{
					X: types.Int64Value(pt.X),
					Y: types.Float64Value(pt.Y),
				})
			}
		}
		// Map iteration order is random; sort by database name so the state
		// stays stable between runs.
		names := make([]string, 0, len(p.UsageByDatabase))
		for name := range p.UsageByDatabase {
			names = append(names, name)
		}
		sort.Strings(names)
		period.Databases = make([]databaseUsageModel, 0, len(names))
		for _, name := range names {
			db := p.UsageByDatabase[name]
			dbModel := databaseUsageModel{
				Name:           types.StringValue(name),
				TotalVCPUHours: types.Float64Value(db.TotalVCPUHours),
				Start:          stringOrNull(db.Start),
				End:            stringOrNull(db.End),
				Instances:      make([]instanceUsageModel, 0, len(db.UsageByInstance)),
			}
			instanceNames := make([]string, 0, len(db.UsageByInstance))
			for instanceName := range db.UsageByInstance {
				instanceNames = append(instanceNames, instanceName)
			}
			sort.Strings(instanceNames)
			for _, instanceName := range instanceNames {
				usage := db.UsageByInstance[instanceName]
				dbModel.Instances = append(dbModel.Instances, instanceUsageModel{
					Name:           types.StringValue(instanceName),
					Size:           stringOrNull(usage.Size),
					TotalVCPUHours: types.Float64Value(usage.TotalVCPUHours),
					Start:          stringOrNull(usage.Start),
					End:            stringOrNull(usage.End),
				})
			}
			period.Databases = append(period.Databases, dbModel)
		}
		data.Periods = append(data.Periods, period)
	}
}
