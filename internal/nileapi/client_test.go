// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const sampleInstance = `{
	"instanceId": "inst-1",
	"instanceName": "primary",
	"instanceType": {"id": "tier-1", "computeSize": "large", "memory": "8GB", "hourlyCost": 0.42},
	"status": "READY",
	"region": "AWS_US_WEST_2",
	"created": "2025-06-01T12:00:00Z"
}`

// instantSleep replaces backoff waits in retry tests so they stay fast and
// deterministic.
func instantSleep(context.Context, time.Duration) error { return nil }

func TestDecodeInstancesBareArray(t *testing.T) {
	body := []byte("[" + sampleInstance + `, {"instanceId": "inst-2", "status": "TERMINATED"}]`)
	got, next, err := decodeInstances(t.Context(), body)
	if err != nil {
		t.Fatalf("decodeInstances failed: %v", err)
	}
	if next != "" {
		t.Errorf("bare array must not carry a continuation token, got %q", next)
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
		got, _, err := decodeInstances(t.Context(), body)
		if err != nil {
			t.Fatalf("key %q: decodeInstances failed: %v", key, err)
		}
		if len(got) != 1 || got[0].ID != "wrapped-1" {
			t.Errorf("key %q: got %+v", key, got)
		}
	}
}

func TestDecodeInstancesContinuation(t *testing.T) {
	for _, key := range continuationKeys {
		body := []byte(`{"instances": [], "` + key + `": "tok-42"}`)
		_, next, err := decodeInstances(t.Context(), body)
		if err != nil {
			t.Fatalf("key %q: decodeInstances failed: %v", key, err)
		}
		if next != "tok-42" {
			t.Errorf("key %q: continuation token = %q, want %q", key, next, "tok-42")
		}
	}

	// Missing, empty, and non-string values count as "no continuation".
	for _, body := range []string{
		`{"instances": []}`,
		`{"instances": [], "nextPageToken": ""}`,
		`{"instances": [], "cursor": 7}`,
	} {
		if _, next, err := decodeInstances(t.Context(), []byte(body)); err != nil || next != "" {
			t.Errorf("body %s: next = %q, err = %v; want empty token, no error", body, next, err)
		}
	}
}

func TestDecodeInstancesEmptyArray(t *testing.T) {
	got, _, err := decodeInstances(t.Context(), []byte(`[]`))
	if err != nil {
		t.Fatalf("decodeInstances failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 instances, got %d", len(got))
	}
}

func TestDecodeInstancesUnknownObject(t *testing.T) {
	if _, _, err := decodeInstances(t.Context(), []byte(`{"foo": "bar"}`)); err == nil {
		t.Fatal("expected error for object without an instance array")
	}
}

func TestDecodeInstancesWrapperNotArray(t *testing.T) {
	if _, _, err := decodeInstances(t.Context(), []byte(`{"instances": {"instanceId": "x"}}`)); err == nil {
		t.Fatal("expected error for wrapper whose value is not an array")
	}
}

func TestListComputeInstances(t *testing.T) {
	var gotRequestURI, gotAuth, gotUA, gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
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
	c.UserAgent = "terraform-provider-nile/test"
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
	if gotUA != "terraform-provider-nile/test" {
		t.Errorf("user agent = %q", gotUA)
	}
	if gotStart != "2025-01-01T00:00:00Z" || gotEnd != "" {
		t.Errorf("start=%q end=%q", gotStart, gotEnd)
	}
	if len(got) != 1 || got[0].ID != "inst-x" {
		t.Errorf("instances = %+v", got)
	}
}

func TestListComputeInstancesPaginates(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.URL.Query().Get("pageToken"))
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"instances": [{"instanceId": "p1"}], "nextPageToken": "page-2"}`))
			return
		}
		// Last page may return the documented bare-array shape.
		_, _ = w.Write([]byte(`[{"instanceId": "p2"}]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err != nil {
		t.Fatalf("ListComputeInstances: %v", err)
	}
	if len(got) != 2 || got[0].ID != "p1" || got[1].ID != "p2" {
		t.Fatalf("instances = %+v, want pages concatenated", got)
	}
	if len(tokens) != 2 || tokens[0] != "" || tokens[1] != "page-2" {
		t.Errorf("page tokens sent = %v, want [\"\", \"page-2\"]", tokens)
	}
}

func TestListComputeInstancesPaginationStuck(t *testing.T) {
	// A server that keeps returning the same token must produce an error,
	// not an endless loop.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"instances": [], "nextPageToken": "same-token"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err == nil || !strings.Contains(err.Error(), "did not advance") {
		t.Fatalf("expected pagination-stuck error, got %v", err)
	}
}

func TestListComputeInstancesRetriesTransient(t *testing.T) {
	var calls atomic.Int32
	responses := []struct {
		status int
		body   string
	}{
		{http.StatusServiceUnavailable, `{"errorCode": "internal_error"}`},
		{http.StatusTooManyRequests, `{"errorCode": "rate_limited"}`},
		{http.StatusOK, `[{"instanceId": "ok"}]`},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(calls.Add(1)) - 1
		if responses[i].status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "0")
		}
		w.WriteHeader(responses[i].status)
		_, _ = w.Write([]byte(responses[i].body))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = instantSleep
	got, err := c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err != nil {
		t.Fatalf("ListComputeInstances: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (two transient failures retried)", calls.Load())
	}
	if len(got) != 1 || got[0].ID != "ok" {
		t.Errorf("instances = %+v", got)
	}
}

func TestListComputeInstancesRetriesExhausted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"errorCode": "internal_error"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = instantSleep
	_, err = c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected exhausted-retries error, got %v", err)
	}
	if want := int64(c.MaxRetries + 1); int64(calls.Load()) != want {
		t.Errorf("calls = %d, want %d (MaxRetries + 1)", calls.Load(), want)
	}
}

func TestListComputeInstancesNoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorCode": "bad_request"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = instantSleep
	if _, err := c.ListComputeInstances(t.Context(), "ws", "db", "", ""); err == nil {
		t.Fatal("expected error on 400")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (client errors are not retried)", calls.Load())
	}
}

func TestListComputeInstancesRetriesNetworkErrors(t *testing.T) {
	// Hijacking and closing the connection makes the request fail at the
	// network level, below any HTTP status code.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("server does not support hijacking")
			w.WriteHeader(http.StatusOK)
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.MaxRetries = 2
	c.retrySleep = instantSleep
	_, err = c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err == nil {
		t.Fatal("expected error after exhausting network-error retries")
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestListComputeInstancesRetrySleepErrorPropagates(t *testing.T) {
	// If waiting is aborted (e.g. context cancellation), the error must
	// surface instead of being retried away.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = func(context.Context, time.Duration) error { return errors.New("wait aborted") }
	_, err = c.ListComputeInstances(t.Context(), "ws", "db", "", "")
	if err == nil || !strings.Contains(err.Error(), "wait aborted") {
		t.Fatalf("expected sleep error to propagate, got %v", err)
	}
}

func TestRetryAfterDuration(t *testing.T) {
	for _, tc := range []struct {
		in        string
		want      time.Duration
		wantFound bool
	}{
		{"", 0, false},
		{"abc", 0, false},
		{"-1", 0, false},
		{"0", 0, true},
		{"2", 2 * time.Second, true},
		{" 3 ", 3 * time.Second, true},
		{"3600", retryAfterCap, true},                // capped
		{"9223372036854775807", retryAfterCap, true}, // overflow-safe cap
	} {
		got, ok := retryAfterDuration(tc.in)
		if ok != tc.wantFound || got != tc.want {
			t.Errorf("retryAfterDuration(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantFound)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	// A parseable Retry-After wins verbatim (no jitter that could shorten it).
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"7"}}}
	if d := backoffDelay(0, resp); d != 7*time.Second {
		t.Errorf("Retry-After ignored: delay = %v", d)
	}

	// Without Retry-After: exponential growth with equal jitter.
	for _, tc := range []struct {
		attempt  int
		min, max time.Duration
	}{
		{0, 100 * time.Millisecond, 200 * time.Millisecond},
		{1, 200 * time.Millisecond, 400 * time.Millisecond},
		{2, 400 * time.Millisecond, 800 * time.Millisecond},
		{30, retryMaxDelay / 2, retryMaxDelay}, // shift overflow capped
	} {
		d := backoffDelay(tc.attempt, nil)
		if d < tc.min || d > tc.max {
			t.Errorf("attempt %d: delay = %v, want within [%v, %v]", tc.attempt, d, tc.min, tc.max)
		}
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

func TestNewClientRequiresHTTPScheme(t *testing.T) {
	for _, baseURL := range []string{
		"global.thenile.dev",        // missing scheme
		"ftp://global.thenile.dev",  // wrong scheme
		"http://global.thenile.dev", // remote plaintext HTTP
		"://thenile.dev",            // unparseable
	} {
		if _, err := NewClient(baseURL, "tok"); err == nil {
			t.Errorf("expected error for base URL %q", baseURL)
		}
	}
}

func TestNewClientAllowsLoopbackHTTP(t *testing.T) {
	if _, err := NewClient("http://127.0.0.1:12345", "tok"); err != nil {
		t.Fatalf("loopback HTTP should be allowed for local test endpoints: %v", err)
	}
}

func TestNewClientRejectsBaseURLCredentialsAndQuery(t *testing.T) {
	for _, baseURL := range []string{
		"https://user@example.com",
		"https://example.com?token=secret",
		"https://example.com/prefix",
	} {
		if _, err := NewClient(baseURL, "tok"); err == nil {
			t.Errorf("expected error for unsafe base URL %q", baseURL)
		}
	}
}

func TestNewClientDoesNotFollowRedirectWithAuthorization(t *testing.T) {
	var targetAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	c, err := NewClient(redirect.URL, "redirect-secret")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.MaxRetries = 0
	if _, err := c.ListComputeInstances(t.Context(), "ws", "db", "", ""); err == nil {
		t.Fatal("expected redirect response to be returned as an error")
	}
	if targetAuth != "" {
		t.Fatalf("redirect target received Authorization header %q", targetAuth)
	}
}

func TestDecodeAPIErrorDoesNotExposeSecrets(t *testing.T) {
	err := decodeAPIError(http.StatusBadRequest, []byte(`{"errorCode":"bad_request","message":"password=s3cret","statusCode":400}`))
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("API error exposed secret: %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "[REDACTED]" {
		t.Fatalf("APIError = %+v, want redacted message", apiErr)
	}

	fallback := decodeAPIError(http.StatusInternalServerError, []byte(`{"password":"s3cret"}`))
	if strings.Contains(fallback.Error(), "s3cret") {
		t.Fatalf("fallback API error exposed response body: %v", fallback)
	}
}

func TestRequestErrorsDoNotExposeQueryValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var out map[string]string
	u := withQuery(c.endpoint("resource"), "pageToken", "opaque-secret")
	if err := c.get(t.Context(), u, &out); err == nil {
		t.Fatal("expected JSON decoding error")
	} else if strings.Contains(err.Error(), "opaque-secret") {
		t.Fatalf("request error exposed query value: %v", err)
	}
}

func TestPost429IsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errorCode":"rate_limited"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.retrySleep = instantSleep
	if err := c.post(t.Context(), c.endpoint("resource"), nil, nil); err == nil {
		t.Fatal("expected POST 429 error")
	}
	if calls.Load() != 1 {
		t.Fatalf("POST calls = %d, want 1", calls.Load())
	}
}

func TestDecodeInstancesNull(t *testing.T) {
	// A bare JSON null must not silently decode as an empty list.
	if _, _, err := decodeInstances(t.Context(), []byte("null")); err == nil {
		t.Fatal("expected error for bare null response")
	}
	if _, _, err := decodeInstances(t.Context(), []byte(`{"instances": null}`)); err == nil {
		t.Fatal("expected error for null wrapper value")
	}
}

func TestMapInstancesTypeMismatchKeepsRaw(t *testing.T) {
	// If the API changes a field type (here instanceId becomes a number),
	// promotion fails but the raw payload must survive; the mismatch is
	// surfaced via a log warning (tflog is a no-op without a logger in ctx).
	got := mapInstances(t.Context(), []json.RawMessage{json.RawMessage(`{"instanceId": 42}`)})
	if len(got) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(got))
	}
	if got[0].ID != "" {
		t.Errorf("ID = %q, want empty", got[0].ID)
	}
	if string(got[0].Raw) != `{"instanceId": 42}` {
		t.Errorf("Raw = %q", got[0].Raw)
	}
}

func TestEndpointRejectsProtocolRelativeHijack(t *testing.T) {
	c, err := NewClient("https://global.thenile.dev", "tok")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	u := c.endpoint("", "attacker.com", "evil")
	if u.Host != "global.thenile.dev" {
		t.Fatalf("endpoint allowed protocol-relative host hijack: %s", u.String())
	}
}

func TestNewClientAllowsLocalhostHTTP(t *testing.T) {
	for _, rawURL := range []string{
		"http://localhost",
		"http://localhost:8080",
		"http://LOCALHOST:12345",
	} {
		if _, err := NewClient(rawURL, "tok"); err != nil {
			t.Errorf("expected localhost HTTP to be allowed, got error: %v", err)
		}
	}
}

func TestDecodeAPIErrorSanitizesErrorCode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "redacts secret",
			payload: `{"errorCode":"secret_token_expired","message":"failed"}`,
			want:    "[REDACTED]",
		},
		{
			name:    "sanitizes newlines and spaces",
			payload: "{\"errorCode\":\"invalid\\r\\ncode\\tfoo\",\"message\":\"failed\"}",
			want:    "invalid  code foo",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := decodeAPIError(http.StatusBadRequest, []byte(tc.payload))
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %v", err)
			}
			if apiErr.ErrorCode != tc.want {
				t.Errorf("errorCode = %q, want %q", apiErr.ErrorCode, tc.want)
			}
		})
	}
}
