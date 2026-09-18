// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringOrNull maps an API string to a Terraform value: the empty string means
// "not set" and becomes null, so optional API fields do not show up as ""
// in state.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// optionalBool turns a configured bool into the *bool the API client expects
// for optional query parameters: null and unknown mean "not set".
func optionalBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// splitResourceID splits an import identifier into exactly n non-empty parts.
// Import IDs use the form "part1/part2/...". Parts may be URL-escaped so a
// workspace or database name containing a slash remains unambiguous.
func splitResourceID(id string, n int) ([]string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != n {
		return nil, fmt.Errorf("expected %d slash-separated parts, got %d", n, len(parts))
	}
	for i, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("parts must not be empty")
		}
		decoded, err := url.PathUnescape(p)
		if err != nil {
			return nil, fmt.Errorf("invalid URL escaping in part %d: %w", i+1, err)
		}
		if decoded == "" {
			return nil, fmt.Errorf("parts must not be empty")
		}
		parts[i] = decoded
	}
	return parts, nil
}

// joinResourceID builds an import identifier from its parts. Escaping each
// part keeps slash-containing names compatible with splitResourceID.
func joinResourceID(parts ...string) string {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = url.PathEscape(part)
	}
	return strings.Join(escaped, "/")
}

// redactedRawJSON preserves useful API payloads without putting common secret
// fields into Terraform state or diagnostics. Invalid or empty payloads are
// represented as null rather than risking storage of an unredacted value.
func redactedRawJSON(raw json.RawMessage) types.String {
	if len(raw) == 0 {
		return types.StringNull()
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return types.StringNull()
	}
	redactJSONSecrets(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(encoded))
}

func apiFieldPresent(raw json.RawMessage, field string) bool {
	if len(raw) == 0 {
		// A manually constructed API value has no presence metadata; treat its
		// typed fields as authoritative in that case.
		return true
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	_, present := object[field]
	return present
}

func redactJSONSecrets(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if isSensitiveJSONKey(key) {
				value[key] = "[REDACTED]"
				continue
			}
			redactJSONSecrets(child)
		}
	case []any:
		for _, child := range value {
			redactJSONSecrets(child)
		}
	}
}

func isSensitiveJSONKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	switch normalized {
	case "password", "code", "secret", "claimcode", "token", "accesstoken", "refreshtoken", "apitoken", "clientsecret":
		return true
	default:
		return false
	}
}
