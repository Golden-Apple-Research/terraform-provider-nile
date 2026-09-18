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

	"github.com/mal-2/terraform-provider-nile/internal/nileapi"
)

// Ensure the data source satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &computeInstancesDataSource{}
	_ datasource.DataSourceWithConfigure = &computeInstancesDataSource{}
)

// NewDatabaseComputeInstancesDataSource constructs the data source for
// listing dedicated compute instances of a Nile database.
func NewDatabaseComputeInstancesDataSource() datasource.DataSource {
	return &computeInstancesDataSource{}
}

type computeInstancesDataSource struct {
	client *nileapi.Client
}

type computeInstanceModel struct {
	ID        types.String `tfsdk:"id"`
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

func (d *computeInstancesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_compute_instances"
}

func (d *computeInstancesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the dedicated compute instances attached to a Nile database, via " +
			"`GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute`. " +
			"Optionally restricted to instances active within a time window (`start`/`end`).",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the Nile workspace that owns the database.",
			},
			"database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the database whose compute instances are listed.",
			},
			"start": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "RFC3339 timestamp marking the start of a time window to search for active instances.",
			},
			"end": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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
							MarkdownDescription: "Instance identifier, if reported by the API.",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance status, if reported by the API.",
						},
						"size": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance size/tier, if reported by the API.",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance region/location, if reported by the API.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Instance creation timestamp, if reported by the API.",
						},
						"raw_json": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Full, unparsed JSON payload of the instance as returned by the API.",
						},
					},
				},
			},
		},
	}
}

func (d *computeInstancesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*nileapi.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *nileapi.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *computeInstancesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data computeInstancesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := data.WorkspaceSlug.ValueString()
	databaseName := data.DatabaseName.ValueString()

	// Validate the optional time-window parameters early so the user gets
	// schema-level feedback instead of an opaque upstream 4xx.
	for name, v := range map[string]types.String{"start": data.Start, "end": data.End} {
		if !v.IsNull() && !v.IsUnknown() {
			if _, err := time.Parse(time.RFC3339, v.ValueString()); err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root(name),
					fmt.Sprintf("Invalid %q value", name),
					fmt.Sprintf("%q must be an RFC3339 timestamp, got: %q", name, v.ValueString()),
				)
			}
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	instances, err := d.client.ListComputeInstances(
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

	data.ID = types.StringValue(workspaceSlug + "/" + databaseName)
	data.Instances = make([]computeInstanceModel, 0, len(instances))
	for _, ci := range instances {
		data.Instances = append(data.Instances, computeInstanceModel{
			ID:        stringOrNull(ci.ID),
			Status:    stringOrNull(ci.Status),
			Size:      stringOrNull(ci.Size),
			Region:    stringOrNull(ci.Region),
			CreatedAt: stringOrNull(ci.CreatedAt),
			Raw:       types.StringValue(string(ci.Raw)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
