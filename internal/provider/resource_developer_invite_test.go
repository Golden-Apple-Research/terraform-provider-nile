// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/Golden-Apple-Research/nile-terraform/internal/nileapi"
)

func TestDeveloperInviteResourceSchema(t *testing.T) {
	sch := resourceSchemaOf(t, NewDeveloperInviteResource())
	for _, name := range []string{"workspace_slug", "email", "programmatic", "id", "verification_state", "code", "sender_email", "created", "updated", "raw_json"} {
		if _, ok := sch.Attributes[name]; !ok {
			t.Errorf("missing attribute %q", name)
		}
	}
	if attr := sch.Attributes["code"]; attr == nil || !attr.IsSensitive() {
		t.Error("code must be sensitive")
	}
}

// inviteAPIServer routes by method: POST creates, GET lists, DELETE deletes.
func inviteAPIServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *nileapi.Client {
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

func configuredInviteResource(t *testing.T, client *nileapi.Client) *developerInviteResource {
	t.Helper()
	r := NewDeveloperInviteResource().(*developerInviteResource)
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
	return r
}

func TestDeveloperInviteResourceCreate(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected %s request", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"inv-1","email":"b@example.com","verificationState":"EMAIL_PENDING","code":"invite-code","created":"2025-06-01T00:00:00Z"}`))
	})
	res := configuredInviteResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"email":          stringAttr("b@example.com"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "id"); got != "inv-1" {
		t.Errorf("id = %q", got)
	}
	if got := stateString(t, resp.State, "code"); got != "invite-code" {
		t.Errorf("code = %q", got)
	}
	var programmatic types.Bool
	if diags := resp.State.GetAttribute(context.Background(), path.Root("programmatic"), &programmatic); diags.HasError() {
		t.Fatalf("GetAttribute: %v", diags)
	}
	if programmatic.ValueBool() {
		t.Errorf("programmatic = %v, want false", programmatic)
	}
}

func TestDeveloperInviteResourceCreateFallsBackToList(t *testing.T) {
	// The email-only flow returns no body; the resource must find the invite
	// by listing and pick the most recent one for the email.
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":"inv-old","email":"b@example.com","created":"2025-01-01T00:00:00Z"},
			{"id":"inv-1","email":"b@example.com","created":"2025-06-01T00:00:00Z"},
			{"id":"inv-other","email":"c@example.com","created":"2025-07-01T00:00:00Z"}
		]`))
	})
	res := configuredInviteResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"email":          stringAttr("b@example.com"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "id"); got != "inv-1" {
		t.Errorf("id = %q, want the newest invite for the email", got)
	}
}

func TestDeveloperInviteResourceCreateFallbackMissing(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	res := configuredInviteResource(t, client)

	resp := resource.CreateResponse{State: crudResponseState(t, res)}
	res.Create(context.Background(), resource.CreateRequest{
		Plan: planWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"email":          stringAttr("b@example.com"),
		}),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when the invite cannot be found after creation")
	}
}

func TestFindInviteByEmail(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"inv-1","email":"b@example.com","created":"2025-01-01T00:00:00Z"},
			{"id":"inv-2","email":"b@example.com","created":"2025-06-01T00:00:00Z"}
		]`))
	})
	res := configuredInviteResource(t, client)

	invite, err := res.findInviteByEmail(context.Background(), "acme", "b@example.com")
	if err != nil {
		t.Fatalf("findInviteByEmail: %v", err)
	}
	if invite.ID != "inv-2" {
		t.Errorf("id = %q, want the newest invite", invite.ID)
	}

	if _, err := res.findInviteByEmail(context.Background(), "acme", "nobody@example.com"); err == nil {
		t.Error("expected an error for an email without invites")
	}
}

func TestDeveloperInviteResourceRead(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"inv-1","email":"b@example.com","verificationState":"EMAIL_SENT","code":"secret"}]`))
	})
	res := configuredInviteResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"id":             stringAttr("inv-1"),
			"email":          stringAttr("b@example.com"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	if got := stateString(t, resp.State, "verification_state"); got != "EMAIL_SENT" {
		t.Errorf("verification_state = %q", got)
	}
	// A code that only exists in state (imported resources) is kept as-is;
	// here the API returned one, and it must not leak into raw_json.
	if got := stateString(t, resp.State, "raw_json"); strings.Contains(got, "secret") {
		t.Errorf("raw_json = %s, want code redacted", got)
	}
}

func TestDeveloperInviteResourceReadRemovesMissing(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	res := configuredInviteResource(t, client)

	resp := resource.ReadResponse{State: crudResponseState(t, res)}
	res.Read(context.Background(), resource.ReadRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"id":             stringAttr("inv-gone"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}

func TestDeveloperInviteResourceUpdateNotSupported(t *testing.T) {
	res := NewDeveloperInviteResource().(*developerInviteResource)

	var resp resource.UpdateResponse
	res.Update(context.Background(), resource.UpdateRequest{}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error: updates must be rejected")
	}
}

func TestDeveloperInviteResourceDelete(t *testing.T) {
	client := inviteAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected %s request", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	res := configuredInviteResource(t, client)

	resp := resource.DeleteResponse{State: crudResponseState(t, res)}
	res.Delete(context.Background(), resource.DeleteRequest{
		State: stateWith(t, res, map[string]tftypes.Value{
			"workspace_slug": stringAttr("acme"),
			"id":             stringAttr("inv-1"),
		}),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want removed", resp.State.Raw)
	}
}
