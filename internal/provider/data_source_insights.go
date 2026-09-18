// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// insightsArgs adds the attributes every database insights data source shares:
// the workspace, the database, and the optional time window.
func insightsArgs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"workspace_slug": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
			MarkdownDescription: "Slug of the Nile workspace that owns the database.",
		},
		"database": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
			},
			MarkdownDescription: "Identifier (id) of the database. The API path names this parameter `databaseId`; " +
				"the live API rejects database names here with `Invalid id`. Use the `id` attribute of `nile_database`.",
		},
		"start": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				isRFC3339Validator{},
			},
			MarkdownDescription: "RFC3339 timestamp marking the start of the metrics window (`start` query parameter). " +
				"Must be aligned to whole minutes; the live API rejects sub-minute timestamps.",
		},
		"end": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				isRFC3339Validator{},
			},
			MarkdownDescription: "RFC3339 timestamp marking the end of the metrics window (`end` query parameter). " +
				"Must be aligned to whole minutes; the live API rejects sub-minute timestamps.",
		},
		"granularity": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Bucket size of the returned samples (`granularity` query parameter).",
		},
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Stable identifier of this data source instance (`<workspaceSlug>/<database>`).",
		},
		"raw_json": schema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "Redacted JSON payload of the response as returned by the API.",
		},
	}
}

func insightsQuery(dataStart, dataEnd, dataGranularity types.String) nileapi.InsightsQuery {
	return nileapi.InsightsQuery{
		Start:       dataStart.ValueString(),
		End:         dataEnd.ValueString(),
		Granularity: dataGranularity.ValueString(),
	}
}

// --- nile_database_uptime_insights -----------------------------------------

// NewDatabaseUptimeInsightsDataSource constructs the data source for database
// uptime metrics.
func NewDatabaseUptimeInsightsDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_database_uptime_insights",
		databaseUptimeInsightsSchema,
		readDatabaseUptimeInsights,
	)
}

type uptimePointModel struct {
	Timestamp        types.String  `tfsdk:"timestamp"`
	UptimePercentage types.Float64 `tfsdk:"uptime_percentage"`
	UptimeSeconds    types.Int64   `tfsdk:"uptime_seconds"`
	ObservedSeconds  types.Int64   `tfsdk:"observed_seconds"`
}

type uptimeSummaryModel struct {
	UptimePercentage types.Float64 `tfsdk:"uptime_percentage"`
	UptimeSeconds    types.Int64   `tfsdk:"uptime_seconds"`
	ObservedSeconds  types.Int64   `tfsdk:"observed_seconds"`
}

type databaseUptimeInsightsDataSourceModel struct {
	WorkspaceSlug types.String        `tfsdk:"workspace_slug"`
	Database      types.String        `tfsdk:"database"`
	Start         types.String        `tfsdk:"start"`
	End           types.String        `tfsdk:"end"`
	Granularity   types.String        `tfsdk:"granularity"`
	ID            types.String        `tfsdk:"id"`
	Source        types.String        `tfsdk:"source"`
	Scope         types.String        `tfsdk:"scope"`
	Calculation   types.String        `tfsdk:"calculation"`
	Summary       *uptimeSummaryModel `tfsdk:"summary"`
	Points        []uptimePointModel  `tfsdk:"points"`
	Raw           types.String        `tfsdk:"raw_json"`
}

func databaseUptimeInsightsSchema(_ context.Context) schema.Schema {
	attrs := insightsArgs()
	attrs["source"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Source of the samples (`source` in the API response).",
	}
	attrs["scope"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Scope of the calculation (`scope` in the API response).",
	}
	attrs["calculation"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "How uptime is calculated (`calculation` in the API response).",
	}
	attrs["summary"] = schema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Time-weighted uptime summary over the requested window.",
		Attributes: map[string]schema.Attribute{
			"uptime_percentage": schema.Float64Attribute{Computed: true, MarkdownDescription: "Uptime percentage."},
			"uptime_seconds":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Seconds of uptime."},
			"observed_seconds":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Seconds observed."},
		},
	}
	attrs["points"] = schema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Uptime samples.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"timestamp":         schema.StringAttribute{Computed: true, MarkdownDescription: "Sample timestamp."},
			"uptime_percentage": schema.Float64Attribute{Computed: true, MarkdownDescription: "Uptime percentage."},
			"uptime_seconds":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Seconds of uptime."},
			"observed_seconds":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Seconds observed."},
		}},
	}
	return schema.Schema{
		MarkdownDescription: "Lists time-weighted database uptime samples via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/uptime`.",
		Attributes: attrs,
	}
}

func readDatabaseUptimeInsights(ctx context.Context, client *nileapi.Client, data *databaseUptimeInsightsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	database := data.Database.ValueString()

	out, err := client.ListDatabaseUptime(ctx, workspaceSlug, database, insightsQuery(data.Start, data.End, data.Granularity))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing database uptime metrics",
			fmt.Sprintf("Could not list uptime metrics of database %q in workspace %q: %s",
				database, workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(dataSourceID(workspaceSlug, database))
	data.Source = stringOrNull(out.Source)
	data.Scope = stringOrNull(out.Scope)
	data.Calculation = stringOrNull(out.Calculation)
	if out.Summary != nil {
		data.Summary = &uptimeSummaryModel{
			UptimePercentage: types.Float64Value(out.Summary.UptimePercentage),
			UptimeSeconds:    types.Int64Value(out.Summary.UptimeSeconds),
			ObservedSeconds:  types.Int64Value(out.Summary.ObservedSeconds),
		}
	}
	data.Points = make([]uptimePointModel, 0, len(out.Points))
	for _, p := range out.Points {
		data.Points = append(data.Points, uptimePointModel{
			Timestamp:        stringOrNull(p.Timestamp),
			UptimePercentage: types.Float64Value(p.UptimePercentage),
			UptimeSeconds:    types.Int64Value(p.UptimeSeconds),
			ObservedSeconds:  types.Int64Value(p.ObservedSeconds),
		})
	}
	data.Raw = redactedRawJSON(out.Raw)
}

// --- nile_database_error_insights ------------------------------------------

// NewDatabaseErrorInsightsDataSource constructs the data source for database
// error metrics.
func NewDatabaseErrorInsightsDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_database_error_insights",
		databaseErrorInsightsSchema,
		readDatabaseErrorInsights,
	)
}

type errorPointModel struct {
	Timestamp  types.String `tfsdk:"timestamp"`
	Source     types.String `tfsdk:"source"`
	ErrorCount types.Int64  `tfsdk:"error_count"`
}

type databaseErrorInsightsDataSourceModel struct {
	WorkspaceSlug types.String      `tfsdk:"workspace_slug"`
	Database      types.String      `tfsdk:"database"`
	Start         types.String      `tfsdk:"start"`
	End           types.String      `tfsdk:"end"`
	Granularity   types.String      `tfsdk:"granularity"`
	ID            types.String      `tfsdk:"id"`
	Points        []errorPointModel `tfsdk:"points"`
	Raw           types.String      `tfsdk:"raw_json"`
}

func databaseErrorInsightsSchema(_ context.Context) schema.Schema {
	attrs := insightsArgs()
	attrs["points"] = schema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Bucketed SQL and connection error counts.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"timestamp":   schema.StringAttribute{Computed: true, MarkdownDescription: "Bucket timestamp."},
			"source":      schema.StringAttribute{Computed: true, MarkdownDescription: "Reporter (`source` in the API response)."},
			"error_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "Errors observed in the bucket."},
		}},
	}
	return schema.Schema{
		MarkdownDescription: "Lists bucketed SQL and connection error counts via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/errors`.",
		Attributes: attrs,
	}
}

func readDatabaseErrorInsights(ctx context.Context, client *nileapi.Client, data *databaseErrorInsightsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	database := data.Database.ValueString()

	out, err := client.ListDatabaseErrors(ctx, workspaceSlug, database, insightsQuery(data.Start, data.End, data.Granularity))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing database error metrics",
			fmt.Sprintf("Could not list error metrics of database %q in workspace %q: %s",
				database, workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(dataSourceID(workspaceSlug, database))
	data.Points = make([]errorPointModel, 0, len(out.Points))
	for _, p := range out.Points {
		data.Points = append(data.Points, errorPointModel{
			Timestamp:  stringOrNull(p.Timestamp),
			Source:     stringOrNull(p.Source),
			ErrorCount: types.Int64Value(p.ErrorCount),
		})
	}
	data.Raw = redactedRawJSON(out.Raw)
}

// --- nile_database_query_performance_insights --------------------------------

// NewDatabaseQueryPerformanceInsightsDataSource constructs the data source for
// database query performance metrics.
func NewDatabaseQueryPerformanceInsightsDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_database_query_performance_insights",
		databaseQueryPerformanceInsightsSchema,
		readDatabaseQueryPerformanceInsights,
	)
}

type queryPerformancePointModel struct {
	Timestamp             types.String  `tfsdk:"timestamp"`
	ProxyQueriesPerSecond types.Float64 `tfsdk:"proxy_queries_per_second"`
	ThothQueriesPerSecond types.Float64 `tfsdk:"thoth_queries_per_second"`
	ThothP99LatencyMs     types.Float64 `tfsdk:"thoth_p99_latency_ms"`
	ThothCPUMilliseconds  types.Float64 `tfsdk:"thoth_cpu_milliseconds"`
}

type databaseQueryPerformanceInsightsDataSourceModel struct {
	WorkspaceSlug types.String                 `tfsdk:"workspace_slug"`
	Database      types.String                 `tfsdk:"database"`
	Start         types.String                 `tfsdk:"start"`
	End           types.String                 `tfsdk:"end"`
	Granularity   types.String                 `tfsdk:"granularity"`
	ID            types.String                 `tfsdk:"id"`
	Source        types.String                 `tfsdk:"source"`
	Points        []queryPerformancePointModel `tfsdk:"points"`
	Raw           types.String                 `tfsdk:"raw_json"`
}

func databaseQueryPerformanceInsightsSchema(_ context.Context) schema.Schema {
	attrs := insightsArgs()
	attrs["source"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Source of the samples (`source` in the API response).",
	}
	attrs["points"] = schema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Query performance samples.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"timestamp":                schema.StringAttribute{Computed: true, MarkdownDescription: "Sample timestamp."},
			"proxy_queries_per_second": schema.Float64Attribute{Computed: true, MarkdownDescription: "Queries per second observed by Nile Proxy."},
			"thoth_queries_per_second": schema.Float64Attribute{Computed: true, MarkdownDescription: "Queries per second observed by Thoth."},
			"thoth_p99_latency_ms":     schema.Float64Attribute{Computed: true, MarkdownDescription: "P99 query latency in milliseconds reported by Thoth."},
			"thoth_cpu_milliseconds":   schema.Float64Attribute{Computed: true, MarkdownDescription: "CPU milliseconds reported by Thoth."},
		}},
	}
	return schema.Schema{
		MarkdownDescription: "Lists Proxy query rate, Thoth query rate and Thoth P99 latency via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/query-performance`.",
		Attributes: attrs,
	}
}

func readDatabaseQueryPerformanceInsights(ctx context.Context, client *nileapi.Client, data *databaseQueryPerformanceInsightsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	database := data.Database.ValueString()

	out, err := client.ListDatabaseQueryPerformance(ctx, workspaceSlug, database, insightsQuery(data.Start, data.End, data.Granularity))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing database query performance metrics",
			fmt.Sprintf("Could not list query performance metrics of database %q in workspace %q: %s",
				database, workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(dataSourceID(workspaceSlug, database))
	data.Source = stringOrNull(out.Source)
	data.Points = make([]queryPerformancePointModel, 0, len(out.Points))
	for _, p := range out.Points {
		data.Points = append(data.Points, queryPerformancePointModel{
			Timestamp:             stringOrNull(p.Timestamp),
			ProxyQueriesPerSecond: types.Float64Value(p.ProxyQueriesPerSecond),
			ThothQueriesPerSecond: types.Float64Value(p.ThothQueriesPerSecond),
			ThothP99LatencyMs:     types.Float64Value(p.ThothP99LatencyMs),
			ThothCPUMilliseconds:  types.Float64Value(p.ThothCPUMilliseconds),
		})
	}
	data.Raw = redactedRawJSON(out.Raw)
}
