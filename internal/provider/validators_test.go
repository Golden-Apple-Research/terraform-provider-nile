// SPDX-FileCopyrightText: 2026 Golden Apple Research
// SPDX-License-Identifier: EUPL-1.2

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIsRFC3339Validator(t *testing.T) {
	v := isRFC3339Validator{}

	valid := []string{
		"2025-01-01T00:00:00Z",
		"2025-06-15T12:30:45+02:00",
		"2025-06-15T12:30:45.123-07:00",
	}
	for _, s := range valid {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: types.StringValue(s),
			Path:        path.Root("start"),
		}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%q: unexpected diagnostics: %v", s, resp.Diagnostics)
		}
	}

	invalid := []string{"", "not-a-timestamp", "2025-13-01T00:00:00Z"}
	for _, s := range invalid {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: types.StringValue(s),
			Path:        path.Root("start"),
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("%q: expected diagnostics", s)
		}
	}

	// Null and unknown values are skipped: they may become valid later.
	for _, s := range []types.String{types.StringNull(), types.StringUnknown()} {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			ConfigValue: s,
			Path:        path.Root("start"),
		}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%v: unexpected diagnostics: %v", s, resp.Diagnostics)
		}
	}
}

func TestIsRFC3339ValidatorDescriptions(t *testing.T) {
	v := isRFC3339Validator{}
	ctx := context.Background()
	if v.Description(ctx) == "" {
		t.Error("Description must not be empty")
	}
	if v.MarkdownDescription(ctx) != v.Description(ctx) {
		t.Errorf("MarkdownDescription = %q, want %q", v.MarkdownDescription(ctx), v.Description(ctx))
	}
}
