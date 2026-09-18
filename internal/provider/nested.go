// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// This file holds the model types and schema builders that several data
// sources and resources share. Keeping them in one place ensures that the
// same API entity is represented identically everywhere.

// --- workspace -------------------------------------------------------------

type workspaceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	StripeCustomerID types.String `tfsdk:"stripe_customer_id"`
	Created          types.String `tfsdk:"created"`
}

func workspaceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Workspace identifier (`id` in the API response).",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Workspace name.",
		},
		"slug": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Globally unique workspace slug used in API paths.",
		},
		"stripe_customer_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Stripe customer linked to the workspace, if any.",
		},
		"created": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creation timestamp.",
		},
	}
}

func workspaceModelFromAPI(w nileapi.Workspace) workspaceModel {
	return workspaceModel{
		ID:               stringOrNull(w.ID),
		Name:             stringOrNull(w.Name),
		Slug:             stringOrNull(w.Slug),
		StripeCustomerID: stringOrNull(w.StripeCustomerID),
		Created:          stringOrNull(w.Created),
	}
}

func workspaceModelsFromAPI(ws []nileapi.Workspace) []workspaceModel {
	out := make([]workspaceModel, 0, len(ws))
	for _, w := range ws {
		out = append(out, workspaceModelFromAPI(w))
	}
	return out
}

// --- database --------------------------------------------------------------

type databaseModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
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

func databaseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Database identifier (`id` in the API response).",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Database name.",
		},
		"status": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).",
		},
		"region": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Region the database runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).",
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
	}
}

func databaseModelFromAPI(db nileapi.Database) databaseModel {
	m := databaseModel{
		ID:         stringOrNull(db.ID),
		Name:       stringOrNull(db.Name),
		Status:     stringOrNull(db.Status),
		Region:     stringOrNull(db.Region),
		APIHost:    stringOrNull(db.APIHost),
		DBHost:     stringOrNull(db.DBHost),
		Expandable: types.BoolValue(db.Expandable),
		Created:    stringOrNull(db.Created),
		Deleted:    stringOrNull(db.Deleted),
		Raw:        types.StringValue(string(db.Raw)),
	}
	if db.Parent != nil {
		m.ParentID = stringOrNull(db.Parent.ID)
		m.ParentName = stringOrNull(db.Parent.Name)
	}
	return m
}

func databaseModelsFromAPI(dbs []nileapi.Database) []databaseModel {
	out := make([]databaseModel, 0, len(dbs))
	for _, db := range dbs {
		out = append(out, databaseModelFromAPI(db))
	}
	return out
}

// --- compute instance type -------------------------------------------------

type computeTypeModel struct {
	ID          types.String  `tfsdk:"id"`
	ComputeSize types.String  `tfsdk:"compute_size"`
	Memory      types.String  `tfsdk:"memory"`
	HourlyCost  types.Float64 `tfsdk:"hourly_cost"`
}

func computeTypeAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Compute type identifier.",
		},
		"compute_size": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "CPU size of the compute type (`computeSize` in the API response).",
		},
		"memory": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Memory of the compute type (for example `8GB`).",
		},
		"hourly_cost": schema.Float64Attribute{
			Computed:            true,
			MarkdownDescription: "Hourly cost of the compute type in USD.",
		},
	}
}

func computeTypeModelFromAPI(t nileapi.ComputeInstanceType) computeTypeModel {
	return computeTypeModel{
		ID:          stringOrNull(t.ID),
		ComputeSize: stringOrNull(t.ComputeSize),
		Memory:      stringOrNull(t.Memory),
		HourlyCost:  types.Float64Value(t.HourlyCost),
	}
}

func computeTypeModelsFromAPI(ts []nileapi.ComputeInstanceType) []computeTypeModel {
	out := make([]computeTypeModel, 0, len(ts))
	for _, t := range ts {
		out = append(out, computeTypeModelFromAPI(t))
	}
	return out
}

// --- credential ------------------------------------------------------------

type credentialModel struct {
	ID       types.String `tfsdk:"id"`
	Tenant   types.String `tfsdk:"tenant"`
	Internal types.Bool   `tfsdk:"internal"`
	Created  types.String `tfsdk:"created"`
	APIHost  types.String `tfsdk:"api_host"`
	DBHost   types.String `tfsdk:"db_host"`
	Raw      types.String `tfsdk:"raw_json"`
}

func credentialAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Credential identifier.",
		},
		"tenant": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Tenant the credential is scoped to (`tenant` in the API response).",
		},
		"internal": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether this is an internal credential.",
		},
		"created": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creation timestamp.",
		},
		"api_host": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Host of the database's API endpoint.",
		},
		"db_host": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Host of the database's PostgreSQL endpoint, if provisioned.",
		},
		"raw_json": schema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "Redacted JSON payload of the credential as returned by the API.",
		},
	}
}

func credentialModelFromAPI(c nileapi.Credential) credentialModel {
	m := credentialModel{
		ID:       stringOrNull(c.ID),
		Tenant:   stringOrNull(c.Tenant),
		Internal: types.BoolValue(c.Internal),
		Created:  stringOrNull(c.Created),
		Raw:      redactedRawJSON(c.Raw),
	}
	if c.Database != nil {
		m.APIHost = stringOrNull(c.Database.APIHost)
		m.DBHost = stringOrNull(c.Database.DBHost)
	}
	return m
}

func credentialModelsFromAPI(cs []nileapi.Credential) []credentialModel {
	out := make([]credentialModel, 0, len(cs))
	for _, c := range cs {
		out = append(out, credentialModelFromAPI(c))
	}
	return out
}

// --- developer invite ------------------------------------------------------

type inviteModel struct {
	ID                types.String `tfsdk:"id"`
	Email             types.String `tfsdk:"email"`
	VerificationState types.String `tfsdk:"verification_state"`
	SenderEmail       types.String `tfsdk:"sender_email"`
	Created           types.String `tfsdk:"created"`
	Updated           types.String `tfsdk:"updated"`
	Raw               types.String `tfsdk:"raw_json"`
}

func inviteAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Invite identifier.",
		},
		"email": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Email address the invite was sent to.",
		},
		"verification_state": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Verification state (`EMAIL_PENDING`, `EMAIL_SENT`, `VERIFIED` or `EXPIRED`).",
		},
		"sender_email": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Email address of the developer who sent the invite.",
		},
		"created": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Creation timestamp.",
		},
		"updated": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Last update timestamp.",
		},
		"raw_json": schema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "Redacted JSON payload of the invite as returned by the API.",
		},
	}
}

func inviteModelFromAPI(i nileapi.DeveloperInvite) inviteModel {
	m := inviteModel{
		ID:                stringOrNull(i.ID),
		Email:             stringOrNull(i.Email),
		VerificationState: stringOrNull(i.VerificationState),
		Created:           stringOrNull(i.Created),
		Updated:           stringOrNull(i.Updated),
		Raw:               redactedRawJSON(i.Raw),
	}
	if i.Sender != nil {
		m.SenderEmail = stringOrNull(i.Sender.Email)
	}
	return m
}

func inviteModelsFromAPI(invites []nileapi.DeveloperInvite) []inviteModel {
	out := make([]inviteModel, 0, len(invites))
	for _, i := range invites {
		out = append(out, inviteModelFromAPI(i))
	}
	return out
}

// --- subscription ----------------------------------------------------------

type subscriptionModel struct {
	Workspace            types.String `tfsdk:"workspace"`
	Level                types.String `tfsdk:"level"`
	ValidFrom            types.String `tfsdk:"valid_from"`
	ValidTo              types.String `tfsdk:"valid_to"`
	SubscriptionID       types.String `tfsdk:"subscription_id"`
	DefaultPaymentMethod types.String `tfsdk:"default_payment_method"`
	Raw                  types.String `tfsdk:"raw_json"`
}

func subscriptionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"workspace": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Workspace the subscription belongs to.",
		},
		"level": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Subscription level.",
		},
		"valid_from": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Start of the subscription period.",
		},
		"valid_to": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "End of the subscription period, if scheduled.",
		},
		"subscription_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Subscription identifier.",
		},
		"default_payment_method": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Default payment method of the subscription, if any.",
		},
		"raw_json": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Full, unparsed JSON payload of the subscription as returned by the API.",
		},
	}
}

func subscriptionModelFromAPI(s nileapi.WorkspaceSubscription) subscriptionModel {
	return subscriptionModel{
		Workspace:            stringOrNull(s.Workspace),
		Level:                stringOrNull(s.Level),
		ValidFrom:            stringOrNull(s.ValidFrom),
		ValidTo:              stringOrNull(s.ValidTo),
		SubscriptionID:       stringOrNull(s.SubscriptionID),
		DefaultPaymentMethod: stringOrNull(s.DefaultPaymentMethod),
		Raw:                  types.StringValue(string(s.Raw)),
	}
}

func subscriptionModelsFromAPI(ss []nileapi.WorkspaceSubscription) []subscriptionModel {
	out := make([]subscriptionModel, 0, len(ss))
	for _, s := range ss {
		out = append(out, subscriptionModelFromAPI(s))
	}
	return out
}

// --- developer -------------------------------------------------------------

type developerModel struct {
	ID         types.String     `tfsdk:"id"`
	Email      types.String     `tfsdk:"email"`
	Kind       types.String     `tfsdk:"kind"`
	Workspaces []workspaceModel `tfsdk:"workspaces"`
	Databases  []databaseModel  `tfsdk:"databases"`
}

func developerAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Developer identifier.",
		},
		"email": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Developer email address.",
		},
		"kind": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Developer kind (`HUMAN` or `API`).",
		},
		"workspaces": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Workspaces the developer has access to.",
			NestedObject:        schema.NestedAttributeObject{Attributes: workspaceAttributes()},
		},
		"databases": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Databases the developer has access to.",
			NestedObject:        schema.NestedAttributeObject{Attributes: databaseAttributes()},
		},
	}
}

func developerModelFromAPI(d nileapi.Developer) developerModel {
	return developerModel{
		ID:         stringOrNull(d.ID),
		Email:      stringOrNull(d.Email),
		Kind:       stringOrNull(d.Kind),
		Workspaces: workspaceModelsFromAPI(d.Workspaces),
		Databases:  databaseModelsFromAPI(d.Databases),
	}
}
