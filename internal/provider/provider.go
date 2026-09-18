// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

// Package provider defines the Terraform provider for Nile.
package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// Environment variables that configure the provider as an alternative to the
// provider block attributes.
const (
	// envAPIURL overrides the api_url provider attribute.
	envAPIURL = "NILE_API_URL"
	// envAPIToken overrides the api_token provider attribute.
	envAPIToken = "NILE_API_TOKEN"
	// envOAuthClientID overrides the oauth_client_id provider attribute.
	envOAuthClientID = "NILE_OAUTH_CLIENT_ID"
	// envOAuthRefreshToken overrides the oauth_refresh_token provider attribute.
	envOAuthRefreshToken = "NILE_OAUTH_REFRESH_TOKEN"
)

// New instantiates the provider.
func New(version string) provider.Provider {
	return &nileProvider{version: version}
}

// nileProvider implements the Terraform provider for the Nile control plane.
type nileProvider struct {
	// version is the provider version reported to Terraform.
	version string
}

// Compile-time assertion that nileProvider implements provider.Provider.
var _ provider.Provider = &nileProvider{}

// Metadata sets the provider type name ("nile") and its version.
func (p *nileProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nile"
	resp.Version = p.version
}

// Schema defines the provider block attributes: the API base URL, the static
// API token, and the OAuth refresh-token credentials.
func (p *nileProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Interact with the Nile control plane API (https://www.thenile.dev).",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: fmt.Sprintf("Base URL of the Nile API. Defaults to `%s`. HTTPS is required; plain HTTP is accepted only for loopback test endpoints. May also be set via `%s`.", nileapi.DefaultBaseURL, envAPIURL),
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Bearer token used to authenticate against the Nile API. May also be set via `" + envAPIToken + "`. Takes precedence over OAuth authentication.",
			},
			"oauth_client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth client ID used when exchanging a refresh token for an access token. May also be set via `" + envOAuthClientID + "`.",
			},
			"oauth_refresh_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OAuth refresh token exchanged for an access token via `POST /oauth2/token` when no static `api_token` is configured. May also be set via `" + envOAuthRefreshToken + "`.",
			},
		},
	}
}

// providerData mirrors the provider block configuration attributes.
type providerData struct {
	// APIToken is the static bearer token for the Nile API.
	APIToken types.String `tfsdk:"api_token"`
	// APIURL is the base URL of the Nile API.
	APIURL types.String `tfsdk:"api_url"`
	// OAuthClientID identifies the OAuth client used for the token exchange.
	OAuthClientID types.String `tfsdk:"oauth_client_id"`
	// OAuthRefreshToken is exchanged for an access token when no static
	// APIToken is configured.
	OAuthRefreshToken types.String `tfsdk:"oauth_refresh_token"`
}

// Configure validates the provider configuration and hands a *nileapi.Client
// to every data source and resource. Authentication prefers a static API
// token and otherwise exchanges the configured OAuth refresh token for an
// access token.
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
	// Without a static token, fall back to exchanging an OAuth refresh
	// token for an access token.
	if apiToken == "" {
		oauthClientID := os.Getenv(envOAuthClientID)
		if !config.OAuthClientID.IsNull() && config.OAuthClientID.ValueString() != "" {
			oauthClientID = config.OAuthClientID.ValueString()
		}
		oauthRefreshToken := os.Getenv(envOAuthRefreshToken)
		if !config.OAuthRefreshToken.IsNull() && config.OAuthRefreshToken.ValueString() != "" {
			oauthRefreshToken = config.OAuthRefreshToken.ValueString()
		}
		if oauthRefreshToken != "" {
			form := url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {oauthRefreshToken},
			}
			if oauthClientID != "" {
				form.Set("client_id", oauthClientID)
			}
			// The bootstrap client never sends its placeholder token: the
			// exchange request goes out unauthenticated.
			boot, err := nileapi.NewClient(apiURL, "oauth-exchange")
			if err != nil {
				resp.Diagnostics.AddError("Failed to create Nile API client", err.Error())
				return
			}
			token, err := boot.ExchangeToken(ctx, form)
			if err != nil {
				resp.Diagnostics.AddError(
					"OAuth token exchange failed",
					fmt.Sprintf("Exchanging the refresh token for an access token against %s failed: %s", apiURL, err.Error()),
				)
				return
			}
			if token.AccessToken == "" {
				resp.Diagnostics.AddError(
					"OAuth token exchange returned no access token",
					fmt.Sprintf("The token response from %s did not contain an access_token.", apiURL),
				)
				return
			}
			apiToken = token.AccessToken
		}
	}
	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Nile API token",
			fmt.Sprintf("The provider cannot create the Nile API client as there is no API token configured. "+
				"Set the api_token attribute in the provider block, the %s environment variable, "+
				"or configure OAuth via oauth_refresh_token (or %s).",
				envAPIToken, envOAuthRefreshToken),
		)
		return
	}

	client, err := nileapi.NewClient(apiURL, apiToken)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Nile API client", err.Error())
		return
	}
	client.UserAgent = "terraform-provider-nile/" + p.version

	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources returns the constructors of all Nile data sources.
func (p *nileProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDatabaseComputeInstancesDataSource,
		NewDatabaseDataSource,
		NewDatabasesDataSource,
		NewDatabaseCredentialsDataSource,
		NewDatabaseUptimeInsightsDataSource,
		NewDatabaseErrorInsightsDataSource,
		NewDatabaseQueryPerformanceInsightsDataSource,
		NewComputeTypesDataSource,
		NewRegionsDataSource,
		NewWorkspaceDataSource,
		NewWorkspacesDataSource,
		NewWorkspaceComputeUsageDataSource,
		NewWorkspaceDevelopersDataSource,
		NewWorkspaceInvitesDataSource,
		NewWorkspaceSubscriptionDataSource,
		NewWorkspaceSubscriptionHistoryDataSource,
		NewWorkspaceBillingReadinessDataSource,
		NewWorkspaceBillingTotalsDataSource,
		NewDeveloperDataSource,
	}
}

// Resources returns the constructors of all Nile resources.
func (p *nileProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDatabaseResource,
		NewComputeInstanceResource,
		NewDatabaseCredentialResource,
		NewDeveloperInviteResource,
		NewWorkspaceResource,
		NewWorkspaceSubscriptionResource,
		NewBillingCustomerResource,
		NewProvisionedDatabaseResource,
	}
}
