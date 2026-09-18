package nileapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleInstance = `{
	"instanceId": "inst-1",
	"instanceName": "primary",
	"instanceType": {"id": "tier-1", "computeSize": "large", "memory": "8GB", "hourlyCost": 0.42},
	"status": "READY",
	"region": "AWS_US_WEST_2",
	"created": "2025-06-01T12:00:00Z"
}`

func TestDecodeInstancesBareArray(t *testing.T) {
	body := []byte("[" + sampleInstance + `, {"instanceId": "inst-2", "status": "TERMINATED"}]`)
	got, err := decodeInstances(body)
	if err != nil {
		t.Fatalf("decodeInstances failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(got))
	}
	want := ComputeInstance{
		ID:        "inst-1",
		Name:      "primary",
		Status:    "READY",
		Size:      "large",
		Region:    "AWS_US_WEST_2",
		CreatedAt: "2025-06-01T12:00:00Z",
	}
	if got[0].ID != want.ID || got[0].Name != want.Name || got[0].Status != want.Status ||
		got[0].Size != want.Size || got[0].Region != want.Region || got[0].CreatedAt != want.CreatedAt {
		t.Errorf("instance 0 = %+v, want %+v", got[0], want)
	}
	// Fields missing from the payload must stay empty rather than fail the decode.
	if got[1].ID != "inst-2" || got[1].Name != "" || got[1].Size != "" || got[1].CreatedAt != "" {
		t.Errorf("instance 1 = %+v", got[1])
	}
	var raw map[string]any
	if err := json.Unmarshal(got[0].Raw, &raw); err != nil {
		t.Errorf("raw_json not valid: %v", err)
	}
}

func TestDecodeInstancesWrapper(t *testing.T) {
	for _, key := range wrapperKeys {
		body := []byte(`{"` + key + `": [{"instanceId": "wrapped-1"}], "count": 1}`)
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

func TestDecodeInstancesUnknownObject(t *testing.T) {
	if _, err := decodeInstances([]byte(`{"foo": "bar"}`)); err == nil {
		t.Fatal("expected error for object without an instance array")
	}
}

func TestDecodeInstancesWrapperNotArray(t *testing.T) {
	if _, err := decodeInstances([]byte(`{"instances": {"instanceId": "x"}}`)); err == nil {
		t.Fatal("expected error for wrapper whose value is not an array")
	}
}

func TestListComputeInstances(t *testing.T) {
	var gotRequestURI, gotAuth, gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		gotAuth = r.Header.Get("Authorization")
		q := r.URL.Query()
		gotStart = q.Get("start")
		gotEnd = q.Get("end")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"instanceId":"inst-x","status":"READY"}]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := c.ListComputeInstances(t.Context(), "ws 1", "db/2", "2025-01-01T00:00:00Z", "")
	if err != nil {
		t.Fatalf("ListComputeInstances: %v", err)
	}
	// RequestURI keeps the escaped path segments: spaces as %20 and slashes
	// inside a segment as %2F.
	if !strings.HasPrefix(gotRequestURI, "/workspaces/ws%201/databases/db%2F2/compute?") {
		t.Errorf("request URI = %q", gotRequestURI)
	}
	if gotAuth != "Bearer tok-1" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotStart != "2025-01-01T00:00:00Z" || gotEnd != "" {
		t.Errorf("start=%q end=%q", gotStart, gotEnd)
	}
	if len(got) != 1 || got[0].ID != "inst-x" {
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
