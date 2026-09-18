// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// --- nile_workspace_billing_readiness ---------------------------------------

// NewWorkspaceBillingReadinessDataSource constructs the data source for the
// billing readiness of a workspace.
func NewWorkspaceBillingReadinessDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_billing_readiness",
		workspaceBillingReadinessSchema,
		readWorkspaceBillingReadiness,
	)
}

type workspaceBillingReadinessDataSourceModel struct {
	WorkspaceSlug        types.String `tfsdk:"workspace_slug"`
	ID                   types.String `tfsdk:"id"`
	Workspace            types.String `tfsdk:"workspace"`
	WorkspaceID          types.String `tfsdk:"workspace_id"`
	StripeCustomerID     types.String `tfsdk:"stripe_customer_id"`
	DefaultPaymentMethod types.String `tfsdk:"default_payment_method_id"`
	Status               types.String `tfsdk:"status"`
	CheckedAt            types.String `tfsdk:"checked_at"`
	LastError            types.String `tfsdk:"last_error"`
	Source               types.String `tfsdk:"source"`
	Detail               types.String `tfsdk:"detail"`
	Raw                  types.String `tfsdk:"raw_json"`
}

func workspaceBillingReadinessSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Resolves the billing readiness of a workspace via " +
			"`GET /workspaces/{workspaceSlug}/billing/readiness`, without creating a billing customer.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose billing readiness is read.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"workspace": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace slug as returned by the API.",
			},
			"workspace_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace identifier.",
			},
			"stripe_customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stripe customer linked to the workspace, if any.",
			},
			"default_payment_method_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Default payment method of the customer, if any.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Readiness status (`ready`, `missing_customer`, `missing_payment_method`, `lookup_failed` or `manual_review_required`).",
			},
			"checked_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp of the check.",
			},
			"last_error": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last error observed while resolving billing state, if any.",
			},
			"source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Source of the readiness result.",
			},
			"detail": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable detail about the readiness result.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full, unparsed JSON payload of the response as returned by the API.",
			},
		},
	}
}

func readWorkspaceBillingReadiness(ctx context.Context, client *nileapi.Client, data *workspaceBillingReadinessDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	out, err := client.GetBillingReadiness(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace billing readiness",
			fmt.Sprintf("Could not read billing readiness of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Workspace = stringOrNull(out.Workspace)
	data.WorkspaceID = stringOrNull(out.WorkspaceID)
	data.StripeCustomerID = stringOrNull(out.StripeCustomerID)
	data.DefaultPaymentMethod = stringOrNull(out.DefaultPaymentMethod)
	data.Status = stringOrNull(out.Status)
	data.CheckedAt = stringOrNull(out.CheckedAt)
	data.LastError = stringOrNull(out.LastError)
	data.Source = stringOrNull(out.Source)
	data.Detail = stringOrNull(out.Detail)
	data.Raw = types.StringValue(string(out.Raw))
}

// --- nile_workspace_billing_totals ------------------------------------------

// NewWorkspaceBillingTotalsDataSource constructs the data source for the
// monthly rated totals of a workspace.
func NewWorkspaceBillingTotalsDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_billing_totals",
		workspaceBillingTotalsSchema,
		readWorkspaceBillingTotals,
	)
}

type workspaceBillingTotalsDataSourceModel struct {
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	Month         types.String `tfsdk:"month"`
	ID            types.String `tfsdk:"id"`
	YM            types.String `tfsdk:"ym"`
	Totals        types.Map    `tfsdk:"totals"`
	Raw           types.String `tfsdk:"raw_json"`
}

func workspaceBillingTotalsSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Fetches the monthly totals by component, from rated lines, via " +
			"`GET /workspaces/{workspaceSlug}/billing/{ym}/totals`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose billing totals are read.",
			},
			"month": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						monthRegexp,
						"must be a month in YYYY-MM form, for example 2025-06",
					),
				},
				MarkdownDescription: "Month to report on, in `YYYY-MM` form.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (`<workspaceSlug>/<month>`).",
			},
			"ym": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Month as returned by the API (`ym` in the API response).",
			},
			"totals": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Float64Type,
				MarkdownDescription: "Totals keyed by billing component.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full, unparsed JSON payload of the response as returned by the API.",
			},
		},
	}
}

func readWorkspaceBillingTotals(ctx context.Context, client *nileapi.Client, data *workspaceBillingTotalsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	month := data.Month.ValueString()

	out, err := client.GetMonthlyTotals(ctx, workspaceSlug, month)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace billing totals",
			fmt.Sprintf("Could not read billing totals of workspace %q for %s: %s", workspaceSlug, month, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(dataSourceID(workspaceSlug, month))
	data.YM = stringOrNull(out.YM)
	totals, diags := types.MapValueFrom(ctx, types.Float64Type, out.Totals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Totals = totals
	data.Raw = types.StringValue(string(out.Raw))
}
