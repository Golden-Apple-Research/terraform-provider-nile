---
page_title: "Nile: nile_workspace_compute_usage"
subcategory: "Workspaces"
---

# nile_workspace_compute_usage (Data Source)

Lists the compute usage of a workspace via `GET /workspaces/{workspaceSlug}/metrics/compute`.

## Example Usage

```terraform
data "nile_workspace_compute_usage" "example" {
  workspace_slug = "my-workspace"
  start          = "2025-06-01T00:00:00Z"
  end            = "2025-07-01T00:00:00Z"
}
```

## Argument Reference

The following arguments are supported:

- `end` - (Optional) RFC3339 timestamp marking the end of the usage window (`end` query parameter).
- `start` - (Optional) RFC3339 timestamp marking the start of the usage window (`start` query parameter).
- `workspace_slug` - (Required) Slug of the workspace whose compute usage is read.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (the workspace slug).
- `periods` - (Computed) Compute usage periods returned by the API.
  - `chart_points` - (Computed) Chart data points of the period.
    - `x` - (Computed) Point on the x axis.
    - `y` - (Computed) Point on the y axis.
  - `databases` - (Computed) Per-database usage in the period.
    - `end` - (Computed) End of the database usage window.
    - `instances` - (Computed) Per-instance usage of the database.
      - `end` - (Computed) End of the instance usage window.
      - `name` - (Computed) Instance name.
      - `size` - (Computed) Instance compute size.
      - `start` - (Computed) Start of the instance usage window.
      - `total_vcpu_hours` - (Computed) Total vCPU hours of the instance.
    - `name` - (Computed) Database name.
    - `start` - (Computed) Start of the database usage window.
    - `total_vcpu_hours` - (Computed) Total vCPU hours of the database.
  - `end` - (Computed) End of the period.
  - `max_cpu_count` - (Computed) Maximum CPU count in the chart data.
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the period.
  - `start` - (Computed) Start of the period.
  - `total_vcpu_hours` - (Computed) Total vCPU hours in the period.

