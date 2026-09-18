// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBillingSchemas(t *testing.T) {
	readiness := dataSourceSchemaOf(t, NewWorkspaceBillingReadinessDataSource())
	for _, name := range []string{"workspace_slug", "id", "workspace", "workspace_id", "stripe_customer_id", "default_payment_method_id", "status", "checked_at", "last_error", "source", "detail", "raw_json"} {
		if _, ok := readiness.Attributes[name]; !ok {
			t.Errorf("nile_workspace_billing_readiness: missing attribute %q", name)
		}
	}

	totals := dataSourceSchemaOf(t, NewWorkspaceBillingTotalsDataSource())
	for _, name := range []string{"workspace_slug", "month", "id", "ym", "totals", "raw_json"} {
		if _, ok := totals.Attributes[name]; !ok {
			t.Errorf("nile_workspace_billing_totals: missing attribute %q", name)
		}
	}
	if attr := totals.Attributes["month"]; attr == nil || !attr.IsRequired() {
		t.Error("month must be required on nile_workspace_billing_totals")
	}
}

func TestReadWorkspaceBillingReadinessModel(t *testing.T) {
	client := testAPIServer(t, `{"workspace":"acme","workspaceId":"ws-1","stripeCustomerId":"cus_1","defaultPaymentMethodId":"pm_1","status":"ready","checkedAt":"2025-06-01T00:00:00Z","source":"stripe","detail":"all set"}`)
	data := &workspaceBillingReadinessDataSourceModel{WorkspaceSlug: types.StringValue("acme")}

	var resp datasource.ReadResponse
	readWorkspaceBillingReadiness(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if data.WorkspaceID.ValueString() != "ws-1" || data.StripeCustomerID.ValueString() != "cus_1" ||
		data.DefaultPaymentMethod.ValueString() != "pm_1" || data.Status.ValueString() != "ready" ||
		data.CheckedAt.ValueString() != "2025-06-01T00:00:00Z" ||
		data.Source.ValueString() != "stripe" || data.Detail.ValueString() != "all set" {
		t.Errorf("data = %+v", data)
	}
	// lastError is absent from the payload and must map to null.
	if !data.LastError.IsNull() {
		t.Errorf("last_error = %q, want null", data.LastError)
	}
}

func TestReadWorkspaceBillingTotalsModel(t *testing.T) {
	client := testAPIServer(t, `{"ym":"2025-06","totals":{"compute":1.5,"storage":0.25}}`)
	data := &workspaceBillingTotalsDataSourceModel{
		WorkspaceSlug: types.StringValue("acme"),
		Month:         types.StringValue("2025-06"),
	}

	var resp datasource.ReadResponse
	readWorkspaceBillingTotals(context.Background(), client, data, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("diagnostics: %v", resp.Diagnostics)
	}
	if data.ID.ValueString() != "acme/2025-06" {
		t.Errorf("id = %q", data.ID.ValueString())
	}
	if data.YM.ValueString() != "2025-06" {
		t.Errorf("ym = %q", data.YM.ValueString())
	}
	var totals map[string]float64
	if diags := data.Totals.ElementsAs(context.Background(), &totals, false); diags.HasError() {
		t.Fatalf("totals diagnostics: %v", diags)
	}
	if totals["compute"] != 1.5 || totals["storage"] != 0.25 {
		t.Errorf("totals = %v", totals)
	}
}
