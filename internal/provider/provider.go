// Package provider defines the Terraform provider for Nile.
package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

const (
	envAPIURL   = "NILE_API_URL"
	envAPIToken = "NILE_API_TOKEN"
)

// New instantiates the provider.
func New(version string) provider.Provider {
	return &nileProvider{version: version}
}

type nileProvider struct {
	version string
}

var _ provider.Provider = &nileProvider{}

func (p *nileProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nile"
	resp.Version = p.version
}

func (p *nileProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Interact with the Nile control plane API (https://www.thenile.dev).",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: fmt.Sprintf("Base URL of the Nile API. Defaults to `%s`. May also be set via `%s`.", nileapi.DefaultBaseURL, envAPIURL),
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Bearer token used to authenticate against the Nile API. May also be set via `" + envAPIToken + "`.",
			},
		},
	}
}

type providerData struct {
	APIToken types.String `tfsdk:"api_token"`
	APIURL   types.String `tfsdk:"api_url"`
}

func (p *nileProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerData
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Unknown Nile API URL",
			"The provider cannot create the Nile API client as there is an unknown configuration value for the Nile API URL. "+
				"Set the value statically in the provider block or via the "+envAPIURL+" environment variable.",
		)
	}
	if config.APIToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown Nile API token",
			"The provider cannot create the Nile API client as there is an unknown configuration value for the Nile API token. "+
				"Set the value statically in the provider block or via the "+envAPIToken+" environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := os.Getenv(envAPIURL)
	if !config.APIURL.IsNull() && config.APIURL.ValueString() != "" {
		apiURL = config.APIURL.ValueString()
	}

	apiToken := os.Getenv(envAPIToken)
	if !config.APIToken.IsNull() && config.APIToken.ValueString() != "" {
		apiToken = config.APIToken.ValueString()
	}
	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Nile API token",
			fmt.Sprintf("The provider cannot create the Nile API client as there is no API token configured. "+
				"Set the api_token attribute in the provider block or the %s environment variable.", envAPIToken),
		)
		return
	}

	client, err := nileapi.NewClient(apiURL, apiToken)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Nile API client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *nileProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDatabaseComputeInstancesDataSource,
	}
}

func (p *nileProvider) Resources(ctx context.Context) []func() resource.Resource {
	return nil
}
