// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

var (
	_ resource.Resource                = &databaseResource{}
	_ resource.ResourceWithConfigure   = &databaseResource{}
	_ resource.ResourceWithImportState = &databaseResource{}
)

// NewDatabaseResource constructs the nile_database resource.
func NewDatabaseResource() resource.Resource {
	return &databaseResource{}
}

type databaseResource struct {
	client *nileapi.Client
}

type databaseResourceModel struct {
	WorkspaceSlug types.String `tfsdk:"workspace_slug"`
	Name          types.String `tfsdk:"name"`
	Region        types.String `tfsdk:"region"`
	ID            types.String `tfsdk:"id"`
	Status        types.String `tfsdk:"status"`
	APIHost       types.String `tfsdk:"api_host"`
	DBHost        types.String `tfsdk:"db_host"`
	Expandable    types.Bool   `tfsdk:"expandable"`
	Created       types.String `tfsdk:"created"`
	Deleted       types.String `tfsdk:"deleted"`
	ParentID      types.String `tfsdk:"parent_id"`
	ParentName    types.String `tfsdk:"parent_name"`
	Raw           types.String `tfsdk:"raw_json"`
}

func (r *databaseResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_database"
}

func (r *databaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nile database via `/workspaces/{workspaceSlug}/databases`. " +
			"Creating a database is asynchronous; the resource waits until the database reports `READY` " +
			"before it completes.",
		Attributes: map[string]schema.Attribute{
			"workspace_slug": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Slug of the Nile workspace that owns the database. Changing it forces replacement.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Database name. Renaming updates the database in place via " +
					"`PUT /workspaces/{workspaceSlug}/databases/{databaseName}`.",
			},
			"region": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Region the database runs in (for example `AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or " +
					"`AZURE_EASTUS`). Changing it forces replacement.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Database identifier (`id` in the API response).",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).",
			},
			"api_host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Host of the database's API endpoint.",
			},
			"db_host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Host of the database's PostgreSQL endpoint, if provisioned.",
			},
			"expandable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the database can be expanded with read replicas (`expandable` in the API response).",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
			},
			"deleted": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp at which the database was marked for deletion, if any.",
			},
			"parent_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the parent (primary) database for read replicas.",
			},
			"parent_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the parent (primary) database for read replicas.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full, unparsed JSON payload of the database as returned by the API.",
			},
		},
	}
}

func (r *databaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*nileapi.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *nileapi.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *databaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	name := plan.Name.ValueString()

	db, err := r.client.CreateDatabase(ctx, workspaceSlug, nileapi.CreateDatabaseRequest{
		DatabaseName: name,
		Region:       plan.Region.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating database",
			fmt.Sprintf("Could not create database %q in workspace %q: %s", name, workspaceSlug, err.Error()),
		)
		return
	}

	// The create response may already be usable; otherwise wait until the
	// database is ready so dependent resources (credentials, compute) can be
	// created right away.
	if !db.Ready() || db.ID == "" {
		db, err = r.client.WaitForDatabaseReady(ctx, workspaceSlug, name)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for database",
				fmt.Sprintf("Database %q in workspace %q was created but did not become ready: %s",
					name, workspaceSlug, err.Error()),
			)
			return
		}
	}
	if db.ID == "" {
		resp.Diagnostics.AddError(
			"API returned no database identifier",
			fmt.Sprintf("The create response for database %q in workspace %q did not contain an id.", name, workspaceSlug),
		)
		return
	}

	applyDatabaseResource(&plan, db, workspaceSlug, name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	name := state.Name.ValueString()

	db, err := r.client.GetDatabase(ctx, workspaceSlug, name)
	if nileapi.IsNotFound(err) {
		tflog.Warn(ctx, "database no longer exists; removing it from state", map[string]any{
			"workspace": workspaceSlug,
			"database":  name,
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading database",
			fmt.Sprintf("Could not read database %q in workspace %q: %s", name, workspaceSlug, err.Error()),
		)
		return
	}

	applyDatabaseResource(&state, db, workspaceSlug, name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state databaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	// Every other attribute is marked RequiresReplace, so a rename is the
	// only in-place update there is.
	if oldName == newName {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	db, err := r.client.RenameDatabase(ctx, workspaceSlug, oldName, newName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error renaming database",
			fmt.Sprintf("Could not rename database %q to %q in workspace %q: %s",
				oldName, newName, workspaceSlug, err.Error()),
		)
		return
	}
	if !db.Ready() || db.ID == "" {
		db, err = r.client.WaitForDatabaseReady(ctx, workspaceSlug, newName)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for renamed database",
				fmt.Sprintf("Database %q in workspace %q was renamed to %q but did not become ready: %s",
					oldName, workspaceSlug, newName, err.Error()),
			)
			return
		}
	}
	if db.ID == "" {
		resp.Diagnostics.AddError(
			"API returned no database identifier",
			fmt.Sprintf("The rename response for database %q in workspace %q did not contain an id.", newName, workspaceSlug),
		)
		return
	}

	applyDatabaseResource(&plan, db, workspaceSlug, newName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	name := state.Name.ValueString()

	_, err := r.client.DeleteDatabase(ctx, workspaceSlug, name)
	if err != nil && !nileapi.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error deleting database",
			fmt.Sprintf("Could not delete database %q in workspace %q: %s", name, workspaceSlug, err.Error()),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitResourceID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Import a database as `workspace_slug/database_name`, got %q: %s", req.ID, err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_slug"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	// The state ID is replaced with the server-side identifier by the read
	// that follows the import.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// applyDatabaseResource merges an API database into the model. Fields the API
// omitted keep their previous value, so a partial response never clears state.
func applyDatabaseResource(m *databaseResourceModel, db nileapi.Database, workspaceSlug, fallbackName string) {
	m.WorkspaceSlug = types.StringValue(workspaceSlug)
	if db.Name != "" {
		m.Name = types.StringValue(db.Name)
	} else {
		m.Name = types.StringValue(fallbackName)
	}
	if db.ID != "" {
		m.ID = types.StringValue(db.ID)
	} else {
		m.ID = types.StringValue(joinResourceID(workspaceSlug, m.Name.ValueString()))
	}
	if db.Region != "" {
		m.Region = types.StringValue(db.Region)
	}

	api := databaseModelFromAPI(db)
	if db.Status != "" {
		m.Status = api.Status
	} else if m.Status.IsUnknown() {
		m.Status = types.StringNull()
	}
	if db.APIHost != "" {
		m.APIHost = api.APIHost
	} else if m.APIHost.IsUnknown() {
		m.APIHost = types.StringNull()
	}
	if db.DBHost != "" {
		m.DBHost = api.DBHost
	} else if m.DBHost.IsUnknown() {
		m.DBHost = types.StringNull()
	}
	if apiFieldPresent(db.Raw, "expandable") {
		m.Expandable = api.Expandable
	} else if m.Expandable.IsUnknown() {
		m.Expandable = types.BoolNull()
	}
	if apiFieldPresent(db.Raw, "created") {
		m.Created = api.Created
	} else if m.Created.IsUnknown() {
		m.Created = types.StringNull()
	}
	if apiFieldPresent(db.Raw, "deleted") {
		m.Deleted = api.Deleted
	} else if m.Deleted.IsUnknown() {
		m.Deleted = types.StringNull()
	}
	if apiFieldPresent(db.Raw, "parent") {
		m.ParentID = api.ParentID
		m.ParentName = api.ParentName
	} else {
		if m.ParentID.IsUnknown() {
			m.ParentID = types.StringNull()
		}
		if m.ParentName.IsUnknown() {
			m.ParentName = types.StringNull()
		}
	}
	if len(db.Raw) > 0 {
		m.Raw = api.Raw
	} else if m.Raw.IsUnknown() {
		m.Raw = types.StringNull()
	}
}
