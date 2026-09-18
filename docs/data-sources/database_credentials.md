---
page_title: "Nile: nile_database_credentials"
subcategory: "Databases"
---

# nile_database_credentials (Data Source)

Lists the credentials of a Nile database via `GET
/workspaces/{workspaceSlug}/databases/{databaseName}/credentials`. Passwords are **not**
included: the API returns them only once, when a credential is created.

## Example Usage

```terraform
data "nile_database_credentials" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}
```

## Argument Reference
  - `database_name` - (Required) Name of the database whose credentials are listed.
  - `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference
  - `credentials` - (Computed) The credentials found for the database.
    - `api_host` - (Computed) Host of the database's API endpoint.
    - `created` - (Computed) Creation timestamp.
    - `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
    - `id` - (Computed) Credential identifier.
    - `internal` - (Computed) Whether this is an internal credential.
    - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the credential as returned by the API.
    - `tenant` - (Computed) Tenant the credential is scoped to (`tenant` in the API response).
  - `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<databaseName>`).
  - `internal` - (Optional) Only list credentials with this internal flag (`internal` query parameter).
  - `tenant_id` - (Optional) Only list credentials scoped to this tenant (`tenantId` query parameter).

