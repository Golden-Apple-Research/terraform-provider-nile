// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
	"fmt"
)

// RotateCredentialRequest is the body of RotateCredential. The zero value
// rotates immediately; DelayOldSecretsExpirationHours keeps existing
// credentials valid for that many hours.
type RotateCredentialRequest struct {
	DelayOldSecretsExpirationHours int64  `json:"delayOldSecretsExpirationHours,omitempty"`
	Reason                         string `json:"reason,omitempty"`
}

// ListCredentials calls
// GET /workspaces/{workspaceSlug}/databases/{databaseName}/credentials.
// tenantID and internal are optional filters.
func (c *Client) ListCredentials(ctx context.Context, workspaceSlug, databaseName, tenantID string, internal *bool) ([]Credential, error) {
	u := withBoolQuery(
		withQuery(c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "credentials"), "tenantId", tenantID),
		"internal", internal,
	)
	var out []Credential
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateCredential calls
// POST /workspaces/{workspaceSlug}/databases/{databaseName}/credentials.
// The generated password is returned exactly once, in Credential.Password.
func (c *Client) CreateCredential(ctx context.Context, workspaceSlug, databaseName, tenantID string, internal *bool) (Credential, error) {
	u := withBoolQuery(
		withQuery(c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "credentials"), "tenantId", tenantID),
		"internal", internal,
	)
	var out Credential
	if err := c.post(ctx, u, nil, &out); err != nil {
		return Credential{}, err
	}
	return out, nil
}

// RotateCredential calls
// POST /workspaces/{workspaceSlug}/databases/{databaseName}/credentials/rotate.
// The API does not document the response schema, so the new password is
// promoted best-effort: check Credential.Password (and Credential.Raw when it
// is empty).
func (c *Client) RotateCredential(ctx context.Context, workspaceSlug, databaseName, tenantID string, internal *bool, req RotateCredentialRequest) (Credential, error) {
	u := withBoolQuery(
		withQuery(c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "credentials", "rotate"), "tenantId", tenantID),
		"internal", internal,
	)
	var out Credential
	if err := c.post(ctx, u, req, &out); err != nil {
		return Credential{}, err
	}
	return out, nil
}

// DeleteCredential calls
// DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/credentials/{credentialId}.
func (c *Client) DeleteCredential(ctx context.Context, workspaceSlug, databaseName, credentialID string) error {
	if credentialID == "" {
		return fmt.Errorf("credential ID must not be empty")
	}
	u := c.endpoint("workspaces", workspaceSlug, "databases", databaseName, "credentials", credentialID)
	return c.delete(ctx, u, nil)
}
