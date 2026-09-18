// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

// assertAttributesCoverModel fails when the tfsdk tags of a model do not
// exactly match the keys of its attribute map, in either direction.
func assertAttributesCoverModel(t *testing.T, attrs map[string]schema.Attribute, model any) {
	t.Helper()
	rt := reflect.TypeOf(model)
	want := make(map[string]bool, rt.NumField())
	for i := range rt.NumField() {
		tag, _, _ := strings.Cut(rt.Field(i).Tag.Get("tfsdk"), ",")
		if tag != "" {
			want[tag] = true
		}
	}
	got := make(map[string]bool, len(attrs))
	for name := range attrs {
		got[name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("%T: attribute %q missing from schema map", model, name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("%T: schema attribute %q has no model field", model, name)
		}
	}
}

func TestNestedAttributesCoverModels(t *testing.T) {
	assertAttributesCoverModel(t, workspaceAttributes(), workspaceModel{})
	assertAttributesCoverModel(t, databaseAttributes(), databaseModel{})
	assertAttributesCoverModel(t, computeTypeAttributes(), computeTypeModel{})
	assertAttributesCoverModel(t, credentialAttributes(), credentialModel{})
	assertAttributesCoverModel(t, inviteAttributes(), inviteModel{})
	assertAttributesCoverModel(t, subscriptionAttributes(), subscriptionModel{})
}

func TestWorkspaceModelFromAPI(t *testing.T) {
	m := workspaceModelFromAPI(nileapi.Workspace{
		ID: "ws-1", Name: "Acme", Slug: "acme",
		StripeCustomerID: "cus_1", Created: "2025-01-01T00:00:00Z",
	})
	if m.ID.ValueString() != "ws-1" || m.Name.ValueString() != "Acme" ||
		m.Slug.ValueString() != "acme" || m.StripeCustomerID.ValueString() != "cus_1" ||
		m.Created.ValueString() != "2025-01-01T00:00:00Z" {
		t.Errorf("model = %+v", m)
	}

	empty := workspaceModelFromAPI(nileapi.Workspace{})
	if !empty.ID.IsNull() || !empty.StripeCustomerID.IsNull() {
		t.Errorf("empty workspace should map to nulls, got %+v", empty)
	}

	if got := workspaceModelsFromAPI(nil); len(got) != 0 {
		t.Errorf("nil slice should map to empty slice, got %d", len(got))
	}
	if got := workspaceModelsFromAPI([]nileapi.Workspace{{ID: "a"}, {ID: "b"}}); len(got) != 2 || got[1].ID.ValueString() != "b" {
		t.Errorf("models = %+v", got)
	}
}

func TestDatabaseModelFromAPI(t *testing.T) {
	db := nileapi.Database{
		ID: "db-1", Name: "app", Status: "READY", Region: "AWS_US_WEST_2",
		Expandable: true, Created: "2025-06-01T00:00:00Z",
		APIHost: "api.example", DBHost: "db.example",
		Raw: json.RawMessage(`{"id":"db-1"}`),
	}
	m := databaseModelFromAPI(db)
	if m.ID.ValueString() != "db-1" || m.Status.ValueString() != "READY" ||
		m.Region.ValueString() != "AWS_US_WEST_2" || !m.Expandable.ValueBool() ||
		m.APIHost.ValueString() != "api.example" || m.DBHost.ValueString() != "db.example" {
		t.Errorf("model = %+v", m)
	}
	if m.Raw.ValueString() != `{"id":"db-1"}` {
		t.Errorf("raw_json = %q", m.Raw.ValueString())
	}
	// Without a parent, the parent attributes are null, not empty strings.
	if !m.ParentID.IsNull() || !m.ParentName.IsNull() {
		t.Errorf("parent = %q/%q, want nulls", m.ParentID, m.ParentName)
	}

	replica := databaseModelFromAPI(nileapi.Database{
		Parent: &nileapi.DatabaseParent{ID: "db-1", Name: "app"},
	})
	if replica.ParentID.ValueString() != "db-1" || replica.ParentName.ValueString() != "app" {
		t.Errorf("parent = %q/%q", replica.ParentID, replica.ParentName)
	}

	if got := databaseModelsFromAPI([]nileapi.Database{{ID: "db-1"}}); len(got) != 1 || got[0].ID.ValueString() != "db-1" {
		t.Errorf("models = %+v", got)
	}
}

func TestComputeTypeModelFromAPI(t *testing.T) {
	m := computeTypeModelFromAPI(nileapi.ComputeInstanceType{
		ID: "tier-1", ComputeSize: "large", Memory: "8GB", HourlyCost: 0.42,
	})
	if m.ID.ValueString() != "tier-1" || m.ComputeSize.ValueString() != "large" ||
		m.Memory.ValueString() != "8GB" || m.HourlyCost.ValueFloat64() != 0.42 {
		t.Errorf("model = %+v", m)
	}
	if got := computeTypeModelsFromAPI(nil); len(got) != 0 {
		t.Errorf("nil slice should map to empty slice, got %d", len(got))
	}
}

func TestCredentialModelFromAPIRedactsSecrets(t *testing.T) {
	c := nileapi.Credential{
		ID: "cred-1", Tenant: "acme", Internal: true, Created: "2025-06-01T00:00:00Z",
		Database: &nileapi.Database{APIHost: "api.example", DBHost: "db.example"},
		Raw:      json.RawMessage(`{"id":"cred-1","password":"s3cret"}`),
	}
	m := credentialModelFromAPI(c)
	if m.ID.ValueString() != "cred-1" || m.Tenant.ValueString() != "acme" || !m.Internal.ValueBool() {
		t.Errorf("model = %+v", m)
	}
	if m.APIHost.ValueString() != "api.example" || m.DBHost.ValueString() != "db.example" {
		t.Errorf("hosts = %q/%q", m.APIHost, m.DBHost)
	}
	if raw := m.Raw.ValueString(); strings.Contains(raw, "s3cret") || !strings.Contains(raw, "[REDACTED]") {
		t.Errorf("raw_json = %s, want password redacted", raw)
	}

	// Without an embedded database, hosts stay null.
	bare := credentialModelFromAPI(nileapi.Credential{ID: "cred-2"})
	if !bare.APIHost.IsNull() || !bare.DBHost.IsNull() {
		t.Errorf("hosts = %q/%q, want nulls", bare.APIHost, bare.DBHost)
	}
}

func TestInviteModelFromAPI(t *testing.T) {
	i := nileapi.DeveloperInvite{
		ID: "inv-1", Email: "b@example.com", VerificationState: "EMAIL_SENT",
		Created: "2025-06-01T00:00:00Z", Updated: "2025-06-02T00:00:00Z",
		Sender: &nileapi.Developer{Email: "a@example.com"},
		Raw:    json.RawMessage(`{"id":"inv-1","code":"invite-code"}`),
	}
	m := inviteModelFromAPI(i)
	if m.ID.ValueString() != "inv-1" || m.Email.ValueString() != "b@example.com" ||
		m.VerificationState.ValueString() != "EMAIL_SENT" || m.SenderEmail.ValueString() != "a@example.com" {
		t.Errorf("model = %+v", m)
	}
	// The invite code is a secret and must not survive in raw_json.
	if raw := m.Raw.ValueString(); strings.Contains(raw, "invite-code") {
		t.Errorf("raw_json = %s, want code redacted", raw)
	}

	// Without a sender, sender_email is null.
	bare := inviteModelFromAPI(nileapi.DeveloperInvite{ID: "inv-2"})
	if !bare.SenderEmail.IsNull() {
		t.Errorf("sender_email = %q, want null", bare.SenderEmail)
	}

	if got := inviteModelsFromAPI([]nileapi.DeveloperInvite{{ID: "inv-1"}}); len(got) != 1 {
		t.Errorf("models = %+v", got)
	}
}

func TestSubscriptionModelFromAPI(t *testing.T) {
	m := subscriptionModelFromAPI(nileapi.WorkspaceSubscription{
		Workspace: "acme", Level: "paid", ValidFrom: "2025-01-01T00:00:00Z",
		SubscriptionID: "sub_1", DefaultPaymentMethod: "pm_1",
		Raw: json.RawMessage(`{"level":"paid"}`),
	})
	if m.Workspace.ValueString() != "acme" || m.Level.ValueString() != "paid" ||
		m.ValidFrom.ValueString() != "2025-01-01T00:00:00Z" ||
		m.SubscriptionID.ValueString() != "sub_1" || m.DefaultPaymentMethod.ValueString() != "pm_1" {
		t.Errorf("model = %+v", m)
	}
	if !m.ValidTo.IsNull() {
		t.Errorf("valid_to = %q, want null", m.ValidTo)
	}
	if got := subscriptionModelsFromAPI(nil); len(got) != 0 {
		t.Errorf("nil slice should map to empty slice, got %d", len(got))
	}
}

func TestDeveloperModelFromAPI(t *testing.T) {
	m := developerModelFromAPI(nileapi.Developer{
		ID: "dev-1", Email: "a@example.com", Kind: "HUMAN",
		Workspaces: []nileapi.Workspace{{Slug: "acme"}},
		Databases:  []nileapi.Database{{ID: "db-1", Name: "app"}},
	})
	if m.ID.ValueString() != "dev-1" || m.Email.ValueString() != "a@example.com" || m.Kind.ValueString() != "HUMAN" {
		t.Errorf("model = %+v", m)
	}
	if len(m.Workspaces) != 1 || m.Workspaces[0].Slug.ValueString() != "acme" {
		t.Errorf("workspaces = %+v", m.Workspaces)
	}
	if len(m.Databases) != 1 || m.Databases[0].Name.ValueString() != "app" {
		t.Errorf("databases = %+v", m.Databases)
	}

	// Missing relations map to empty lists so they serialize as [].
	empty := developerModelFromAPI(nileapi.Developer{ID: "dev-2"})
	if len(empty.Workspaces) != 0 || len(empty.Databases) != 0 {
		t.Errorf("empty relations = %+v", empty)
	}
}
