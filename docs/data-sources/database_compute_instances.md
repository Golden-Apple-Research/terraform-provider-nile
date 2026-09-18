---
subcategory: "Databases"
---

# nile_database_compute_instances (Data Source)

Lists the dedicated compute instances attached to a Nile database.

The data source calls the Nile API endpoint `GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute`. You can optionally restrict the result to instances that were active during an RFC3339 time window.

## Example Usage

```terraform
terraform {
  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
  }
}

provider "nile" {
  # The token can also be supplied through NILE_API_TOKEN.
  api_token = var.nile_api_token
}

data "nile_database_compute_instances" "current" {
  workspace_slug = "my-workspace"
  database_name  = "my-database"
}

output "instance_ids" {
  value = [for instance in data.nile_database_compute_instances.current.instances : instance.id]
}
```

### Query a Time Window

Set `start` and `end` to RFC3339 timestamps to return instances that were active during that period:

```terraform
data "nile_database_compute_instances" "historical" {
  workspace_slug = "my-workspace"
  database_name  = "my-database"
  start          = "2025-01-01T00:00:00Z"
  end            = "2025-02-01T00:00:00Z"
}
```

## Argument Reference

- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database. Must not be empty.
- `database_name` - (Required) Name of the database whose compute instances are listed. Must not be empty.
- `start` - (Optional) RFC3339 timestamp marking the start of the time window to search for active instances. Must not be after `end`.
- `end` - (Optional) RFC3339 timestamp marking the end of the time window to search for active instances.

Invalid timestamps, empty identifiers, and inverted time windows (`start` after `end`) are rejected during `terraform validate`/`plan` (timestamps and identifiers) or before any API call is made (time-window coherence).

## Attribute Reference

- `id` - Stable identifier of this data source instance in the form `<workspace_slug>/<database_name>`.
- `instances` - List of dedicated compute instances found for the database. Each object contains:
  - `id` - Instance identifier (`instanceId` in the Nile API response).
  - `name` - Instance name (`instanceName` in the Nile API response).
  - `status` - Instance status. Possible values are `PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED`, and `TERMINATED`.
  - `size` - Compute size of the instance's current type (`instanceType.computeSize` in the Nile API response).
  - `region` - Region where the instance runs. Possible values are `AWS_US_WEST_2`, `AWS_EU_CENTRAL_1`, and `AZURE_EASTUS`.
  - `created_at` - Instance creation timestamp (`created` in the Nile API response).
  - `raw_json` - Full, unparsed JSON payload for the instance. Use `jsondecode()` to access fields that are not promoted to typed attributes.

## Notes

The provider preserves the complete API payload in `raw_json`, so fields introduced by the Nile API in the future remain accessible even before the provider exposes dedicated typed attributes for them.
