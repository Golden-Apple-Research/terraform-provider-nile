---
page_title: "Nile: nile_workspace_invites"
subcategory: "Developers"
---

# nile_workspace_invites (Data Source)

Lists the developer invites of a workspace via `GET /workspaces/{workspaceSlug}/invites`.

## Example Usage

```terraform
data "nile_workspace_invites" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference

The following arguments are supported:

- `verification_state` - (Optional) Only list invites in this verification state (`verificationState` query parameter).
- `workspace_slug` - (Required) Slug of the workspace whose invites are listed.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
- `invites` - (Computed) The invites found in the workspace.
  - `created` - (Computed) Creation timestamp.
  - `email` - (Computed) Email address the invite was sent to.
  - `id` - (Computed) Invite identifier.
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the invite as returned by the API.
  - `sender_email` - (Computed) Email address of the developer who sent the invite.
  - `updated` - (Computed) Last update timestamp.
  - `verification_state` - (Computed) Verification state (`EMAIL_PENDING`, `EMAIL_SENT`, `VERIFIED` or `EXPIRED`).

