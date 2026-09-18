// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// Compile-time assertions that provisionedDatabaseResource implements the
// required resource interfaces.
var (
	_ resource.Resource              = &provisionedDatabaseResource{}
	_ resource.ResourceWithConfigure = &provisionedDatabaseResource{}
)

// NewProvisionedDatabaseResource constructs the nile_provisioned_database
// resource.
func NewProvisionedDatabaseResource() resource.Resource {
	return &provisionedDatabaseResource{}
}

// provisionedDatabaseResource implements the nile_provisioned_database
// resource.
type provisionedDatabaseResource struct {
	// client is the Nile API client injected by Configure.
	client *nileapi.Client
}

// provisionedDatabaseResourceModel is the Terraform state of a
// nile_provisioned_database resource.
type provisionedDatabaseResourceModel struct {
	// Region is the region the database was provisioned in.
	Region types.String `tfsdk:"region"`
	// ClaimCode is the code that attaches the database to a workspace; it is
	// consumed by the claim.
	ClaimCode types.String `tfsdk:"claim_code"`
	// DatabaseID is the identifier of the provisioned database.
	DatabaseID types.String `tfsdk:"database_id"`
	// DatabaseName is the server-assigned name of the provisioned database.
	DatabaseName types.String `tfsdk:"database_name"`
	// APIHost is the HTTPS API host of the dedicated database.
	APIHost types.String `tfsdk:"api_host"`
	// DBHost is the Postgres connection host of the dedicated database.
	DBHost types.String `tfsdk:"db_host"`
	// CredentialID is the identifier of the bootstrap credential.
	CredentialID types.String `tfsdk:"credential_id"`
	// Username is the one-time bootstrap username.
	Username types.String `tfsdk:"username"`
	// Password is the one-time bootstrap password returned at provisioning.
	Password types.String `tfsdk:"password"`
	// Sharded reports whether the provisioned database is sharded.
	Sharded types.Bool `tfsdk:"sharded"`
	// Raw is the redacted JSON payload of the provisioning response.
	Raw types.String `tfsdk:"raw_json"`
}

// Metadata sets the resource type name to "nile_provisioned_database".
func (r *provisionedDatabaseResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_provisioned_database"
}

// Schema defines the nile_provisioned_database attributes.
func (r *provisionedDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provisions a dedicated database via the unauthenticated " +
			"`POST /databases/provision` endpoint and exposes the resulting claim code. Feed the claim code " +
			"into the `claim_code` attribute of a `nile_database` resource to attach the provisioned " +
			"database to a workspace. Dedicated compute requires a paid plan; the free tier answers with " +
			"403 `feature_ineligible`. The API documents no read or delete endpoint for provisioned " +
			"databases: this resource performs no API call on refresh or destroy.",
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Region to provision the dedicated database in (for example `AWS_EU_CENTRAL_1`). Changing it forces replacement.",
			},
			"claim_code": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Claim code of the provisioned database. Pass it to `nile_database.claim_code` " +
					"to attach the database to a workspace; the code is consumed by the claim.",
			},
			"database_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "ID of the provisioned database.",
			},
			"database_name": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Server-assigned name of the provisioned database (pattern `unauth_…`).",
			},
			"api_host": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "HTTPS API host of the dedicated database.",
			},
			"db_host": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Postgres connection host of the dedicated database.",
			},
			"credential_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "ID of the bootstrap credential created with the database.",
			},
			"username": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Bootstrap username (`NILEDB_USER` from the response env).",
			},
			"password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "One-time bootstrap password. The API returns it exactly once; it is not part of later responses.",
			},
			"sharded": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Whether the provisioned database is sharded.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Raw API response of the provisioning call, as JSON (secret fields redacted).",
			},
		},
	}
}

// Configure stores the *nileapi.Client handed out by the provider.
func (r *provisionedDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*nileapi.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T, want *nileapi.Client", req.ProviderData))
		return
	}
	r.client = client
}

// Create provisions a dedicated database via the unauthenticated
// provision endpoint and stores the claim code and connection details.
func (r *provisionedDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan provisionedDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	region := plan.Region.ValueString()
	provisioned, err := r.client.ProvisionDatabase(ctx, region)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error provisioning database",
			fmt.Sprintf("Could not provision a dedicated database in region %q: %s", region, err.Error()),
		)
		return
	}
	if provisioned.ClaimCode == "" {
		resp.Diagnostics.AddError(
			"API returned no claim code",
			fmt.Sprintf("The provisioning response for region %q did not contain a claimCode.", region),
		)
		return
	}
	applyProvisionedDatabaseResource(&plan, provisioned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read performs no API call: the Nile API documents no endpoint to read a
// provisioned (not yet claimed) database, so the state is authoritative.
func (r *provisionedDatabaseResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update performs no API call: region changes force replacement.
func (r *provisionedDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan provisionedDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete performs no API call: the Nile API documents no endpoint to
// decommission a provisioned database.
func (r *provisionedDatabaseResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	tflog.Warn(ctx, "the Nile API does not expose deletion of provisioned databases; the provisioned database is removed from Terraform state only")
}

// applyProvisionedDatabaseResource merges a provisioning response into the
// model.
func applyProvisionedDatabaseResource(m *provisionedDatabaseResourceModel, provisioned nileapi.ProvisionedDatabase) {
	m.ClaimCode = stringOrNull(provisioned.ClaimCode)
	m.DatabaseID = stringOrNull(provisioned.DatabaseID)
	m.DatabaseName = stringOrNull(provisioned.DatabaseName)
	m.APIHost = stringOrNull(provisioned.APIHost)
	m.DBHost = stringOrNull(provisioned.DBHost)
	m.CredentialID = stringOrNull(provisioned.CredentialID)
	m.Username = stringOrNull(provisioned.Env["NILEDB_USER"])
	m.Password = stringOrNull(provisioned.Password)
	m.Sharded = types.BoolValue(provisioned.Sharded)
	m.Raw = redactedRawJSON(provisioned.Raw)
}
