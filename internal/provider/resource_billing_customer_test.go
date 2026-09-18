// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestBillingCustomerResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewBillingCustomerResource())
	for _, name := range []string{"workspace_slug", "stripe_customer_id", "default_payment_method", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

func billingAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *nileapi.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func configuredBillingCustomerResource(t *testing.T, client *nileapi.Client) *billingCustomerResource {
	t.Helper()
	r := NewBillingCustomerResource().(*billingCustomerResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func TestBillingCustomerResourceCreateEnsures(t *testing.T) {
	var method, path string
	client := billingAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"workspace":"ws","stripeCustomerId":"cus_42","defaultPaymentMethod":"pm_42"}`))
	})
	res := configuredBillingCustomerResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if method != http.MethodPut || path != "/workspaces/ws/billing/customer" {
		t.Errorf("request = %s %s", method, path)
	}
	if got := stateString(t, resp.State, "stripe_customer_id"); got != "cus_42" {
		t.Errorf("stripe_customer_id = %q", got)
	}
	if got := stateString(t, resp.State, "default_payment_method"); got != "pm_42" {
		t.Errorf("default_payment_method = %q", got)
	}
}

func TestBillingCustomerResourceDeleteIsStateOnly(t *testing.T) {
	client := billingAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete must not call the API, got %s %s", r.Method, r.URL.Path)
	})
	res := configuredBillingCustomerResource(t, client)

	resp := resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":     stringAttr("ws"),
			"stripe_customer_id": stringAttr("cus_42"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
}
