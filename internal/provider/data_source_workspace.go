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

// --- nile_workspace --------------------------------------------------------

// NewWorkspaceDataSource constructs the data source for a single workspace.
func NewWorkspaceDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_workspace", workspaceDataSourceSchema, readWorkspace)
}

// workspaceDataSourceModel is the Terraform model of the nile_workspace
// data source, which reads a single workspace.
type workspaceDataSourceModel struct {
	// Slug is the globally unique slug of the workspace to read.
	Slug types.String `tfsdk:"slug"`
	// ID is the workspace identifier (`id` in the API response).
	ID types.String `tfsdk:"id"`
	// Name is the workspace name.
	Name types.String `tfsdk:"name"`
	// StripeCustomerID is the Stripe customer linked to the workspace, if any.
	StripeCustomerID types.String `tfsdk:"stripe_customer_id"`
	// Created is the creation timestamp.
	Created types.String `tfsdk:"created"`
}

// workspaceDataSourceSchema returns the schema of the nile_workspace data source.
func workspaceDataSourceSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Fetches a single Nile workspace via " +
			"`GET /workspaces/{workspaceSlug}`.",
		Attributes: map[string]schema.Attribute{
			"slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Globally unique slug of the workspace.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace identifier (`id` in the API response).",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workspace name.",
			},
			"stripe_customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stripe customer linked to the workspace, if any.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
			},
		},
	}
}

// readWorkspace fetches a single workspace and fills the data source model.
func readWorkspace(ctx context.Context, client *nileapi.Client, data *workspaceDataSourceModel, resp *datasource.ReadResponse) {
	slug := data.Slug.ValueString()

	ws, err := client.GetWorkspace(ctx, slug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading workspace",
			fmt.Sprintf("Could not read workspace %q: %s", slug, err.Error()),
		)
		return
	}

	m := workspaceModelFromAPI(ws)
	data.ID = m.ID
	data.Name = m.Name
	data.StripeCustomerID = m.StripeCustomerID
	data.Created = m.Created
}

// --- nile_workspaces -------------------------------------------------------

// NewWorkspacesDataSource constructs the data source for listing workspaces.
func NewWorkspacesDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_workspaces", workspacesDataSourceSchema, readWorkspaces)
}

// workspacesDataSourceModel is the Terraform model of the nile_workspaces
// list data source.
type workspacesDataSourceModel struct {
	// ID is the stable identifier of this data source instance (`workspaces`).
	ID types.String `tfsdk:"id"`
	// Workspaces are the workspaces visible to the authenticated developer.
	Workspaces []workspaceModel `tfsdk:"workspaces"`
}

// workspacesDataSourceSchema returns the schema of the nile_workspaces data source.
func workspacesDataSourceSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists all Nile workspaces the authenticated developer " +
			"has access to, via `GET /workspaces`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (`workspaces`).",
			},
			"workspaces": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The workspaces visible to the authenticated developer.",
				NestedObject:        schema.NestedAttributeObject{Attributes: workspaceAttributes()},
			},
		},
	}
}

// readWorkspaces lists the workspaces the authenticated developer has access to.
func readWorkspaces(ctx context.Context, client *nileapi.Client, data *workspacesDataSourceModel, resp *datasource.ReadResponse) {
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing workspaces",
			fmt.Sprintf("Could not list workspaces: %s", err.Error()),
		)
		return
	}

	data.ID = types.StringValue("workspaces")
	data.Workspaces = workspaceModelsFromAPI(workspaces)
}

// --- nile_workspace_developers ---------------------------------------------

// NewWorkspaceDevelopersDataSource constructs the data source for listing the
// developers of a workspace.
func NewWorkspaceDevelopersDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_workspace_developers",
		workspaceDevelopersSchema,
		readWorkspaceDevelopers,
	)
}

// workspaceDevelopersDataSourceModel is the Terraform model of the
// nile_workspace_developers data source.
type workspaceDevelopersDataSourceModel struct {
	// WorkspaceSlug is the slug of the workspace whose developers are listed.
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	// ID is the stable identifier of this data source instance (the workspace slug).
	ID types.String `tfsdk:"id"`
	// Developers are the developers with access to the workspace.
	Developers []developerModel `tfsdk:"developers"`
}

// workspaceDevelopersSchema returns the schema of the nile_workspace_developers data source.
func workspaceDevelopersSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the developers with access to a workspace via " +
			"`GET /workspaces/{workspaceSlug}/developers`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose developers are listed.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"developers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The developers with access to the workspace.",
				NestedObject:        schema.NestedAttributeObject{Attributes: developerAttributes()},
			},
		},
	}
}

// readWorkspaceDevelopers lists the developers with access to a workspace.
func readWorkspaceDevelopers(ctx context.Context, client *nileapi.Client, data *workspaceDevelopersDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	developers, err := client.ListWorkspaceDevelopers(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing workspace developers",
			fmt.Sprintf("Could not list developers of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Developers = make([]developerModel, 0, len(developers))
	for _, d := range developers {
		data.Developers = append(data.Developers, developerModelFromAPI(d))
	}
}
