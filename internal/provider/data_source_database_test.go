// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestDatabaseSchemas(t *testing.T) {
	single := dataSourceSchemaOf(t, NewDatabaseDataSource())
	for _, name := range []string{"workspace_slug", "name", "id", "status", "region", "api_host", "raw_json"} {
		if _, ok := single.Attributes[name]; !ok {
			t.Errorf("nile_database: missing attribute %q", name)
		}
	}
	if attr := single.Attributes["workspace_slug"]; attr == nil || !attr.IsRequired() {
		t.Error("workspace_slug must be required on nile_database")
	}

	list := dataSourceSchemaOf(t, NewDatabasesDataSource())
	for _, name := range []string{"workspace_slug", "id", "databases"} {
		if _, ok := list.Attributes[name]; !ok {
			t.Errorf("nile_databases: missing attribute %q", name)
		}
	}
}

func TestReadDatabaseModel(t *testing.T) {
	client := testAPIServer(t, `{"id":"db-1","name":"app","status":"READY","region":"AWS_US_WEST_2","expandable":true,"apiHost":"api.example","created":"2025-06-01T00:00:00Z"}`)
	data := &databaseDataSourceModel{}

	var resp datasource.ReadResponse
	readDatabase(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "db-1" || data.Status.ValueString() != "READY" ||
		!data.Expandable.ValueBool() || data.APIHost.ValueString() != "api.example" {
		t.Errorf("data = %+v", data)
	}
	if !data.ParentID.IsNull() {
		t.Errorf("parent_id = %q, want null", data.ParentID)
	}
}

func TestReadDatabaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"entity_not_found","message":"no such database","statusCode":404}`))
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	data := &databaseDataSourceModel{}

	var resp datasource.ReadResponse
	readDatabase(context.Background(), client, data, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a missing database")
	}
}

func TestReadDatabasesModel(t *testing.T) {
	client := testAPIServer(t, `[
		{"id":"db-1","name":"app","status":"READY","region":"AWS_US_WEST_2","expandable":true,
		 "apiHost":"api.example","dbHost":"db.example","created":"2025-06-01T00:00:00Z",
		 "workspace":{"id":"ws-1","slug":"acme"}},
		{"id":"db-2","name":"replica","status":"PENDING","region":"AZURE_EASTUS"}
	]`)
	data := &databasesDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readDatabases(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Databases) != 2 {
		t.Fatalf("databases = %d, want 2", len(data.Databases))
	}
	first := data.Databases[0]
	if first.Name.ValueString() != "app" || first.Status.ValueString() != "READY" ||
		!first.Expandable.ValueBool() || first.APIHost.ValueString() != "api.example" {
		t.Errorf("first = %+v", first)
	}
	if !strings.Contains(first.Raw.ValueString(), `"id":"db-1"`) {
		t.Errorf("raw_json = %q", first.Raw.ValueString())
	}
}

func TestApplyDatabaseDataSource(t *testing.T) {
	data := &databaseDataSourceModel{}
	applyDatabaseDataSource(data, nileapi.Database{ID: "db-1", Name: "app", Status: "READY", Region: "AWS_US_WEST_2"})
	if data.ID.ValueString() != "db-1" || data.Status.ValueString() != "READY" || data.Region.ValueString() != "AWS_US_WEST_2" {
		t.Fatalf("data = %+v", data)
	}
	if !data.Deleted.IsNull() || !data.ParentName.IsNull() {
		t.Errorf("unset fields must stay null: %+v", data)
	}
}
