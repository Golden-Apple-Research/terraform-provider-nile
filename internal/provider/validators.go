package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// isRFC3339Validator checks that a string attribute holds an RFC3339
// timestamp. Applying it at schema level means misconfigured values are
// reported by `terraform validate`/`plan` before any API call is made.
type isRFC3339Validator struct{}

var _ validator.String = isRFC3339Validator{}

// Description returns a human-readable description of the validator.
func (v isRFC3339Validator) Description(_ context.Context) string {
	return "value must be an RFC3339 timestamp"
}

// MarkdownDescription returns a markdown description of the validator.
func (v isRFC3339Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v isRFC3339Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		// Null is "not configured"; unknown values may still become valid
		// once referenced values are known, so both are skipped here.
		return
	}
	if _, err := time.Parse(time.RFC3339, req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			fmt.Sprintf("Invalid value for %q", req.Path.String()),
			fmt.Sprintf("The value must be an RFC3339 timestamp, got: %q", req.ConfigValue.ValueString()),
		)
	}
}
