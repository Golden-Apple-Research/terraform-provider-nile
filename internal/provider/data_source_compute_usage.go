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

type chartPointModel struct {
	X types.Int64   `tfsdk:"x"`
	Y types.Float64 `tfsdk:"y"`
}

type instanceUsageModel struct {
	Name           types.String  `tfsdk:"name"`
	Size           types.String  `tfsdk:"size"`
	TotalVCPUHours types.Float64 `tfsdk:"total_vcpu_hours"`
	Start          types.String  `tfsdk:"start"`
	End            types.String  `tfsdk:"end"`
}

type databaseUsageModel struct {
	Name           types.String         `tfsdk:"name"`
	TotalVCPUHours types.Float64        `tfsdk:"total_vcpu_hours"`
	Start          types.String         `tfsdk:"start"`
	End            types.String         `tfsdk:"end"`
	Instances      []instanceUsageModel `tfsdk:"instances"`
}

type computeUsagePeriodModel struct {
	TotalVCPUHours types.Float64        `tfsdk:"total_vcpu_hours"`
	Start          types.String         `tfsdk:"start"`
	End            types.String         `tfsdk:"end"`
	MaxCPUCount    types.Int64          `tfsdk:"max_cpu_count"`
	ChartPoints    []chartPointModel    `tfsdk:"chart_points"`
	Databases      []databaseUsageModel `tfsdk:"databases"`
	Raw            types.String         `tfsdk:"raw_json"`
}

type workspaceComputeUsageDataSourceModel struct {
	WorkspaceSlug types.String              `tfsdk:"workspace_slug"`
	Start         types.String              `tfsdk:"start"`
	End           types.String              `tfsdk:"end"`
	ID            types.String              `tfsdk:"id"`
	Periods       []computeUsagePeriodModel `tfsdk:"periods"`
}

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
