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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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
