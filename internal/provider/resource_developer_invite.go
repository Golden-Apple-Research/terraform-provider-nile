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

// Compile-time assertions that developerInviteResource implements the
// required resource interfaces.
var (
	_ resource.Resource                = &developerInviteResource{}
	_ resource.ResourceWithConfigure   = &developerInviteResource{}
	_ resource.ResourceWithImportState = &developerInviteResource{}
)

// NewDeveloperInviteResource constructs the nile_developer_invite resource.
func NewDeveloperInviteResource() resource.Resource {
	return &developerInviteResource{}
}

// developerInviteResource implements the nile_developer_invite resource.
type developerInviteResource struct {
	// client is the Nile API client injected by Configure.
	client *nileapi.Client
}

// developerInviteResourceModel is the Terraform state of a
// nile_developer_invite resource.
type developerInviteResourceModel struct {
	// WorkspaceSlug is the workspace the invite belongs to.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// Email is the email address of the invitee.
	Email types.String `tfsdk:"email"`
	// Programmatic reports whether the invite returns a code instead of
	// sending an email.
	Programmatic types.Bool `tfsdk:"programmatic"`
	// ID is the invite identifier assigned by the API.
	ID types.String `tfsdk:"id"`
	// VerificationState is the verification state of the invite.
	VerificationState types.String `tfsdk:"verification_state"`
	// Code is the one-time invite code of a programmatic invite.
	Code types.String `tfsdk:"code"`
	// SenderEmail is the email address of the developer who sent the invite.
	SenderEmail types.String `tfsdk:"sender_email"`
	// Created is the creation timestamp.
	Created types.String `tfsdk:"created"`
	// Updated is the last update timestamp.
	Updated types.String `tfsdk:"updated"`
	// Raw is the redacted JSON payload of the API response.
	Raw types.String `tfsdk:"raw_json"`
}

// Metadata sets the resource type name to "nile_developer_invite".
func (r *developerInviteResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_developer_invite"
}

// Schema defines the nile_developer_invite attributes.
func (r *developerInviteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a developer invite of a Nile workspace via " +
			"`/workspaces/{workspaceSlug}/invites`. Inviting sends an email; with `programmatic = true` " +
			"the API returns an invite code instead (stored in the sensitive `code` attribute) so the " +
			"invite can be redeemed without email. There is no update endpoint: changing `email` or " +
			"`programmatic` forces replacement.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace to invite the developer to. Changing it forces replacement.",
			},
			"email": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Email address of the invitee. Changing it forces replacement.",
			},
			"programmatic": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplaceIfConfigured(),
				},
				MarkdownDescription: "If true, the API returns an invite code instead of only sending an email. " +
					"Changing it forces replacement.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Invite identifier.",
			},
			"verification_state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Verification state (`EMAIL_PENDING`, `EMAIL_SENT`, `VERIFIED` or `EXPIRED`).",
			},
			"code": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Invite code, only returned when `programmatic = true`. " +
					"Stored in state at creation and never copied from later API responses.",
			},
			"sender_email": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Email address of the developer who sent the invite.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
			},
			"updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last update timestamp.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Redacted JSON payload of the invite as returned by the API.",
			},
		},
	}
}

// Configure stores the *nileapi.Client handed out by the provider.
func (r *developerInviteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create invites a developer to the workspace, falling back to a lookup by
// email when the create response carries no invite body.
func (r *developerInviteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan developerInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	email := plan.Email.ValueString()
	programmatic := !plan.Programmatic.IsNull() && !plan.Programmatic.IsUnknown() && plan.Programmatic.ValueBool()

	invite, err := r.client.CreateDeveloperInvite(ctx, workspaceSlug, email, programmatic)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating developer invite",
			fmt.Sprintf("Could not invite %q to workspace %q: %s", email, workspaceSlug, err.Error()),
		)
		return
	}

	// Some API responses (and the email-only flow in particular) carry no
	// invite body. Fall back to finding the invite by email.
	if invite.ID == "" {
		invite, err = r.findInviteByEmail(ctx, workspaceSlug, email)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error finding created developer invite",
				fmt.Sprintf("The invite for %q in workspace %q was created but could not be read back: %s",
					email, workspaceSlug, err.Error()),
			)
			return
		}
	}

	plan.Programmatic = types.BoolValue(programmatic)
	applyInviteResource(&plan, invite, workspaceSlug)
	if invite.Code != "" {
		plan.Code = types.StringValue(invite.Code)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the invite state from the workspace invite list and removes
// the resource from state when the invite no longer exists.
func (r *developerInviteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state developerInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	inviteID := state.ID.ValueString()

	invites, err := r.client.ListDeveloperInvites(ctx, workspaceSlug, "")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading developer invites",
			fmt.Sprintf("Could not list invites of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	for _, invite := range invites {
		if invite.ID != inviteID {
			continue
		}
		applyInviteResource(&state, invite, workspaceSlug)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	tflog.Warn(ctx, "developer invite no longer exists; removing it from state", map[string]any{
		"workspace": workspaceSlug,
		"invite":    inviteID,
	})
	resp.State.RemoveResource(ctx)
}

// Update always fails: every configurable attribute forces replacement, so
// the framework should replace the resource instead of calling Update.
func (r *developerInviteResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every configurable attribute is ForceNew, so the framework replaces the
	// resource instead of calling Update.
	resp.Diagnostics.AddError(
		"Update not supported",
		"nile_developer_invite does not support in-place updates. This is a provider bug; "+
			"please report it with the configuration that triggered it.",
	)
}

// Delete revokes the invite via the Nile API and removes the resource from
// state.
func (r *developerInviteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state developerInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	inviteID := state.ID.ValueString()

	err := r.client.DeleteDeveloperInvite(ctx, workspaceSlug, inviteID)
	if err != nil && !nileapi.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error deleting developer invite",
			fmt.Sprintf("Could not delete invite %q in workspace %q: %s", inviteID, workspaceSlug, err.Error()),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports an invite from a `workspace_slug/invite_id` ID.
func (r *developerInviteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitResourceID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Import an invite as `workspace_slug/invite_id`, got %q: %s", req.ID, err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_slug"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// findInviteByEmail resolves the email-only create flow only when the API
// returns exactly one matching invite. Picking the newest invite is racy: a
// concurrent invite for the same address could otherwise be adopted and later
// deleted by this Terraform resource.
func (r *developerInviteResource) findInviteByEmail(ctx context.Context, workspaceSlug, email string) (nileapi.DeveloperInvite, error) {
	invites, err := r.client.ListDeveloperInvites(ctx, workspaceSlug, "")
	if err != nil {
		return nileapi.DeveloperInvite{}, err
	}
	matches := make([]nileapi.DeveloperInvite, 0, 1)
	for _, invite := range invites {
		if invite.Email == email {
			matches = append(matches, invite)
		}
	}
	if len(matches) == 0 {
		return nileapi.DeveloperInvite{}, fmt.Errorf("no invite for %q found in workspace %q", email, workspaceSlug)
	}
	if len(matches) > 1 {
		return nileapi.DeveloperInvite{}, fmt.Errorf(
			"found %d invites for %q in workspace %q; the API did not identify which invite was created",
			len(matches), email, workspaceSlug,
		)
	}
	return matches[0], nil
}

// applyInviteResource merges an API invite into the model. Programmatic and
// code are create-only and are left untouched, so a refresh cannot clobber a
// one-time invite code and imports do not invent a programmatic value.
func applyInviteResource(m *developerInviteResourceModel, invite nileapi.DeveloperInvite, workspaceSlug string) {
	m.WorkspaceSlug = types.StringValue(workspaceSlug)
	if invite.ID != "" {
		m.ID = types.StringValue(invite.ID)
	}
	if invite.Email != "" {
		m.Email = types.StringValue(invite.Email)
	}
	if invite.VerificationState != "" {
		m.VerificationState = types.StringValue(invite.VerificationState)
	}
	if invite.Sender != nil && invite.Sender.Email != "" {
		m.SenderEmail = types.StringValue(invite.Sender.Email)
	}
	if invite.Created != "" {
		m.Created = types.StringValue(invite.Created)
	}
	if invite.Updated != "" {
		m.Updated = types.StringValue(invite.Updated)
	}
	if len(invite.Raw) > 0 {
		m.Raw = redactedRawJSON(invite.Raw)
	}
}
