---
page_title: "Nile: nile_workspace"
subcategory: "Workspaces"
---

# nile_workspace (Data Source)

Fetches a single Nile workspace via `GET /workspaces/{workspaceSlug}`.

## Example Usage

```terraform
data "nile_workspace" "example" {
  slug = "my-workspace"
}
```

## Argument Reference

The following arguments are supported:

- `slug` - (Required) Globally unique slug of the workspace.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `created` - (Computed) Creation timestamp.
- `id` - (Computed) Workspace identifier (`id` in the API response).
- `name` - (Computed) Workspace name.
- `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, if any.

