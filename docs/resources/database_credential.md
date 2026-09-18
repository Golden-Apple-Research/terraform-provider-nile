---
page_title: "Nile: nile_database_credential"
subcategory: "Databases"
---

# nile_database_credential (Resource)

Manages a credential of a Nile database via
`/workspaces/{workspaceSlug}/databases/{databaseName}/credentials`. The generated password is
returned by the API exactly once and stored in state as a sensitive value; the API cannot return
it again. Changing `tenant_id` or `internal` forces replacement, because the API has no update
endpoint. Changing `rotation_trigger` rotates the credential in place via `POST
/workspaces/{workspaceSlug}/databases/{databaseName}/credentials/rotate`.

## Example Usage

```terraform
resource "nile_database_credential" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}
```

## Argument Reference

The following arguments are supported:

- `database_name` - (Required) Name of the database the credential belongs to. Changing it forces replacement.
- `internal` - (Optional) Whether to create an internal credential (`internal` query parameter). Changing it forces replacement.
- `rotation_delay_hours` - (Optional) When rotating, keep the old secrets valid for this many more hours (`delayOldSecretsExpirationHours`, default 0: old secrets expire immediately).
- `rotation_reason` - (Optional) Optional reason recorded with the rotation (`reason` request field).
- `rotation_trigger` - (Optional) Arbitrary trigger value: changing it rotates the credential in place (for example a timestamp or release identifier). The value itself is never sent to the API.
- `tenant_id` - (Optional) Tenant the credential is scoped to (`tenantId` query parameter). Changing it forces replacement.
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database. Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `api_host` - (Computed) Host of the database's API endpoint.
- `created` - (Computed) Creation timestamp.
- `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
- `id` - (Computed) Credential identifier.
- `internal` - (Computed) Whether to create an internal credential (`internal` query parameter). Changing it forces replacement.
- `password` - (Computed, Sensitive) Password of the credential. The API returns it only once, at creation time; it is stored in state and never copied from later API responses.
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the credential as returned by the API.
- `tenant_id` - (Computed) Tenant the credential is scoped to (`tenantId` query parameter). Changing it forces replacement.

## Notes

### One-time password

The API returns the credential password exactly once, in the create response. It
is stored in state as a sensitive value and is never copied from later responses.

### Rotation

The API has no credential update endpoint, so changing `tenant_id` or `internal`
forces replacement. To rotate the password in place, use the `RotateCredential`
client method or the Nile API directly.

## Import

Import an existing object with:

```sh
terraform import nile_database_credential.example my-workspace/app-database/cred-abc123
```

