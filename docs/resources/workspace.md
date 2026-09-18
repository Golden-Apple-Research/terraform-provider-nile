---
page_title: "Nile: nile_workspace"
subcategory: "Workspaces"
---

# nile_workspace (Resource)

Manages a Nile workspace via `POST /workspaces`. The Nile API does not expose workspace
deletion: destroying this resource removes it from Terraform state but leaves the workspace in
the control plane. Renaming is not supported either, so changing `name` forces replacement
(which, given the missing delete endpoint, means creating a second workspace).

## Example Usage

```terraform
resource "nile_workspace" "example" {
  name = "Research"
}
```

## Argument Reference

The following arguments are supported:

- `name` - (Required) Display name of the workspace. The API derives the slug from it. Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `created` - (Computed) Creation timestamp of the workspace.
- `id` - (Computed) Workspace identifier.
- `raw_json` - (Computed) Raw API response of the workspace object, as JSON (secret fields redacted).
- `slug` - (Computed) Slug of the workspace; used by all other resources to reference it.
- `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, when billing is set up.

## Notes

### Session-token authentication

Creating a workspace is rejected with 403 `forbidden_operation` when the
provider authenticates with an API key; use OAuth credentials
(`oauth_client_id`/`oauth_refresh_token`) or a session token instead.

### Deletion is state-only

The Nile API does not expose workspace deletion. Destroying this resource
removes it from Terraform state but leaves the workspace running in the
control plane.

## Import

Import an existing object with:

```sh
terraform import nile_workspace.example my-workspace
```

