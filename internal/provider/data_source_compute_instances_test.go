// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestComputeInstancesDataSourceMetadata(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource()

	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "nile"}, &resp)

	if resp.TypeName != "nile_database_compute_instances" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "nile_database_compute_instances")
	}
}

func TestComputeInstancesDataSourceSchema(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource()

	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	for _, name := range []string{"workspace_slug", "database_name", "start", "end", "id", "instances"} {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

func TestComputeInstancesDataSourceConfigure(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource().(*computeInstancesDataSource)
	client := &nileapi.Client{}

	var resp datasource.ConfigureResponse
	ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: client}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if ds.client != client {
		t.Error("client was not stored on the data source")
	}
}

func TestComputeInstancesDataSourceConfigureNil(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource().(*computeInstancesDataSource)

	var resp datasource.ConfigureResponse
	ds.Configure(context.Background(), datasource.ConfigureRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if ds.client != nil {
		t.Error("client should stay nil without provider data")
	}
}

func TestComputeInstancesDataSourceConfigureWrongType(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource().(*computeInstancesDataSource)

	var resp datasource.ConfigureResponse
	ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for unexpected provider data type")
	}
}

func TestStringOrNull(t *testing.T) {
	if got := stringOrNull(""); !got.IsNull() {
		t.Errorf("empty string should map to null, got %v", got)
	}
	if got := stringOrNull("value"); got.ValueString() != "value" {
		t.Errorf("ValueString() = %q, want %q", got.ValueString(), "value")
	}
}

func TestIsRFC3339Validator(t *testing.T) {
	v := isRFC3339Validator{}

	valid := []string{
		"2025-01-01T00:00:00Z",
		"2025-06-15T12:30:45+02:00",
		"2025-06-15T12:30:45.123-07:00",
	}
	for _, s := range valid {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: types.StringValue(s),
			Path:        path.Root("start"),
		}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%q: unexpected diagnostics: %v", s, resp.Diagnostics)
		}
	}

	invalid := []string{"", "not-a-timestamp", "2025-13-01T00:00:00Z"}
	for _, s := range invalid {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: types.StringValue(s),
			Path:        path.Root("start"),
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("%q: expected diagnostics", s)
		}
	}

	// Null and unknown values are skipped: they may become valid later.
	for _, s := range []types.String{types.StringNull(), types.StringUnknown()} {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: s,
			Path:        path.Root("start"),
		}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%v: unexpected diagnostics: %v", s, resp.Diagnostics)
		}
	}
}

// dataSourceSchema returns the data source schema for building requests.
func dataSourceSchema(t *testing.T) schema.Schema {
	t.Helper()
	ds := NewDatabaseComputeInstancesDataSource()
	var sResp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &sResp)
	if sResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", sResp.Diagnostics)
	}
	return sResp.Schema
}

// dataSourceConfigRaw builds a raw data source configuration value with the
// given string attributes set; all other attributes are null.
func dataSourceConfigRaw(t *testing.T, overrides map[string]string) tftypes.Value {
	t.Helper()
	objType := dataSourceSchema(t).Type().TerraformType(context.Background()).(tftypes.Object)

	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		if s, ok := overrides[name]; ok {
			vals[name] = tftypes.NewValue(tftypes.String, s)
			continue
		}
		vals[name] = tftypes.NewValue(typ, nil)
	}
	return tftypes.NewValue(objType, vals)
}

func TestComputeInstancesDataSourceReadStartAfterEnd(t *testing.T) {
	ds := NewDatabaseComputeInstancesDataSource().(*computeInstancesDataSource)

	// The window check must reject the request before any HTTP call, so a
	// client pointed at an unroutable address is good enough here.
	c, err := nileapi.NewClient("http://127.0.0.1:1", "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ds.client = c

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Raw: dataSourceConfigRaw(t, map[string]string{
				"workspace_slug": "ws",
				"database_name":  "db",
				"start":          "2025-02-01T00:00:00Z",
				"end":            "2025-01-01T00:00:00Z",
			}),
			Schema: dataSourceSchema(t),
		},
	}
	var resp datasource.ReadResponse
	ds.Read(context.Background(), req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a start timestamp after end")
	}
	if !strings.Contains(fmt.Sprintf("%v", resp.Diagnostics), "Invalid time window") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}
