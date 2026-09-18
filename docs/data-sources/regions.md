---
page_title: "Nile: nile_regions"
subcategory: "Workspaces"
---

# nile_regions (Data Source)

Lists the region identifiers available to a workspace via `GET
/workspaces/{workspaceSlug}/regions`. Identifiers combine a cloud provider prefix with a
provider region, for example `AWS_US_WEST_2` or `AZURE_EASTUS`.

## Example Usage

```terraform
data "nile_regions" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference

The following arguments are supported:

- `workspace_slug` - (Required) Slug of the workspace whose regions are listed.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
- `regions` - (Computed) Region identifiers available to the workspace.

