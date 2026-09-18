// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
)

// WorkspaceSubscriptionRequest is the body of StartSubscription and
// ChangeSubscription.
type WorkspaceSubscriptionRequest struct {
	Level string `json:"level"`
}

// EnsureBillingCustomer calls
// PUT /workspaces/{workspaceSlug}/billing/customer. It finds or creates the
// Stripe customer linked to the workspace; the call has side effects and is
// therefore not exposed as a read-only data source.
func (c *Client) EnsureBillingCustomer(ctx context.Context, workspaceSlug string) (WorkspaceBillingCustomer, error) {
	var out WorkspaceBillingCustomer
	if err := c.put(ctx, c.endpoint("workspaces", workspaceSlug, "billing", "customer"), nil, &out); err != nil {
		return WorkspaceBillingCustomer{}, err
	}
	return out, nil
}

// GetBillingReadiness calls
// GET /workspaces/{workspaceSlug}/billing/readiness.
func (c *Client) GetBillingReadiness(ctx context.Context, workspaceSlug string) (WorkspaceBillingReadiness, error) {
	var out WorkspaceBillingReadiness
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "billing", "readiness"), &out); err != nil {
		return WorkspaceBillingReadiness{}, err
	}
	return out, nil
}

// GetMonthlyTotals calls
// GET /workspaces/{workspaceSlug}/billing/{ym}/totals. ym is a month in
// YYYY-MM form.
func (c *Client) GetMonthlyTotals(ctx context.Context, workspaceSlug, ym string) (WorkspaceMonthlyTotals, error) {
	var out WorkspaceMonthlyTotals
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "billing", ym, "totals"), &out); err != nil {
		return WorkspaceMonthlyTotals{}, err
	}
	return out, nil
}

// GetCurrentSubscription calls
// GET /workspaces/{workspaceSlug}/subscription.
func (c *Client) GetCurrentSubscription(ctx context.Context, workspaceSlug string) (WorkspaceSubscription, error) {
	var out WorkspaceSubscription
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "subscription"), &out); err != nil {
		return WorkspaceSubscription{}, err
	}
	return out, nil
}

// StartSubscription calls POST /workspaces/{workspaceSlug}/subscription.
func (c *Client) StartSubscription(ctx context.Context, workspaceSlug, level string) error {
	return c.post(ctx, c.endpoint("workspaces", workspaceSlug, "subscription"),
		WorkspaceSubscriptionRequest{Level: level}, nil)
}

// ChangeSubscription calls PUT /workspaces/{workspaceSlug}/subscription.
func (c *Client) ChangeSubscription(ctx context.Context, workspaceSlug, level string) error {
	return c.put(ctx, c.endpoint("workspaces", workspaceSlug, "subscription"),
		WorkspaceSubscriptionRequest{Level: level}, nil)
}

// CloseSubscription calls
// DELETE /workspaces/{workspaceSlug}/subscription/{subscriptionId}.
func (c *Client) CloseSubscription(ctx context.Context, workspaceSlug, subscriptionID string) error {
	return c.delete(ctx, c.endpoint("workspaces", workspaceSlug, "subscription", subscriptionID), nil)
}

// ListSubscriptionHistory calls
// GET /workspaces/{workspaceSlug}/subscription/history, most recent first.
func (c *Client) ListSubscriptionHistory(ctx context.Context, workspaceSlug string) ([]WorkspaceSubscription, error) {
	var out []WorkspaceSubscription
	if err := c.get(ctx, c.endpoint("workspaces", workspaceSlug, "subscription", "history"), &out); err != nil {
		return nil, err
	}
	return out, nil
}
