---
page_title: "Nile: nile_database_error_insights"
subcategory: "Databases"
---

# nile_database_error_insights (Data Source)

Lists bucketed SQL and connection error counts via `GET
/workspaces/{workspaceSlug}/databases/{databaseId}/insights/errors`.

## Example Usage

```terraform
data "nile_database_error_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
  granularity    = "5m"
}
```

## Argument Reference

The following arguments are supported:

- `database` - (Required) Identifier (id) of the database. The API path names this parameter `databaseId`; the live API rejects database names here with `Invalid id`. Use the `id` attribute of `nile_database`.
- `end` - (Optional) RFC3339 timestamp marking the end of the metrics window (`end` query parameter). Must be aligned to whole minutes; the live API rejects sub-minute timestamps.
- `granularity` - (Optional) Bucket size of the returned samples (`granularity` query parameter).
- `start` - (Optional) RFC3339 timestamp marking the start of the metrics window (`start` query parameter). Must be aligned to whole minutes; the live API rejects sub-minute timestamps.
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<database>`).
- `points` - (Computed) Bucketed SQL and connection error counts.
  - `error_count` - (Computed) Errors observed in the bucket.
  - `source` - (Computed) Reporter (`source` in the API response).
  - `timestamp` - (Computed) Bucket timestamp.
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the response as returned by the API.

