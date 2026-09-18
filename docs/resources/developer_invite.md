---
page_title: "Nile: nile_developer_invite"
subcategory: "Developers"
---

# nile_developer_invite (Resource)

Manages a developer invite of a Nile workspace via `/workspaces/{workspaceSlug}/invites`.
Inviting sends an email; with `programmatic = true` the API returns an invite code instead
(stored in the sensitive `code` attribute) so the invite can be redeemed without email. There is
no update endpoint: changing `email` or `programmatic` forces replacement.

## Example Usage

```terraform
resource "nile_developer_invite" "example" {
  workspace_slug = "my-workspace"
  email          = "qa@example.com"
  programmatic   = true
}
```

## Argument Reference

The following arguments are supported:

- `email` - (Required) Email address of the invitee. Changing it forces replacement.
- `programmatic` - (Optional) If true, the API returns an invite code instead of only sending an email. Changing it forces replacement.
- `workspace_slug` - (Required) Slug of the workspace to invite the developer to. Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `code` - (Computed, Sensitive) Invite code, only returned when `programmatic = true`. Stored in state at creation and never copied from later API responses.
- `created` - (Computed) Creation timestamp.
- `id` - (Computed) Invite identifier.
- `programmatic` - (Computed) If true, the API returns an invite code instead of only sending an email. Changing it forces replacement.
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the invite as returned by the API.
- `sender_email` - (Computed) Email address of the developer who sent the invite.
- `updated` - (Computed) Last update timestamp.
- `verification_state` - (Computed) Verification state (`EMAIL_PENDING`, `EMAIL_SENT`, `VERIFIED` or `EXPIRED`).

## Notes

### Email vs. programmatic invites

With `programmatic = true` the API returns a one-time invite code in the sensitive
`code` attribute instead of only sending an email. The code is returned once and
never copied from later responses.

### Replacement

There is no invite update endpoint, so changing `email` or `programmatic` forces
replacement.

## Import

Import an existing object with:

```sh
terraform import nile_developer_invite.example my-workspace/inv-abc123
```

