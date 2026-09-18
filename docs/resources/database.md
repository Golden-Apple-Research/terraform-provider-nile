---
page_title: "Nile: nile_database"
subcategory: "Databases"
---

# nile_database (Resource)

Manages a Nile database via `/workspaces/{workspaceSlug}/databases`. Creating a database is
asynchronous; the resource waits until the database reports `READY` before it completes.

## Example Usage

```terraform
resource "nile_database" "example" {
  workspace_slug = "my-workspace"
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}
```

## Argument Reference

The following arguments are supported:

- `name` - (Required) Database name. Renaming updates the database in place via `PUT /workspaces/{workspaceSlug}/databases/{databaseName}`.
- `region` - (Required) Region the database runs in (for example `AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`). Changing it forces replacement.
- `timeouts` - (Optional) Timeouts for asynchronous operations. Unspecified operations use the default of `20m`.
  - `create` - (Optional) Time to wait for the resource to be created and become ready. Defaults to `20m`.
  - `delete` - (Optional) Time to wait for the resource to be deleted. Defaults to `20m`.
  - `update` - (Optional) Time to wait for the resource update to settle. Defaults to `20m`.
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database. Changing it forces replacement.

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
- `status` - (Computed) Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).

## Notes

### Asynchronous lifecycle

Creating a database and renaming it (`name`) are asynchronous. The resource
polls the API until the database reports `READY` before it completes, so
dependent resources such as [nile_database_compute_instance](database_compute_instance.md)
and [nile_database_credential](database_credential.md) can be created in the same
apply. Waiting is bounded by the `create` and `update` timeouts; deletion is
bounded by the `delete` timeout.

### Partial success

If the API has already created or renamed the database when a later wait fails,
the resource is still recorded in state, so a subsequent refresh can adopt it
instead of orphaning it.

## Import

Import an existing object with:

```sh
terraform import nile_database.example my-workspace/app-database
```

