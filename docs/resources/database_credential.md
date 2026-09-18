---
page_title: "Nile: nile_database_credential"
subcategory: "Databases"
---

# nile_database_credential (Resource)

Manages a credential of a Nile database via
`/workspaces/{workspaceSlug}/databases/{databaseName}/credentials`. The generated password is
returned by the API exactly once and stored in state as a sensitive value; the API cannot return
it again. Changing `tenant_id` or `internal` forces replacement, because the API has no update
endpoint. Use the `RotateCredential` client method (or the API directly) to rotate a credential
in place.

## Example Usage

```terraform
resource "nile_database_credential" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}
```

## Argument Reference
  - `database_name` - (Required) Name of the database the credential belongs to. Changing it forces replacement.
  - `workspace_slug` - (Required) Slug of the Nile workspace that owns the database. Changing it forces replacement.

## Attribute Reference
  - `api_host` - (Computed) Host of the database's API endpoint.
  - `created` - (Computed) Creation timestamp.
  - `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
  - `id` - (Computed) Credential identifier.
  - `internal` - (Optional, Computed) Whether to create an internal credential (`internal` query parameter). Changing it forces replacement.
  - `password` - (Computed, Sensitive) Password of the credential. The API returns it only once, at creation time; it is stored in state and never refreshed.
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the credential as returned by the API.
  - `tenant_id` - (Optional, Computed) Tenant the credential is scoped to (`tenantId` query parameter). Changing it forces replacement.

## Import

Import an existing object with:

```sh
terraform import nile_database_credential.example my-workspace/app-database/cred-abc123
```

