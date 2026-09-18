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

// insightsQuery converts the optional data source window attributes into an
// API query.
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

// uptimePointModel is one uptime sample of the metrics window.
type uptimePointModel struct {
	// Timestamp is the sample time.
	Timestamp types.String `tfsdk:"timestamp"`
	// UptimePercentage is the time-weighted uptime share of the sample.
	UptimePercentage types.Float64 `tfsdk:"uptime_percentage"`
	// UptimeSeconds is the number of seconds the database was up.
	UptimeSeconds types.Int64 `tfsdk:"uptime_seconds"`
	// ObservedSeconds is the number of seconds covered by the sample.
	ObservedSeconds types.Int64 `tfsdk:"observed_seconds"`
}

// uptimeSummaryModel aggregates the uptime over the whole window.
type uptimeSummaryModel struct {
	// UptimePercentage is the time-weighted uptime share over the window.
	UptimePercentage types.Float64 `tfsdk:"uptime_percentage"`
	// UptimeSeconds is the number of seconds the database was up.
	UptimeSeconds types.Int64 `tfsdk:"uptime_seconds"`
	// ObservedSeconds is the number of seconds covered by the window.
	ObservedSeconds types.Int64 `tfsdk:"observed_seconds"`
}

// databaseUptimeInsightsDataSourceModel is the state of the
// nile_database_uptime_insights data source.
type databaseUptimeInsightsDataSourceModel struct {
	// WorkspaceSlug is the workspace owning the database.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Database is the id of the database.
	Database types.String `tfsdk:"database"`
	// Start is the optional start of the metrics window.
	Start types.String `tfsdk:"start"`
	// End is the optional end of the metrics window.
	End types.String `tfsdk:"end"`
	// Granularity is the optional bucket size of the samples.
	Granularity types.String `tfsdk:"granularity"`
	// ID is the stable identifier of this data source instance.
	ID types.String `tfsdk:"id"`
	// Source names the component that measured the uptime.
	Source types.String `tfsdk:"source"`
	// Scope describes what the uptime was measured over.
	Scope types.String `tfsdk:"scope"`
	// Calculation describes how the uptime is calculated.
	Calculation types.String `tfsdk:"calculation"`
	// Summary aggregates the uptime over the whole window.
	Summary *uptimeSummaryModel `tfsdk:"summary"`
	// Points are the individual uptime samples.
	Points []uptimePointModel `tfsdk:"points"`
	// Raw is the redacted JSON payload of the API response.
	Raw types.String `tfsdk:"raw_json"`
}

// databaseUptimeInsightsSchema builds the schema of the uptime insights data
// source on top of the shared insightsArgs attributes.
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

// readDatabaseUptimeInsights fetches the uptime metrics of a database and
// populates the data source state from them.
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

// errorPointModel is one bucketed error count of the metrics window.
type errorPointModel struct {
	// Timestamp is the bucket time.
	Timestamp types.String `tfsdk:"timestamp"`
	// Source names the component that reported the errors.
	Source types.String `tfsdk:"source"`
	// ErrorCount is the number of errors observed in the bucket.
	ErrorCount types.Int64 `tfsdk:"error_count"`
}

// databaseErrorInsightsDataSourceModel is the state of the
// nile_database_error_insights data source.
type databaseErrorInsightsDataSourceModel struct {
	// WorkspaceSlug is the workspace owning the database.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Database is the id of the database.
	Database types.String `tfsdk:"database"`
	// Start is the optional start of the metrics window.
	Start types.String `tfsdk:"start"`
	// End is the optional end of the metrics window.
	End types.String `tfsdk:"end"`
	// Granularity is the optional bucket size of the samples.
	Granularity types.String `tfsdk:"granularity"`
	// ID is the stable identifier of this data source instance.
	ID types.String `tfsdk:"id"`
	// Points are the bucketed error counts.
	Points []errorPointModel `tfsdk:"points"`
	// Raw is the redacted JSON payload of the API response.
	Raw types.String `tfsdk:"raw_json"`
}

// databaseErrorInsightsSchema builds the schema of the error insights data
// source on top of the shared insightsArgs attributes.
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

// readDatabaseErrorInsights fetches the error metrics of a database and
// populates the data source state from them.
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

// queryPerformancePointModel is one query performance sample.
type queryPerformancePointModel struct {
	// Timestamp is the sample time.
	Timestamp types.String `tfsdk:"timestamp"`
	// ProxyQueriesPerSecond is the query rate observed by Nile Proxy.
	ProxyQueriesPerSecond types.Float64 `tfsdk:"proxy_queries_per_second"`
	// ThothQueriesPerSecond is the query rate observed by Thoth.
	ThothQueriesPerSecond types.Float64 `tfsdk:"thoth_queries_per_second"`
	// ThothP99LatencyMs is the P99 query latency in milliseconds reported by
	// Thoth.
	ThothP99LatencyMs types.Float64 `tfsdk:"thoth_p99_latency_ms"`
	// ThothCPUMilliseconds is the CPU time in milliseconds reported by Thoth.
	ThothCPUMilliseconds types.Float64 `tfsdk:"thoth_cpu_milliseconds"`
}

// databaseQueryPerformanceInsightsDataSourceModel is the state of the
// nile_database_query_performance_insights data source.
type databaseQueryPerformanceInsightsDataSourceModel struct {
	// WorkspaceSlug is the workspace owning the database.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Database is the id of the database.
	Database types.String `tfsdk:"database"`
	// Start is the optional start of the metrics window.
	Start types.String `tfsdk:"start"`
	// End is the optional end of the metrics window.
	End types.String `tfsdk:"end"`
	// Granularity is the optional bucket size of the samples.
	Granularity types.String `tfsdk:"granularity"`
	// ID is the stable identifier of this data source instance.
	ID types.String `tfsdk:"id"`
	// Source names the component that measured the samples.
	Source types.String `tfsdk:"source"`
	// Points are the query performance samples.
	Points []queryPerformancePointModel `tfsdk:"points"`
	// Raw is the redacted JSON payload of the API response.
	Raw types.String `tfsdk:"raw_json"`
}

// databaseQueryPerformanceInsightsSchema builds the schema of the query
// performance insights data source on top of the shared insightsArgs
// attributes.
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

// readDatabaseQueryPerformanceInsights fetches the query performance metrics
// of a database and populates the data source state from them.
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
