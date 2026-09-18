// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package nileapi

import (
	"context"
	"net/url"
)

// ExchangeToken calls POST /oauth2/token with the given form values. The
// provider itself authenticates with a bearer token; this endpoint is exposed
// for completeness (for example to exchange a refresh token for a new access
// token).
func (c *Client) ExchangeToken(ctx context.Context, form url.Values) (TokenResponse, error) {
	var out TokenResponse
	if err := c.post(ctx, c.endpoint("oauth2", "token"), form, &out); err != nil {
		return TokenResponse{}, err
	}
	return out, nil
}
