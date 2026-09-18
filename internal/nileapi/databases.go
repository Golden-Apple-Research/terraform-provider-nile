// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
)

// CreateDatabaseRequest is the body of CreateDatabase.
type CreateDatabaseRequest struct {
	DatabaseName string `json:"databaseName"`
	Region       string `json:"region"`
}

// UpdateDatabaseRequest is the body of RenameDatabase.
type UpdateDatabaseRequest struct {
	Name string `json:"name"`
}

// ClaimDatabaseRequest is the body of ClaimDatabase.
type ClaimDatabaseRequest struct {
	ClaimCode string `json:"claimCode"`
}

// UnauthenticatedProvisionDatabaseRequest is the body of ProvisionDatabase.
type UnauthenticatedProvisionDatabaseRequest struct {
	Region string `json:"region"`
}

// ListDatabases calls GET /workspaces/{workspaceSlug}/databases.
func (c *Client) ListDatabases(ctx context.Context, workspaceSlug string) ([]Database, error) {
	var out []Database
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "databases"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateDatabase calls POST /workspaces/{workspaceSlug}/databases. Creation is
// asynchronous; use WaitForDatabaseReady to block until the database is
// usable.
func (c *Client) CreateDatabase(ctx context.Context, workspaceSlug string, req CreateDatabaseRequest) (Database, error) {
	var out Database
	if err := c.post(ctx, c.endpoint("workspaces", workspaceSlug, "databases"), req, &out); err != nil {
		return Database{}, err
	}
	return out, nil
}

// GetDatabase calls GET /workspaces/{workspaceSlug}/databases/{databaseName}.
func (c *Client) GetDatabase(ctx context.Context, workspaceSlug, databaseName string) (Database, error) {
	var out Database
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "databases", databaseName), &out); err != nil {
		return Database{}, err
	}
	return out, nil
}

// RenameDatabase calls PUT /workspaces/{workspaceSlug}/databases/{databaseName}.
func (c *Client) RenameDatabase(ctx context.Context, workspaceSlug, databaseName, newName string) (Database, error) {
	var out Database
	if err := c.put(ctx, c.endpoint("workspaces", workspaceSlug, "databases", databaseName),
		UpdateDatabaseRequest{Name: newName}, &out); err != nil {
		return Database{}, err
	}
	return out, nil
}

// DeleteDatabase calls DELETE /workspaces/{workspaceSlug}/databases/{databaseName}.
// The database is queued for deletion; the call itself only marks it.
func (c *Client) DeleteDatabase(ctx context.Context, workspaceSlug, databaseName string) (Database, error) {
	var out Database
	if err := c.delete(ctx, c.endpoint("workspaces", workspaceSlug, "databases", databaseName), &out); err != nil {
		return Database{}, err
	}
	return out, nil
}

// ProvisionDatabase calls POST /databases/provision without an authenticated
// workspace. The API documents neither the request nor the response fully, so
// only the claim code (needed for ClaimDatabase) is promoted and the full
// payload stays in Raw.
func (c *Client) ProvisionDatabase(ctx context.Context, region string) (ProvisionedDatabase, error) {
	var out ProvisionedDatabase
	if err := c.post(ctx, c.endpoint("databases", "provision"),
		UnauthenticatedProvisionDatabaseRequest{Region: region}, &out); err != nil {
		return ProvisionedDatabase{}, err
	}
	return out, nil
}

// ClaimDatabase calls POST /workspaces/{workspaceSlug}/databases/claim with a
// claim code from ProvisionDatabase.
func (c *Client) ClaimDatabase(ctx context.Context, workspaceSlug, claimCode string) (Database, error) {
	var out Database
	if err := c.post(ctx, c.endpoint("workspaces", workspaceSlug, "databases", "claim"),
		ClaimDatabaseRequest{ClaimCode: claimCode}, &out); err != nil {
		return Database{}, err
	}
	return out, nil
}
