// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

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

// dataSourceSchemaOf returns the schema of a data source.
func dataSourceSchemaOf(t *testing.T, ds datasource.DataSource) datasourceschema.Schema {
	t.Helper()
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// rawObject fills an object type with the given values, leaving every other
// attribute null. It is the minimal raw value needed to drive a config, plan
// or state through the framework without a full Terraform run.
func rawObject(objType tftypes.Object, overrides map[string]tftypes.Value) tftypes.Value {
	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		if v, ok := overrides[name]; ok {
			vals[name] = v
			continue
		}
		vals[name] = tftypes.NewValue(typ, nil)
	}
	return tftypes.NewValue(objType, vals)
}

// stringAttr wraps a string for rawObject overrides.
func stringAttr(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

// timeoutsAttr builds a raw `timeouts` attribute value. The map keys must
// match the operations the resource's timeouts schema enables (for example
// create and update); an empty duration leaves that operation null.
func timeoutsAttr(durations map[string]string) tftypes.Value {
	types := make(map[string]tftypes.Type, len(durations))
	vals := make(map[string]tftypes.Value, len(durations))
	for op, d := range durations {
		types[op] = tftypes.String
		if d == "" {
			vals[op] = tftypes.NewValue(tftypes.String, nil)
		} else {
			vals[op] = stringAttr(d)
		}
	}
	return tftypes.NewValue(tftypes.Object{AttributeTypes: types}, vals)
}

// resourceSchemaOf returns the schema of a resource.
func resourceSchemaOf(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// resourceObjType returns the Terraform object type of a resource schema.
func resourceObjType(t *testing.T, r resource.Resource) tftypes.Object {
	t.Helper()
	objType, ok := resourceSchemaOf(t, r).Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("resource schema type is not an object")
	}
	return objType
}

// planWith builds a create/update plan that sets the given attributes and
// leaves every other attribute null.
func planWith(t *testing.T, r resource.Resource, overrides map[string]tftypes.Value) tfsdk.Plan {
	t.Helper()
	return tfsdk.Plan{Schema: resourceSchemaOf(t, r), Raw: rawObject(resourceObjType(t, r), overrides)}
}

// stateWith builds a resource state that sets the given attributes and leaves
// every other attribute null.
func stateWith(t *testing.T, r resource.Resource, overrides map[string]tftypes.Value) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: resourceSchemaOf(t, r), Raw: rawObject(resourceObjType(t, r), overrides)}
}

// createResponse / readResponse / updateResponse / deleteResponse prewire a
// response with a null state so the CRUD methods can fill it in.
func crudResponseState(t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()
	return stateWith(t, r, nil)
}
