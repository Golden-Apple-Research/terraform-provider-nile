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

// NewWorkspaceSubscriptionDataSource constructs the data source for the
// current subscription of a workspace.
func NewWorkspaceSubscriptionDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_subscription",
		workspaceSubscriptionSchema,
		readWorkspaceSubscription,
	)
}

type workspaceSubscriptionDataSourceModel struct {
	WorkspaceSlug        types.String `tfsdk:"workspace_slug"`
	ID                   types.String `tfsdk:"id"`
	Level                types.String `tfsdk:"level"`
	ValidFrom            types.String `tfsdk:"valid_from"`
	ValidTo              types.String `tfsdk:"valid_to"`
	SubscriptionID       types.String `tfsdk:"subscription_id"`
	DefaultPaymentMethod types.String `tfsdk:"default_payment_method"`
	Raw                  types.String `tfsdk:"raw_json"`
}

func workspaceSubscriptionSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Fetches the current subscription of a workspace via " +
			"`GET /workspaces/{workspaceSlug}/subscription`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose subscription is read.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"level": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current subscription level.",
			},
			"valid_from": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Start of the current subscription period.",
			},
			"valid_to": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "End of the current subscription period, if scheduled.",
			},
			"subscription_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the current subscription.",
			},
			"default_payment_method": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Default payment method of the subscription, if any.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Redacted JSON payload of the subscription as returned by the API.",
			},
		},
	}
}

func readWorkspaceSubscription(ctx context.Context, client *nileapi.Client, data *workspaceSubscriptionDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	sub, err := client.GetCurrentSubscription(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace subscription",
			fmt.Sprintf("Could not read the current subscription of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	m := subscriptionModelFromAPI(sub)
	data.ID = types.StringValue(workspaceSlug)
	data.Level = m.Level
	data.ValidFrom = m.ValidFrom
	data.ValidTo = m.ValidTo
	data.SubscriptionID = m.SubscriptionID
	data.DefaultPaymentMethod = m.DefaultPaymentMethod
	data.Raw = m.Raw
}

// NewWorkspaceSubscriptionHistoryDataSource constructs the data source for the
// subscription history of a workspace.
func NewWorkspaceSubscriptionHistoryDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_subscription_history",
		workspaceSubscriptionHistorySchema,
		readWorkspaceSubscriptionHistory,
	)
}

type workspaceSubscriptionHistoryDataSourceModel struct {
	WorkspaceSlug types.String        `tfsdk:"workspace_slug"`
	ID            types.String        `tfsdk:"id"`
	Subscriptions []subscriptionModel `tfsdk:"subscriptions"`
}

func workspaceSubscriptionHistorySchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists all subscription records of a workspace, most recent first, via " +
			"`GET /workspaces/{workspaceSlug}/subscription/history`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose subscription history is read.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"subscriptions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Subscription records, most recent first.",
				NestedObject:        schema.NestedAttributeObject{Attributes: subscriptionAttributes()},
			},
		},
	}
}

func readWorkspaceSubscriptionHistory(ctx context.Context, client *nileapi.Client, data *workspaceSubscriptionHistoryDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	subscriptions, err := client.ListSubscriptionHistory(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading subscription history",
			fmt.Sprintf("Could not read the subscription history of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Subscriptions = subscriptionModelsFromAPI(subscriptions)
}
