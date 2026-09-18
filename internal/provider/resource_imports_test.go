// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestSplitResourceID(t *testing.T) {
	parts, err := splitResourceID("ws/db/inst-1", 3)
	if err != nil || len(parts) != 3 || parts[2] != "inst-1" {
		t.Errorf("splitResourceID = %v, %v", parts, err)
	}
	encoded, err := splitResourceID("ws/foo%2Fbar/inst%2F1", 3)
	if err != nil || len(encoded) != 3 || encoded[1] != "foo/bar" || encoded[2] != "inst/1" {
		t.Errorf("splitResourceID encoded = %v, %v", encoded, err)
	}
	if got := joinResourceID("ws", "foo/bar", "inst/1"); got != "ws/foo%2Fbar/inst%2F1" {
		t.Errorf("joinResourceID = %q", got)
	}
	for _, id := range []string{"ws/db", "ws//db", "ws/db/", "/ws/db", ""} {
		if _, err := splitResourceID(id, 3); err == nil {
			t.Errorf("expected error for %q", id)
		}
	}
}

func TestRedactedRawJSON(t *testing.T) {
	got := redactedRawJSON(json.RawMessage(`{"password":"pw","code":"invite","nested":{"access_token":"token"},"name":"safe"}`))
	value := got.ValueString()
	for _, secret := range []string{`:"pw"`, `:"invite"`, `:"token"`} {
		if strings.Contains(value, secret) {
			t.Errorf("redacted JSON contains secret %q: %s", secret, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") || !strings.Contains(value, "safe") {
		t.Errorf("redacted JSON = %s", value)
	}
}

func TestApplyDatabaseResourcePreservesOmittedFields(t *testing.T) {
	model := &databaseResourceModel{
		Expandable: types.BoolValue(true),
		Created:    types.StringValue("created"),
		ParentID:   types.StringValue("parent-id"),
		ParentName: types.StringValue("parent"),
	}
	db := nileapi.Database{
		ID:   "db-1",
		Name: "app",
		Raw:  json.RawMessage(`{"id":"db-1","name":"app","status":"READY"}`),
	}
	applyDatabaseResource(model, db, "ws", "app")
	if !model.Expandable.ValueBool() || model.Created.ValueString() != "created" ||
		model.ParentID.ValueString() != "parent-id" || model.ParentName.ValueString() != "parent" {
		t.Fatalf("partial response cleared state: %+v", model)
	}
}

func TestApplyCredentialResourceMapsInternalWithoutDatabase(t *testing.T) {
	model := &databaseCredentialResourceModel{}
	applyCredentialResource(model, nileapi.Credential{ID: "cred-1", Internal: true}, "ws", "db")
	if !model.Internal.ValueBool() {
		t.Fatalf("internal = %v, want true", model.Internal)
	}
}

func TestOptionalBool(t *testing.T) {
	if got := optionalBool(types.BoolNull()); got != nil {
		t.Errorf("null must map to nil, got %v", *got)
	}
	if got := optionalBool(types.BoolUnknown()); got != nil {
		t.Errorf("unknown must map to nil, got %v", *got)
	}
	if got := optionalBool(types.BoolValue(true)); got == nil || !*got {
		t.Errorf("true must map to &true, got %v", got)
	}
	if got := optionalBool(types.BoolValue(false)); got == nil || *got {
		t.Errorf("false must map to &false, got %v", got)
	}
}

// resourceSchema returns the schema of a resource for building import states.
func resourceSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// emptyResourceState builds a state whose attributes are all null, like the
// framework prepares before it calls ImportState.
func emptyResourceState(t *testing.T, s resourceschema.Schema) tfsdk.State {
	t.Helper()
	objType, ok := s.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}
	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(objType, vals)}
}

// assertImportState imports with the given ID and returns the resulting state.
func assertImportState(t *testing.T, r resource.Resource, id string) tfsdk.State {
	t.Helper()
	imp, ok := r.(resource.ResourceWithImportState)
	if !ok {
		t.Fatalf("resource does not support import")
	}
	resp := resource.ImportStateResponse{State: emptyResourceState(t, resourceSchema(t, r))}
	imp.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("import %q diagnostics: %v", id, resp.Diagnostics)
	}
	return resp.State
}

func stateString(t *testing.T, state tfsdk.State, name string) string {
	t.Helper()
	var got types.String
	if diags := state.GetAttribute(context.Background(), path.Root(name), &got); diags.HasError() {
		t.Fatalf("GetAttribute(%q): %v", name, diags)
	}
	return got.ValueString()
}

func TestDatabaseResourceImport(t *testing.T) {
	state := assertImportState(t, NewDatabaseResource(), "acme/app")
	if got := stateString(t, state, "workspace_slug"); got != "acme" {
		t.Errorf("workspace_slug = %q", got)
	}
	if got := stateString(t, state, "name"); got != "app" {
		t.Errorf("name = %q", got)
	}
}

func TestDatabaseResourceImportInvalid(t *testing.T) {
	r := NewDatabaseResource()
	resp := resource.ImportStateResponse{State: emptyResourceState(t, resourceSchema(t, r))}
	r.(resource.ResourceWithImportState).ImportState(context.Background(), resource.ImportStateRequest{ID: "only-one"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a one-part import ID")
	}
}

func TestComputeInstanceResourceImport(t *testing.T) {
	state := assertImportState(t, NewComputeInstanceResource(), "acme/app/inst-1")
	if got := stateString(t, state, "workspace_slug"); got != "acme" {
		t.Errorf("workspace_slug = %q", got)
	}
	if got := stateString(t, state, "database_name"); got != "app" {
		t.Errorf("database_name = %q", got)
	}
	if got := stateString(t, state, "id"); got != "inst-1" {
		t.Errorf("id = %q", got)
	}
}

func TestCredentialResourceImport(t *testing.T) {
	state := assertImportState(t, NewDatabaseCredentialResource(), "acme/app/cred-1")
	if got := stateString(t, state, "id"); got != "cred-1" {
		t.Errorf("id = %q", got)
	}
}

func TestDeveloperInviteResourceImport(t *testing.T) {
	state := assertImportState(t, NewDeveloperInviteResource(), "acme/inv-1")
	if got := stateString(t, state, "id"); got != "inv-1" {
		t.Errorf("id = %q", got)
	}
}

// testAPIServer serves a canned response for every request.
func testAPIServer(t *testing.T, response string) *nileapi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
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

func TestReadRegionsIsSorted(t *testing.T) {
	client := testAPIServer(t, `["AZURE_EASTUS","AWS_US_WEST_2"]`)
	data := &regionsDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readRegions(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if len(data.Regions) != 2 || data.Regions[0].ValueString() != "AWS_US_WEST_2" || data.Regions[1].ValueString() != "AZURE_EASTUS" {
		t.Errorf("regions = %v", data.Regions)
	}
}

func TestReadComputeUsageSortsMaps(t *testing.T) {
	client := testAPIServer(t, `[{
		"totalVCPUHours":3,
		"usageByDatabase":{
			"zebra":{"totalVCPUHours":2,"usageByInstance":{"b":{"totalVCPUHours":1},"a":{"totalVCPUHours":1}}},
			"alpha":{"totalVCPUHours":1}
		}
	}]`)
	data := &workspaceComputeUsageDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceComputeUsage(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if len(data.Periods) != 1 || len(data.Periods[0].Databases) != 2 {
		t.Fatalf("periods = %+v", data.Periods)
	}
	if data.Periods[0].Databases[0].Name.ValueString() != "alpha" ||
		data.Periods[0].Databases[1].Name.ValueString() != "zebra" {
		t.Errorf("database order = %s, %s", data.Periods[0].Databases[0].Name, data.Periods[0].Databases[1].Name)
	}
	instances := data.Periods[0].Databases[1].Instances
	if len(instances) != 2 || instances[0].Name.ValueString() != "a" || instances[1].Name.ValueString() != "b" {
		t.Errorf("instance order = %+v", instances)
	}
}

func TestReadDeveloperModel(t *testing.T) {
	client := testAPIServer(t, `{"id":"dev-1","email":"a@example.com","kind":"HUMAN","workspaces":[{"slug":"acme"}],"databases":[{"id":"db-1","name":"app"}]}`)
	data := &developerDataSourceModel{}

	var resp datasource.ReadResponse
	readDeveloper(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "dev-1" || data.Email.ValueString() != "a@example.com" {
		t.Errorf("developer = %+v", data)
	}
	if len(data.Workspaces) != 1 || data.Workspaces[0].Slug.ValueString() != "acme" {
		t.Errorf("workspaces = %+v", data.Workspaces)
	}
}
