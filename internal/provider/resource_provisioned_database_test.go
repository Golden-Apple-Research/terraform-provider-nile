// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestProvisionedDatabaseResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewProvisionedDatabaseResource())
	for _, name := range []string{"region", "claim_code", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
	if attr := sch.Attributes["claim_code"]; attr == nil || !attr.IsSensitive() {
		t.Error("claim_code must be sensitive")
	}
}

func TestProvisionedDatabaseResourceCreate(t *testing.T) {
	var method, path, body, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		_, _ = w.Write([]byte(`{"claimCode":"claim-xyz","region":"AWS_EU_CENTRAL_1",` +
			`"databaseId":"db-9","databaseName":"unauth_deadbeef","apiHost":"https://eu.api.thenile.dev/v2/databases/db-9",` +
			`"dbHost":"postgres://eu.db.thenile.dev/unauth_deadbeef","credentialId":"cred-9","password":"prov-secret",` +
			`"sharded":true,` +
			`"env":{"NILEDB_USER":"u-9","NILEDB_PASSWORD":"env-secret",` +
			`"NILEDB_URL":"postgres://u-9:env-secret@eu.db.thenile.dev/unauth_deadbeef",` +
			`"NILEDB_API_URL":"https://eu.api.thenile.dev/v2/databases/db-9"}}`))
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	res := NewProvisionedDatabaseResource().(*provisionedDatabaseResource)
	var cResp resource.ConfigureResponse
	res.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &cResp)
	if cResp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", cResp.Diagnostics)
	}

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"region": stringAttr("AWS_EU_CENTRAL_1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if method != http.MethodPost || path != "/databases/provision" || body != `{"region":"AWS_EU_CENTRAL_1"}` {
		t.Errorf("request = %s %s %s", method, path, body)
	}
	if auth != "" {
		t.Errorf("provisioning must be unauthenticated, got Authorization %q", auth)
	}
	if got := stateString(t, resp.State, "claim_code"); got != "claim-xyz" {
		t.Errorf("claim_code = %q", got)
	}
	if got := stateString(t, resp.State, "database_id"); got != "db-9" {
		t.Errorf("database_id = %q", got)
	}
	if got := stateString(t, resp.State, "database_name"); got != "unauth_deadbeef" {
		t.Errorf("database_name = %q", got)
	}
	if got := stateString(t, resp.State, "api_host"); got != "https://eu.api.thenile.dev/v2/databases/db-9" {
		t.Errorf("api_host = %q", got)
	}
	if got := stateString(t, resp.State, "db_host"); got != "postgres://eu.db.thenile.dev/unauth_deadbeef" {
		t.Errorf("db_host = %q", got)
	}
	if got := stateString(t, resp.State, "credential_id"); got != "cred-9" {
		t.Errorf("credential_id = %q", got)
	}
	if got := stateString(t, resp.State, "username"); got != "u-9" {
		t.Errorf("username = %q", got)
	}
	if got := stateString(t, resp.State, "password"); got != "prov-secret" {
		t.Errorf("password = %q", got)
	}
	raw := stateString(t, resp.State, "raw_json")
	for _, leak := range []string{"claim-xyz", "prov-secret", "env-secret", "u-9:env-secret@"} {
		if strings.Contains(raw, leak) {
			t.Errorf("raw_json leaks %q: %s", leak, raw)
		}
	}
	if !strings.Contains(raw, "[REDACTED]@") {
		t.Errorf("raw_json must scrub URL credentials: %s", raw)
	}
}

func TestProvisionedDatabaseResourceReadPerformsNoAPICall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("read must not call the API, got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	res := NewProvisionedDatabaseResource().(*provisionedDatabaseResource)
	var cResp resource.ConfigureResponse
	res.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &cResp)
	if cResp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", cResp.Diagnostics)
	}

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"region":     stringAttr("AWS_EU_CENTRAL_1"),
			"claim_code": stringAttr("claim-xyz"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
}
