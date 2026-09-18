---
page_title: "Nile: nile_developer"
subcategory: "Developers"
---

# nile_developer (Data Source)

Returns the developer associated with the configured API token via `GET /developers/me`.

## Example Usage

```terraform
data "nile_developer" "example" {}
```

## Argument Reference

This object has no arguments.

## Attribute Reference

The following attributes are exported:

- `databases` - (Computed) Databases the developer has access to.
  - `api_host` - (Computed) Host of the database's API endpoint.
  - `created` - (Computed) Creation timestamp.
  - `db_host` - (Computed) Host of the database's PostgreSQL endpoint, if provisioned.
  - `deleted` - (Computed) Timestamp at which the database was marked for deletion, if any.
  - `expandable` - (Computed) Whether the database can be expanded with read replicas (`expandable` in the API response).
  - `id` - (Computed) Database identifier (`id` in the API response).
  - `name` - (Computed) Database name.
  - `parent_id` - (Computed) Identifier of the parent (primary) database for read replicas.
  - `parent_name` - (Computed) Name of the parent (primary) database for read replicas.
  - `raw_json` - (Computed, Sensitive) Redacted JSON payload of the database as returned by the API.
  - `region` - (Computed) Region the database runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).
  - `status` - (Computed) Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).
- `email` - (Computed) Developer email address.
- `id` - (Computed) Developer identifier (`id` in the API response).
- `kind` - (Computed) Developer kind (`HUMAN` or `API`).
- `raw_json` - (Computed, Sensitive) Redacted JSON payload of the developer as returned by the API.
- `workspaces` - (Computed) Workspaces the developer has access to.
  - `created` - (Computed) Creation timestamp.
  - `id` - (Computed) Workspace identifier (`id` in the API response).
  - `name` - (Computed) Workspace name.
  - `slug` - (Computed) Globally unique workspace slug used in API paths.
  - `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, if any.

