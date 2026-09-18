---
page_title: "Nile: nile_workspace_subscription"
subcategory: "Workspaces"
---

# nile_workspace_subscription (Data Source)

Fetches the current subscription of a workspace via `GET
/workspaces/{workspaceSlug}/subscription`.

## Example Usage

```terraform
data "nile_workspace_subscription" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference
  - `workspace_slug` - (Required) Slug of the workspace whose subscription is read.

## Attribute Reference
  - `default_payment_method` - (Computed) Default payment method of the subscription, if any.
  - `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
  - `level` - (Computed) Current subscription level.
  - `raw_json` - (Computed) Redacted JSON payload of the subscription as returned by the API.
  - `subscription_id` - (Computed) Identifier of the current subscription.
  - `valid_from` - (Computed) Start of the current subscription period.
  - `valid_to` - (Computed) End of the current subscription period, if scheduled.

