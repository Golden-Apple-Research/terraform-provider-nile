package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

func TestProviderMetadata(t *testing.T) {
	p := New("1.2.3")

	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	if resp.TypeName != "nile" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "nile")
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", resp.Version, "1.2.3")
	}
}

func TestProviderSchema(t *testing.T) {
	p := New("test")

	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	apiURL, ok := resp.Schema.Attributes["api_url"].(providerschema.StringAttribute)
	if !ok {
		t.Fatalf("api_url missing or wrong type: %T", resp.Schema.Attributes["api_url"])
	}
	if !apiURL.Optional {
		t.Error("api_url should be optional")
	}

	apiToken, ok := resp.Schema.Attributes["api_token"].(providerschema.StringAttribute)
	if !ok {
		t.Fatalf("api_token missing or wrong type: %T", resp.Schema.Attributes["api_token"])
	}
	if !apiToken.Optional {
		t.Error("api_token should be optional")
	}
	if !apiToken.Sensitive {
		t.Error("api_token should be sensitive")
	}
}

func TestProviderDataSources(t *testing.T) {
	p := New("test")

	fns := p.DataSources(context.Background())
	if len(fns) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(fns))
	}

	var resp datasource.MetadataResponse
	fns[0]().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "nile"}, &resp)
	if resp.TypeName != "nile_database_compute_instances" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "nile_database_compute_instances")
	}
}

func TestProviderHasNoResources(t *testing.T) {
	p := New("test")

	if got := len(p.Resources(context.Background())); got != 0 {
		t.Errorf("expected 0 resources, got %d", got)
	}
}
