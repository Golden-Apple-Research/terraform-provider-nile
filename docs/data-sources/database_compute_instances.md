---
page_title: "Nile: nile_database_compute_instances"
subcategory: "Databases"
---

# nile_database_compute_instances (Data Source)

Lists the dedicated compute instances attached to a Nile database, via `GET
/workspaces/{workspaceSlug}/databases/{databaseName}/compute`. Optionally restricted to
instances active within a time window (`start`/`end`).

## Example Usage

```terraform
data "nile_database_compute_instances" "example" {
  workspace_slug = "my-workspace"
  database_name  = "app-database"
}
```

## Argument Reference

The following arguments are supported:

- `database_name` - (Required) Name of the database whose compute instances are listed.
- `end` - (Optional) RFC3339 timestamp marking the end of a time window to search for active instances.
- `start` - (Optional) RFC3339 timestamp marking the start of a time window to search for active instances.
- `workspace_slug` - (Required) Slug of the Nile workspace that owns the database.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - (Computed) Stable identifier of this data source instance (`<workspaceSlug>/<databaseName>`).
- `instances` - (Computed) The dedicated compute instances found for the database.
  - `created_at` - (Computed) Instance creation timestamp (`created` in the API response).
  - `id` - (Computed) Instance identifier (`instanceId` in the API response).
  - `name` - (Computed) Instance name (`instanceName` in the API response).
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the instance as returned by the API.
  - `region` - (Computed) Region the instance runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).
  - `size` - (Computed) Compute size of the instance's current type (`instanceType.computeSize` in the API response).
  - `status` - (Computed) Instance status (`PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED` or `TERMINATED`).

## Notes

### Time window

Set `start` and `end` to RFC3339 timestamps to restrict the result to instances
that were active during that period. `start` must not be later than `end`; the
provider rejects an inverted window before making any API call.

### Pagination

The Nile API currently returns all matching instances in a single response. If it
ever introduces pagination (a response object carrying a continuation token such
as `nextPageToken`), the provider follows the token automatically and concatenates
all pages, so `instances` always contains the complete result. Safety guards abort
with an error if a server fails to advance its page tokens.

### Retries

Transient failures — HTTP `408`, `429`, `5xx`, and network errors — are retried up
to three times with exponential backoff and jitter for replay-safe methods. A
`Retry-After` response header takes precedence over the computed backoff (capped
at 30 seconds). Client errors such as `400` or `401` are never retried.

