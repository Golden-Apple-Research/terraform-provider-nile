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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// Compile-time assertions that workspaceResource implements the required
// resource interfaces.
var (
	_ resource.Resource                = &workspaceResource{}
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// NewWorkspaceResource constructs the nile_workspace resource.
func NewWorkspaceResource() resource.Resource {
	return &workspaceResource{}
}

// workspaceResource implements the nile_workspace resource.
type workspaceResource struct {
	// client is the Nile API client injected by Configure.
	client *nileapi.Client
}

// workspaceResourceModel is the Terraform state of a nile_workspace resource.
type workspaceResourceModel struct {
	// Name is the display name of the workspace.
	Name types.String `tfsdk:"name"`
	// ID is the workspace identifier assigned by the API.
	ID types.String `tfsdk:"id"`
	// Slug is the URL-safe identifier used to reference the workspace.
	Slug types.String `tfsdk:"slug"`
	// Created is the creation timestamp.
	Created types.String `tfsdk:"created"`
	// StripeCustomerID is the billing customer linked to the workspace.
	StripeCustomerID types.String `tfsdk:"stripe_customer_id"`
	// Raw is the redacted JSON of the API response.
	Raw types.String `tfsdk:"raw_json"`
}

// Metadata sets the resource type name to "nile_workspace".
func (r *workspaceResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_workspace"
}

// Schema defines the nile_workspace attributes.
func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nile workspace via `POST /workspaces`. " +
			"The Nile API does not expose workspace deletion: destroying this resource removes it from " +
			"Terraform state but leaves the workspace in the control plane. Renaming is not supported either, " +
			"so changing `name` forces replacement (which, given the missing delete endpoint, means " +
			"creating a second workspace).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Display name of the workspace. The API derives the slug from it. Changing it forces replacement.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace identifier.",
			},
			"slug": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Slug of the workspace; used by all other resources to reference it.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp of the workspace.",
			},
			"stripe_customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stripe customer linked to the workspace, when billing is set up.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw API response of the workspace object, as JSON (secret fields redacted).",
			},
		},
	}
}

// Configure stores the *nileapi.Client handed out by the provider.
func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates the workspace via the Nile API and writes it to state.
func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	name := plan.Name.ValueString()
	workspace, err := r.client.CreateWorkspace(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating workspace",
			fmt.Sprintf("Could not create workspace %q: %s", name, err.Error()),
		)
		return
	}
	if workspace.Slug == "" && workspace.ID == "" {
		resp.Diagnostics.AddError(
			"API returned no workspace identifier",
			fmt.Sprintf("The create response for workspace %q contained neither a slug nor an id.", name),
		)
		return
	}
	applyWorkspaceResource(&plan, workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the workspace state from the API and removes the resource
// from state when the workspace no longer exists.
func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	slug := state.Slug.ValueString()
	workspace, err := r.client.GetWorkspace(ctx, slug)
	if err != nil {
		if nileapi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading workspace",
			fmt.Sprintf("Could not read workspace %q: %s", slug, err.Error()),
		)
		return
	}
	applyWorkspaceResource(&state, workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update copies the plan into state; every configurable attribute forces
// replacement, so there is nothing to update remotely.
func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every configurable attribute forces replacement; the framework calls
	// Update only for computed drift, which Create and Read already covered.
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the resource from state; the Nile API has no workspace
// deletion endpoint, so the workspace keeps running in the control plane.
func (r *workspaceResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// The Nile API has no workspace deletion endpoint. Removing the resource
	// from state is the only sensible action; log it loudly.
	tflog.Warn(ctx, "the Nile API does not support workspace deletion; the workspace is removed from Terraform state only and keeps running in the control plane")
}

// ImportState imports a workspace by its slug.
func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("slug"), req, resp)
}

// applyWorkspaceResource merges an API workspace into the model.
func applyWorkspaceResource(m *workspaceResourceModel, workspace nileapi.Workspace) {
	m.ID = stringOrNull(workspace.ID)
	m.Slug = stringOrNull(workspace.Slug)
	m.Created = stringOrNull(workspace.Created)
	m.StripeCustomerID = stringOrNull(workspace.StripeCustomerID)
	m.Raw = redactedRawJSON(workspace.Raw)
}
