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

// NewDatabaseCredentialsDataSource constructs the data source for listing
// database credentials.
func NewDatabaseCredentialsDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_database_credentials",
		databaseCredentialsSchema,
		readDatabaseCredentials,
	)
}

type databaseCredentialsDataSourceModel struct {
	WorkspaceSlug types.String      `tfsdk:"workspace_slug"`
	DatabaseName  types.String      `tfsdk:"database_name"`
	TenantID      types.String      `tfsdk:"tenant_id"`
	Internal      types.Bool        `tfsdk:"internal"`
	ID            types.String      `tfsdk:"id"`
	Credentials   []credentialModel `tfsdk:"credentials"`
}

func databaseCredentialsSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the credentials of a Nile database via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseName}/credentials`. " +
			"Passwords are **not** included: the API returns them only once, when a credential is created.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the Nile workspace that owns the database.",
			},
			"database_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Name of the database whose credentials are listed.",
			},
			"tenant_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only list credentials scoped to this tenant (`tenantId` query parameter).",
			},
			"internal": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Only list credentials with this internal flag (`internal` query parameter).",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (`<workspaceSlug>/<databaseName>`).",
			},
			"credentials": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The credentials found for the database.",
				NestedObject:        schema.NestedAttributeObject{Attributes: credentialAttributes()},
			},
		},
	}
}

func readDatabaseCredentials(ctx context.Context, client *nileapi.Client, data *databaseCredentialsDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	databaseName := data.DatabaseName.ValueString()

	credentials, err := client.ListCredentials(
		ctx, workspaceSlug, databaseName,
		data.TenantID.ValueString(), optionalBool(data.Internal),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing database credentials",
			fmt.Sprintf("Could not list credentials of database %q in workspace %q: %s",
				databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(dataSourceID(workspaceSlug, databaseName))
	data.Credentials = credentialModelsFromAPI(credentials)
}
