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

Each element of `instances` promotes the documented API fields:

| Attribute | API field |
|---|---|
| `id` | `instanceId` |
| `name` | `instanceName` |
| `status` | `status` (`PENDING`, `PROVISIONING`, `READY`, `RESIZING`, `DELETING`, `FAILED`, `TERMINATED`) |
| `size` | `instanceType.computeSize` |
| `region` | `region` (`AWS_US_WEST_2`, `AWS_EU_CENTRAL_1`, `AZURE_EASTUS`) |
| `created_at` | `created` |

plus `raw_json` containing the full, unparsed instance payload returned by
the API — so attributes the API adds later remain accessible via
`jsondecode()`.

## Development

Requirements: Go ≥ 1.24, Terraform ≥ 1.5, Python 3 (only for the smoke test).

```sh
make build   # build the provider binary into bin/
make test    # run unit tests
make vet     # run go vet
make smoke   # end-to-end test against the local mock API
```

### Local development with `dev_overrides`

`make build` writes the provider binary to `bin/`. Point Terraform at that
directory — not at the repository root:

```sh
cat > ~/.terraformrc <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/golden-apple-research/nile" = "$(pwd)/bin"
  }
  direct {}
}
EOF
```

Then run `terraform plan` from a configuration that uses the provider; the
configuration in `tests/smoke/` is a complete example
(`NILE_API_TOKEN=test-token-123`, mock API on `127.0.0.1:18080`).

### Installing into the local plugin mirror

Alternatively, `make install VERSION=0.1.0` copies the binary into
`~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/0.1.0/<os_arch>/`,
where `terraform init` picks it up without a CLI config. `VERSION` must be a
valid semver version that satisfies the version constraint of the
configuration (the examples use `~> 0.1`); the default `VERSION=dev` is not
installable.

## License

This provider is licensed under the [European Union Public License, Version 1.2](LICENSE)
(SPIDX identifier: `EUPL-1.2`).

### Publishing

The provider address `registry.terraform.io/golden-apple-research/nile`
requires the GitHub repository to be named `terraform-provider-nile`
(currently `nile-terraform`).
