// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestDeveloperDataSourceSchema(t *testing.T) {
	sch := dataSourceSchemaOf(t, NewDeveloperDataSource())
	for _, name := range []string{"id", "email", "kind", "workspaces", "databases", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("nile_developer: missing attribute %q", name)
		}
	}
}

func TestReadDeveloperModel(t *testing.T) {
	client := testAPIServer(t, `{"id":"dev-1","email":"a@example.com","kind":"HUMAN","workspaces":[{"slug":"acme"}],"databases":[{"id":"db-1","name":"app"}]}`)
	data := &developerDataSourceModel{}

	var resp datasource.ReadResponse
	readDeveloper(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "dev-1" || data.Email.ValueString() != "a@example.com" {
		t.Errorf("developer = %+v", data)
	}
	if len(data.Workspaces) != 1 || data.Workspaces[0].Slug.ValueString() != "acme" {
		t.Errorf("workspaces = %+v", data.Workspaces)
	}
	if len(data.Databases) != 1 || data.Databases[0].Name.ValueString() != "app" {
		t.Errorf("databases = %+v", data.Databases)
	}
	if data.Raw.ValueString() == "" {
		t.Error("raw_json must preserve the payload")
	}
}
