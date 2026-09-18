// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	// The remote credential exists; it must be in state so Terraform can destroy it.
	if got := stateString(t, resp.State, "id"); got != "cred-1" {
		t.Errorf("id = %q, want the created credential recorded in state", got)
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
		_, _ = w.Write([]byte(`[{"id":"cred-1","tenant":"acme","internal":true,"password":"********"}]`))
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
	// A non-empty (masked) password in the list response must not clobber state.
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

func TestCredentialResourceRotatesOnTriggerChange(t *testing.T) {
	var rotatePath, rotateQuery, rotateBody string
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/credentials/rotate") {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		rotatePath = r.URL.Path
		rotateQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		rotateBody = string(body)
		_, _ = w.Write([]byte(`{"id":"cred-2","password":"rotated-secret","database":{"apiHost":"api.example","dbHost":"db.example"}}`))
	})
	res := configuredCredentialResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, res)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug":       stringAttr("ws"),
			"database_name":        stringAttr("db"),
			"rotation_trigger":     stringAttr("2026-02-01"),
			"rotation_delay_hours": int64Attr(24),
			"rotation_reason":      stringAttr("monthly rotation"),
		}),
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"rotation_trigger": stringAttr("2026-01-01"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
	if rotatePath != "/workspaces/ws/databases/db/credentials/rotate" {
		t.Errorf("rotate path = %q", rotatePath)
	}
	if !strings.Contains(rotateQuery, "tenantId") && rotateQuery != "" {
		t.Errorf("rotate query = %q", rotateQuery)
	}
	if !strings.Contains(rotateBody, `"delayOldSecretsExpirationHours":24`) ||
		!strings.Contains(rotateBody, `"reason":"monthly rotation"`) {
		t.Errorf("rotate body = %q", rotateBody)
	}
	if got := stateString(t, resp.State, "password"); got != "rotated-secret" {
		t.Errorf("password after rotation = %q", got)
	}
	if got := stateString(t, resp.State, "id"); got != "cred-2" {
		t.Errorf("id after rotation = %q", got)
	}
}

func TestCredentialResourceUpdateWithoutTriggerChangeIsNoop(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("update without trigger change must not call the API, got %s %s", r.Method, r.URL.Path)
	})
	res := configuredCredentialResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, res)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"rotation_trigger": stringAttr("2026-01-01"),
		}),
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"rotation_trigger": stringAttr("2026-01-01"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
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

func TestApplyCredentialResourcePreservesSecretsAndOmittedInternal(t *testing.T) {
	model := &databaseCredentialResourceModel{
		Internal: types.BoolValue(true),
		Password: types.StringValue("stored"),
	}
	applyCredentialResource(model, nileapi.Credential{
		ID:       "cred-1",
		Password: "********",
		Raw:      json.RawMessage(`{"id":"cred-1"}`),
	}, "ws", "db")
	if model.Password.ValueString() != "stored" {
		t.Errorf("password = %q, want stored", model.Password.ValueString())
	}
	if !model.Internal.ValueBool() {
		t.Fatal("omitted internal should keep the previous value")
	}
}

// Regression test: workspace-wide credentials (no tenant) must not leave
// unknown values behind after apply. Terraform rejects state objects that
// still contain unknowns ("Provider returned invalid result object after
// apply"), which previously happened whenever the API response omitted a
// value for an Optional+Computed or Computed attribute.
func TestApplyCredentialResourceResolvesUnknownsToNull(t *testing.T) {
	model := &databaseCredentialResourceModel{
		TenantID: types.StringUnknown(),
		APIHost:  types.StringUnknown(),
		DBHost:   types.StringUnknown(),
		Created:  types.StringUnknown(),
		Raw:      types.StringUnknown(),
	}
	applyCredentialResource(model, nileapi.Credential{ID: "cred-1"}, "ws", "db")

	for name, v := range map[string]types.String{
		"tenant_id": model.TenantID,
		"api_host":  model.APIHost,
		"db_host":   model.DBHost,
		"created":   model.Created,
		"raw_json":  model.Raw,
	} {
		if v.IsUnknown() {
			t.Errorf("%s is still unknown after apply; want null", name)
		}
		if !v.IsNull() {
			t.Errorf("%s = %q, want null", name, v.ValueString())
		}
	}
}

func TestApplyCredentialResourceMapsTenant(t *testing.T) {
	model := &databaseCredentialResourceModel{
		TenantID: types.StringUnknown(),
	}
	applyCredentialResource(model, nileapi.Credential{ID: "cred-1", Tenant: "acme"}, "ws", "db")
	if got := model.TenantID.ValueString(); got != "acme" {
		t.Fatalf("tenant_id = %q, want acme", got)
	}
}

func TestCredentialResourceModifyPlanMarksPasswordUnknownOnRotation(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {})
	res := configuredCredentialResource(t, client)

	resp := resource.ModifyPlanResponse{Plan: planWith(t, res, map[string]tftypes.Value{
		"workspace_slug":   stringAttr("ws"),
		"database_name":    stringAttr("db"),
		"rotation_trigger": stringAttr("v2"),
	})}
	res.ModifyPlan(context.Background(), resource.ModifyPlanRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"password":         stringAttr("old-secret"),
			"rotation_trigger": stringAttr("v2"),
		}),
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"password":         stringAttr("old-secret"),
			"rotation_trigger": stringAttr("v1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("modify plan diagnostics: %v", resp.Diagnostics)
	}
	var plan databaseCredentialResourceModel
	if diags := resp.Plan.Get(context.Background(), &plan); diags.HasError() {
		t.Fatalf("plan get: %v", diags)
	}
	if !plan.Password.IsUnknown() {
		t.Errorf("password must be unknown when rotating, got %q", plan.Password)
	}
}

func TestCredentialResourceModifyPlanKeepsPasswordWithoutRotation(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {})
	res := configuredCredentialResource(t, client)

	resp := resource.ModifyPlanResponse{Plan: planWith(t, res, map[string]tftypes.Value{
		"workspace_slug":   stringAttr("ws"),
		"database_name":    stringAttr("db"),
		"password":         stringAttr("old-secret"),
		"rotation_trigger": stringAttr("v1"),
	})}
	res.ModifyPlan(context.Background(), resource.ModifyPlanRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"password":         stringAttr("old-secret"),
			"rotation_trigger": stringAttr("v1"),
		}),
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":   stringAttr("ws"),
			"database_name":    stringAttr("db"),
			"password":         stringAttr("old-secret"),
			"rotation_trigger": stringAttr("v1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("modify plan diagnostics: %v", resp.Diagnostics)
	}
	var plan databaseCredentialResourceModel
	if diags := resp.Plan.Get(context.Background(), &plan); diags.HasError() {
		t.Fatalf("plan get: %v", diags)
	}
	if plan.Password.ValueString() != "old-secret" {
		t.Errorf("password must stay unchanged without rotation, got %q", plan.Password)
	}
}

// tenantIDModifiers returns the plan modifiers of the tenant_id attribute.
func tenantIDModifiers(t *testing.T, r resource.Resource) []planmodifier.String {
	t.Helper()
	sch := resourceSchemaOf(t, r)
	attr, ok := sch.Attributes["tenant_id"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("tenant_id is not a schema.StringAttribute")
	}
	return attr.PlanModifiers
}

// TestCredentialTenantIDModifierNoReplaceOnNull pins the plan-modifier
// semantics of tenant_id: with a null state value (the live API returns an
// empty tenant for freshly created credentials) and an unconfigured
// attribute, an unrelated update such as a rotation must NOT force
// replacement. RequiresReplace() here would compare unknown against null and
// replace on every update.
func TestCredentialTenantIDModifierNoReplaceOnNull(t *testing.T) {
	res := &databaseCredentialResource{}
	for _, mod := range tenantIDModifiers(t, res) {
		req := planmodifier.StringRequest{
			Config: tfsdk.Config{
				Schema: resourceSchemaOf(t, res),
				Raw:    rawObject(resourceObjType(t, res), nil),
			},
			State: stateWith(t, res, map[string]tftypes.Value{
				"rotation_trigger": stringAttr("v1"),
			}),
			Plan: planWith(t, res, map[string]tftypes.Value{
				"tenant_id":        tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"rotation_trigger": stringAttr("v2"),
			}),
			ConfigValue: types.StringNull(),
			StateValue:  types.StringNull(),
			PlanValue:   types.StringUnknown(),
		}
		resp := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
		mod.PlanModifyString(context.Background(), req, &resp)
		if resp.RequiresReplace {
			t.Errorf("%T forced replacement for an unconfigured tenant_id with null state", mod)
		}
		if resp.Diagnostics.HasError() {
			t.Fatalf("%T diagnostics: %v", mod, resp.Diagnostics)
		}
	}
}

// TestCredentialTenantIDModifierReplacesOnConfiguredChange is the flip side:
// a user-visible tenant_id change must still force replacement, because the
// API has no update endpoint for credentials.
func TestCredentialTenantIDModifierReplacesOnConfiguredChange(t *testing.T) {
	res := &databaseCredentialResource{}
	config := tfsdk.Config{
		Schema: resourceSchemaOf(t, res),
		Raw: rawObject(resourceObjType(t, res), map[string]tftypes.Value{
			"tenant_id":        stringAttr("tenant-b"),
			"rotation_trigger": stringAttr("v2"),
		}),
	}
	state := stateWith(t, res, map[string]tftypes.Value{
		"tenant_id":        stringAttr("tenant-a"),
		"rotation_trigger": stringAttr("v1"),
	})
	plan := planWith(t, res, map[string]tftypes.Value{
		"tenant_id":        stringAttr("tenant-b"),
		"rotation_trigger": stringAttr("v2"),
	})
	for _, mod := range tenantIDModifiers(t, res) {
		req := planmodifier.StringRequest{
			Config:      config,
			State:       state,
			Plan:        plan,
			ConfigValue: types.StringValue("tenant-b"),
			StateValue:  types.StringValue("tenant-a"),
			PlanValue:   types.StringValue("tenant-b"),
		}
		resp := planmodifier.StringResponse{PlanValue: types.StringValue("tenant-b")}
		mod.PlanModifyString(context.Background(), req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%T diagnostics: %v", mod, resp.Diagnostics)
		}
		if resp.RequiresReplace {
			return
		}
	}
	t.Error("no modifier forces replacement for a configured tenant_id change")
}
