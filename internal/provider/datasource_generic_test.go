// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// genericModel is a minimal model used to exercise the shared read-only data
// source plumbing without coupling the test to a concrete Nile entity.
type genericModel struct {
	Name types.String `tfsdk:"name"`
	Out  types.String `tfsdk:"out"`
}

func genericSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true},
			"out":  schema.StringAttribute{Computed: true},
		},
	}
}

func genericRead(_ context.Context, _ *nileapi.Client, data *genericModel, _ *datasource.ReadResponse) {
	data.Out = types.StringValue("read:" + data.Name.ValueString())
}

func newGenericDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_generic_test", genericSchema, genericRead)
}

func TestReadOnlyDataSourceMetadata(t *testing.T) {
	ds := newGenericDataSource()

	var resp datasource.MetadataResponse
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "nile"}, &resp)

	if resp.TypeName != "nile_generic_test" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "nile_generic_test")
	}
}

func TestReadOnlyDataSourceSchema(t *testing.T) {
	ds := newGenericDataSource()

	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, name := range []string{"name", "out"} {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

func TestReadOnlyDataSourceConfigure(t *testing.T) {
	ds := newGenericDataSource().(*readOnlyDataSource[genericModel])
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

func TestReadOnlyDataSourceConfigureNil(t *testing.T) {
	ds := newGenericDataSource().(*readOnlyDataSource[genericModel])

	var resp datasource.ConfigureResponse
	ds.Configure(context.Background(), datasource.ConfigureRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if ds.client != nil {
		t.Error("client should stay nil without provider data")
	}
}

func TestReadOnlyDataSourceConfigureWrongType(t *testing.T) {
	ds := newGenericDataSource().(*readOnlyDataSource[genericModel])

	var resp datasource.ConfigureResponse
	ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for unexpected provider data type")
	}
}

// genericReadRequest builds a read request whose config sets the given name.
func genericReadRequest(t *testing.T, name string) (datasource.ReadRequest, datasource.ReadResponse) {
	t.Helper()
	ds := newGenericDataSource()
	sch := dataSourceSchemaOf(t, ds)
	objType, ok := sch.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}
	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Raw:    rawObject(objType, map[string]tftypes.Value{"name": stringAttr(name)}),
			Schema: sch,
		},
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sch, Raw: rawObject(objType, nil)}}
	return req, resp
}

func TestReadOnlyDataSourceRead(t *testing.T) {
	ds := newGenericDataSource().(*readOnlyDataSource[genericModel])
	// The read function makes no HTTP call; the client only has to be set.
	ds.client = testAPIServer(t, `{}`)

	req, resp := genericReadRequest(t, "x")
	ds.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	var out types.String
	if diags := resp.State.GetAttribute(context.Background(), path.Root("out"), &out); diags.HasError() {
		t.Fatalf("GetAttribute: %v", diags)
	}
	if out.ValueString() != "read:x" {
		t.Errorf("out = %q, want %q", out.ValueString(), "read:x")
	}
}

func TestReadOnlyDataSourceReadWithoutConfigure(t *testing.T) {
	ds := newGenericDataSource()

	req, resp := genericReadRequest(t, "x")
	ds.Read(context.Background(), req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the provider client is missing")
	}
}

func TestDataSourceID(t *testing.T) {
	if got := dataSourceID("ws", "db"); got != "ws/db" {
		t.Errorf("dataSourceID = %q, want %q", got, "ws/db")
	}
	// Parts are escaped, so a slash inside a part stays unambiguous.
	if got := dataSourceID("ws", "db/replica"); got != "ws/db%2Freplica" {
		t.Errorf("dataSourceID = %q, want %q", got, "ws/db%2Freplica")
	}
}
