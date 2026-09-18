// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSubscriptionSchemas(t *testing.T) {
	current := dataSourceSchemaOf(t, NewWorkspaceSubscriptionDataSource())
	for _, name := range []string{"workspace_slug", "id", "level", "valid_from", "valid_to", "subscription_id", "default_payment_method", "raw_json"} {
		if _, ok := current.Attributes[name]; !ok {
			t.Errorf("nile_workspace_subscription: missing attribute %q", name)
		}
	}

	history := dataSourceSchemaOf(t, NewWorkspaceSubscriptionHistoryDataSource())
	for _, name := range []string{"workspace_slug", "id", "subscriptions"} {
		if _, ok := history.Attributes[name]; !ok {
			t.Errorf("nile_workspace_subscription_history: missing attribute %q", name)
		}
	}
}

func TestReadWorkspaceSubscriptionModel(t *testing.T) {
	client := testAPIServer(t, `{"workspace":"acme","level":"paid","validFrom":"2025-01-01T00:00:00Z","validTo":"","subscriptionId":"sub_1","defaultPaymentMethod":"pm_1"}`)
	data := &workspaceSubscriptionDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceSubscription(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if data.Level.ValueString() != "paid" || data.SubscriptionID.ValueString() != "sub_1" ||
		data.ValidFrom.ValueString() != "2025-01-01T00:00:00Z" ||
		data.DefaultPaymentMethod.ValueString() != "pm_1" {
		t.Errorf("data = %+v", data)
	}
	// An empty validTo in the API maps to null, not to "".
	if !data.ValidTo.IsNull() {
		t.Errorf("valid_to = %q, want null", data.ValidTo)
	}
	if !strings.Contains(data.Raw.ValueString(), `"level":"paid"`) {
		t.Errorf("raw_json = %q", data.Raw.ValueString())
	}
}

func TestReadWorkspaceSubscriptionHistoryModel(t *testing.T) {
	client := testAPIServer(t, `[
		{"workspace":"acme","level":"paid","subscriptionId":"sub_2","validFrom":"2025-06-01T00:00:00Z"},
		{"workspace":"acme","level":"free","subscriptionId":"sub_1","validFrom":"2025-01-01T00:00:00Z"}
	]`)
	data := &workspaceSubscriptionHistoryDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceSubscriptionHistory(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if len(data.Subscriptions) != 2 {
		t.Fatalf("subscriptions = %d, want 2", len(data.Subscriptions))
	}
	if data.Subscriptions[0].Level.ValueString() != "paid" ||
		data.Subscriptions[1].SubscriptionID.ValueString() != "sub_1" {
		t.Errorf("subscriptions = %+v", data.Subscriptions)
	}
}
