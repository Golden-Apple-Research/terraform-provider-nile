// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestDatabaseResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewDatabaseResource())
	for _, name := range []string{"workspace_slug", "name", "region", "id", "status", "raw_json", "timeouts"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
	for _, name := range []string{"workspace_slug", "name", "region"} {
		if attr := sch.Attributes[name]; attr == nil || !attr.IsRequired() {
			t.Errorf("%s must be required", name)
		}
	}
}

func TestDatabaseResourceCreateTimeoutExpires(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		// The database never leaves PENDING, so only the create timeout can
		// end the wait.
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"PENDING","region":"AWS_US_WEST_2"}`))
	})
	// Safety net: if the timeouts wiring regresses, the client's own wait
	// bound fails the test quickly instead of hanging for DefaultWaitTimeout.
	client.WaitTimeout = 2 * time.Second
	r := configuredDatabaseResource(t, client)

	start := time.Now()
	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_US_WEST_2"),
			"timeouts":       timeoutsAttr(map[string]string{"create": "5ms", "update": ""}),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create timeout expires")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("create aborted after %v; the 5ms timeout was not honored", elapsed)
	}
}

// databaseAPIServer routes requests by method: POST creates, PUT renames,
// GET describes, DELETE deletes. The handler bodies are canned.
func databaseAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, calls int)) *nileapi.Client {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, int(calls.Add(1)))
	}))
	t.Cleanup(srv.Close)
	client, err := nileapi.NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.PollInterval = time.Millisecond
	return client
}

func configuredDatabaseResource(t *testing.T, client *nileapi.Client) *databaseResource {
	t.Helper()
	r := NewDatabaseResource().(*databaseResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func TestDatabaseResourceCreateReadyImmediately(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, calls int) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY","region":"AWS_US_WEST_2","apiHost":"api.example"}`))
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_US_WEST_2"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "id"); got != "db-1" {
		t.Errorf("id = %q", got)
	}
	if got := stateString(t, resp.State, "status"); got != "READY" {
		t.Errorf("status = %q", got)
	}
}

func TestDatabaseResourceCreateWaitsForReady(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, calls int) {
		if r.Method == http.MethodPost {
			// The create response is not ready yet: the resource must poll.
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"PENDING","region":"AWS_US_WEST_2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY","region":"AWS_US_WEST_2"}`))
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_US_WEST_2"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "status"); got != "READY" {
		t.Errorf("status = %q, want READY after polling", got)
	}
}

func TestDatabaseResourceReadRemovesMissing(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"entity_not_found","statusCode":404}`))
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, r)}
	r.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"id":             stringAttr("db-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestDatabaseResourceUpdateRenames(t *testing.T) {
	var gotBody string
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"new-name","status":"READY","region":"AWS_US_WEST_2"}`))
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, r)}
	r.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("new-name"),
			"region":         stringAttr("AWS_US_WEST_2"),
		}),
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("old-name"),
			"region":         stringAttr("AWS_US_WEST_2"),
			"id":             stringAttr("db-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
	if gotBody != `{"name":"new-name"}` {
		t.Errorf("rename body = %q", gotBody)
	}
	if got := stateString(t, resp.State, "name"); got != "new-name" {
		t.Errorf("name = %q", got)
	}
}

func TestDatabaseResourceUpdateSameNameSkipsAPI(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		t.Errorf("no API call expected, got %s %s", r.Method, r.URL.Path)
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, r)}
	r.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_US_WEST_2"),
		}),
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_US_WEST_2"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
}

func TestDatabaseResourceDelete(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"READY"}`))
	})
	r := configuredDatabaseResource(t, client)

	resp := resource.DeleteResponse{State: crudResponseState(t, r)}
	r.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"id":             stringAttr("db-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestApplyDatabaseResourcePreservesOmittedFields(t *testing.T) {
	model := &databaseResourceModel{
		Expandable: types.BoolValue(true),
		Created:    types.StringValue("created"),
		ParentID:   types.StringValue("parent-id"),
		ParentName: types.StringValue("parent"),
	}
	db := nileapi.Database{
		ID:   "db-1",
		Name: "app",
		Raw:  json.RawMessage(`{"id":"db-1","name":"app","status":"READY"}`),
	}
	applyDatabaseResource(model, db, "ws", "app")
	if !model.Expandable.ValueBool() || model.Created.ValueString() != "created" ||
		model.ParentID.ValueString() != "parent-id" || model.ParentName.ValueString() != "parent" {
		t.Fatalf("partial response cleared state: %+v", model)
	}
}
