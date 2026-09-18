// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// monthRegexp matches a YYYY-MM month.
var monthRegexp = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// NewDeveloperDataSource constructs the data source for the authenticated
// developer.
func NewDeveloperDataSource() datasource.DataSource {
	return newReadOnlyDataSource("nile_developer", developerDataSourceSchema, readDeveloper)
}

type developerDataSourceModel struct {
	ID         types.String     `tfsdk:"id"`
	Email      types.String     `tfsdk:"email"`
	Kind       types.String     `tfsdk:"kind"`
	Workspaces []workspaceModel `tfsdk:"workspaces"`
	Databases  []databaseModel  `tfsdk:"databases"`
	Raw        types.String     `tfsdk:"raw_json"`
}

func developerDataSourceSchema(_ context.Context) schema.Schema {
	attrs := developerAttributes()
	attrs["raw_json"] = schema.StringAttribute{
		Computed:            true,
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
