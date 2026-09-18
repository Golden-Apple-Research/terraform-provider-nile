// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// NewWorkspaceInvitesDataSource constructs the data source for listing
// developer invites of a workspace.
func NewWorkspaceInvitesDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_workspace_invites", workspaceInvitesSchema, readWorkspaceInvites)
}

type workspaceInvitesDataSourceModel struct {
	WorkspaceSlug     types.String  `tfsdk:"workspace_slug"`
	VerificationState types.String  `tfsdk:"verification_state"`
	ID                types.String  `tfsdk:"id"`
	Invites           []inviteModel `tfsdk:"invites"`
}

func workspaceInvitesSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the developer invites of a workspace via " +
			"`GET /workspaces/{workspaceSlug}/invites`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose invites are listed.",
			},
			"verification_state": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("EMAIL_PENDING", "EMAIL_SENT", "VERIFIED", "EXPIRED"),
				},
				MarkdownDescription: "Only list invites in this verification state (`verificationState` query parameter).",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"invites": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The invites found in the workspace.",
				NestedObject:        schema.NestedAttributeObject{Attributes: inviteAttributes()},
			},
		},
	}
}

func readWorkspaceInvites(ctx context.Context, client *nileapi.Client, data *workspaceInvitesDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	invites, err := client.ListDeveloperInvites(ctx, workspaceSlug, data.VerificationState.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing developer invites",
			fmt.Sprintf("Could not list invites of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Invites = inviteModelsFromAPI(invites)
}

// --- nile_regions ----------------------------------------------------------

// NewRegionsDataSource constructs the data source for the regions available
// to a workspace.
func NewRegionsDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_regions", regionsSchema, readRegions)
}

type regionsDataSourceModel struct {
	WorkspaceSlug types.String   `tfsdk:"workspace_slug"`
	ID            types.String   `tfsdk:"id"`
	Regions       []types.String `tfsdk:"regions"`
}

func regionsSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the region identifiers available to a workspace via " +
			"`GET /workspaces/{workspaceSlug}/regions`. Identifiers combine a cloud provider prefix " +
			"with a provider region, for example `AWS_US_WEST_2` or `AZURE_EASTUS`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose regions are listed.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"regions": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Region identifiers available to the workspace.",
			},
		},
	}
}

func readRegions(ctx context.Context, client *nileapi.Client, data *regionsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	regions, err := client.ListRegions(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing regions",
			fmt.Sprintf("Could not list regions of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}
	sort.Strings(regions)

	data.ID = types.StringValue(workspaceSlug)
	data.Regions = make([]types.String, 0, len(regions))
	for _, r := range regions {
		data.Regions = append(data.Regions, types.StringValue(r))
	}
}

// --- nile_compute_types ----------------------------------------------------

// NewComputeTypesDataSource constructs the data source for the dedicated
// compute types available to a workspace.
func NewComputeTypesDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_compute_types", computeTypesSchema, readComputeTypes)
}

type computeTypesDataSourceModel struct {
	WorkspaceSlug types.String       `tfsdk:"workspace_slug"`
	ID            types.String       `tfsdk:"id"`
	ComputeTypes  []computeTypeModel `tfsdk:"compute_types"`
}

func computeTypesSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the dedicated compute instance types available to a workspace via " +
			"`GET /workspaces/{workspaceSlug}/compute-types`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the workspace whose compute types are listed.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"compute_types": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The compute types available to the workspace.",
				NestedObject:        schema.NestedAttributeObject{Attributes: computeTypeAttributes()},
			},
		},
	}
}

func readComputeTypes(ctx context.Context, client *nileapi.Client, data *computeTypesDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	computeTypes, err := client.ListComputeTypes(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing compute types",
			fmt.Sprintf("Could not list compute types of workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.ComputeTypes = computeTypeModelsFromAPI(computeTypes)
}
