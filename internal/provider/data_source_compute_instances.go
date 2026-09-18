// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// computeInstancesDataSource is the generic data source implementation for
// nile_database_compute_instances; the alias keeps the concrete name usable in
// tests.
type computeInstancesDataSource = readOnlyDataSource[computeInstancesDataSourceModel]

// NewDatabaseComputeInstancesDataSource constructs the data source for
// listing dedicated compute instances of a Nile database.
func NewDatabaseComputeInstancesDataSource() datasource.DataSource {
	return newReadOnlyDataSource(
		"nile_database_compute_instances",
		computeInstancesSchema,
		readComputeInstances,
	)
}

type computeInstanceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Status    types.String `tfsdk:"status"`
	Size      types.String `tfsdk:"size"`
	Region    types.String `tfsdk:"region"`
	CreatedAt types.String `tfsdk:"created_at"`
	Raw       types.String `tfsdk:"raw_json"`
}

type computeInstancesDataSourceModel struct {
	WorkspaceSlug types.String           `tfsdk:"workspace_slug"`
	DatabaseName  types.String           `tfsdk:"database_name"`
	Start         types.String           `tfsdk:"start"`
	End           types.String           `tfsdk:"end"`
	ID            types.String           `tfsdk:"id"` // data-source id
	Instances     []computeInstanceModel `tfsdk:"instances"`
}

func computeInstancesSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Lists the dedicated compute instances attached to a Nile database, via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute`. " +
			"Optionally restricted to instances active within a time window (`start`/`end`).",
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
				MarkdownDescription: "Name of the database whose compute instances are listed.",
			},
			"start": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					// LengthAtLeast(1) is subsumed: the RFC3339 parser rejects
					// the empty string, and valid timestamps are never empty.
					isRFC3339Validator{},
				},
				MarkdownDescription: "RFC3339 timestamp marking the start of a time window to search for active instances.",
			},
			"end": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					isRFC3339Validator{},
				},
				MarkdownDescription: "RFC3339 timestamp marking the end of a time window to search for active instances.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable identifier of this data source instance (`<workspaceSlug>/<databaseName>`).",
			},
			"instances": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The dedicated compute instances found for the database.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance identifier (`instanceId` in the API response).",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance name (`instanceName` in the API response).",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance status (`PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED` or `TERMINATED`).",
						},
						"size": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Compute size of the instance's current type (`instanceType.computeSize` in the API response).",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Region the instance runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance creation timestamp (`created` in the API response).",
						},
						"raw_json": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Redacted JSON payload of the instance as returned by the API.",
						},
					},
				},
			},
		},
	}
}

func readComputeInstances(ctx context.Context, client *nileapi.Client, data *computeInstancesDataSourceModel, resp *datasource.ReadResponse) {
	workspaceSlug := data.WorkspaceSlug.ValueString()
	databaseName := data.DatabaseName.ValueString()

	// Schema-level validators guarantee RFC3339 syntax for start/end; the
	// coherence of the window itself is checked here (data-source schemas
	// have no cross-attribute validators in the framework). Unknown values
	// are deferred: they cannot be ordered before they are known.
	if !data.Start.IsNull() && !data.Start.IsUnknown() &&
		!data.End.IsNull() && !data.End.IsUnknown() {
		start, errStart := time.Parse(time.RFC3339, data.Start.ValueString())
		end, errEnd := time.Parse(time.RFC3339, data.End.ValueString())
		if errStart == nil && errEnd == nil && start.After(end) {
			resp.Diagnostics.AddAttributeError(
				path.Root("start"),
				"Invalid time window",
				fmt.Sprintf("The time window is inverted: start %q is after end %q. "+
					"Set start to a timestamp earlier than or equal to end.",
					data.Start.ValueString(), data.End.ValueString()),
			)
			return
		}
	}

	instances, err := client.ListComputeInstances(
		ctx, workspaceSlug, databaseName,
		data.Start.ValueString(), data.End.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing compute instances",
			fmt.Sprintf("Could not list compute instances for database %q in workspace %q: %s",
				databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "listed compute instances", map[string]any{
		"workspace": workspaceSlug, "database": databaseName, "count": len(instances),
	})

	data.ID = types.StringValue(dataSourceID(workspaceSlug, databaseName))
	data.Instances = make([]computeInstanceModel, 0, len(instances))
	for _, ci := range instances {
		data.Instances = append(data.Instances, computeInstanceModel{
			ID:        stringOrNull(ci.ID),
			Name:      stringOrNull(ci.Name),
			Status:    stringOrNull(ci.Status),
			Size:      stringOrNull(ci.Size),
			Region:    stringOrNull(ci.Region),
			CreatedAt: stringOrNull(ci.CreatedAt),
			Raw:       redactedRawJSON(ci.Raw),
		})
	}
}
