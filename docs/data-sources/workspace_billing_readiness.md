---
page_title: "Nile: nile_workspace_billing_readiness"
subcategory: "Workspaces"
---

# nile_workspace_billing_readiness (Data Source)

Resolves the billing readiness of a workspace via `GET
/workspaces/{workspaceSlug}/billing/readiness`, without creating a billing customer.

## Example Usage

```terraform
data "nile_workspace_billing_readiness" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference
  - `workspace_slug` - (Required) Slug of the workspace whose billing readiness is read.

## Attribute Reference
  - `checked_at` - (Computed) Timestamp of the check.
  - `default_payment_method_id` - (Computed) Default payment method of the customer, if any.
  - `detail` - (Computed) Human-readable detail about the readiness result.
  - `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
  - `last_error` - (Computed) Last error observed while resolving billing state, if any.
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the response as returned by the API.
  - `source` - (Computed) Source of the readiness result.
  - `status` - (Computed) Readiness status (`ready`, `missing_customer`, `missing_payment_method`, `lookup_failed` or `manual_review_required`).
  - `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, if any.
  - `workspace` - (Computed) Workspace slug as returned by the API.
  - `workspace_id` - (Computed) Workspace identifier.

