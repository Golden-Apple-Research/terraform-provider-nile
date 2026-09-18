// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringOrNull(t *testing.T) {
	if got := stringOrNull(""); !got.IsNull() {
		t.Errorf("empty string should map to null, got %v", got)
	}
	if got := stringOrNull("value"); got.ValueString() != "value" {
		t.Errorf("ValueString() = %q, want %q", got.ValueString(), "value")
	}
}

func TestOptionalBool(t *testing.T) {
	if got := optionalBool(types.BoolNull()); got != nil {
		t.Errorf("null must map to nil, got %v", *got)
	}
	if got := optionalBool(types.BoolUnknown()); got != nil {
		t.Errorf("unknown must map to nil, got %v", *got)
	}
	if got := optionalBool(types.BoolValue(true)); got == nil || !*got {
		t.Errorf("true must map to &true, got %v", got)
	}
	if got := optionalBool(types.BoolValue(false)); got == nil || *got {
		t.Errorf("false must map to &false, got %v", got)
	}
}

func TestSplitResourceID(t *testing.T) {
	parts, err := splitResourceID("ws/db/inst-1", 3)
	if err != nil || len(parts) != 3 || parts[2] != "inst-1" {
		t.Errorf("splitResourceID = %v, %v", parts, err)
	}
	encoded, err := splitResourceID("ws/foo%2Fbar/inst%2F1", 3)
	if err != nil || len(encoded) != 3 || encoded[1] != "foo/bar" || encoded[2] != "inst/1" {
		t.Errorf("splitResourceID encoded = %v, %v", encoded, err)
	}
	if got := joinResourceID("ws", "foo/bar", "inst/1"); got != "ws/foo%2Fbar/inst%2F1" {
		t.Errorf("joinResourceID = %q", got)
	}
	for _, id := range []string{"ws/db", "ws//db", "ws/db/", "/ws/db", ""} {
		if _, err := splitResourceID(id, 3); err == nil {
			t.Errorf("expected error for %q", id)
		}
	}
	// Percent-sequences that are not valid path escapes must be rejected
	// instead of silently producing a broken name.
	if _, err := splitResourceID("ws/db%zz", 3); err == nil {
		t.Error("expected error for invalid URL escaping")
	}
}

func TestRedactedRawJSON(t *testing.T) {
	got := redactedRawJSON(json.RawMessage(`{"password":"pw","code":"invite","nested":{"access_token":"token","apiKey":"api-key","secret_key":"secret-key","privateKey":"private-key"},"name":"safe"}`))
	value := got.ValueString()
	for _, secret := range []string{`:"pw"`, `:"invite"`, `:"token"`, `:"api-key"`, `:"secret-key"`, `:"private-key"`} {
		if strings.Contains(value, secret) {
			t.Errorf("redacted JSON contains secret %q: %s", secret, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") || !strings.Contains(value, "safe") {
		t.Errorf("redacted JSON = %s", value)
	}

	// Empty and undecodable payloads must become null rather than risk
	// storing an unredacted value.
	if got := redactedRawJSON(nil); !got.IsNull() {
		t.Errorf("empty payload = %v, want null", got)
	}
	if got := redactedRawJSON(json.RawMessage(`{not json`)); !got.IsNull() {
		t.Errorf("invalid payload = %v, want null", got)
	}
}

func TestAPIFieldPresent(t *testing.T) {
	// An empty raw payload means the value was built locally, not decoded;
	// its typed fields are treated as authoritative.
	if !apiFieldPresent(nil, "anything") {
		t.Error("empty payload should report presence")
	}
	if !apiFieldPresent(json.RawMessage(`{"created":"2025-06-01T00:00:00Z"}`), "created") {
		t.Error("field should be reported present")
	}
	if apiFieldPresent(json.RawMessage(`{"created":"2025-06-01T00:00:00Z"}`), "deleted") {
		t.Error("missing field should be reported absent")
	}
	if apiFieldPresent(json.RawMessage(`{"created":`), "created") {
		t.Error("undecodable payload should be treated as absent")
	}
	// Explicit JSON null still counts as present: the API answered the field.
	if !apiFieldPresent(json.RawMessage(`{"deleted":null}`), "deleted") {
		t.Error("explicit null should count as present")
	}
}

func TestIsSensitiveJSONKey(t *testing.T) {
	sensitive := []string{
		"password", "PASSWORD", "Password", "dbPassword", "passwd",
		"code", "claimCode", "claim_code", "claim-code",
		"secret", "clientSecret", "client_secret", "secretKey", "secret_key",
		"token", "accessToken", "access_token", "refreshToken", "apiToken", "api-token",
		"idToken", "session_token", "apiKey", "api_key", "privateKey", "private-key",
		"authorization", "bearer",
	}
	for _, key := range sensitive {
		if !isSensitiveJSONKey(key) {
			t.Errorf("%q should be sensitive", key)
		}
	}
	for _, key := range []string{"id", "name", "email", "createdAt", "codesharing", "tokenizer", "monkey", "keyboard", "accessKeyId"} {
		if isSensitiveJSONKey(key) {
			t.Errorf("%q should not be sensitive", key)
		}
	}
}
