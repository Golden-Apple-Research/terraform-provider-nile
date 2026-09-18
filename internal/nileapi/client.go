// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

// Package nileapi is a minimal client for the Nile control plane REST API.
package nileapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	DefaultBaseURL = "https://global.thenile.dev"
)

// Client talks to the Nile REST API using bearer-token authentication.
type Client struct {
	BaseURL   *url.URL
	AuthToken string
	// UserAgent, when set, is sent as the User-Agent header on requests so
	// the API can identify the provider.
	UserAgent string
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
	// Catch scheme mistakes (e.g. a bare "global.thenile.dev") here rather
	// than at first request time with an opaque "unsupported protocol scheme"
	// error from net/http.
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid base URL %q: it must include an http:// or https:// scheme", baseURL)
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
//
// The promoted fields map to the API response as follows:
//
//	ID        <- instanceId
//	Name      <- instanceName
//	Status    <- status
//	Size      <- instanceType.computeSize
//	Region    <- region
//	CreatedAt <- created
//
// Attributes the API adds or renames later remain accessible via Raw.
type ComputeInstance struct {
	ID        string
	Name      string
	Status    string
	Size      string
	Region    string
	CreatedAt string
	Raw       json.RawMessage
}

// ListComputeInstances calls
// GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute
// start/end are optional RFC3339 timestamps restricting the result to
// instances active in that time window.
func (c *Client) ListComputeInstances(ctx context.Context, workspaceSlug, databaseName, start, end string) ([]ComputeInstance, error) {
	// Escape the path segments individually so that characters like "/" in a
	// slug or database name cannot change the request path. url.Parse keeps the
	// escaped form in RawPath, which ResolveReference preserves.
	rel, err := url.Parse(fmt.Sprintf("/workspaces/%s/databases/%s/compute",
		url.PathEscape(workspaceSlug), url.PathEscape(databaseName)))
	if err != nil {
		return nil, fmt.Errorf("building request path: %w", err)
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
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

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
	instances, err := decodeInstances(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("decoding response from %s: %w", endpoint.String(), err)
	}
	return instances, nil
}

// wrapperKeys are the object keys that may contain the instance array in
// responses that wrap the list instead of returning a bare array.
var wrapperKeys = []string{"instances", "compute", "items", "data", "results"}

func decodeInstances(ctx context.Context, body []byte) ([]ComputeInstance, error) {
	// The documented response is a bare JSON array. A bare JSON null is
	// rejected instead of being treated as an empty list: silently accepting
	// it would hide API changes from the user.
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		if raw == nil {
			return nil, fmt.Errorf("unexpected JSON shape: null")
		}
		return mapInstances(ctx, raw), nil
	}

	// Tolerate a wrapper object, but only if it actually contains an array:
	// silently treating an unknown object as a single instance would hide API
	// changes from the user.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("unexpected JSON shape: %v", err)
	}
	for _, key := range wrapperKeys {
		if inner, ok := wrapper[key]; ok {
			if err := json.Unmarshal(inner, &raw); err != nil {
				return nil, fmt.Errorf("wrapper key %q does not contain an array: %v", key, err)
			}
			if raw == nil {
				return nil, fmt.Errorf("wrapper key %q is null: expected an array", key)
			}
			return mapInstances(ctx, raw), nil
		}
	}
	return nil, fmt.Errorf("unexpected JSON shape: expected an array or an object with one of the keys %s",
		strings.Join(wrapperKeys, ", "))
}

// apiInstance mirrors the documented API fields promoted by the provider.
// Fields the API adds later stay available via ComputeInstance.Raw.
type apiInstance struct {
	InstanceID   *string `json:"instanceId"`
	InstanceName *string `json:"instanceName"`
	Status       *string `json:"status"`
	Region       *string `json:"region"`
	Created      *string `json:"created"`
	InstanceType *struct {
		ComputeSize *string `json:"computeSize"`
	} `json:"instanceType"`
}

func mapInstances(ctx context.Context, raw []json.RawMessage) []ComputeInstance {
	out := make([]ComputeInstance, 0, len(raw))
	for _, r := range raw {
		ci := ComputeInstance{Raw: append(json.RawMessage(nil), r...)}
		// Best-effort promotion of the documented fields; a type mismatch in
		// one field must not discard the others. The payload stays available
		// via Raw, and the mismatch is surfaced as a log warning instead of
		// being silently ignored.
		var ai apiInstance
		if err := json.Unmarshal(r, &ai); err != nil {
			tflog.Warn(ctx, "could not promote instance fields; only raw_json will be populated", map[string]any{
				"error": err.Error(),
				"raw":   truncate(r, 256),
			})
		}
		ci.ID = derefString(ai.InstanceID)
		ci.Name = derefString(ai.InstanceName)
		ci.Status = derefString(ai.Status)
		ci.Region = derefString(ai.Region)
		ci.CreatedAt = derefString(ai.Created)
		if ai.InstanceType != nil {
			ci.Size = derefString(ai.InstanceType.ComputeSize)
		}
		out = append(out, ci)
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
