---
page_title: "Nile: nile_database_compute_instance"
subcategory: "Databases"
---

# nile_database_compute_instance (Resource)

Manages a dedicated compute instance of a Nile database via
`/workspaces/{workspaceSlug}/databases/{databaseName}/compute`. Renaming and resizing update the
instance in place; creating, resizing and deleting are asynchronous and the resource waits for
the instance to settle.

## Example Usage

```terraform
resource "nile_database_compute_instance" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
  instance_name  = "primary"
  instance_size  = "large"
}
```

## Argument Reference
  - `database_name` - (Required) Name of the database the instance belongs to. Changing it forces replacement.
  - `instance_name` - (Required) Name of the compute instance. Renaming updates the instance in place.
  - `instance_size` - (Required) Compute size of the instance. Use `nile_compute_types` to discover the sizes available to a workspace. Resizing updates the instance in place.
  - `workspace_slug` - (Required) Slug of the Nile workspace that owns the database. Changing it forces replacement.

## Attribute Reference
  - `created_at` - (Computed) Instance creation timestamp (`created` in the API response).
  - `hourly_cost` - (Computed) Hourly cost of the instance's current type in USD (`instanceType.hourlyCost` in the API response).
  - `id` - (Computed) Instance identifier (`instanceId` in the API response).
  - `memory` - (Computed) Memory of the instance's current type (`instanceType.memory` in the API response).
  - `raw_json` - (Computed) Full, unparsed JSON payload of the instance as returned by the API.
  - `region` - (Computed) Region the instance runs in.
  - `status` - (Computed) Instance status (`PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED` or `TERMINATED`).
  - `updated_at` - (Computed) Timestamp of the last instance update (`updated` in the API response).

## Import

Import an existing object with:

```sh
terraform import nile_database_compute_instance.example my-workspace/app-database/inst-abc123
```

