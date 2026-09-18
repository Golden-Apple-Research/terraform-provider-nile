---
page_title: "Nile: nile_workspace_developers"
subcategory: "Workspaces"
---

# nile_workspace_developers (Data Source)

Lists the developers with access to a workspace via `GET
/workspaces/{workspaceSlug}/developers`.

## Example Usage

```terraform
data "nile_workspace_developers" "example" {
  workspace_slug = "my-workspace"
}
```

## Argument Reference
  - `workspace_slug` - (Required) Slug of the workspace whose developers are listed.

## Attribute Reference
  - `developers` - (Computed) The developers with access to the workspace.
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
      - `raw_json` - (Computed) Full, unparsed JSON payload of the database as returned by the API.
      - `region` - (Computed) Region the database runs in (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1` or `AZURE_EASTUS`).
      - `status` - (Computed) Database status (`PENDING`, `REQUESTED`, `BUILT`, `POOLED` or `READY`).
    - `email` - (Computed) Developer email address.
    - `id` - (Computed) Developer identifier.
    - `kind` - (Computed) Developer kind (`HUMAN` or `API`).
    - `workspaces` - (Computed) Workspaces the developer has access to.
      - `created` - (Computed) Creation timestamp.
      - `id` - (Computed) Workspace identifier (`id` in the API response).
      - `name` - (Computed) Workspace name.
      - `slug` - (Computed) Globally unique workspace slug used in API paths.
      - `stripe_customer_id` - (Computed) Stripe customer linked to the workspace, if any.
  - `id` - (Computed) Stable identifier of this data source instance (the workspace slug).

