// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestDatabaseCredentialsSchema(t *testing.T) {
	sch := dataSourceSchemaOf(t, NewDatabaseCredentialsDataSource())
	for _, name := range []string{"workspace_slug", "database_name", "tenant_id", "internal", "id", "credentials"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
	if attr := sch.Attributes["credentials"]; attr == nil || attr.IsRequired() {
		t.Error("credentials must be a computed list")
	}
}

func TestReadDatabaseCredentials(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"cred-1","tenant":"acme","internal":true,"password":"leak","database":{"apiHost":"api.example","dbHost":"db.example"},"created":"2025-06-01T00:00:00Z"}]`))
	}))
	defer srv.Close()
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	internal := true
	data := &databaseCredentialsDataSourceModel{
		WorkspaceSlug: types.StringValue("ws"),
		DatabaseName:  types.StringValue("db"),
		TenantID:      types.StringValue("acme"),
		Internal:      types.BoolValue(internal),
	}

	var resp datasource.ReadResponse
	readDatabaseCredentials(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if gotQuery.Get("tenantId") != "acme" || gotQuery.Get("internal") != "true" {
		t.Errorf("query = %v", gotQuery)
	}
	if data.ID.ValueString() != "ws/db" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Credentials) != 1 {
		t.Fatalf("credentials = %d, want 1", len(data.Credentials))
	}
	cred := data.Credentials[0]
	if cred.ID.ValueString() != "cred-1" || !cred.Internal.ValueBool() || cred.Tenant.ValueString() != "acme" {
		t.Errorf("credential = %+v", cred)
	}
	if cred.APIHost.ValueString() != "api.example" || cred.DBHost.ValueString() != "db.example" {
		t.Errorf("hosts = %q/%q", cred.APIHost, cred.DBHost)
	}
	// List responses must never leak a password, even if the API includes one.
	if raw := cred.Raw.ValueString(); strings.Contains(raw, "leak") {
		t.Errorf("raw_json = %s, want password redacted", raw)
	}
}
