# terraform-provider-nile

A [Terraform](https://www.terraform.io) provider for the
[Nile](https://www.thenile.dev) control plane API, implementing the full
[management API](https://www.thenile.dev/docs/api-reference). It lets you
manage Nile databases, dedicated compute instances, database credentials and
developer access to workspaces declaratively from Terraform, instead of
through the Nile console or ad-hoc API calls.

> [!NOTE]
> This is an independent, community-maintained project by
> [Golden Apple Research](https://github.com/Golden-Apple-Research). It is
> not developed by, affiliated with, sponsored by or endorsed by
> Nile (thenile.dev). "Nile" and the Nile product names are trademarks of
> their respective owners; this project simply builds on their public API.

The provider configuration, every resource and data source, and the behaviour
notes below are documented in detail under [`docs/index.md`](docs/index.md).

## Managed resources

The provider implements eight managed resources, one for each Nile object with
a declarative lifecycle.

The `nile_database` resource covers the complete lifecycle of a database —
creating it, reading it back, renaming it and deleting it — via
`POST/GET/PUT/DELETE /workspaces/{workspaceSlug}/databases/{databaseName}`
([create](https://www.thenile.dev/docs/api-reference/databases/create-a-database),
[get](https://www.thenile.dev/docs/api-reference/databases/get-a-database),
[rename](https://www.thenile.dev/docs/api-reference/databases/rename-a-database),
[delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-database)).

`nile_database_compute_instance` manages a dedicated compute instance attached
to a database, via
`POST/GET/PUT/DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/compute/{instanceId}`
([create](https://www.thenile.dev/docs/api-reference/databases/create-a-dedicated-compute-instance),
[describe](https://www.thenile.dev/docs/api-reference/databases/describe-a-dedicated-compute-instance),
[update](https://www.thenile.dev/docs/api-reference/databases/update-a-dedicated-compute-instance),
[delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-dedicated-compute-instance)).
Updates include resizing an instance, which the API performs asynchronously
(see the behaviour notes below).

`nile_database_credential` manages credentials for a database via
`POST/GET/DELETE /workspaces/{workspaceSlug}/databases/{databaseName}/credentials/{credentialId}`
([create](https://www.thenile.dev/docs/api-reference/databases/create-a-database-credential),
[list](https://www.thenile.dev/docs/api-reference/databases/lists-credentials-for-a-database),
[delete](https://www.thenile.dev/docs/api-reference/databases/delete-a-database-credential)).
Because Nile has no update endpoint for credentials, identity changes always
replace the credential; the returned password is shown exactly once and is
handled as a sensitive value. Changing `rotation_trigger` rotates the
credential in place via the [rotate
endpoint](https://www.thenile.dev/docs/api-reference/databases/rotate-a-database-credential),
promoting the new one-time password into state.

Finally, `nile_developer_invite` manages invitations for developers to join a
workspace, via `POST/GET/DELETE /workspaces/{workspaceSlug}/invites/{inviteId}`
([invite](https://www.thenile.dev/docs/api-reference/developers/invite-a-developer-to-a-workspace),
[list](https://www.thenile.dev/docs/api-reference/developers/list-developer-invites),
[delete](https://www.thenile.dev/docs/api-reference/developers/deletes-a-previously-issued-invite)).

`nile_workspace` creates workspaces via `POST /workspaces`
([create](https://www.thenile.dev/docs/api-reference/workspaces/create-a-workspace)).
The API exposes no workspace deletion or rename: destroy removes the resource
from state only, and changing `name` starts a new workspace. Creation is
rejected with 403 for API keys — the resource needs OAuth credentials or a
session token.

`nile_workspace_subscription` manages the subscription of a workspace via
`POST/PUT/DELETE /workspaces/{workspaceSlug}/subscription`
([start](https://www.thenile.dev/docs/api-reference/workspaces/start-a-subscription-row),
[change](https://www.thenile.dev/docs/api-reference/workspaces/change-level-effective-now-or-at-timestamp),
[close](https://www.thenile.dev/docs/api-reference/workspaces/close-a-subscription-row-at-given-time-now-if-omitted)).
Changing `level` updates the subscription in place; destroy closes it.
Billing endpoints need a session (developer) token — API keys get 403.

`nile_billing_customer` ensures a Stripe customer is linked to a workspace via
`PUT /workspaces/{workspaceSlug}/billing/customer`
([ensure](https://www.thenile.dev/docs/api-reference/workspaces/ensure-workspace-billing-customer)).
The idempotent call runs on create, refresh and update; the API cannot unlink
a customer, so destroy is state-only.

`nile_provisioned_database` provisions a dedicated database without
authentication via `POST /databases/provision`
([provision](https://www.thenile.dev/docs/api-reference/databases/provision-a-database-without-authentication))
and exposes the one-time claim code. Feed the code into `nile_database`'s
`claim_code` attribute to [claim the database for a
workspace](https://www.thenile.dev/docs/api-reference/databases/claim-a-database-provisioned-without-auth).
Dedicated compute requires a paid plan.

## Data sources

On the read-only side, the provider offers a broad set of data sources.

Around databases, `data.nile_databases` [lists the
databases](https://www.thenile.dev/docs/api-reference/databases/list-databases)
of a workspace, while `data.nile_database` reads [a single
database](https://www.thenile.dev/docs/api-reference/databases/get-a-database)
by name. For each database, `data.nile_database_compute_instances` [lists its
dedicated compute
instances](https://www.thenile.dev/docs/api-reference/databases/list-dedicated-compute-instances-for-a-database)
and `data.nile_database_credentials` [lists its
credentials](https://www.thenile.dev/docs/api-reference/databases/lists-credentials-for-a-database).
Three insight data sources expose metrics: `data.nile_database_uptime_insights`
for [uptime](https://www.thenile.dev/docs/api-reference/databases/list-database-uptime-metrics),
`data.nile_database_error_insights` for
[errors](https://www.thenile.dev/docs/api-reference/databases/list-database-error-metrics)
and `data.nile_database_query_performance_insights` for [query
performance](https://www.thenile.dev/docs/api-reference/databases/list-database-query-performance-metrics).

Two reference data sources help parameterise the resources above:
`data.nile_regions` returns the [available
regions](https://www.thenile.dev/docs/api-reference/databases/list-available-regions),
and `data.nile_compute_types` returns the [compute
types](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces-compute-types)
of a workspace.

At the workspace level, `data.nile_workspaces` [lists all
workspaces](https://www.thenile.dev/docs/api-reference/workspaces/list-workspaces)
and `data.nile_workspace` reads [one
workspace](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces)
by slug. `data.nile_workspace_developers` lists the [developers with
access](https://www.thenile.dev/docs/api-reference/workspaces/list-developers-with-access-to-a-workspace)
to a workspace, and `data.nile_workspace_invites` lists its [outstanding
developer
invites](https://www.thenile.dev/docs/api-reference/developers/list-developer-invites).
Usage and billing are covered by `data.nile_workspace_compute_usage` for the
[workspace compute
metrics](https://www.thenile.dev/docs/api-reference/workspaces/get-workspaces-metricscompute),
`data.nile_workspace_subscription` for the [current
subscription](https://www.thenile.dev/docs/api-reference/workspaces/get-current-subscription-from-db),
`data.nile_workspace_subscription_history` for the [subscription
history](https://www.thenile.dev/docs/api-reference/workspaces/list-subscription-history-most-recent-first),
`data.nile_workspace_billing_readiness` for the [billing
readiness](https://www.thenile.dev/docs/api-reference/workspaces/get-workspace-billing-readiness)
and `data.nile_workspace_billing_totals` for the [monthly totals by
component](https://www.thenile.dev/docs/api-reference/workspaces/monthly-totals-by-component-from-rated-lines).
Last but not least, `data.nile_developer` identifies the [authenticated
developer](https://www.thenile.dev/docs/api-reference/developers/identify-developer-details)
behind the API token.

## Client-only endpoints

A few endpoints of the management API are implemented in the Go client
(`internal/nileapi`) but deliberately have no Terraform object, because they
are imperative operations without a declarative lifecycle.

Workspace administration beyond creation is available through
`RemoveWorkspaceDeveloper` ([remove a
developer](https://www.thenile.dev/docs/api-reference/workspaces/remove-a-developer-from-a-workspace))
— there is no API to *add* a developer to a workspace directly, developers
join through invites, which the `nile_developer_invite` resource manages.

## Example

The following configuration creates a database with one compute instance and
a credential, and shows how to reference both from other resources and
outputs:

```hcl
provider "nile" {
  # Bearer token for the Nile API. Can also be set via NILE_API_TOKEN.
  api_token = var.nile_api_token

  # Instead of a static token, the provider can exchange an OAuth refresh
  # token for an access token (POST /oauth2/token). Can also be set via
  # NILE_OAUTH_CLIENT_ID / NILE_OAUTH_REFRESH_TOKEN; api_token wins.
  # oauth_client_id     = "my-client"
  # oauth_refresh_token = var.nile_oauth_refresh_token

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

All resources and data sources, including every argument and attribute, are
documented under [`docs/index.md`](docs/index.md).

## Behaviour notes

### Asynchronous operations

Creating a database or compute instance — and resizing an instance — is
asynchronous on the Nile side. The resources therefore poll the API until the
object reports `READY` before they complete, which allows dependent resources
to be created within the same apply. Waiting is bounded: by default an
operation may take up to 20 minutes (`Client.WaitTimeout`), with the API
polled every 5 seconds (`Client.PollInterval`). If waiting fails after the API
has already created (or renamed) the object, the resource is still recorded in
the state, so the next apply can refresh or destroy it instead of orphaning
it.

### Transport security

The API URL must use HTTPS; plain HTTP is accepted only for loopback test
endpoints. Redirects are deliberately not followed, so the bearer token can
never be forwarded to a different endpoint.

### Passwords and invite codes

The API returns credential passwords and programmatic invite codes exactly
once. They are stored as sensitive values in the state and are never copied
from later API responses, including masked values. Terraform state must
therefore be kept in an encrypted backend with restricted access.

### `raw_json`

Most data sources expose the raw API payload as `raw_json`, so that fields
the Nile API introduces in the future remain accessible through
`jsondecode()` even before the provider promotes them to typed attributes.
Raw payload attributes are marked sensitive, and common secret fields —
passwords, tokens, secrets, connection strings and invite codes — are replaced
with `[REDACTED]` before the payload reaches the state.

### Retries

Transient failures — HTTP `408`, `429` and `5xx` responses as well as network
errors — are retried up to three times with exponential backoff and jitter,
but only for replay-safe HTTP methods; non-idempotent `POST` requests are
never retried. A `Retry-After` header takes precedence over the backoff
schedule, capped at 30 seconds.

### Pagination

The compute instance list automatically follows a continuation token
(`nextPageToken`) if the API introduces one, with guards against
non-advancing tokens, an aggregate response limit of 100 MiB and a limit of
100,000 instances.

### Import

All resources support `terraform import`: a `nile_database` is imported as
`workspace_slug/database_name`, a `nile_database_compute_instance` as
`workspace_slug/database_name/instance_id`, a `nile_database_credential` as
`workspace_slug/database_name/credential_id` and a `nile_developer_invite` as
`workspace_slug/invite_id`.

## Installation

The provider is published on the public Terraform Registry under
`golden-apple-research/nile`:

```hcl
terraform {
  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
  }
}
```

`terraform init` then downloads a signed release binary for the current
platform.

## Development

Building and testing the provider requires Go ≥ 1.27 and Terraform ≥ 1.5; the
smoke test additionally needs Python 3. The common tasks are available through
`make`: `make build` builds the provider binary into `bin/`, `make test` runs
the unit tests, `make vet` runs `go vet`, and `make smoke` runs an end-to-end
test against a local mock API. `make docs` regenerates the registry
documentation under `docs/` (served by the Terraform Registry from the tagged
release). Releasing is handled by
[`.github/workflows/release.yml`](.github/workflows/release.yml): pushing a
`vX.Y.Z` tag builds the multi-platform archives, signs the checksums with the
release GPG key, and publishes the GitHub release that the Terraform Registry
imports.


### Local development with `dev_overrides`

For local testing, `make build` writes the provider binary to `bin/`. Point
Terraform at that directory — not at the repository root:

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

With that in place, `terraform plan` works directly on any configuration that
uses the provider; the configuration in `tests/smoke/` is a complete example
(`NILE_API_TOKEN=test-token-123`, mock API on `127.0.0.1:18080`).

### Installing into the local plugin mirror

Alternatively, `make install VERSION=0.1.0` copies the binary into
`~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/0.1.0/<os_arch>/`,
where `terraform init` picks it up without any CLI configuration. Note that
`VERSION` must be a valid semver version satisfying the version constraint of
the configuration (the examples use `~> 0.1`); the default `VERSION=dev` is
not installable.

## Security

Found a potential security issue? Please report it privately via the channels
in [`SECURITY.md`](SECURITY.md). Note that the software is provided "AS IS"
with no warranties; security handling is strictly voluntary and best-effort,
with no guaranteed response or remedy.

## License

This provider is licensed under the [European Union Public License, Version
1.2](LICENSE) (SPDX identifier: `EUPL-1.2`).
