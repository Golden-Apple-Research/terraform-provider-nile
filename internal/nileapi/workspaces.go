// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// CreateWorkspaceRequest is the body of CreateWorkspace.
type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

// ListWorkspaces calls GET /workspaces.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var out []Workspace
	if err := c.get(ctx, c.endpoint("workspaces"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateWorkspace calls POST /workspaces.
func (c *Client) CreateWorkspace(ctx context.Context, name string) (Workspace, error) {
	var out Workspace
	if err := c.post(ctx, c.endpoint("workspaces"), CreateWorkspaceRequest{Name: name}, &out); err != nil {
		return Workspace{}, err
	}
	return out, nil
}

// GetWorkspace calls GET /workspaces/{workspaceSlug}. The live API answers
// with a single workspace object; an array response (the shape older mocks
// assumed) is accepted as well, with the first entry returned.
func (c *Client) GetWorkspace(ctx context.Context, workspaceSlug string) (Workspace, error) {
	var raw json.RawMessage
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug), &raw); err != nil {
		return Workspace{}, err
	}
	if bytes.HasPrefix(bytes.TrimLeft(raw, " \t\r\n"), []byte("[")) {
		var list []Workspace
		if err := json.Unmarshal(raw, &list); err != nil {
			return Workspace{}, err
		}
		if len(list) == 0 {
			return Workspace{}, &APIError{
				StatusCode: 404,
				ErrorCode:  "entity_not_found",
				Message:    fmt.Sprintf("workspace %q not found", workspaceSlug),
			}
		}
		return list[0], nil
	}
	var out Workspace
	if err := json.Unmarshal(raw, &out); err != nil {
		return Workspace{}, err
	}
	return out, nil
}

// ListRegions calls GET /workspaces/{workspaceSlug}/regions.
func (c *Client) ListRegions(ctx context.Context, workspaceSlug string) ([]string, error) {
	var out []string
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "regions"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListComputeTypes calls GET /workspaces/{workspaceSlug}/compute-types.
func (c *Client) ListComputeTypes(ctx context.Context, workspaceSlug string) ([]ComputeInstanceType, error) {
	var out []ComputeInstanceType
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "compute-types"), &out); err != nil {
		return nil, err
	}
	return out, nil
}
