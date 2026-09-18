// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

// Package nileapi is a client for the Nile control plane REST API
// (https://www.thenile.dev/docs/api-reference).
package nileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	DefaultBaseURL = "https://global.thenile.dev"

	// Retry policy for transient failures (rate limiting, 5xx, network
	// errors). Retries only happen for requests that are safe to replay; see
	// doWithRetries.
	defaultMaxRetries = 3
	retryBaseDelay    = 200 * time.Millisecond
	retryMaxDelay     = 2 * time.Second
	retryAfterCap     = 30 * time.Second

	// DefaultPollInterval is the delay between readiness polls for
	// asynchronous operations.
	DefaultPollInterval = 5 * time.Second
	// DefaultWaitTimeout bounds how long the provider waits for an
	// asynchronous operation when the caller's context has no deadline.
	DefaultWaitTimeout = 20 * time.Minute

	// maxResponseBytes caps how much of a response body is buffered.
	maxResponseBytes = 10 << 20 // 10 MiB
)

// Client talks to the Nile REST API using bearer-token authentication.
type Client struct {
	BaseURL   *url.URL
	AuthToken string
	// UserAgent, when set, is sent as the User-Agent header on requests so
	// the API can identify the provider.
	UserAgent string
	// MaxRetries is how often a transient failure on a replay-safe request
	// (HTTP 408/429/5xx or a network error) is retried with exponential
	// backoff. It defaults to 3; 0 disables retries.
	MaxRetries int
	HTTP       *http.Client
	// PollInterval is the delay between polls while waiting for an
	// asynchronous operation. It defaults to DefaultPollInterval.
	PollInterval time.Duration
	// WaitTimeout bounds the total wait for an asynchronous operation when
	// the caller's context has no deadline. It defaults to DefaultWaitTimeout.
	WaitTimeout time.Duration

	// retrySleep is swappable so tests can make backoff waits instant.
	retrySleep func(ctx context.Context, d time.Duration) error
}

// NewClient builds a client. An empty baseURL falls back to DefaultBaseURL.
func NewClient(baseURL, authToken string) (*Client, error) {
	if authToken == "" {
		return nil, fmt.Errorf("auth token must not be empty")
	}
	if strings.ContainsAny(authToken, "\r\n") {
		return nil, fmt.Errorf("auth token must not contain newlines")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	// Production traffic must use TLS. Plain HTTP is accepted only for an
	// explicit loopback endpoint, which keeps local mock-server tests possible
	// without allowing a remote API URL to exfiltrate the bearer token.
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return nil, fmt.Errorf("invalid base URL: an HTTPS scheme is required (HTTP is allowed only for loopback test endpoints)")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid base URL: host is required")
	}
	if u.User != nil {
		return nil, fmt.Errorf("invalid base URL: user information is not allowed")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid base URL: query and fragment are not allowed")
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("invalid base URL: path prefixes are not supported")
	}
	return &Client{
		BaseURL:    u,
		AuthToken:  authToken,
		MaxRetries: defaultMaxRetries,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
			// The API is not expected to redirect. Returning the 3xx response
			// prevents net/http from forwarding Authorization to another
			// endpoint, including a different port on the same host.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		PollInterval: DefaultPollInterval,
		WaitTimeout:  DefaultWaitTimeout,
	}, nil
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// APIError is a structured error returned by the Nile API. It implements
// error, so callers can use errors.As or the IsNotFound/IsConflict helpers.
type APIError struct {
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"errorCode"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Nile API error (HTTP %d", e.StatusCode)
	if e.ErrorCode != "" {
		b.WriteString(" " + e.ErrorCode)
	}
	b.WriteString(")")
	if e.Message != "" {
		b.WriteString(": " + safeAPIErrorMessage(e.Message))
	}
	return b.String()
}

// IsNotFound reports whether err is an API error with HTTP status 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsConflict reports whether err is an API error with HTTP status 409.
func IsConflict(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

// IsForbidden reports whether err is an API error with HTTP status 403.
func IsForbidden(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden
}

// ErrorCode returns the machine-readable API error code, or "" if err did not
// originate from a structured API error.
func ErrorCode(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode
	}
	return ""
}

// endpoint builds an absolute URL from path segments, escaping every segment
// individually so that characters like "/" inside a workspace slug or database
// name cannot change the request path. Dot segments are percent-encoded too:
// url.Parse otherwise normalizes "." and ".." while resolving the reference.
func (c *Client) endpoint(segments ...string) *url.URL {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = escapePathSegment(s)
	}
	// escapePathSegment never emits characters that make url.Parse fail, so the
	// error branch is unreachable; returning the base URL keeps the signature
	// free of an error that callers cannot handle meaningfully.
	rel, err := url.Parse("/" + strings.Join(escaped, "/"))
	if err != nil {
		return c.BaseURL
	}
	return c.BaseURL.ResolveReference(rel)
}

func escapePathSegment(segment string) string {
	switch segment {
	case ".":
		return "%2E"
	case "..":
		return "%2E%2E"
	default:
		return url.PathEscape(segment)
	}
}

// withQuery adds non-empty key/value pairs to a URL's query string. Pairs are
// passed as alternating key and value arguments; a trailing key without a
// value is ignored.
func withQuery(u *url.URL, kv ...string) *url.URL {
	if len(kv) == 0 {
		return u
	}
	q := u.Query()
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i] != "" && kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	u.RawQuery = q.Encode()
	return u
}

// withBoolQuery adds a boolean query parameter if v is non-nil.
func withBoolQuery(u *url.URL, key string, v *bool) *url.URL {
	if v == nil {
		return u
	}
	q := u.Query()
	q.Set(key, strconv.FormatBool(*v))
	u.RawQuery = q.Encode()
	return u
}

// request performs an authenticated HTTP request and, on a 2xx status,
// decodes the JSON response into out (unless out is nil). A non-2xx status is
// turned into an *APIError when the body carries the documented error shape.
//
// body may be nil, a url.Values (sent form-encoded, used by /oauth2/token), or
// any value that is marshalled to JSON.
func (c *Client) request(ctx context.Context, method string, u *url.URL, body, out any) error {
	return c.requestWithAuth(ctx, method, u, body, out, true)
}

func (c *Client) requestWithoutAuth(ctx context.Context, method string, u *url.URL, body, out any) error {
	return c.requestWithAuth(ctx, method, u, body, out, false)
}

func (c *Client) requestWithAuth(ctx context.Context, method string, u *url.URL, body, out any, authenticated bool) error {
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case url.Values:
		reader = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.doWithRetries(ctx, req)
	if err != nil {
		return err
	}
	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("reading response body: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return decodeAPIError(resp.StatusCode, payload)
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if raw, ok := out.(*json.RawMessage); ok {
		*raw = append(json.RawMessage(nil), payload...)
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", safeRequestPath(u), err)
	}
	return nil
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, u *url.URL, out any) error {
	return c.request(ctx, http.MethodGet, u, nil, out)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, u *url.URL, body, out any) error {
	return c.request(ctx, http.MethodPost, u, body, out)
}

func (c *Client) postUnauthenticated(ctx context.Context, u *url.URL, body, out any) error {
	return c.requestWithoutAuth(ctx, http.MethodPost, u, body, out)
}

// put performs a PUT request.
func (c *Client) put(ctx context.Context, u *url.URL, body, out any) error {
	return c.request(ctx, http.MethodPut, u, body, out)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, u *url.URL, out any) error {
	return c.request(ctx, http.MethodDelete, u, nil, out)
}

// decodeAPIError turns an error response into an *APIError when the payload
// carries the documented {errorCode, message, statusCode} shape, and into a
// plain error otherwise. The HTTP status wins over a contradicting statusCode
// in the body.
func decodeAPIError(status int, payload []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(payload, &apiErr); err == nil && (apiErr.ErrorCode != "" || apiErr.Message != "") {
		apiErr.StatusCode = status
		apiErr.Message = safeAPIErrorMessage(apiErr.Message)
		return &apiErr
	}
	// Do not copy arbitrary server response bytes into diagnostics. Error
	// responses can contain reflected credentials or other sensitive data.
	return fmt.Errorf("Nile API returned HTTP %d", status)
}

func safeRequestPath(u *url.URL) string {
	if u == nil || u.EscapedPath() == "" {
		return "/"
	}
	return u.EscapedPath()
}

func safeAPIErrorMessage(message string) string {
	message = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(message))
	if message == "" {
		return ""
	}
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(message))
	for _, marker := range []string{
		"password", "passwd", "secret", "token", "apikey", "privatekey",
		"authorization", "bearer", "connectionstring", "dsn", "databaseurl", "databaseuri",
	} {
		if strings.Contains(normalized, marker) {
			return "[REDACTED]"
		}
	}
	return truncate([]byte(message), 256)
}

// doWithRetries executes req, retrying transient failures (rate limiting,
// 5xx, network errors) with exponential backoff and jitter. A Retry-After
// header takes precedence over the computed backoff. Context cancellation is
// never retried.
//
// Only replay-safe requests are retried after 5xx/network errors: GET, HEAD,
// PUT, DELETE and OPTIONS. POST is never retried: even a 429 response does not
// universally guarantee that the server did not process the request.
//
// The returned response body is open and owned by the caller; bodies of
// responses that are retried away are drained and closed here.
func (c *Client) doWithRetries(ctx context.Context, req *http.Request) (*http.Response, error) {
	attempts := c.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	replayable := idempotentMethod(req.Method)
	for attempt := 0; ; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			// Rewind the body so a retried request sends its payload again.
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("rewinding request body: %w", err)
			}
			req.Body = body
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			// Retrying after the caller gave up would be pointless (and
			// http.Client already wraps the ctx error).
			if ctx.Err() != nil || attempt == attempts-1 || !replayable {
				return nil, fmt.Errorf("calling %s: %w", safeRequestPath(req.URL), err)
			}
			if waitErr := c.sleepBackoff(ctx, attempt, nil); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		if !retryableForMethod(req.Method, resp.StatusCode) || attempt == attempts-1 {
			return resp, nil
		}
		tflog.Debug(ctx, "transient failure, retrying", map[string]any{
			"attempt": attempt + 1,
			"status":  resp.Status,
		})
		// Drain so the connection can be reused for the retry.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if waitErr := c.sleepBackoff(ctx, attempt, resp); waitErr != nil {
			return nil, waitErr
		}
	}
}

// idempotentMethod reports whether a request with the given method is safe to
// replay after a transient failure.
func idempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut,
		http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

// retryableForMethod reports whether a response with the given status may be
// retried for the request's method. Non-idempotent methods are never retried.
func retryableForMethod(method string, code int) bool {
	return idempotentMethod(method) && retryableStatus(code)
}

// sleepBackoff waits before the next attempt. Network errors pass a nil
// response and fall back to pure exponential backoff.
func (c *Client) sleepBackoff(ctx context.Context, attempt int, resp *http.Response) error {
	d := backoffDelay(attempt, resp)
	sleep := c.retrySleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	return sleep(ctx, d)
}

// backoffDelay returns the wait before the given attempt. A parseable
// Retry-After header (seconds) wins verbatim — it is a server-mandated
// minimum, so no jitter is applied that could shorten it. Otherwise the
// delay grows exponentially from retryBaseDelay with equal jitter
// (half fixed, half random) to avoid synchronized retry storms.
func backoffDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := retryAfterDuration(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}
	d := retryBaseDelay << attempt // may overflow for very large attempt counts
	if d <= 0 || d > retryMaxDelay {
		d = retryMaxDelay
	}
	half := d / 2
	return half + time.Duration(rand.Float64()*float64(half))
}

// retryAfterDuration parses a Retry-After header in delay-seconds form and
// caps it at retryAfterCap so a misbehaving server cannot stall a plan.
func retryAfterDuration(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		return 0, false
	}
	if secs > int(retryAfterCap/time.Second) {
		return retryAfterCap, true
	}
	return time.Duration(secs) * time.Second, true
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// sleepWithContext waits for d (already jittered) and aborts early on context
// cancellation.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// pollInterval returns the configured poll interval or the default.
func (c *Client) pollInterval() time.Duration {
	if c.PollInterval > 0 {
		return c.PollInterval
	}
	return DefaultPollInterval
}

// waitContext bounds ctx with WaitTimeout unless the caller already supplied a
// deadline.
func (c *Client) waitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	timeout := c.WaitTimeout
	if timeout <= 0 {
		timeout = DefaultWaitTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// WaitForDatabaseReady polls GetDatabase until the database reaches READY
// (or the API reports no status). It fails when the wait exceeds the caller's
// deadline or WaitTimeout.
func (c *Client) WaitForDatabaseReady(ctx context.Context, workspaceSlug, databaseName string) (Database, error) {
	ctx, cancel := c.waitContext(ctx)
	defer cancel()
	interval := c.pollInterval()
	for {
		db, err := c.GetDatabase(ctx, workspaceSlug, databaseName)
		switch {
		case err == nil && databaseReady(db.Status):
			return db, nil
		case err != nil && ctx.Err() != nil:
			return Database{}, fmt.Errorf("waiting for database %q in workspace %q: %w", databaseName, workspaceSlug, ctx.Err())
		case err != nil && !IsNotFound(err):
			return Database{}, err
		}
		tflog.Debug(ctx, "waiting for database to become ready", map[string]any{
			"workspace": workspaceSlug,
			"database":  databaseName,
			"status":    db.Status,
		})
		if err := sleepWithContext(ctx, interval); err != nil {
			return Database{}, fmt.Errorf("waiting for database %q in workspace %q: %w", databaseName, workspaceSlug, err)
		}
	}
}

// databaseReady reports whether a database status means the database can be
// used. An omitted status is not considered ready: it may be a partial or
// asynchronous response and must be followed by another poll.
func databaseReady(status string) bool {
	return status == "READY"
}

// WaitForComputeInstanceReady polls DescribeComputeInstance until the instance
// reaches READY. FAILED and TERMINATED are terminal errors.
func (c *Client) WaitForComputeInstanceReady(ctx context.Context, workspaceSlug, databaseName, instanceID string) (ComputeInstance, error) {
	ctx, cancel := c.waitContext(ctx)
	defer cancel()
	interval := c.pollInterval()
	for {
		instance, err := c.DescribeComputeInstance(ctx, workspaceSlug, databaseName, instanceID)
		switch {
		case err == nil:
			switch instance.Status {
			case "READY":
				return instance, nil
			case "FAILED", "TERMINATED":
				return instance, fmt.Errorf("compute instance %q in database %q entered terminal status %s",
					instanceID, databaseName, instance.Status)
			}
		case ctx.Err() != nil:
			return ComputeInstance{}, fmt.Errorf("waiting for compute instance %q in database %q: %w", instanceID, databaseName, ctx.Err())
		case !IsNotFound(err):
			return ComputeInstance{}, err
		}
		tflog.Debug(ctx, "waiting for compute instance to become ready", map[string]any{
			"workspace": workspaceSlug,
			"database":  databaseName,
			"instance":  instanceID,
			"status":    instance.Status,
		})
		if err := sleepWithContext(ctx, interval); err != nil {
			return ComputeInstance{}, fmt.Errorf("waiting for compute instance %q in database %q: %w",
				instanceID, databaseName, err)
		}
	}
}

// WaitForComputeInstanceDeleted polls DescribeComputeInstance until the
// instance is gone (404) or reports a terminal status.
func (c *Client) WaitForComputeInstanceDeleted(ctx context.Context, workspaceSlug, databaseName, instanceID string) error {
	ctx, cancel := c.waitContext(ctx)
	defer cancel()
	interval := c.pollInterval()
	for {
		instance, err := c.DescribeComputeInstance(ctx, workspaceSlug, databaseName, instanceID)
		switch {
		case IsNotFound(err):
			return nil
		case ctx.Err() != nil:
			return fmt.Errorf("waiting for compute instance %q in database %q: %w", instanceID, databaseName, ctx.Err())
		case err != nil:
			return err
		case instance.Status == "TERMINATED" || instance.Status == "DELETED":
			return nil
		}
		tflog.Debug(ctx, "waiting for compute instance to be deleted", map[string]any{
			"workspace": workspaceSlug,
			"database":  databaseName,
			"instance":  instanceID,
			"status":    instance.Status,
		})
		if err := sleepWithContext(ctx, interval); err != nil {
			return fmt.Errorf("waiting for compute instance %q in database %q to be deleted: %w",
				instanceID, databaseName, err)
		}
	}
}

// truncate returns s clipped to at most n bytes, without splitting a UTF-8
// rune, with an ellipsis appended when it was clipped.
func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		// Do not cut a multi-byte UTF-8 rune in half.
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}
