---
page_title: "Nile: nile_databases"
subcategory: "Databases"
---

# nile_databases (Data Source)

Lists all Nile databases in a workspace via `GET /workspaces/{workspaceSlug}/databases`.

## Example Usage

```terraform
data "nile_databases" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference
  - `workspace_slug` - (Required) Slug of the Nile workspace whose databases are listed.

## Attribute Reference
  - `databases` - (Computed) The databases found in the workspace.
    - `api_host` - (Computed) Host of the database's API endpoint.
    - `created` - (Computed) Creation timestamp.
    - `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
    - `deleted` - (Computed) Timestamp at which the database was marked for deletion, if any.
    - `expandable` - (Computed) Whether the database can be expanded with read replicas (`expandable` in the API response).
    - `id` - (Computed) Database identifier (`id` in the API response).
    - `name` - (Computed) Database name.
    - `parent_id` - (Computed) Identifier of the parent (primary) database for read replicas.
    - `parent_name` - (Computed) Name of the parent (primary) database for read replicas.
    - `raw_json` - (Computed) Redacted JSON payload of the database as returned by the API.
    - `region` - (Computed) Region the database runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).
    - `status` - (Computed) Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).
  - `id` - (Computed) Stable identifier of this data source instance (the workspace slug).

