// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// NewDeveloperDataSource constructs the data source for the authenticated
// developer.
func NewDeveloperDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_developer", developerDataSourceSchema, readDeveloper)
}

// developerDataSourceModel holds the Terraform state of the nile_developer
// data source for the authenticated developer.
type developerDataSourceModel struct {
	// ID is the developer identifier.
	ID types.String `tfsdk:"id"`
	// Email is the developer's login email address.
	Email types.String `tfsdk:"email"`
	// Kind is the developer account kind as reported by the API.
	Kind types.String `tfsdk:"kind"`
	// Workspaces are the workspaces associated with the developer.
	Workspaces []workspaceModel `tfsdk:"workspaces"`
	// Databases are the databases associated with the developer.
	Databases []databaseModel `tfsdk:"databases"`
	// Raw is the redacted raw JSON payload returned by the API.
	Raw types.String `tfsdk:"raw_json"`
}

// developerDataSourceSchema builds the schema of the nile_developer data
// source.
func developerDataSourceSchema(_ context.Context) schema.Schema {
	attrs := developerAttributes()
	attrs["raw_json"] = schema.StringAttribute{
		Computed:            true,
		Sensitive:           true,
		MarkdownDescription: "Redacted JSON payload of the developer as returned by the API.",
	}
	attrs["id"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Developer identifier (`id` in the API response).",
	}
	return schema.Schema{
		MarkdownDescription: "Returns the developer associated with the configured API token via " +
			"`GET /developers/me`.",
		Attributes: attrs,
	}
}

// readDeveloper fetches the authenticated developer via GET /developers/me
// and fills the data source model from the response.
func readDeveloper(ctx context.Context, client *nileapi.Client, data *developerDataSourceModel, resp *datasource.ReadResponse) {
	dev, err := client.GetDeveloper(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading developer",
			fmt.Sprintf("Could not read the authenticated developer: %s", err.Error()),
		)
		return
	}

	m := developerModelFromAPI(dev)
	data.ID = m.ID
	data.Email = m.Email
	data.Kind = m.Kind
	data.Workspaces = m.Workspaces
	data.Databases = m.Databases
	data.Raw = redactedRawJSON(dev.Raw)
}
