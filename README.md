# terraform-provider-nile

A [Terraform](https://www.terraform.io) provider for the
[Nile](https://www.thenile.dev) control plane API, implementing the full
[management API](https://www.thenile.dev/docs/api-reference).

## Implemented Endpoints

### Resources

| API Endpoint | Terraform Object |
|---|---|
| `POST/GET/PUT/DELETE /workspaces/{workspaceSlug}/databases[/{databaseName}]` — [Create](https://www.thenile.dev/docs/api-reference/databases/create-a-database), [Get](https://www.thenile.dev/docs/api-reference/databases/get-a-database), [Rename](https://www.thenile.dev/docs/api-reference/databases/rename-a-database), [Delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-database) a database | `nile_database` |
| `POST/GET/PUT/DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/compute[/{instanceId}]` — [Create](https://www.thenile.dev/docs/api-reference/databases/create-a-dedicated-compute-instance), [Describe](https://www.thenile.dev/docs/api-reference/databases/describe-a-dedicated-compute-instance), [Update](https://www.thenile.dev/docs/api-reference/databases/update-a-dedicated-compute-instance), [Delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-dedicated-compute-instance) a compute instance | `nile_database_compute_instance` |
| `POST/GET/DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/credentials[/{credentialId}]` — [Create](https://www.thenile.dev/docs/api-reference/databases/create-a-database-credential), [List](https://www.thenile.dev/docs/api-reference/databases/lists-credentials-for-a-database), [Delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-database-credential) a credential | `nile_database_credential` |
| `POST/GET/DELETE /workspaces/{workspaceSlug}/invites[/{inviteId}]` — [Invite](https://www.thenile.dev/docs/api-reference/developers/invite-a-developer-to-a-workspace), [List](https://www.thenile.dev/docs/api-reference/developers/list-developer-invites), [Delete](https://www.thenile.dev/docs/api-reference/developers/deletes-a-previously-issued-invite) an invite | `nile_developer_invite` |

### Data Sources

| API Endpoint | Terraform Object |
|---|---|
| `GET /workspaces/{workspaceSlug}/databases` — [List databases](https://www.thenile.dev/docs/api-reference/databases/list-databases) | `data.nile_databases` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseName}` — [Get a database](https://www.thenile.dev/docs/api-reference/databases/get-a-database) | `data.nile_database` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseName}/compute` — [List dedicated compute instances](https://www.thenile.dev/docs/api-reference/databases/list-dedicated-compute-instances-for-a-database) | `data.nile_database_compute_instances` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseName}/credentials` — [List credentials](https://www.thenile.dev/docs/api-reference/databases/lists-credentials-for-a-database) | `data.nile_database_credentials` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/uptime` — [List database uptime metrics](https://www.thenile.dev/docs/api-reference/databases/list-database-uptime-metrics) | `data.nile_database_uptime_insights` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/errors` — [List database error metrics](https://www.thenile.dev/docs/api-reference/databases/list-database-error-metrics) | `data.nile_database_error_insights` |
| `GET /workspaces/{workspaceSlug}/databases/{databaseId}/insights/query-performance` — [List query performance metrics](https://www.thenile.dev/docs/api-reference/databases/list-database-query-performance-metrics) | `data.nile_database_query_performance_insights` |
| `GET /workspaces/{workspaceSlug}/regions` — [List available regions](https://www.thenile.dev/docs/api-reference/databases/list-available-regions) | `data.nile_regions` |
| `GET /workspaces/{workspaceSlug}/compute-types` — [Get a workspace's compute types](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces-compute-types) | `data.nile_compute_types` |
| `GET /workspaces` — [List workspaces](https://www.thenile.dev/docs/api-reference/workspaces/list-workspaces) | `data.nile_workspaces` |
| `GET /workspaces/{workspaceSlug}` — [Get a workspace](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces) | `data.nile_workspace` |
| `GET /workspaces/{workspaceSlug}/developers` — [List developers with access](https://www.thenile.dev/docs/api-reference/workspaces/list-developers-with-access-to-a-workspace) | `data.nile_workspace_developers` |
| `GET /workspaces/{workspaceSlug}/invites` — [List developer invites](https://www.thenile.dev/docs/api-reference/developers/list-developer-invites) | `data.nile_workspace_invites` |
| `GET /workspaces/{workspaceSlug}/metrics/compute` — [Get a workspace's compute metrics](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces-metricscompute) | `data.nile_workspace_compute_usage` |
| `GET /workspaces/{workspaceSlug}/subscription` — [Get the current subscription](https://www.thenile.dev/docs/api-reference/workspaces/get-current-subscription-from-db) | `data.nile_workspace_subscription` |
| `GET /workspaces/{workspaceSlug}/subscription/history` — [List subscription history](https://www.thenile.dev/docs/api-reference/workspaces/list-subscription-history-most-recent-first) | `data.nile_workspace_subscription_history` |
| `GET /workspaces/{workspaceSlug}/billing/readiness` — [Get workspace billing readiness](https://www.thenile.dev/docs/api-reference/workspaces/get-workspace-billing-readiness) | `data.nile_workspace_billing_readiness` |
| `GET /workspaces/{workspaceSlug}/billing/{ym}/totals` — [Monthly totals by component](https://www.thenile.dev/docs/api-reference/workspaces/monthly-totals-by-component-from-rated-lines) | `data.nile_workspace_billing_totals` |
| `GET /developers/me` — [Identify developer details](https://www.thenile.dev/docs/api-reference/developers/identify-developer-details) | `data.nile_developer` |

### Client-only endpoints

These endpoints are available through the Go client
(`internal/nileapi`) but intentionally have no Terraform object, because
they are either unauthenticated provisioning flows, billing mutations, or
operations without a declarative lifecycle:

| API Endpoint | Client method |
|---|---|
| `POST /databases/provision` — [Provision a database without authentication](https://www.thenile.dev/docs/api-reference/databases/provision-a-database-without-authentication) | `ProvisionDatabase` |
| `POST /workspaces/{workspaceSlug}/databases/claim` — [Claim a provisioned database](https://www.thenile.dev/docs/api-reference/databases/claim-a-database-provisioned-without-auth) | `ClaimDatabase` |
| `POST /workspaces/{workspaceSlug}/databases/{databaseName}/credentials/rotate` — [Rotate a credential](https://www.thenile.dev/docs/api-reference/databases/rotate-a-database-credential) | `RotateCredential` |
| `POST /workspaces` — [Create a workspace](https://www.thenile.dev/docs/api-reference/workspaces/create-a-workspace) | `CreateWorkspace` |
| `DELETE /workspaces/{workspaceSlug}/developers/{developerId}` — [Remove a developer](https://www.thenile.dev/docs/api-reference/workspaces/remove-a-developer-from-a-workspace) | `RemoveWorkspaceDeveloper` |
| `PUT /workspaces/{workspaceSlug}/billing/customer` — [Ensure billing customer](https://www.thenile.dev/docs/api-reference/workspaces/ensure-workspace-billing-customer) | `EnsureBillingCustomer` |
| `POST/PUT/DELETE /workspaces/{workspaceSlug}/subscription[/{subscriptionId}]` — [Start](https://www.thenile.dev/docs/api-reference/workspaces/start-a-subscription-row), [Change](https://www.thenile.dev/docs/api-reference/workspaces/change-level-effective-now-or-at-timestamp), [Close](https://www.thenile.dev/docs/api-reference/workspaces/close-a-subscription-row-at-given-time-now-if-omitted) a subscription | `StartSubscription`, `ChangeSubscription`, `CloseSubscription` |
| `POST /oauth2/token` — [Post oauth2token](https://www.thenile.dev/docs/api-reference/post-oauth2token) | `ExchangeToken` |

## Example

```hcl
provider "nile" {
  # Bearer token for the Nile API. Can also be set via NILE_API_TOKEN.
  api_token = var.nile_api_token

  # Optional. Defaults to https://global.thenile.dev
  # HTTPS is required; plain HTTP is allowed only for loopback test endpoints.
  # Can also be set via NILE_API_URL.
  # api_url = "https://global.thenile.dev"
}

resource "nile_database" "app" {
  workspace_slug = "my-workspace"
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}

resource "nile_database_compute_instance" "primary" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  instance_name  = "primary"
  instance_size  = "large"
}

resource "nile_database_credential" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
}

data "nile_database_compute_instances" "current" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
}

output "database_host" {
  value = nile_database.app.db_host
}

output "credential_password" {
  value     = nile_database_credential.app.password
  sensitive = true
}
```

Resource and data source details, including all arguments and attributes, are
documented under [`docs/`](docs/).

## Behaviour Notes

- **Asynchronous operations.** Creating a database or compute instance (and
  resizing an instance) is asynchronous. The resources poll the API until the
  object reports `READY` before they complete, so dependent resources can be
  created in the same apply. Waiting is bounded: 20 minutes per operation by
  default (`Client.WaitTimeout`), with a poll every 5 seconds
  (`Client.PollInterval`). If waiting fails after the API has created (or
  renamed) the object, the resource is still recorded in state so the next
  apply can refresh or destroy it rather than orphaning it.
- **Transport security.** The API URL must use HTTPS. Plain HTTP is accepted
  only for loopback test endpoints. Redirects are not followed, so the bearer
  token cannot be forwarded to a different endpoint.
- **Passwords and invite codes.** The API returns credential passwords and
  programmatic invite codes exactly once. They are stored as sensitive values
  in state and are never copied from later API responses (including masked
  values). Terraform state must therefore use an encrypted backend with
  restricted access.
- **`raw_json`.** Most data sources expose the API payload as
  `raw_json`, so fields introduced by the Nile API in the future remain
  accessible via `jsondecode()` even before the provider promotes them to
  typed attributes. Raw payload attributes are marked sensitive and common
  secret fields (passwords, tokens, secrets, connection strings and invite
  codes) are replaced with `[REDACTED]` before the payload reaches state.
- **Retries.** Transient failures — HTTP `408`, `429`, `5xx`, and network
  errors — are retried up to three times with exponential backoff and jitter
  only for replay-safe HTTP methods. A `Retry-After` header takes precedence
  (capped at 30 seconds); non-idempotent `POST` requests are never retried.
- **Pagination.** The compute instance list follows a continuation token
  (`nextPageToken`) automatically if the API introduces one, with guards
  against non-advancing tokens, a 100 MiB aggregate response limit and a
  100,000-instance limit.
- **Import.** All resources support `terraform import`:
  - `nile_database`: `workspace_slug/database_name`
  - `nile_database_compute_instance`: `workspace_slug/database_name/instance_id`
  - `nile_database_credential`: `workspace_slug/database_name/credential_id`
  - `nile_developer_invite`: `workspace_slug/invite_id`

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
