// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestInsightsSchemas(t *testing.T) {
	uptime := dataSourceSchemaOf(t, NewDatabaseUptimeInsightsDataSource())
	for _, name := range []string{"workspace_slug", "database", "start", "end", "granularity", "id", "source", "scope", "calculation", "summary", "points", "raw_json"} {
		if _, ok := uptime.Attributes[name]; !ok {
			t.Errorf("nile_database_uptime_insights: missing attribute %q", name)
		}
	}

	errors := dataSourceSchemaOf(t, NewDatabaseErrorInsightsDataSource())
	for _, name := range []string{"workspace_slug", "database", "start", "end", "granularity", "id", "points", "raw_json"} {
		if _, ok := errors.Attributes[name]; !ok {
			t.Errorf("nile_database_error_insights: missing attribute %q", name)
		}
	}
	if _, ok := errors.Attributes["summary"]; ok {
		t.Error("nile_database_error_insights must not expose a summary")
	}

	perf := dataSourceSchemaOf(t, NewDatabaseQueryPerformanceInsightsDataSource())
	for _, name := range []string{"workspace_slug", "database", "start", "end", "granularity", "id", "source", "points", "raw_json"} {
		if _, ok := perf.Attributes[name]; !ok {
			t.Errorf("nile_database_query_performance_insights: missing attribute %q", name)
		}
	}
}

// insightsClient serves response for every request and records the query.
func insightsClient(t *testing.T, response string) (*nileapi.Client, *url.Values) {
	t.Helper()
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, &gotQuery
}

func TestInsightsQuery(t *testing.T) {
	q := insightsQuery(types.StringValue("2025-06-01T00:00:00Z"), types.StringNull(), types.StringValue("1h"))
	if q.Start != "2025-06-01T00:00:00Z" || q.End != "" || q.Granularity != "1h" {
		t.Errorf("query = %+v", q)
	}
}

func TestReadDatabaseUptimeInsightsModel(t *testing.T) {
	client, gotQuery := insightsClient(t, `{
		"source":"probe","scope":"database","granularity":"1h","calculation":"time-weighted",
		"summary":{"uptimePercentage":99.9,"uptimeSeconds":3599,"observedSeconds":3600},
		"points":[{"timestamp":"2025-06-01T00:00:00Z","uptimePercentage":99.5,"uptimeSeconds":3590,"observedSeconds":3600}]
	}`)
	data := &databaseUptimeInsightsDataSourceModel{
		WorkspaceSlug: types.StringValue("ws"),
		Database:      types.StringValue("db"),
		Start:         types.StringValue("2025-06-01T00:00:00Z"),
		Granularity:   types.StringValue("1h"),
	}

	var resp datasource.ReadResponse
	readDatabaseUptimeInsights(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if gotQuery.Get("start") != "2025-06-01T00:00:00Z" || gotQuery.Get("granularity") != "1h" {
		t.Errorf("query = %v", gotQuery)
	}
	if data.ID.ValueString() != "ws/db" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if data.Source.ValueString() != "probe" || data.Scope.ValueString() != "database" || data.Calculation.ValueString() != "time-weighted" {
		t.Errorf("meta = %s/%s/%s", data.Source, data.Scope, data.Calculation)
	}
	if data.Summary == nil {
		t.Fatal("summary is nil")
	}
	if data.Summary.UptimePercentage.ValueFloat64() != 99.9 || data.Summary.ObservedSeconds.ValueInt64() != 3600 {
		t.Errorf("summary = %+v", data.Summary)
	}
	if len(data.Points) != 1 || data.Points[0].UptimeSeconds.ValueInt64() != 3590 {
		t.Errorf("points = %+v", data.Points)
	}
	if data.Raw.ValueString() == "" {
		t.Error("raw_json must preserve the payload")
	}
}

func TestReadDatabaseErrorInsightsModel(t *testing.T) {
	client, _ := insightsClient(t, `{"granularity":"5m","points":[{"timestamp":"t","source":"proxy","errorCount":3}]}`)
	data := &databaseErrorInsightsDataSourceModel{
		WorkspaceSlug: types.StringValue("ws"),
		Database:      types.StringValue("db"),
	}

	var resp datasource.ReadResponse
	readDatabaseErrorInsights(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "ws/db" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Points) != 1 || data.Points[0].ErrorCount.ValueInt64() != 3 ||
		data.Points[0].Source.ValueString() != "proxy" {
		t.Errorf("points = %+v", data.Points)
	}
}

func TestReadDatabaseQueryPerformanceInsightsModel(t *testing.T) {
	client, _ := insightsClient(t, `{"source":"proxy","points":[{"timestamp":"t","proxyQueriesPerSecond":1.5,"thothQueriesPerSecond":1.2,"thothP99LatencyMs":10,"thothCpuMilliseconds":50}]}`)
	data := &databaseQueryPerformanceInsightsDataSourceModel{
		WorkspaceSlug: types.StringValue("ws"),
		Database:      types.StringValue("db"),
	}

	var resp datasource.ReadResponse
	readDatabaseQueryPerformanceInsights(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.Source.ValueString() != "proxy" {
		t.Errorf("source = %q", data.Source.ValueString())
	}
	if len(data.Points) != 1 {
		t.Fatalf("points = %d, want 1", len(data.Points))
	}
	p := data.Points[0]
	if p.ProxyQueriesPerSecond.ValueFloat64() != 1.5 || p.ThothQueriesPerSecond.ValueFloat64() != 1.2 ||
		p.ThothP99LatencyMs.ValueFloat64() != 10 || p.ThothCPUMilliseconds.ValueFloat64() != 50 {
		t.Errorf("point = %+v", p)
	}
}
