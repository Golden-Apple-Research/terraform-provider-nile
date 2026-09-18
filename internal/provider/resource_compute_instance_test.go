// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestComputeInstanceResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewComputeInstanceResource())
	for _, name := range []string{"workspace_slug", "database_name", "instance_name", "instance_size", "id", "status", "memory", "hourly_cost", "raw_json", "timeouts"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
}

// computeAPIServer routes by method: POST creates, PUT updates, GET describes,
// DELETE deletes; calls counts requests in arrival order.
func computeAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, calls int)) *nileapi.Client {
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

func configuredComputeInstanceResource(t *testing.T, client *nileapi.Client) *computeInstanceResource {
	t.Helper()
	r := NewComputeInstanceResource().(*computeInstanceResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func computePlan(r resource.Resource) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"workspace_slug": stringAttr("ws"),
		"database_name":  stringAttr("db"),
		"instance_name":  stringAttr("primary"),
		"instance_size":  stringAttr("large"),
	}
}

func TestComputeInstanceResourceCreateReadyImmediately(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write([]byte(`{"instanceId":"inst-1","instanceName":"primary","instanceType":{"computeSize":"large","memory":"8GB","hourlyCost":0.42},"status":"READY","region":"AWS_US_WEST_2","created":"2025-06-01T00:00:00Z"}`))
	})
	r := configuredComputeInstanceResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, r, computePlan(r))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "id"); got != "inst-1" {
		t.Errorf("id = %q", got)
	}
	if got := stateString(t, resp.State, "memory"); got != "8GB" {
		t.Errorf("memory = %q", got)
	}
	var cost types.Float64
	if diags := resp.State.GetAttribute(context.Background(), path.Root("hourly_cost"), &cost); diags.HasError() {
		t.Fatalf("GetAttribute: %v", diags)
	}
	if cost.ValueFloat64() != 0.42 {
		t.Errorf("hourly_cost = %v", cost)
	}
}

func TestComputeInstanceResourceCreateWaitsForReady(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"PROVISIONING"}`))
			return
		}
		_, _ = w.Write([]byte(`{"instanceId":"inst-1","instanceName":"primary","status":"READY","region":"AWS_US_WEST_2"}`))
	})
	r := configuredComputeInstanceResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, r, computePlan(r))}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "status"); got != "READY" {
		t.Errorf("status = %q, want READY after polling", got)
	}
}

func TestComputeInstanceResourceCreateWithoutID(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		_, _ = w.Write([]byte(`{}`))
	})
	r := configuredComputeInstanceResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, r, computePlan(r))}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create response has no instanceId")
	}
}

func TestComputeInstanceResourceUpdate(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		// PUT settles the update; the wait that follows reads via GET.
		_, _ = w.Write([]byte(`{"instanceId":"inst-1","instanceName":"renamed","instanceType":{"computeSize":"xlarge"},"status":"READY"}`))
	})
	r := configuredComputeInstanceResource(t, client)

	resp := resource.UpdateResponse{State: crudResponseState(t, r)}
	r.Update(context.Background(), resource.UpdateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"instance_name":  stringAttr("renamed"),
			"instance_size":  stringAttr("xlarge"),
			"id":             stringAttr("inst-1"),
		}),
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"instance_name":  stringAttr("primary"),
			"instance_size":  stringAttr("large"),
			"id":             stringAttr("inst-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "instance_name"); got != "renamed" {
		t.Errorf("instance_name = %q", got)
	}
	if got := stateString(t, resp.State, "instance_size"); got != "xlarge" {
		t.Errorf("instance_size = %q", got)
	}
}

func TestComputeInstanceResourceDeleteWaitsForGone(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, calls int) {
		switch {
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"DELETING"}`))
		case r.Method == http.MethodGet && calls >= 2:
			// The instance has disappeared after the delete.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorCode":"entity_not_found","statusCode":404}`))
		default:
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"DELETING"}`))
		}
	})
	r := configuredComputeInstanceResource(t, client)

	resp := resource.DeleteResponse{State: crudResponseState(t, r)}
	r.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"id":             stringAttr("inst-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestComputeInstanceResourceCreateTimeoutExpires(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		// The instance never becomes READY, so only the create timeout can
		// end the wait.
		_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"PROVISIONING"}`))
	})
	// Safety net: if the timeouts wiring regresses, the client's own wait
	// bound fails the test quickly instead of hanging for DefaultWaitTimeout.
	client.WaitTimeout = 2 * time.Second
	r := configuredComputeInstanceResource(t, client)

	start := time.Now()
	resp := resource.CreateResponse{State: crudResponseState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"instance_name":  stringAttr("primary"),
			"instance_size":  stringAttr("large"),
			"timeouts":       timeoutsAttr(map[string]string{"create": "5ms", "update": "", "delete": ""}),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the create timeout expires")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("create aborted after %v; the 5ms timeout was not honored", elapsed)
	}
}

func TestComputeInstanceResourceDeleteTimeoutExpires(t *testing.T) {
	client := computeAPIServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		switch r.Method {
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"DELETING"}`))
		default:
			// The instance never disappears, so only the delete timeout can
			// end the wait.
			_, _ = w.Write([]byte(`{"instanceId":"inst-1","status":"DELETING"}`))
		}
	})
	client.WaitTimeout = 2 * time.Second
	r := configuredComputeInstanceResource(t, client)

	start := time.Now()
	resp := resource.DeleteResponse{State: crudResponseState(t, r)}
	r.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, r, map[string]tftypes.Value{
			"workspace_slug": stringAttr("ws"),
			"database_name":  stringAttr("db"),
			"id":             stringAttr("inst-1"),
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

func TestApplyComputeInstanceResource(t *testing.T) {
	m := &computeInstanceResourceModel{InstanceName: types.StringValue("old")}
	applyComputeInstanceResource(m, nileapi.ComputeInstance{
		ID:     "inst-1",
		Status: "READY",
		Raw:    []byte(`{"instanceId":"inst-1"}`),
	}, "ws", "db", "fallback")
	if m.ID.ValueString() != "inst-1" || m.Status.ValueString() != "READY" {
		t.Errorf("model = %+v", m)
	}
	if m.WorkspaceSlug.ValueString() != "ws" || m.DatabaseName.ValueString() != "db" {
		t.Errorf("scopes = %q/%q", m.WorkspaceSlug, m.DatabaseName)
	}
	// A missing name keeps the fallback instead of clearing it.
	if m.InstanceName.ValueString() != "fallback" {
		t.Errorf("instance_name = %q", m.InstanceName)
	}
	// Fields the payload omits keep their previous values.
	if !m.Memory.IsNull() {
		t.Errorf("memory = %q, want null", m.Memory)
	}
	if m.Raw.ValueString() != `{"instanceId":"inst-1"}` {
		t.Errorf("raw_json = %q", m.Raw.ValueString())
	}
}
