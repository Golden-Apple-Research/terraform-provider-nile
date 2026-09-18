// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
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

// isolateEnv saves, clears and restores an environment variable so tests do
// not depend on (or leak into) the developer's environment.
func isolateEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
			return
		}
		os.Unsetenv(key)
	})
}

// providerConfigRaw builds a raw provider configuration value. nil means
// the attribute is unset (null), tftypes.UnknownValue means unknown.
func providerConfigRaw(token, apiURL any) tftypes.Value {
	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"api_token": tftypes.String,
			"api_url":   tftypes.String,
		},
	}, map[string]tftypes.Value{
		"api_token": tftypes.NewValue(tftypes.String, token),
		"api_url":   tftypes.NewValue(tftypes.String, apiURL),
	})
}

// newConfigureRequest builds a ConfigureRequest carrying the given raw
// config value and the provider schema.
func newConfigureRequest(t *testing.T, raw tftypes.Value) provider.ConfigureRequest {
	t.Helper()
	p := New("test")
	var sResp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &sResp)
	if sResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", sResp.Diagnostics)
	}
	return provider.ConfigureRequest{
		Config: tfsdk.Config{Raw: raw, Schema: sResp.Schema},
	}
}

func configure(t *testing.T, raw tftypes.Value) provider.ConfigureResponse {
	t.Helper()
	var resp provider.ConfigureResponse
	New("test").Configure(context.Background(), newConfigureRequest(t, raw), &resp)
	return resp
}

func TestProviderConfigureFromConfig(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)

	resp := configure(t, providerConfigRaw("cfg-token", "https://example.test"))

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	client, ok := resp.DataSourceData.(*nileapi.Client)
	if !ok {
		t.Fatalf("DataSourceData = %T, want *nileapi.Client", resp.DataSourceData)
	}
	if client.AuthToken != "cfg-token" {
		t.Errorf("AuthToken = %q, want %q", client.AuthToken, "cfg-token")
	}
	if client.BaseURL.String() != "https://example.test" {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL.String(), "https://example.test")
	}
	if client.UserAgent != "terraform-provider-nile/test" {
		t.Errorf("UserAgent = %q, want %q", client.UserAgent, "terraform-provider-nile/test")
	}
	if resp.ResourceData != client {
		t.Error("ResourceData should carry the same client as DataSourceData")
	}
}

func TestProviderConfigureFromEnv(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)
	t.Setenv(envAPIToken, "env-token")
	t.Setenv(envAPIURL, "https://env.test")

	resp := configure(t, providerConfigRaw(nil, nil))

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	client, ok := resp.DataSourceData.(*nileapi.Client)
	if !ok {
		t.Fatalf("DataSourceData = %T, want *nileapi.Client", resp.DataSourceData)
	}
	if client.AuthToken != "env-token" {
		t.Errorf("AuthToken = %q, want %q", client.AuthToken, "env-token")
	}
	if client.BaseURL.String() != "https://env.test" {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL.String(), "https://env.test")
	}
}

func TestProviderConfigureConfigOverridesEnv(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)
	t.Setenv(envAPIToken, "env-token")
	t.Setenv(envAPIURL, "https://env.test")

	resp := configure(t, providerConfigRaw("cfg-token", "https://cfg.test"))

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	client := resp.DataSourceData.(*nileapi.Client)
	if client.AuthToken != "cfg-token" || client.BaseURL.String() != "https://cfg.test" {
		t.Errorf("config should win: token=%q url=%q", client.AuthToken, client.BaseURL.String())
	}
}

func TestProviderConfigureMissingToken(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)

	resp := configure(t, providerConfigRaw(nil, nil))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when no token is configured")
	}
	if !strings.Contains(fmt.Sprintf("%v", resp.Diagnostics), "Missing Nile API token") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}

func TestProviderConfigureUnknownValues(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)
	// A token is provided so the failure can only stem from the unknowns.
	t.Setenv(envAPIToken, "env-token")

	resp := configure(t, providerConfigRaw(tftypes.UnknownValue, tftypes.UnknownValue))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for unknown configuration values")
	}
	if !strings.Contains(fmt.Sprintf("%v", resp.Diagnostics), "Unknown Nile API") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}

func TestProviderConfigureInvalidURL(t *testing.T) {
	isolateEnv(t, envAPIToken)
	isolateEnv(t, envAPIURL)

	// A scheme-less URL must fail at configure time, not at first request.
	resp := configure(t, providerConfigRaw("cfg-token", "global.thenile.dev"))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error for a scheme-less base URL")
	}
	if !strings.Contains(fmt.Sprintf("%v", resp.Diagnostics), "scheme") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}
