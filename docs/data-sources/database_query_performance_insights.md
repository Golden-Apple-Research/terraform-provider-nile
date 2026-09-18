---
page_title: "Nile: nile_database_query_performance_insights"
subcategory: "Databases"
---

# nile_database_query_performance_insights (Data Source)

Lists Proxy query rate, Thoth query rate and Thoth P99 latency via `GET
/workspaces/{workspaceSlug}/databases/{databaseId}/insights/query-performance`.

## Example Usage

```terraform
data "nile_database_query_performance_insights" "example" {
  workspace_slug = "my-workspace"
  database       = "app-database"
}
```

## Argument Reference

The following arguments are supported:

- `database` - (Required) Name or identifier of the database. The API path names this parameter `databaseId`.
- `end` - (Optional) RFC3339 timestamp marking the end of the metrics window (`end` query parameter).
- `granularity` - (Optional) Bucket size of the returned samples (`granularity` query parameter).
- `start` - (Optional) RFC3339 timestamp marking the start of the metrics window (`start` query parameter).
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<database>`).
- `points` - (Computed) Query performance samples.
  - `proxy_queries_per_second` - (Computed) Queries per second observed by Nile Proxy.
  - `thoth_cpu_milliseconds` - (Computed) CPU milliseconds reported by Thoth.
  - `thoth_p99_latency_ms` - (Computed) P99 query latency in milliseconds reported by Thoth.
  - `thoth_queries_per_second` - (Computed) Queries per second observed by Thoth.
  - `timestamp` - (Computed) Sample timestamp.
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the response as returned by the API.
- `source` - (Computed) Source of the samples (`source` in the API response).

