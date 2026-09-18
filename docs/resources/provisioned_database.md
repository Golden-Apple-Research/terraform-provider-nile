---
page_title: "Nile: nile_provisioned_database"
subcategory: "Databases"
---

# nile_provisioned_database (Resource)

Provisions a dedicated database via the unauthenticated `POST /databases/provision` endpoint and
exposes the resulting claim code. Feed the claim code into the `claim_code` attribute of a
`nile_database` resource to attach the provisioned database to a workspace. Dedicated compute
requires a paid plan; the free tier answers with 403 `feature_ineligible`. The API documents no
read or delete endpoint for provisioned databases: this resource performs no API call on refresh
or destroy.

## Example Usage

```terraform
resource "nile_provisioned_database" "example" {
  region = "AWS_EU_CENTRAL_1"
}

resource "nile_database" "claimed" {
  workspace_slug = nile_workspace.example.slug
  region         = "AWS_EU_CENTRAL_1"
  claim_code     = nile_provisioned_database.example.claim_code
}
```

## Argument Reference

The following arguments are supported:

- `region` - (Required) Region to provision the dedicated database in (for example `AWS_EU_CENTRAL_1`). Changing it forces replacement.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `api_host` - (Computed) HTTPS API host of the dedicated database.
- `claim_code` - (Computed, Sensitive) Claim code of the provisioned database. Pass it to `nile_database.claim_code` to attach the database to a workspace; the code is consumed by the claim.
- `credential_id` - (Computed) ID of the bootstrap credential created with the database.
- `database_id` - (Computed) ID of the provisioned database.
- `database_name` - (Computed) Server-assigned name of the provisioned database (pattern `unauth_…`).
- `db_host` - (Computed) Postgres connection host of the dedicated database.
- `password` - (Computed, Sensitive) One-time bootstrap password. The API returns it exactly once; it is not part of later responses.
- `raw_json` - (Computed, Sensitive) Raw API response of the provisioning call, as JSON (secret fields redacted).
- `sharded` - (Computed) Whether the provisioned database is sharded.
- `username` - (Computed, Sensitive) Bootstrap username (`NILEDB_USER` from the response env).

## Notes

### Claim flow

The provisioning call is unauthenticated and returns a one-time claim code.
Consume it with the `claim_code` attribute of `nile_database` to attach the
dedicated database to a workspace. Dedicated compute requires a paid plan; the
free tier answers with 403. No read or delete endpoint exists for provisioned
databases, so refresh and destroy perform no API call.

