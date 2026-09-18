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

func TestWorkspaceResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewWorkspaceResource())
	for _, name := range []string{"name", "id", "slug", "created", "stripe_customer_id", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

func workspaceAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *nileapi.Client {
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

func configuredWorkspaceResource(t *testing.T, client *nileapi.Client) *workspaceResource {
	t.Helper()
	r := NewWorkspaceResource().(*workspaceResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func TestWorkspaceResourceCreate(t *testing.T) {
	var method, path, body string
	client := workspaceAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		_, _ = w.Write([]byte(`{"id":"ws-9","name":"Acme","slug":"acme","created":"2026-02-01T00:00:00Z","stripe_customer_id":"cus_9"}`))
	})
	res := configuredWorkspaceResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{"name": stringAttr("Acme")}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if method != http.MethodPost || path != "/workspaces" || body != `{"name":"Acme"}` {
		t.Errorf("request = %s %s %s", method, path, body)
	}
	if got := stateString(t, resp.State, "slug"); got != "acme" {
		t.Errorf("slug = %q", got)
	}
	if got := stateString(t, resp.State, "stripe_customer_id"); got != "cus_9" {
		t.Errorf("stripe_customer_id = %q", got)
	}
}

func TestWorkspaceResourceReadRemovesMissing(t *testing.T) {
	client := workspaceAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"entity_not_found","message":"no such workspace","statusCode":404}`))
	})
	res := configuredWorkspaceResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"name": stringAttr("Acme"),
			"slug": stringAttr("acme"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("expected removed state, got %v", resp.State.Raw)
	}
}

func TestWorkspaceResourceDeleteIsStateOnly(t *testing.T) {
	client := workspaceAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete must not call the API, got %s %s", r.Method, r.URL.Path)
	})
	res := configuredWorkspaceResource(t, client)

	resp := resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"name": stringAttr("Acme"),
			"slug": stringAttr("acme"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
}
