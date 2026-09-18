---
page_title: "Nile: nile_database_uptime_insights"
subcategory: "Databases"
---

# nile_database_uptime_insights (Data Source)

Lists time-weighted database uptime samples via `GET
/workspaces/{workspaceSlug}/databases/{databaseId}/insights/uptime`.

## Example Usage

```terraform
data "nile_database_uptime_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
  start          = "2025-06-01T00:00:00Z"
  end            = "2025-06-02T00:00:00Z"
  granularity    = "1h"
}
```

## Argument Reference
  - `database` - (Required) Name or identifier of the database. The API path names this parameter `databaseId`.
  - `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference
  - `calculation` - (Computed) How uptime is calculated (`calculation` in the API response).
  - `end` - (Optional) RFC3339 timestamp marking the end of the metrics window (`end` query parameter).
  - `granularity` - (Optional) Bucket size of the returned samples (`granularity` query parameter).
  - `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<database>`).
  - `points` - (Computed) Uptime samples.
    - `observed_seconds` - (Computed) Seconds observed.
    - `timestamp` - (Computed) Sample timestamp.
    - `uptime_percentage` - (Computed) Uptime percentage.
    - `uptime_seconds` - (Computed) Seconds of uptime.
  - `raw_json` - (Computed) Redacted JSON payload of the response as returned by the API.
  - `scope` - (Computed) Scope of the calculation (`scope` in the API response).
  - `source` - (Computed) Source of the samples (`source` in the API response).
  - `start` - (Optional) RFC3339 timestamp marking the start of the metrics window (`start` query parameter).
  - `summary` - (Computed) Time-weighted uptime summary over the requested window.
    - `observed_seconds` - (Computed) Seconds observed.
    - `uptime_percentage` - (Computed) Uptime percentage.
    - `uptime_seconds` - (Computed) Seconds of uptime.

