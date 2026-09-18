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

// --- nile_database ---------------------------------------------------------

// NewDatabaseDataSource constructs the data source for a single Nile database.
func NewDatabaseDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_database", databaseDataSourceSchema, readDatabase)
}

type databaseDataSourceModel struct {
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	Name          types.String `tfsdk:"name"`
	// The data source ID is the database's server-side identifier.
	ID         types.String `tfsdk:"id"`
	Status     types.String `tfsdk:"status"`
	Region     types.String `tfsdk:"region"`
	APIHost    types.String `tfsdk:"api_host"`
	DBHost     types.String `tfsdk:"db_host"`
	Expandable types.Bool   `tfsdk:"expandable"`
	Created    types.String `tfsdk:"created"`
	Deleted    types.String `tfsdk:"deleted"`
	ParentID   types.String `tfsdk:"parent_id"`
	ParentName types.String `tfsdk:"parent_name"`
	Raw        types.String `tfsdk:"raw_json"`
}

func databaseDataSourceSchema(_ context.Context) schema.Schema {
	attrs := databaseAttributes()
	attrs["workspace_slug"] = schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
		MarkdownDescription: "Slug of the Nile workspace that owns the database.",
	}
	attrs["name"] = schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
		MarkdownDescription: "Name of the database to look up.",
	}
	return schema.Schema{
		MarkdownDescription: "Fetches a single Nile database via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseName}`.",
		Attributes: attrs,
	}
}

func readDatabase(ctx context.Context, client *nileapi.Client, data *databaseDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	name := data.Name.ValueString()

	db, err := client.GetDatabase(ctx, workspaceSlug, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database",
			fmt.Sprintf("Could not read database %q in workspace %q: %s", name, workspaceSlug, err.Error()),
		)
		return
	}

	applyDatabaseDataSource(data, db)
}

func applyDatabaseDataSource(data *databaseDataSourceModel, db nileapi.Database) {
	m := databaseModelFromAPI(db)
	data.ID = m.ID
	data.Status = m.Status
	data.Region = m.Region
	data.APIHost = m.APIHost
	data.DBHost = m.DBHost
	data.Expandable = m.Expandable
	data.Created = m.Created
	data.Deleted = m.Deleted
	data.ParentID = m.ParentID
	data.ParentName = m.ParentName
	data.Raw = m.Raw
}

// --- nile_databases --------------------------------------------------------

// NewDatabasesDataSource constructs the data source for listing Nile
// databases of a workspace.
func NewDatabasesDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_databases", databasesDataSourceSchema, readDatabases)
}

type databasesDataSourceModel struct {
	WorkspaceSlug types.String    `tfsdk:"workspace_slug"`
	ID            types.String    `tfsdk:"id"`
	Databases     []databaseModel `tfsdk:"databases"`
}

func databasesDataSourceSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists all Nile databases in a workspace via " +
			"`GET /workspaces/{workspaceSlug}/databases`.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the Nile workspace whose databases are listed.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (the workspace slug).",
			},
			"databases": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The databases found in the workspace.",
				NestedObject:        schema.NestedAttributeObject{Attributes: databaseAttributes()},
			},
		},
	}
}

func readDatabases(ctx context.Context, client *nileapi.Client, data *databasesDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()

	databases, err := client.ListDatabases(ctx, workspaceSlug)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing databases",
			fmt.Sprintf("Could not list databases in workspace %q: %s", workspaceSlug, err.Error()),
		)
		return
	}

	data.ID = types.StringValue(workspaceSlug)
	data.Databases = databaseModelsFromAPI(databases)
}
