// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
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

var (
	_ resource.Resource                = &databaseCredentialResource{}
	_ resource.ResourceWithConfigure   = &databaseCredentialResource{}
	_ resource.ResourceWithImportState = &databaseCredentialResource{}
)

// NewDatabaseCredentialResource constructs the nile_database_credential
// resource.
func NewDatabaseCredentialResource() resource.Resource {
	return &databaseCredentialResource{}
}

type databaseCredentialResource struct {
	client *nileapi.Client
}

type databaseCredentialResourceModel struct {
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	DatabaseName  types.String `tfsdk:"database_name"`
	TenantID      types.String `tfsdk:"tenant_id"`
	Internal      types.Bool   `tfsdk:"internal"`
	ID            types.String `tfsdk:"id"`
	Password      types.String `tfsdk:"password"`
	APIHost       types.String `tfsdk:"api_host"`
	DBHost        types.String `tfsdk:"db_host"`
	Created       types.String `tfsdk:"created"`
	Raw           types.String `tfsdk:"raw_json"`
}

func (r *databaseCredentialResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_database_credential"
}

func (r *databaseCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a credential of a Nile database via " +
			"`/workspaces/{workspaceSlug}/databases/{databaseName}/credentials`. " +
			"The generated password is returned by the API exactly once and stored in state as a " +
			"sensitive value; the API cannot return it again. Changing `tenant_id` or `internal` " +
			"forces replacement, because the API has no update endpoint. Use the `RotateCredential` " +
			"client method (or the API directly) to rotate a credential in place.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the Nile workspace that owns the database. Changing it forces replacement.",
			},
			"database_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Name of the database the credential belongs to. Changing it forces replacement.",
			},
			"tenant_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Tenant the credential is scoped to (`tenantId` query parameter). " +
					"Changing it forces replacement.",
			},
			"internal": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Whether to create an internal credential (`internal` query parameter). " +
					"Changing it forces replacement.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Credential identifier.",
			},
			"password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				MarkdownDescription: "Password of the credential. The API returns it only once, at creation time; " +
					"it is stored in state and never refreshed.",
			},
			"api_host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Host of the database's API endpoint.",
			},
			"db_host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Host of the database's PostgreSQL endpoint, if provisioned.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Redacted JSON payload of the credential as returned by the API.",
			},
		},
	}
}

func (r *databaseCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*nileapi.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *nileapi.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *databaseCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	databaseName := plan.DatabaseName.ValueString()

	credential, err := r.client.CreateCredential(
		ctx, workspaceSlug, databaseName,
		plan.TenantID.ValueString(), optionalBool(plan.Internal),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating database credential",
			fmt.Sprintf("Could not create a credential for database %q in workspace %q: %s",
				databaseName, workspaceSlug, err.Error()),
		)
		return
	}
	if credential.ID == "" {
		resp.Diagnostics.AddError(
			"API returned no credential identifier",
			fmt.Sprintf("The create response for database %q in workspace %q did not contain an id. "+
				"The request may have succeeded; check the Nile dashboard before retrying.", databaseName, workspaceSlug),
		)
		return
	}
	if credential.Password == "" {
		resp.Diagnostics.AddError(
			"API returned no credential password",
			fmt.Sprintf("The create response for database %q in workspace %q did not contain a password. "+
				"The credential exists, but Terraform cannot store its password; delete it and create a new one.",
				databaseName, workspaceSlug),
		)
		return
	}

	applyCredentialResource(&plan, credential, workspaceSlug, databaseName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	databaseName := state.DatabaseName.ValueString()
	credentialID := state.ID.ValueString()

	credentials, err := r.client.ListCredentials(
		ctx, workspaceSlug, databaseName,
		state.TenantID.ValueString(), optionalBool(state.Internal),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database credentials",
			fmt.Sprintf("Could not list credentials of database %q in workspace %q: %s",
				databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	for _, credential := range credentials {
		if credential.ID != credentialID {
			continue
		}
		applyCredentialResource(&state, credential, workspaceSlug, databaseName)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	tflog.Warn(ctx, "database credential no longer exists; removing it from state", map[string]any{
		"workspace":  workspaceSlug,
		"database":   databaseName,
		"credential": credentialID,
	})
	resp.State.RemoveResource(ctx)
}

func (r *databaseCredentialResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every configurable attribute is ForceNew, so the framework replaces the
	// resource instead of calling Update.
	resp.Diagnostics.AddError(
		"Update not supported",
		"nile_database_credential does not support in-place updates. This is a provider bug; "+
			"please report it with the configuration that triggered it.",
	)
}

func (r *databaseCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	databaseName := state.DatabaseName.ValueString()
	credentialID := state.ID.ValueString()

	err := r.client.DeleteCredential(ctx, workspaceSlug, databaseName, credentialID)
	if err != nil && !nileapi.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error deleting database credential",
			fmt.Sprintf("Could not delete credential %q of database %q in workspace %q: %s",
				credentialID, databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *databaseCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitResourceID(req.ID, 3)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Import a credential as `workspace_slug/database_name/credential_id`, got %q: %s",
				req.ID, err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_slug"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

// applyCredentialResource merges an API credential into the model. Password is
// never cleared: the API only returns it once, so a refresh must keep the
// value already in state.
func applyCredentialResource(m *databaseCredentialResourceModel, credential nileapi.Credential, workspaceSlug, databaseName string) {
	m.WorkspaceSlug = types.StringValue(workspaceSlug)
	m.DatabaseName = types.StringValue(databaseName)
	if credential.ID != "" {
		m.ID = types.StringValue(credential.ID)
	}
	if credential.Password != "" {
		m.Password = types.StringValue(credential.Password)
	}
	if credential.Tenant != "" {
		m.TenantID = types.StringValue(credential.Tenant)
	}
	m.Internal = types.BoolValue(credential.Internal)
	if credential.Database != nil {
		if credential.Database.APIHost != "" {
			m.APIHost = types.StringValue(credential.Database.APIHost)
		}
		if credential.Database.DBHost != "" {
			m.DBHost = types.StringValue(credential.Database.DBHost)
		}
	}
	if credential.Created != "" {
		m.Created = types.StringValue(credential.Created)
	}
	if len(credential.Raw) > 0 {
		m.Raw = redactedRawJSON(credential.Raw)
	}
}
