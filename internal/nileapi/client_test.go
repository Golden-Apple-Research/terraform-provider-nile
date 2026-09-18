package nileapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeInstancesBareArray(t *testing.T) {
	body := []byte(`[
		{"id": "ci-1", "status": "running", "size": "standard-2", "region": "us-east-1", "created_at": "2025-06-01T12:00:00Z"},
		{"id": "ci-2", "status": "terminated"}
	]`)
	got, err := decodeInstances(body)
	if err != nil {
		t.Fatalf("decodeInstances failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(got))
	}
	if got[0].ID != "ci-1" || got[0].Status != "running" || got[0].Size != "standard-2" {
		t.Errorf("instance 0 fields not promoted: %+v", got[0])
	}
	if got[1].Status != "terminated" {
		t.Errorf("instance 1 status = %q, want terminated", got[1].Status)
	}
	var raw map[string]any
	if err := json.Unmarshal(got[0].Raw, &raw); err != nil {
		t.Errorf("raw_json not valid: %v", err)
	}
}

func TestDecodeInstancesWrapper(t *testing.T) {
	for _, key := range []string{"instances", "compute", "items", "data", "results"} {
		body := []byte(`{"` + key + `": [{"id": "wrapped-1"}], "count": 1}`)
		got, err := decodeInstances(body)
		if err != nil {
			t.Fatalf("key %q: decodeInstances failed: %v", key, err)
		}
		if len(got) != 1 || got[0].ID != "wrapped-1" {
			t.Errorf("key %q: got %+v", key, got)
		}
	}
}

func TestDecodeInstancesEmptyArray(t *testing.T) {
	got, err := decodeInstances([]byte(`[]`))
	if err != nil {
		t.Fatalf("decodeInstances failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 instances, got %d", len(got))
	}
}

func TestListComputeInstances(t *testing.T) {
	var gotPath, gotAuth, gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		q := r.URL.Query()
		gotStart = q.Get("start")
		gotEnd = q.Get("end")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"ci-x","status":"running"}]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := c.ListComputeInstances(t.Context(), "ws 1", "db 2", "2025-01-01T00:00:00Z", "")
	if err != nil {
		t.Fatalf("ListComputeInstances: %v", err)
	}
	if gotPath != "/workspaces/ws 1/databases/db 2/compute" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok-1" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotStart != "2025-01-01T00:00:00Z" || gotEnd != "" {
		t.Errorf("start=%q end=%q", gotStart, gotEnd)
	}
	if len(got) != 1 || got[0].ID != "ci-x" {
		t.Errorf("instances = %+v", got)
	}
}

func TestListComputeInstancesErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "bad")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.ListComputeInstances(t.Context(), "ws", "db", "", ""); err == nil {
		t.Fatal("expected error on 403")
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	if _, err := NewClient("", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}
