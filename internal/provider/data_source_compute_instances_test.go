package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"

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
