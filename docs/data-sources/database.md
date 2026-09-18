---
page_title: "Nile: nile_database"
subcategory: "Databases"
---

# nile_database (Data Source)

Fetches a single Nile database via `GET /workspaces/{workspaceSlug}/databases/{databaseName}`.

## Example Usage

```terraform
data "nile_database" "example" {
  workspace_slug = "my-workspace"
  name           = "app-database"
}
```

## Argument Reference

The following arguments are supported:

- `name` - (Required) Name of the database to look up.
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `api_host` - (Computed) Host of the database's API endpoint.
- `created` - (Computed) Creation timestamp.
- `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
- `deleted` - (Computed) Timestamp at which the database was marked for deletion, if any.
- `expandable` - (Computed) Whether the database can be expanded with read replicas (`expandable` in the API response).
- `id` - (Computed) Database identifier (`id` in the API response).
- `parent_id` - (Computed) Identifier of the parent (primary) database for read replicas.
- `parent_name` - (Computed) Name of the parent (primary) database for read replicas.
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the database as returned by the API.
- `region` - (Computed) Region the database runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).
- `status` - (Computed) Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).

