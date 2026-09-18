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
	for _, name := range []string{"workspace_slug", "region"} {
		if attr := sch.Attributes[name]; attr == nil || !attr.IsRequired() {
			t.Errorf("%s must be required", name)
		}
	}
	// name is optional (a claimed database gets its name server-side) but
	// computed so it always shows up in state.
	if attr := sch.Attributes["name"]; attr == nil || !attr.IsOptional() || !attr.IsComputed() {
		t.Error("name must be optional and computed")
	}
	if attr := sch.Attributes["claim_code"]; attr == nil || !attr.IsOptional() || !attr.IsSensitive() {
		t.Error("claim_code must be optional and sensitive")
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
			"timeouts":       timeoutsAttr(map[string]string{"create": "5ms", "update": "", "delete": ""}),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create timeout expires")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("create aborted after %v; the 5ms timeout was not honored", elapsed)
	}
	// The API already created the database; it must be in state so the next
	// apply can refresh or destroy it instead of orphaning it.
	if got := stateString(t, resp.State, "id"); got != "db-1" {
		t.Errorf("id = %q, want the created database recorded in state", got)
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
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, calls int) {
		switch {
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"DELETING"}`))
		case r.Method == http.MethodGet && calls >= 2:
			// The database has disappeared after the delete.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"entity_not_found","statusCode":404}`))
		default:
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"DELETING"}`))
		}
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

func TestDatabaseResourceDeleteTimeoutExpires(t *testing.T) {
	client := databaseAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		switch r.Method {
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"DELETING"}`))
		default:
			// The database never disappears, so only the delete timeout can
			// end the wait.
			_, _ = w.Write([]byte(`{"id":"db-1","name":"app","status":"DELETING"}`))
		}
	})
	client.WaitTimeout = 2 * time.Second
	r := configuredDatabaseResource(t, client)

	start := time.Now()
	resp := resource.DeleteResponse{State: crudResponseState(t, r)}
	r.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"id":             stringAttr("db-1"),
			"timeouts":       timeoutsAttr(map[string]string{"create": "", "update": "", "delete": "5ms"}),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the delete timeout expires")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("delete aborted after %v; the 5ms timeout was not honored", elapsed)
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

func TestDatabaseResourceCreateFromClaimCode(t *testing.T) {
	var method, path, body string
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		_, _ = w.Write([]byte(`{"id":"db-77","name":"claimed_db_1","status":"READY","region":"AWS_EU_CENTRAL_1"}`))
	})
	r := NewDatabaseResource().(*databaseResource)
	var cResp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &cResp)
	if cResp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", cResp.Diagnostics)
	}

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"region":         stringAttr("AWS_EU_CENTRAL_1"),
			"claim_code":     stringAttr("claim-xyz"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if method != http.MethodPost || path != "/workspaces/ws/databases/claim" || body != `{"claimCode":"claim-xyz"}` {
		t.Errorf("request = %s %s %s", method, path, body)
	}
	if got := stateString(t, resp.State, "name"); got != "claimed_db_1" {
		t.Errorf("name = %q", got)
	}
	if got := stateString(t, resp.State, "id"); got != "db-77" {
		t.Errorf("id = %q", got)
	}
}

func TestDatabaseResourceCreateRejectsNameAndClaimCode(t *testing.T) {
	client := credentialAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("must not call the API, got %s %s", r.Method, r.URL.Path)
	})
	r := NewDatabaseResource().(*databaseResource)
	var cResp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &cResp)
	if cResp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", cResp.Diagnostics)
	}

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"name":           stringAttr("app"),
			"region":         stringAttr("AWS_EU_CENTRAL_1"),
			"claim_code":     stringAttr("claim-xyz"),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when name and claim_code are combined")
	}
}
