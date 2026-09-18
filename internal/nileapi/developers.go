// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
)

// CreateInviteRequest is the body of CreateDeveloperInvite. When Programmatic
// is true, the response includes the invite code instead of only sending an
// email.
type CreateInviteRequest struct {
	Email        string `json:"email"`
	Programmatic bool   `json:"programmatic,omitempty"`
}

// GetDeveloper calls GET /developers/me and returns the authenticated
// developer.
func (c *Client) GetDeveloper(ctx context.Context) (Developer, error) {
	var out Developer
	if err := c.get(ctx, c.endpoint("developers", "me"), &out); err != nil {
		return Developer{}, err
	}
	return out, nil
}

// ListWorkspaceDevelopers calls GET /workspaces/{workspaceSlug}/developers.
func (c *Client) ListWorkspaceDevelopers(ctx context.Context, workspaceSlug string) ([]Developer, error) {
	var out []Developer
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "developers"), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveWorkspaceDeveloper calls
// DELETE /workspaces/{workspaceSlug}/developers/{developerId}.
func (c *Client) RemoveWorkspaceDeveloper(ctx context.Context, workspaceSlug, developerID string) error {
	return c.delete(ctx, c.endpoint("workspaces", workspaceSlug, "developers", developerID), nil)
}

// ListDeveloperInvites calls GET /workspaces/{workspaceSlug}/invites.
// verificationState optionally filters by EMAIL_PENDING, EMAIL_SENT, VERIFIED
// or EXPIRED.
func (c *Client) ListDeveloperInvites(ctx context.Context, workspaceSlug, verificationState string) ([]DeveloperInvite, error) {
	u := withQuery(c.endpoint("workspaces", workspaceSlug, "invites"), "verificationState", verificationState)
	var out []DeveloperInvite
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateDeveloperInvite calls POST /workspaces/{workspaceSlug}/invites. The
// response is a DeveloperInvite, extended with the invite code when
// programmatic is true.
func (c *Client) CreateDeveloperInvite(ctx context.Context, workspaceSlug, email string, programmatic bool) (DeveloperInvite, error) {
	var out DeveloperInvite
	if err := c.post(ctx, c.endpoint("workspaces", workspaceSlug, "invites"),
		CreateInviteRequest{Email: email, Programmatic: programmatic}, &out); err != nil {
		return DeveloperInvite{}, err
	}
	return out, nil
}

// DeleteDeveloperInvite calls
// DELETE /workspaces/{workspaceSlug}/invites/{inviteId}.
func (c *Client) DeleteDeveloperInvite(ctx context.Context, workspaceSlug, inviteID string) error {
	return c.delete(ctx, c.endpoint("workspaces", workspaceSlug, "invites", inviteID), nil)
}
