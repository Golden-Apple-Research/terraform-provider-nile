// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestInvitesSchemas(t *testing.T) {
	invites := dataSourceSchemaOf(t, NewWorkspaceInvitesDataSource())
	for _, name := range []string{"workspace_slug", "verification_state", "id", "invites"} {
		if _, ok := invites.Attributes[name]; !ok {
			t.Errorf("nile_workspace_invites: missing attribute %q", name)
		}
	}

	regions := dataSourceSchemaOf(t, NewRegionsDataSource())
	for _, name := range []string{"workspace_slug", "id", "regions"} {
		if _, ok := regions.Attributes[name]; !ok {
			t.Errorf("nile_regions: missing attribute %q", name)
		}
	}

	computeTypes := dataSourceSchemaOf(t, NewComputeTypesDataSource())
	for _, name := range []string{"workspace_slug", "id", "compute_types"} {
		if _, ok := computeTypes.Attributes[name]; !ok {
			t.Errorf("nile_compute_types: missing attribute %q", name)
		}
	}
}

func TestReadWorkspaceInvitesModel(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"inv-1","email":"b@example.com","verificationState":"EMAIL_SENT","code":"secret-code","created":"2025-06-01T00:00:00Z"}]`))
	}))
	defer srv.Close()
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	data := &workspaceInvitesDataSourceModel{
		WorkspaceSlug:     types.StringValue("acme"),
		VerificationState: types.StringValue("EMAIL_SENT"),
	}

	var resp datasource.ReadResponse
	readWorkspaceInvites(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if gotQuery.Get("verificationState") != "EMAIL_SENT" {
		t.Errorf("query = %v", gotQuery)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Invites) != 1 {
		t.Fatalf("invites = %d, want 1", len(data.Invites))
	}
	invite := data.Invites[0]
	if invite.ID.ValueString() != "inv-1" || invite.VerificationState.ValueString() != "EMAIL_SENT" {
		t.Errorf("invite = %+v", invite)
	}
	// Invite codes are secrets; the redacted raw payload must not carry them.
	if raw := invite.Raw.ValueString(); !strings.Contains(raw, "[REDACTED]") || strings.Contains(raw, "secret-code") {
		t.Errorf("raw_json = %s, want code redacted", raw)
	}
}

func TestReadRegionsIsSorted(t *testing.T) {
	client := testAPIServer(t, `["AZURE_EASTUS","AWS_US_WEST_2"]`)
	data := &regionsDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readRegions(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if len(data.Regions) != 2 || data.Regions[0].ValueString() != "AWS_US_WEST_2" || data.Regions[1].ValueString() != "AZURE_EASTUS" {
		t.Errorf("regions = %v", data.Regions)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
}

func TestReadComputeTypesModel(t *testing.T) {
	client := testAPIServer(t, `[{"id":"tier-1","computeSize":"large","memory":"8GB","hourlyCost":0.42},{"id":"tier-2","computeSize":"xlarge","memory":"16GB","hourlyCost":0.84}]`)
	data := &computeTypesDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readComputeTypes(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.ComputeTypes) != 2 {
		t.Fatalf("compute types = %d, want 2", len(data.ComputeTypes))
	}
	first := data.ComputeTypes[0]
	if first.ComputeSize.ValueString() != "large" || first.Memory.ValueString() != "8GB" || first.HourlyCost.ValueFloat64() != 0.42 {
		t.Errorf("first = %+v", first)
	}
}
