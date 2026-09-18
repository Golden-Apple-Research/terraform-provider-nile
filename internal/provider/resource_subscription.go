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

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// Compile-time assertions that workspaceSubscriptionResource implements the
// required Terraform Plugin Framework resource interfaces.
var (
	_ resource.Resource                = &workspaceSubscriptionResource{}
	_ resource.ResourceWithConfigure   = &workspaceSubscriptionResource{}
	_ resource.ResourceWithImportState = &workspaceSubscriptionResource{}
)

// NewWorkspaceSubscriptionResource constructs the nile_workspace_subscription
// resource.
func NewWorkspaceSubscriptionResource() resource.Resource {
	return &workspaceSubscriptionResource{}
}

// workspaceSubscriptionResource implements the nile_workspace_subscription
// resource, which starts, changes and closes the subscription of a workspace.
type workspaceSubscriptionResource struct {
	// client is the Nile API client shared by all provider resources.
	client *nileapi.Client
}

// workspaceSubscriptionResourceModel is the Terraform state model of the
// nile_workspace_subscription resource.
type workspaceSubscriptionResourceModel struct {
	// WorkspaceSlug is the slug of the workspace whose subscription is managed.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Level is the subscription level, for example `free` or `paid`.
	Level types.String `tfsdk:"level"`
	// SubscriptionID is the identifier of the active subscription.
	SubscriptionID types.String `tfsdk:"subscription_id"`
	// ValidFrom is the start of the subscription validity window.
	ValidFrom types.String `tfsdk:"valid_from"`
	// ValidTo is the end of the subscription validity window.
	ValidTo types.String `tfsdk:"valid_to"`
	// Workspace is the workspace reference reported by the API.
	Workspace types.String `tfsdk:"workspace"`
}

// Metadata sets the Terraform resource type name.
func (r *workspaceSubscriptionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_workspace_subscription"
}

// Schema defines the attributes of the nile_workspace_subscription resource.
func (r *workspaceSubscriptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the subscription of a Nile workspace via " +
			"`POST/PUT/DELETE /workspaces/{workspaceSlug}/subscription`. Creating the resource starts a " +
			"subscription at the configured `level`, changing `level` updates it in place and destroying " +
			"the resource closes it. Billing endpoints require a session (developer) token; API keys are " +
			"rejected with 403.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose subscription is managed. Changing it forces replacement.",
			},
			"level": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Subscription level, for example `free` or `paid`. Changing it updates the subscription in place.",
			},
			"subscription_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the active subscription; used to close it on destroy.",
			},
			"valid_from": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Start of the subscription validity window.",
			},
			"valid_to": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "End of the subscription validity window.",
			},
			"workspace": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace reference reported by the API.",
			},
		},
	}
}

// Configure stores the provider's shared Nile API client on the resource.
func (r *workspaceSubscriptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create starts a subscription for the workspace at the configured level and
// reads it back to fill the computed attributes.
func (r *workspaceSubscriptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceSubscriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	level := plan.Level.ValueString()
	if err := r.client.StartSubscription(ctx, workspaceSlug, level); err != nil {
		resp.Diagnostics.AddError(
			"Error starting workspace subscription",
			fmt.Sprintf("Could not start a %q subscription for workspace %q: %s", level, workspaceSlug, err.Error()),
		)
		return
	}
	// StartSubscription returns no body; read the subscription back to fill
	// the computed attributes.
	sub, err := r.client.GetCurrentSubscription(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace subscription",
			fmt.Sprintf("The subscription of workspace %q was started but could not be read back: %s", workspaceSlug, err.Error()),
		)
		return
	}
	applySubscriptionResource(&plan, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the subscription from the Nile API; a subscription that no
// longer exists is removed from state.
func (r *workspaceSubscriptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceSubscriptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	sub, err := r.client.GetCurrentSubscription(ctx, workspaceSlug)
	if err != nil {
		if nileapi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading workspace subscription",
			fmt.Sprintf("Could not read the subscription of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	applySubscriptionResource(&state, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update changes the subscription level in place via the change endpoint.
func (r *workspaceSubscriptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceSubscriptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	level := plan.Level.ValueString()
	if err := r.client.ChangeSubscription(ctx, workspaceSlug, level); err != nil {
		resp.Diagnostics.AddError(
			"Error changing workspace subscription",
			fmt.Sprintf("Could not change the subscription of workspace %q to %q: %s", workspaceSlug, level, err.Error()),
		)
		return
	}
	sub, err := r.client.GetCurrentSubscription(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace subscription",
			fmt.Sprintf("The subscription of workspace %q was changed but could not be read back: %s", workspaceSlug, err.Error()),
		)
		return
	}
	applySubscriptionResource(&plan, sub)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete closes the subscription identified in state, looking up its id first
// when the state predates the computed attribute.
func (r *workspaceSubscriptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceSubscriptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	subscriptionID := state.SubscriptionID.ValueString()
	if subscriptionID == "" {
		// The state may predate the computed id; look it up before closing.
		sub, err := r.client.GetCurrentSubscription(ctx, workspaceSlug)
		if err != nil {
			if nileapi.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError(
				"Error reading workspace subscription",
				fmt.Sprintf("Could not read the subscription of workspace %q before closing it: %s", workspaceSlug, err.Error()),
			)
			return
		}
		subscriptionID = sub.SubscriptionID
	}
	if subscriptionID == "" {
		// No active subscription reported: nothing to close.
		return
	}
	if err := r.client.CloseSubscription(ctx, workspaceSlug, subscriptionID); err != nil {
		if nileapi.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error closing workspace subscription",
			fmt.Sprintf("Could not close subscription %q of workspace %q: %s", subscriptionID, workspaceSlug, err.Error()),
		)
	}
}

// ImportState imports a subscription using the workspace slug as the import ID.
func (r *workspaceSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("workspace_slug"), req, resp)
}

// applySubscriptionResource merges an API subscription into the resource model.
func applySubscriptionResource(m *workspaceSubscriptionResourceModel, sub nileapi.WorkspaceSubscription) {
	m.Workspace = stringOrNull(sub.Workspace)
	m.SubscriptionID = stringOrNull(sub.SubscriptionID)
	m.ValidFrom = stringOrNull(sub.ValidFrom)
	m.ValidTo = stringOrNull(sub.ValidTo)
	m.Level = stringOrNull(sub.Level)
}
