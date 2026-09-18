---
page_title: "Nile: nile_workspace_subscription_history"
subcategory: "Workspaces"
---

# nile_workspace_subscription_history (Data Source)

Lists all subscription records of a workspace, most recent first, via `GET
/workspaces/{workspaceSlug}/subscription/history`.

## Example Usage

```terraform
data "nile_workspace_subscription_history" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference
  - `workspace_slug` - (Required) Slug of the workspace whose subscription history is read.

## Attribute Reference
  - `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
  - `subscriptions` - (Computed) Subscription records, most recent first.
    - `default_payment_method` - (Computed) Default payment method of the subscription, if any.
    - `level` - (Computed) Subscription level.
    - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the subscription as returned by the API.
    - `subscription_id` - (Computed) Subscription identifier.
    - `valid_from` - (Computed) Start of the subscription period.
    - `valid_to` - (Computed) End of the subscription period, if scheduled.
    - `workspace` - (Computed) Workspace the subscription belongs to.

