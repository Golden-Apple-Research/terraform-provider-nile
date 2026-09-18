---
page_title: "Nile: nile_workspaces"
subcategory: "Workspaces"
---

# nile_workspaces (Data Source)

Lists all Nile workspaces the authenticated developer has access to, via `GET /workspaces`.

## Example Usage

```terraform
data "nile_workspaces" "example" {}
```

## Argument Reference

This object has no arguments.

## Attribute Reference

The following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (`workspaces`).
- `workspaces` - (Computed) The workspaces visible to the authenticated developer.
  - `created` - (Computed) Creation timestamp.
  - `id` - (Computed) Workspace identifier (`id` in the API response).
  - `name` - (Computed) Workspace name.
  - `slug` - (Computed) Globally unique workspace slug used in API paths.
  - `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, if any.

