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

func TestCredentialResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewDatabaseCredentialResource())
	for _, name := range []string{"workspace_slug", "database_name", "tenant_id", "internal", "id", "password", "api_host", "db_host", "created", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
	if attr := sch.Attributes["password"]; attr == nil || !attr.IsSensitive() {
		t.Error("password must be sensitive")
	}
	if attr := sch.Attributes["raw_json"]; attr == nil || !attr.IsSensitive() {
		t.Error("raw_json must be sensitive on the credential resource")
	}
}

// credentialAPIServer routes by method: POST creates, GET lists, DELETE
// deletes.
func credentialAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *nileapi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func configuredCredentialResource(t *testing.T, client *nileapi.Client) *databaseCredentialResource {
	t.Helper()
	r := NewDatabaseCredentialResource().(*databaseCredentialResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func credentialPlan(r resource.Resource) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"workspace_slug": stringAttr("ws"),
		"database_name":  stringAttr("db"),
	}
}

func TestCredentialResourceCreate(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"cred-1","password":"s3cret","tenant":"acme","internal":false,"database":{"apiHost":"api.example","dbHost":"db.example"},"created":"2025-06-01T00:00:00Z"}`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, res, credentialPlan(res))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "id"); got != "cred-1" {
		t.Errorf("id = %q", got)
	}
	if got := stateString(t, resp.State, "password"); got != "s3cret" {
		t.Errorf("password = %q", got)
	}
	if got := stateString(t, resp.State, "api_host"); got != "api.example" {
		t.Errorf("api_host = %q", got)
	}
	// The stored raw payload must not repeat the password.
	if got := stateString(t, resp.State, "raw_json"); strings.Contains(got, "s3cret") {
		t.Errorf("raw_json = %s, want password redacted", got)
	}
}

func TestCredentialResourceCreateWithoutPassword(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cred-1"}`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, res, credentialPlan(res))}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create response has no password")
	}
}

func TestCredentialResourceCreateWithoutID(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"password":"s3cret"}`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, res, credentialPlan(res))}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create response has no id")
	}
}

func TestCredentialResourceRead(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cred-1","tenant":"acme","internal":true}]`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"id":             stringAttr("cred-1"),
			"password":       stringAttr("stored"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	// The password is never returned again; the stored value must survive.
	if got := stateString(t, resp.State, "password"); got != "stored" {
		t.Errorf("password = %q, want the value kept in state", got)
	}
	if got := stateString(t, resp.State, "id"); got != "cred-1" {
		t.Errorf("id = %q", got)
	}
}

func TestCredentialResourceReadRemovesMissing(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"id":             stringAttr("cred-gone"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestCredentialResourceUpdateNotSupported(t *testing.T) {
	res := NewDatabaseCredentialResource().(*databaseCredentialResource)

	var resp resource.UpdateResponse
	res.Update(context.Background(), resource.UpdateRequest{}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error: updates must be rejected")
	}
}

func TestCredentialResourceDelete(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected %s request", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	res := configuredCredentialResource(t, client)

	resp := resource.DeleteResponse{State: crudResponseState(t, res)}
	res.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"id":             stringAttr("cred-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestApplyCredentialResourceMapsInternalWithoutDatabase(t *testing.T) {
	model := &databaseCredentialResourceModel{}
	applyCredentialResource(model, nileapi.Credential{ID: "cred-1", Internal: true}, "ws", "db")
	if !model.Internal.ValueBool() {
		t.Fatalf("internal = %v, want true", model.Internal)
	}
	if !model.Password.IsNull() {
		t.Errorf("password = %q, want null", model.Password)
	}
}
