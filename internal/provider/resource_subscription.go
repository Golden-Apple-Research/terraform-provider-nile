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

type workspaceSubscriptionResource struct {
	client *nileapi.Client
}

type workspaceSubscriptionResourceModel struct {
	WorkspaceSlug  types.String `tfsdk:"workspace_slug"`
	Level          types.String `tfsdk:"level"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	ValidFrom      types.String `tfsdk:"valid_from"`
	ValidTo        types.String `tfsdk:"valid_to"`
	Workspace      types.String `tfsdk:"workspace"`
}

func (r *workspaceSubscriptionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_workspace_subscription"
}

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

func (r *workspaceSubscriptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("workspace_slug"), req, resp)
}

func applySubscriptionResource(m *workspaceSubscriptionResourceModel, sub nileapi.WorkspaceSubscription) {
	m.Workspace = stringOrNull(sub.Workspace)
	m.SubscriptionID = stringOrNull(sub.SubscriptionID)
	m.ValidFrom = stringOrNull(sub.ValidFrom)
	m.ValidTo = stringOrNull(sub.ValidTo)
	m.Level = stringOrNull(sub.Level)
}
