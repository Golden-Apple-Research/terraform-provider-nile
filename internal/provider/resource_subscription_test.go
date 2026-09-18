// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestWorkspaceSubscriptionResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewWorkspaceSubscriptionResource())
	for _, name := range []string{"workspace_slug", "level", "subscription_id", "valid_from", "valid_to", "workspace"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

func subscriptionAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *nileapi.Client {
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

func configuredSubscriptionResource(t *testing.T, client *nileapi.Client) *workspaceSubscriptionResource {
	t.Helper()
	r := NewWorkspaceSubscriptionResource().(*workspaceSubscriptionResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func TestWorkspaceSubscriptionResourceCreate(t *testing.T) {
	var startMethod, startPath, startBody string
	client := subscriptionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/subscription"):
			startMethod, startPath = r.Method, r.URL.Path
			body, _ := io.ReadAll(r.Body)
			startBody = string(body)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/subscription"):
			_, _ = w.Write([]byte(`{"workspace":"ws","level":"paid","subscriptionId":"sub-7","validFrom":"2026-02-01T00:00:00Z","validTo":"2027-02-01T00:00:00Z"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	res := configuredSubscriptionResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"level":          stringAttr("paid"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if startMethod != http.MethodPost || startPath != "/workspaces/ws/subscription" || startBody != `{"level":"paid"}` {
		t.Errorf("start request = %s %s %s", startMethod, startPath, startBody)
	}
	if got := stateString(t, resp.State, "subscription_id"); got != "sub-7" {
		t.Errorf("subscription_id = %q", got)
	}
	if got := stateString(t, resp.State, "level"); got != "paid" {
		t.Errorf("level = %q", got)
	}
}

func TestWorkspaceSubscriptionResourceUpdateChangesLevel(t *testing.T) {
	var changeMethod, changeBody string
	client := subscriptionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			changeMethod = r.Method
			body, _ := io.ReadAll(r.Body)
			changeBody = string(body)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"workspace":"ws","level":"enterprise","subscriptionId":"sub-7","validFrom":"2026-02-01T00:00:00Z"}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	res := configuredSubscriptionResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, res)}
	res.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"level":          stringAttr("enterprise"),
		}),
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":  stringAttr("ws"),
			"level":           stringAttr("paid"),
			"subscription_id": stringAttr("sub-7"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
	if changeMethod != http.MethodPut || changeBody != `{"level":"enterprise"}` {
		t.Errorf("change request = %s %s", changeMethod, changeBody)
	}
	if got := stateString(t, resp.State, "level"); got != "enterprise" {
		t.Errorf("level = %q", got)
	}
}

func TestWorkspaceSubscriptionResourceDeleteCloses(t *testing.T) {
	var deletePath string
	client := subscriptionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletePath = r.URL.Path
			_, _ = w.Write([]byte(`{}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	})
	res := configuredSubscriptionResource(t, client)

	resp := resource.DeleteResponse{}
	res.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug":  stringAttr("ws"),
			"level":           stringAttr("paid"),
			"subscription_id": stringAttr("sub-7"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
	if deletePath != "/workspaces/ws/subscription/sub-7" {
		t.Errorf("close path = %q", deletePath)
	}
}
