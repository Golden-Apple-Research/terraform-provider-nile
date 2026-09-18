// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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
	_ resource.Resource                = &computeInstanceResource{}
	_ resource.ResourceWithConfigure   = &computeInstanceResource{}
	_ resource.ResourceWithImportState = &computeInstanceResource{}
)

// NewComputeInstanceResource constructs the nile_database_compute_instance
// resource.
func NewComputeInstanceResource() resource.Resource {
	return &computeInstanceResource{}
}

type computeInstanceResource struct {
	client *nileapi.Client
}

type computeInstanceResourceModel struct {
	WorkspaceSlug types.String   `tfsdk:"workspace_slug"`
	DatabaseName  types.String   `tfsdk:"database_name"`
	InstanceName  types.String   `tfsdk:"instance_name"`
	InstanceSize  types.String   `tfsdk:"instance_size"`
	ID            types.String   `tfsdk:"id"`
	Status        types.String   `tfsdk:"status"`
	Region        types.String   `tfsdk:"region"`
	Memory        types.String   `tfsdk:"memory"`
	HourlyCost    types.Float64  `tfsdk:"hourly_cost"`
	CreatedAt     types.String   `tfsdk:"created_at"`
	UpdatedAt     types.String   `tfsdk:"updated_at"`
	Raw           types.String   `tfsdk:"raw_json"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (r *computeInstanceResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "nile_database_compute_instance"
}

func (r *computeInstanceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a dedicated compute instance of a Nile database via " +
			"`/workspaces/{workspaceSlug}/databases/{databaseName}/compute`. Renaming and resizing " +
			"update the instance in place; creating, resizing and deleting are asynchronous and the " +
			"resource waits for the instance to settle.",
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
			"database_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Name of the database the instance belongs to. Changing it forces replacement.",
			},
			"instance_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Name of the compute instance. Renaming updates the instance in place.",
			},
			"instance_size": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Compute size of the instance. Use `nile_compute_types` to discover the " +
					"sizes available to a workspace. Resizing updates the instance in place.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance identifier (`instanceId` in the API response).",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance status (`PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED` or `TERMINATED`).",
			},
			"region": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Region the instance runs in.",
			},
			"memory": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Memory of the instance's current type (`instanceType.memory` in the API response).",
			},
			"hourly_cost": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Hourly cost of the instance's current type in USD (`instanceType.hourlyCost` in the API response).",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance creation timestamp (`created` in the API response).",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp of the last instance update (`updated` in the API response).",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Redacted JSON payload of the instance as returned by the API.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *computeInstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *computeInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan computeInstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	// Creating is asynchronous; bound the readiness wait with the configured
	// create timeout (default: nileapi.DefaultWaitTimeout).
	createTimeout, diags := plan.Timeouts.Create(ctx, nileapi.DefaultWaitTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	workspaceSlug := plan.WorkspaceSlug.ValueString()
	databaseName := plan.DatabaseName.ValueString()
	name := plan.InstanceName.ValueString()

	instance, err := r.client.CreateComputeInstance(ctx, workspaceSlug, databaseName, name, plan.InstanceSize.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating compute instance",
			fmt.Sprintf("Could not create compute instance %q in database %q of workspace %q: %s",
				name, databaseName, workspaceSlug, err.Error()),
		)
		return
	}
	if instance.ID == "" {
		resp.Diagnostics.AddError(
			"API returned no instance identifier",
			fmt.Sprintf("The create response for compute instance %q in database %q did not contain an instanceId. "+
				"The request may have succeeded; check the Nile dashboard before retrying.", name, databaseName),
		)
		return
	}

	if !instance.Ready() {
		instance, err = r.client.WaitForComputeInstanceReady(ctx, workspaceSlug, databaseName, instance.ID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for compute instance",
				fmt.Sprintf("Compute instance %q in database %q was created but did not become ready: %s",
					name, databaseName, err.Error()),
			)
			return
		}
	}

	applyComputeInstanceResource(&plan, instance, workspaceSlug, databaseName, name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *computeInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state computeInstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	workspaceSlug := state.WorkspaceSlug.ValueString()
	databaseName := state.DatabaseName.ValueString()
	instanceID := state.ID.ValueString()

	instance, err := r.client.DescribeComputeInstance(ctx, workspaceSlug, databaseName, instanceID)
	if nileapi.IsNotFound(err) {
		tflog.Warn(ctx, "compute instance no longer exists; removing it from state", map[string]any{
			"workspace": workspaceSlug,
			"database":  databaseName,
			"instance":  instanceID,
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading compute instance",
			fmt.Sprintf("Could not read compute instance %q in database %q of workspace %q: %s",
				instanceID, databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	applyComputeInstanceResource(&state, instance, workspaceSlug, databaseName, state.InstanceName.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *computeInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state computeInstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	// Resizes settle asynchronously; bound the wait with the configured
	// update timeout (default: nileapi.DefaultWaitTimeout).
	updateTimeout, diags := plan.Timeouts.Update(ctx, nileapi.DefaultWaitTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	workspaceSlug := state.WorkspaceSlug.ValueString()
	databaseName := state.DatabaseName.ValueString()
	instanceID := state.ID.ValueString()

	err := r.client.UpdateComputeInstance(ctx, workspaceSlug, databaseName, instanceID, nileapi.UpdateComputeInstanceRequest{
		InstanceName: plan.InstanceName.ValueString(),
		InstanceSize: plan.InstanceSize.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating compute instance",
			fmt.Sprintf("Could not update compute instance %q in database %q of workspace %q: %s",
				instanceID, databaseName, workspaceSlug, err.Error()),
		)
		return
	}

	// Renames complete quickly, resizes take longer; wait for the instance to
	// settle before reporting success.
	instance, err := r.client.WaitForComputeInstanceReady(ctx, workspaceSlug, databaseName, instanceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for compute instance update",
			fmt.Sprintf("Compute instance %q in database %q did not settle after the update: %s",
				instanceID, databaseName, err.Error()),
		)
		return
	}

	applyComputeInstanceResource(&plan, instance, workspaceSlug, databaseName, plan.InstanceName.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *computeInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state computeInstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider client is nil; was Configure called?")
		return
	}

	// Deletion settles asynchronously too; bound the wait with the configured
	// delete timeout (default: nileapi.DefaultWaitTimeout).
	deleteTimeout, diags := state.Timeouts.Delete(ctx, nileapi.DefaultWaitTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	workspaceSlug := state.WorkspaceSlug.ValueString()
	databaseName := state.DatabaseName.ValueString()
	instanceID := state.ID.ValueString()

	err := r.client.DeleteComputeInstance(ctx, workspaceSlug, databaseName, instanceID)
	if err != nil && !nileapi.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error deleting compute instance",
			fmt.Sprintf("Could not delete compute instance %q in database %q of workspace %q: %s",
				instanceID, databaseName, workspaceSlug, err.Error()),
		)
		return
	}
	if err == nil {
		if err := r.client.WaitForComputeInstanceDeleted(ctx, workspaceSlug, databaseName, instanceID); err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for compute instance deletion",
				fmt.Sprintf("Compute instance %q in database %q was deleted but did not disappear: %s",
					instanceID, databaseName, err.Error()),
			)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *computeInstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitResourceID(req.ID, 3)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Import a compute instance as `workspace_slug/database_name/instance_id`, got %q: %s",
				req.ID, err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_slug"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

// applyComputeInstanceResource merges an API instance into the model. Fields
// the API omitted keep their previous value.
func applyComputeInstanceResource(m *computeInstanceResourceModel, instance nileapi.ComputeInstance, workspaceSlug, databaseName, fallbackName string) {
	m.WorkspaceSlug = types.StringValue(workspaceSlug)
	m.DatabaseName = types.StringValue(databaseName)
	if instance.Name != "" {
		m.InstanceName = types.StringValue(instance.Name)
	} else {
		m.InstanceName = types.StringValue(fallbackName)
	}
	if instance.ID != "" {
		m.ID = types.StringValue(instance.ID)
	}
	if instance.Size != "" {
		m.InstanceSize = types.StringValue(instance.Size)
	}
	if instance.Status != "" {
		m.Status = types.StringValue(instance.Status)
	}
	if instance.Region != "" {
		m.Region = types.StringValue(instance.Region)
	}
	if instance.Memory != "" {
		m.Memory = types.StringValue(instance.Memory)
	}
	if instance.InstanceType != nil {
		m.HourlyCost = types.Float64Value(instance.HourlyCost)
	}
	if instance.CreatedAt != "" {
		m.CreatedAt = types.StringValue(instance.CreatedAt)
	}
	if instance.Updated != "" {
		m.UpdatedAt = types.StringValue(instance.Updated)
	}
	if len(instance.Raw) > 0 {
		m.Raw = redactedRawJSON(instance.Raw)
	}
}
