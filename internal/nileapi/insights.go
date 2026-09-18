// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
	"net/url"
)

// InsightsQuery narrows a database insights request. All fields are optional.
type InsightsQuery struct {
	Start       string
	End         string
	Granularity string
}

// url returns a URL with the non-empty insights parameters applied.
func (q InsightsQuery) url(u *url.URL) *url.URL {
	return withQuery(u, "start", q.Start, "end", q.End, "granularity", q.Granularity)
}

// ListDatabaseUptime calls
// GET /workspaces/{workspaceSlug}/databases/{databaseID}/insights/uptime.
func (c *Client) ListDatabaseUptime(ctx context.Context, workspaceSlug, databaseID string, q InsightsQuery) (DatabaseUptimeResponse, error) {
	u := q.url(c.endpoint("workspaces", workspaceSlug, "databases", databaseID, "insights", "uptime"))
	var out DatabaseUptimeResponse
	if err := c.get(ctx, u, &out); err != nil {
		return DatabaseUptimeResponse{}, err
	}
	return out, nil
}

// ListDatabaseErrors calls
// GET /workspaces/{workspaceSlug}/databases/{databaseID}/insights/errors.
func (c *Client) ListDatabaseErrors(ctx context.Context, workspaceSlug, databaseID string, q InsightsQuery) (DatabaseErrorsResponse, error) {
	u := q.url(c.endpoint("workspaces", workspaceSlug, "databases", databaseID, "insights", "errors"))
	var out DatabaseErrorsResponse
	if err := c.get(ctx, u, &out); err != nil {
		return DatabaseErrorsResponse{}, err
	}
	return out, nil
}

// ListDatabaseQueryPerformance calls
// GET /workspaces/{workspaceSlug}/databases/{databaseID}/insights/query-performance.
func (c *Client) ListDatabaseQueryPerformance(ctx context.Context, workspaceSlug, databaseID string, q InsightsQuery) (DatabaseQueryPerformanceResponse, error) {
	u := q.url(c.endpoint("workspaces", workspaceSlug, "databases", databaseID, "insights", "query-performance"))
	var out DatabaseQueryPerformanceResponse
	if err := c.get(ctx, u, &out); err != nil {
		return DatabaseQueryPerformanceResponse{}, err
	}
	return out, nil
}

// ListWorkspaceComputeUsage calls
// GET /workspaces/{workspaceSlug}/metrics/compute.
func (c *Client) ListWorkspaceComputeUsage(ctx context.Context, workspaceSlug, start, end string) ([]WorkspaceComputeUsage, error) {
	u := withQuery(c.endpoint("workspaces", workspaceSlug, "metrics", "compute"), "start", start, "end", end)
	var out []WorkspaceComputeUsage
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}
