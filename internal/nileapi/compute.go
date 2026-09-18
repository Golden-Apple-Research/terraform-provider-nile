// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	// Pagination safety valves. The API currently returns all instances in a
	// single response; the page cap only guards against a server that keeps
	// handing out fresh continuation tokens forever.
	maxPages       = 1000
	pageTokenParam = "pageToken"
)

// ListComputeInstances calls
// GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute
// start/end are optional RFC3339 timestamps restricting the result to
// instances active in that time window.
//
// The documented response is a single bare array. If the API ever wraps the
// list in an object carrying a continuation token, all pages are followed
// automatically and concatenated, so callers never see partial results.
func (c *Client) ListComputeInstances(ctx context.Context, workspaceSlug, databaseName, start, end string) ([]ComputeInstance, error) {
	var all []ComputeInstance
	pageToken := ""
	for page := 0; ; page++ {
		if page >= maxPages {
			return nil, fmt.Errorf("listing compute instances exceeded %d pages; aborting to avoid an unbounded pagination loop", maxPages)
		}

		u := withQuery(
			c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "compute"),
			"start", start, "end", end, pageTokenParam, pageToken,
		)
		var body json.RawMessage
		if err := c.get(ctx, u, &body); err != nil {
			return nil, err
		}

		instances, next, err := decodeInstances(ctx, body)
		if err != nil {
			return nil, fmt.Errorf("decoding response from %s: %w", u, err)
		}
		all = append(all, instances...)
		if next == "" {
			return all, nil
		}
		// A token identical to the one just sent means the server did not
		// advance; erroring out beats looping until the page cap.
		if next == pageToken {
			return nil, fmt.Errorf("pagination did not advance: the API repeated page token %q", next)
		}
		tflog.Debug(ctx, "following pagination token", map[string]any{"next_page": page + 2, "token": next})
		pageToken = next
	}
}

// CreateComputeInstanceRequest is the body of CreateComputeInstance.
type CreateComputeInstanceRequest struct {
	InstanceName string `json:"instanceName"`
	InstanceSize string `json:"instanceSize"`
}

// UpdateComputeInstanceRequest is the body of UpdateComputeInstance. Empty
// fields are omitted, which makes each field independently optional.
type UpdateComputeInstanceRequest struct {
	InstanceName string `json:"instanceName,omitempty"`
	InstanceSize string `json:"instanceSize,omitempty"`
}

// CreateComputeInstance calls
// POST /workspaces/{workspaceSlug}/databases/{databaseName}/compute.
// The API may answer 200 or 202; both carry the created instance.
func (c *Client) CreateComputeInstance(ctx context.Context, workspaceSlug, databaseName, instanceName, instanceSize string) (ComputeInstance, error) {
	u := c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "compute")
	var body json.RawMessage
	if err := c.post(ctx, u, CreateComputeInstanceRequest{
		InstanceName: instanceName,
		InstanceSize: instanceSize,
	}, &body); err != nil {
		return ComputeInstance{}, err
	}
	return decodeComputeInstance(ctx, body, "")
}

// DescribeComputeInstance calls
// GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute/{instanceId}.
// The documented response is an array; a single object is also accepted.
func (c *Client) DescribeComputeInstance(ctx context.Context, workspaceSlug, databaseName, instanceID string) (ComputeInstance, error) {
	u := c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "compute", instanceID)
	var body json.RawMessage
	if err := c.get(ctx, u, &body); err != nil {
		return ComputeInstance{}, err
	}
	return decodeComputeInstance(ctx, body, instanceID)
}

// UpdateComputeInstance calls
// PUT /workspaces/{workspaceSlug}/databases/{databaseName}/compute/{instanceId}
// to rename and/or resize an instance. The response carries the database, not
// the instance, so callers that need the new state should describe the
// instance (or wait for it) afterwards.
func (c *Client) UpdateComputeInstance(ctx context.Context, workspaceSlug, databaseName, instanceID string, req UpdateComputeInstanceRequest) error {
	u := c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "compute", instanceID)
	return c.put(ctx, u, req, nil)
}

// DeleteComputeInstance calls
// DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/compute/{instanceId}.
// The deletion is queued; use WaitForComputeInstanceDeleted to block until the
// instance is gone.
func (c *Client) DeleteComputeInstance(ctx context.Context, workspaceSlug, databaseName, instanceID string) error {
	u := c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "compute", instanceID)
	return c.delete(ctx, u, nil)
}

// decodeComputeInstance accepts both a single instance object and the
// documented array shape and returns the instance matching wantID (or the
// first one when wantID is empty or not present).
func decodeComputeInstance(ctx context.Context, payload []byte, wantID string) (ComputeInstance, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return ComputeInstance{}, fmt.Errorf("empty compute instance response")
	}
	if trimmed[0] == '[' {
		instances, _, err := decodeInstances(ctx, trimmed)
		if err != nil {
			return ComputeInstance{}, err
		}
		if len(instances) == 0 {
			return ComputeInstance{}, fmt.Errorf("compute instance response contained no instances")
		}
		if wantID != "" {
			for _, instance := range instances {
				if instance.ID == wantID {
					return instance, nil
				}
			}
		}
		return instances[0], nil
	}
	if trimmed[0] != '{' {
		return ComputeInstance{}, fmt.Errorf("unexpected compute instance response shape: %s", truncate(trimmed, 256))
	}
	return mapInstances(ctx, []json.RawMessage{trimmed})[0], nil
}

// wrapperKeys are the object keys that may contain the instance array in
// responses that wrap the list instead of returning a bare array.
var wrapperKeys = []string{"instances", "compute", "items", "data", "results"}

// continuationKeys are object keys that may carry the token for the next
// page in a wrapped response. The Nile API does not paginate today; these
// exist so the provider keeps working unchanged if it starts to.
var continuationKeys = []string{"nextPageToken", "next_page_token", "nextCursor", "cursor"}

// decodeInstances decodes one page. Besides the instances it returns the
// continuation token, if the page carries one ("" means: last page).
func decodeInstances(ctx context.Context, body []byte) ([]ComputeInstance, string, error) {
	// The documented response is a bare JSON array. A bare JSON null is
	// rejected instead of being treated as an empty list: silently accepting
	// it would hide API changes from the user.
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		if raw == nil {
			return nil, "", fmt.Errorf("unexpected JSON shape: null")
		}
		return mapInstances(ctx, raw), "", nil
	}

	// Tolerate a wrapper object, but only if it actually contains an array:
	// silently treating an unknown object as a single instance would hide API
	// changes from the user.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, "", fmt.Errorf("unexpected JSON shape: %v", err)
	}
	for _, key := range wrapperKeys {
		if inner, ok := wrapper[key]; ok {
			if err := json.Unmarshal(inner, &raw); err != nil {
				return nil, "", fmt.Errorf("wrapper key %q does not contain an array: %v", key, err)
			}
			if raw == nil {
				return nil, "", fmt.Errorf("wrapper key %q is null: expected an array", key)
			}
			return mapInstances(ctx, raw), continuationToken(wrapper), nil
		}
	}
	return nil, "", fmt.Errorf("unexpected JSON shape: expected an array or an object with one of the keys %s",
		strings.Join(wrapperKeys, ", "))
}

// continuationToken extracts a non-empty string token from any of the
// continuation keys. Missing, empty, or non-string values count as "no
// continuation".
func continuationToken(wrapper map[string]json.RawMessage) string {
	for _, key := range continuationKeys {
		if v, ok := wrapper[key]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// apiComputeType mirrors ComputeInstanceType with nil-able fields so one bad
// field does not discard the rest of the payload.
type apiComputeType struct {
	ID          *string  `json:"id"`
	ComputeSize *string  `json:"computeSize"`
	Memory      *string  `json:"memory"`
	HourlyCost  *float64 `json:"hourlyCost"`
}

// apiInstance mirrors the documented API fields promoted by the provider.
// Fields the API adds later stay available via ComputeInstance.Raw.
type apiInstance struct {
	InstanceID          *string         `json:"instanceId"`
	InstanceName        *string         `json:"instanceName"`
	Status              *string         `json:"status"`
	Region              *string         `json:"region"`
	Created             *string         `json:"created"`
	Updated             *string         `json:"updated"`
	Deleted             *string         `json:"deleted"`
	InstanceType        *apiComputeType `json:"instanceType"`
	DesiredInstanceType *apiComputeType `json:"desiredInstanceType"`
	Workspace           *Workspace      `json:"workspace"`
	Database            *Database       `json:"database"`
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
		ci.Updated = derefString(ai.Updated)
		ci.Deleted = derefString(ai.Deleted)
		ci.InstanceType = computeTypeFromAPI(ai.InstanceType)
		ci.DesiredInstanceType = computeTypeFromAPI(ai.DesiredInstanceType)
		ci.Workspace = ai.Workspace
		ci.Database = ai.Database
		if ci.InstanceType != nil {
			ci.Size = ci.InstanceType.ComputeSize
			ci.Memory = ci.InstanceType.Memory
			ci.HourlyCost = ci.InstanceType.HourlyCost
		}
		out = append(out, ci)
	}
	return out
}

func computeTypeFromAPI(t *apiComputeType) *ComputeInstanceType {
	if t == nil {
		return nil
	}
	ct := &ComputeInstanceType{
		ID:          derefString(t.ID),
		ComputeSize: derefString(t.ComputeSize),
		Memory:      derefString(t.Memory),
	}
	if t.HourlyCost != nil {
		ct.HourlyCost = *t.HourlyCost
	}
	return ct
}
