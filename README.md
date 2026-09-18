# terraform-provider-nile

A [Terraform](https://www.terraform.io) provider for the
[Nile](https://www.thenile.dev) control plane API.

Currently implemented:

| API Endpoint | Terraform Object |
|---|---|
| `GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute` — [List dedicated compute instances for a database](https://www.thenile.dev/docs/api-reference/databases/list-dedicated-compute-instances-for-a-database) | `data.nile_database_compute_instances` |

## Provider Configuration

```hcl
provider "nile" {
  # Bearer token for the Nile API. Can also be set via NILE_API_TOKEN.
  api_token = var.nile_api_token

  # Optional. Defaults to https://global.thenile.dev
  # Can also be set via NILE_API_URL.
  # api_url = "https://global.thenile.dev"
}
```

<!-- schema: provider -->

## Data Sources

### `nile_database_compute_instances`

Lists the dedicated compute instances attached to a Nile database.
Optionally restricts the result to instances that were active in a
time window (`start`/`end`, RFC3339).

```hcl
data "nile_database_compute_instances" "current" {
  workspace_slug = "my-workspace"
  database_name  = "my-database"
  # start = "2025-01-01T00:00:00Z"
  # end   = "2025-02-01T00:00:00Z"
}

output "instance_ids" {
  value = [for inst in data.nile_database_compute_instances.current.instances : inst.id]
}
```

<!-- schema: data.nile_database_compute_instances -->

Each element of `instances` exposes best-effort promoted fields
(`id`, `status`, `size`, `region`, `created_at`) plus `raw_json`
containing the full, unparsed instance payload returned by the API —
so attributes the API adds later remain accessible via `jsondecode()`.

## Development

Requirements: Go ≥ 1.24, Terraform ≥ 1.5.

```sh
make build   # build the provider binary into bin/
make test    # run unit tests
make install # build + copy into the local plugin mirror for dev_overrides

# Enable local development overrides:
echo 'provider_installation { dev_overrides { "registry.terraform.io/golden-apple-research/nile" = "'"$(pwd)"'" } direct {} }' \
  > ~/.terraformrc
```

Then run `terraform plan` inside `examples/` with `NILE_API_TOKEN` set.
