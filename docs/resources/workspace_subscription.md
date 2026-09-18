---
page_title: "Nile: nile_workspace_subscription"
subcategory: "Workspaces"
---

# nile_workspace_subscription (Resource)

Manages the subscription of a Nile workspace via `POST/PUT/DELETE
/workspaces/{workspaceSlug}/subscription`. Creating the resource starts a subscription at the
configured `level`, changing `level` updates it in place and destroying the resource closes it.
Billing endpoints require a session (developer) token; API keys are rejected with 403.

## Example Usage

```terraform
resource "nile_workspace_subscription" "example" {
  workspace_slug = nile_workspace.example.slug
  level          = "paid"
}
```

## Argument Reference

The following arguments are supported:

- `level` - (Required) Subscription level, for example `free` or `paid`. Changing it updates the subscription in place.
- `workspace_slug` - (Required) Slug of the workspace whose subscription is managed. Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `subscription_id` - (Computed) Identifier of the active subscription; used to close it on destroy.
- `valid_from` - (Computed) Start of the subscription validity window.
- `valid_to` - (Computed) End of the subscription validity window.
- `workspace` - (Computed) Workspace reference reported by the API.

## Notes

### Level changes

Changing `level` updates the subscription in place via the change endpoint.
Destroying the resource closes the subscription. Billing endpoints require a
session (developer) token; API keys are rejected with 403.

## Import

Import an existing object with:

```sh
terraform import nile_workspace_subscription.example my-workspace
```

