---
page_title: "Nile: nile_workspace_billing_totals"
subcategory: "Workspaces"
---

# nile_workspace_billing_totals (Data Source)

Fetches the monthly totals by component, from rated lines, via `GET
/workspaces/{workspaceSlug}/billing/{ym}/totals`.

## Example Usage

```terraform
data "nile_workspace_billing_totals" "example" {
  workspace_slug = "my-workspace"
  month          = "2025-06"
}
```

## Argument Reference
  - `month` - (Required) Month to report on, in `YYYY-MM` form.
  - `workspace_slug` - (Required) Slug of the workspace whose billing totals are read.

## Attribute Reference
  - `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<month>`).
  - `raw_json` - (Computed) Full, unparsed JSON payload of the response as returned by the API.
  - `totals` - (Computed) Totals keyed by billing component.
  - `ym` - (Computed) Month as returned by the API (`ym` in the API response).

