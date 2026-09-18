// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkspaceSchemas(t *testing.T) {
	single := dataSourceSchemaOf(t, NewWorkspaceDataSource())
	for _, name := range []string{"slug", "id", "name", "created"} {
		if _, ok := single.Attributes[name]; !ok {
			t.Errorf("nile_workspace: missing attribute %q", name)
		}
	}
	if attr := single.Attributes["slug"]; attr == nil || !attr.IsRequired() {
		t.Error("slug must be required on nile_workspace")
	}

	list := dataSourceSchemaOf(t, NewWorkspacesDataSource())
	if _, ok := list.Attributes["workspaces"]; !ok {
		t.Error("nile_workspaces: missing attribute workspaces")
	}

	developers := dataSourceSchemaOf(t, NewWorkspaceDevelopersDataSource())
	for _, name := range []string{"workspace_slug", "id", "developers"} {
		if _, ok := developers.Attributes[name]; !ok {
			t.Errorf("nile_workspace_developers: missing attribute %q", name)
		}
	}
}

func TestReadWorkspaceModel(t *testing.T) {
	// GetWorkspace answers with the workspace list filtered by slug.
	client := testAPIServer(t, `[{"id":"ws-1","name":"Acme","slug":"acme","stripe_customer_id":"cus_1","created":"2025-01-01T00:00:00Z"}]`)
	data := &workspaceDataSourceModel{Slug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspace(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "ws-1" || data.Name.ValueString() != "Acme" ||
		data.StripeCustomerID.ValueString() != "cus_1" {
		t.Errorf("data = %+v", data)
	}
}

func TestReadWorkspacesModel(t *testing.T) {
	client := testAPIServer(t, `[{"id":"ws-1","name":"Acme","slug":"acme"},{"id":"ws-2","name":"Beta","slug":"beta"}]`)
	data := &workspacesDataSourceModel{}

	var resp datasource.ReadResponse
	readWorkspaces(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "workspaces" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Workspaces) != 2 || data.Workspaces[1].Slug.ValueString() != "beta" {
		t.Errorf("workspaces = %+v", data.Workspaces)
	}
}

func TestReadWorkspaceDevelopersModel(t *testing.T) {
	client := testAPIServer(t, `[{"id":"dev-1","email":"a@example.com","kind":"HUMAN","workspaces":[{"slug":"acme"}],"databases":[{"id":"db-1","name":"app"}]}]`)
	data := &workspaceDevelopersDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceDevelopers(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Developers) != 1 {
		t.Fatalf("developers = %d, want 1", len(data.Developers))
	}
	dev := data.Developers[0]
	if dev.Email.ValueString() != "a@example.com" || dev.Kind.ValueString() != "HUMAN" {
		t.Errorf("developer = %+v", dev)
	}
	if len(dev.Workspaces) != 1 || dev.Workspaces[0].Slug.ValueString() != "acme" {
		t.Errorf("workspaces = %+v", dev.Workspaces)
	}
	if len(dev.Databases) != 1 || dev.Databases[0].Name.ValueString() != "app" {
		t.Errorf("databases = %+v", dev.Databases)
	}
}
