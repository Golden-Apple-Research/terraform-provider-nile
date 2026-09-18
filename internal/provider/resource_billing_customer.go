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

// Interface compliance assertions for billingCustomerResource.
var (
	_ resource.Resource                = &billingCustomerResource{}
	_ resource.ResourceWithConfigure   = &billingCustomerResource{}
	_ resource.ResourceWithImportState = &billingCustomerResource{}
)

// NewBillingCustomerResource constructs the nile_billing_customer resource.
func NewBillingCustomerResource() resource.Resource {
	return &billingCustomerResource{}
}

// billingCustomerResource manages the Stripe billing customer linked to a
// Nile workspace (nile_billing_customer).
type billingCustomerResource struct {
	client *nileapi.Client
}

// billingCustomerResourceModel holds the Terraform state of the
// nile_billing_customer resource: the Stripe customer linked to a workspace.
type billingCustomerResourceModel struct {
	// WorkspaceSlug is the workspace the billing customer is linked to.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// StripeCustomerID is the linked Stripe customer identifier.
	StripeCustomerID types.String `tfsdk:"stripe_customer_id"`
	// DefaultPaymentMethod is the customer's default payment method, if any.
	DefaultPaymentMethod types.String `tfsdk:"default_payment_method"`
	// Raw is the redacted raw JSON payload returned by the API.
	Raw types.String `tfsdk:"raw_json"`
}

// Metadata sets the Terraform resource type name to nile_billing_customer.
func (r *billingCustomerResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_billing_customer"
}

// Schema defines the attributes of the nile_billing_customer resource.
func (r *billingCustomerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ensures a Stripe billing customer exists for a Nile workspace via " +
			"`PUT /workspaces/{workspaceSlug}/billing/customer`. The call is idempotent: it finds or creates " +
			"the linked Stripe customer. The Nile API offers no way to unlink a billing customer, so " +
			"destroying this resource only removes it from Terraform state. Billing endpoints require a " +
			"session (developer) token; API keys are rejected with 403.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace to ensure the billing customer for. Changing it forces replacement.",
			},
			"stripe_customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stripe customer identifier linked to the workspace.",
			},
			"default_payment_method": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Default payment method attached to the Stripe customer, if any.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw API response of the billing customer object, as JSON (secret fields redacted).",
			},
		},
	}
}

// Configure wires the shared Nile API client from the provider into the
// resource.
func (r *billingCustomerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create ensures a Stripe billing customer exists for the workspace and
// stores the resulting state.
func (r *billingCustomerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan billingCustomerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	customer, err := r.client.EnsureBillingCustomer(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error ensuring billing customer",
			fmt.Sprintf("Could not ensure the billing customer of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	if customer.StripeCustomerID == "" {
		resp.Diagnostics.AddError(
			"API returned no Stripe customer id",
			fmt.Sprintf("The response for workspace %q did not contain a stripeCustomerId.", workspaceSlug),
		)
		return
	}
	applyBillingCustomerResource(&plan, customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read re-issues the idempotent PUT: the Nile API has no dedicated GET for
// the billing customer, so ensuring it is the closest read equivalent.
func (r *billingCustomerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state billingCustomerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	customer, err := r.client.EnsureBillingCustomer(ctx, workspaceSlug)
	if err != nil {
		if nileapi.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading billing customer",
			fmt.Sprintf("Could not ensure the billing customer of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	applyBillingCustomerResource(&state, customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update re-issues the idempotent PUT (for example after credentials were
// rotated on the Stripe side).
func (r *billingCustomerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan billingCustomerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	customer, err := r.client.EnsureBillingCustomer(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error ensuring billing customer",
			fmt.Sprintf("Could not ensure the billing customer of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	applyBillingCustomerResource(&plan, customer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the resource from Terraform state only: the Nile API does
// not support unlinking a billing customer.
func (r *billingCustomerResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// The Nile API cannot unlink a billing customer from a workspace.
	tflog.Warn(ctx, "the Nile API does not support unlinking a billing customer; it is removed from Terraform state only")
}

// ImportState imports the resource by workspace slug.
func (r *billingCustomerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("workspace_slug"), req, resp)
}

// applyBillingCustomerResource copies an API billing customer into the
// Terraform resource model.
func applyBillingCustomerResource(m *billingCustomerResourceModel, customer nileapi.WorkspaceBillingCustomer) {
	m.StripeCustomerID = stringOrNull(customer.StripeCustomerID)
	m.DefaultPaymentMethod = stringOrNull(customer.DefaultPaymentMethod)
	m.Raw = redactedRawJSON(customer.Raw)
}
