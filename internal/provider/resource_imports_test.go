// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

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
