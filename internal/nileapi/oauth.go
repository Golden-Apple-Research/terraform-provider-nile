// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
	"net/url"
)

// ExchangeToken calls POST /oauth2/token with the given form values. The
// request is sent unauthenticated (no bearer header), as OAuth token
// endpoints authenticate through the form body itself.
func (c *Client) ExchangeToken(ctx context.Context, form url.Values) (TokenResponse, error) {
	var out TokenResponse
	if err := c.postUnauthenticated(ctx, c.endpoint("oauth2", "token"), form, &out); err != nil {
		return TokenResponse{}, err
	}
	return out, nil
}
