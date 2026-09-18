---
page_title: "Nile: nile_compute_types"
subcategory: "Workspaces"
---

# nile_compute_types (Data Source)

Lists the dedicated compute instance types available to a workspace via `GET
/workspaces/{workspaceSlug}/compute-types`.

## Example Usage

```terraform
data "nile_compute_types" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference

The following arguments are supported:

- `workspace_slug` - (Required) Slug of the workspace whose compute types are listed.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `compute_types` - (Computed) The compute types available to the workspace.
  - `compute_size` - (Computed) CPU size of the compute type (`computeSize` in the API response).
  - `hourly_cost` - (Computed) Hourly cost of the compute type in USD.
  - `id` - (Computed) Compute type identifier.
  - `memory` - (Computed) Memory of the compute type (for example `8GB`).
- `id` - (Computed) Stable identifier of this data source instance (the workspace slug).

