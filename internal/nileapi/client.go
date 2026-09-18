// Package nileapi is a minimal client for the Nile control plane REST API.
package nileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL = "https://global.thenile.dev"
)

// Client talks to the Nile REST API using bearer-token authentication.
type Client struct {
	BaseURL   *url.URL
	AuthToken string
	HTTP      *http.Client
}

// NewClient builds a client. An empty baseURL falls back to DefaultBaseURL.
func NewClient(baseURL, authToken string) (*Client, error) {
	if authToken == "" {
		return nil, fmt.Errorf("auth token must not be empty")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	return &Client{
		BaseURL:   u,
		AuthToken: authToken,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ComputeInstance is one dedicated compute instance attached to a database.
// The API is evolving, so the full server payload is preserved in Raw while
// the most useful fields are promoted to typed values (best effort).
type ComputeInstance struct {
	ID        string `json:"id,omitempty"`
	Status    string `json:"status,omitempty"`
	Size      string `json:"size,omitempty"`
	Region    string `json:"region,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Raw       json.RawMessage
}

// ListComputeInstances calls
// GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute
// start/end are optional RFC3339 timestamps restricting the result to
// instances active in that time window.
func (c *Client) ListComputeInstances(ctx context.Context, workspaceSlug, databaseName, start, end string) ([]ComputeInstance, error) {
	rel := &url.URL{
		// Assign the decoded path; url.URL.String() percent-encodes as needed.
		Path: fmt.Sprintf("/workspaces/%s/databases/%s/compute", workspaceSlug, databaseName),
	}
	q := rel.Query()
	if start != "" {
		q.Set("start", start)
	}
	if end != "" {
		q.Set("end", end)
	}
	rel.RawQuery = q.Encode()

	endpoint := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", endpoint.String(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MiB guard
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %s: %s",
			endpoint.String(), resp.Status, truncate(body, 512))
	}

	// The API may return a bare array or an object wrapping the array.
	instances, err := decodeInstances(body)
	if err != nil {
		return nil, fmt.Errorf("decoding response from %s: %w", endpoint.String(), err)
	}
	return instances, nil
}

func decodeInstances(body []byte) ([]ComputeInstance, error) {
	// Try bare array first.
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		return mapInstances(raw), nil
	}

	// Otherwise look for a common wrapper key.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("unexpected JSON shape: %v", err)
	}
	for _, key := range []string{"instances", "compute", "items", "data", "results"} {
		if inner, ok := wrapper[key]; ok {
			if err := json.Unmarshal(inner, &raw); err == nil {
				return mapInstances(raw), nil
			}
		}
	}
	// Unknown wrapper: return the single object as one instance so the
	// data source still surfaces the payload instead of failing hard.
	return mapInstances([]json.RawMessage{body}), nil
}

func mapInstances(raw []json.RawMessage) []ComputeInstance {
	out := make([]ComputeInstance, 0, len(raw))
	for _, r := range raw {
		ci := ComputeInstance{Raw: append(json.RawMessage(nil), r...)}
		// Best-effort promotion of common fields; ignore decode noise.
		_ = json.Unmarshal(r, &struct {
			ID        *string `json:"id"`
			Status    *string `json:"status"`
			Size      *string `json:"size"`
			Region    *string `json:"region"`
			CreatedAt *string `json:"created_at"`
		}{
			ID:        &ci.ID,
			Status:    &ci.Status,
			Size:      &ci.Size,
			Region:    &ci.Region,
			CreatedAt: &ci.CreatedAt,
		})
		out = append(out, ci)
	}
	return out
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(bytes.TrimSpace(b[:n])) + "…"
	}
	return string(bytes.TrimSpace(b))
}
