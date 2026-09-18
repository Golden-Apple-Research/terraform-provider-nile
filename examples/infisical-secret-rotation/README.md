# Infisical Integration & Credential Rotation

This example shows how to combine the Nile provider with the
[Infisical provider](https://registry.terraform.io/providers/Infisical/infisical/latest/docs)
to:

1. Provision a Nile database and a database credential with Terraform.
2. Publish the credential (password, hosts, connection URL) as secrets in an
   Infisical project so applications never touch raw Terraform output.
3. **Rotate** the credential on a schedule — without ever handling the
   password by hand.

```
                ┌─────────────────────────────────────────────┐
                │                terraform apply              │
                └───────┬─────────────────────────┬───────────┘
                        │                         │
              creates / rotates           writes secrets
                        ▼                         ▼
        ┌───────────────────────────┐   ┌───────────────────────────┐
        │  Nile                     │   │  Infisical                │
        │  nile_database            │   │  NILE_DB_PASSWORD         │
        │  nile_database_credential │──▶│  NILE_DB_HOST             │
        │  (password is one-time)   │   │  NILE_DB_API_HOST         │
        └───────────────────────────┘   │  NILE_DATABASE_URL        │
                                        └─────────────┬─────────────┘
                                                      │ apps read via Infisical
                                                      ▼ (SDK / CLI / K8s operator /
                                                        secret sync, see below)
```

## Why rotation means *replacement*

Every configurable attribute of `nile_database_credential` forces replacement,
and the Nile API returns the generated password **exactly once** (at creation
time; it is stored in Terraform state as a sensitive value). The Nile API also
offers `POST .../credentials/rotate` for in-place rotation, but the new
password can never be read back — which would leave Terraform state stale.

The Terraform-native pattern is therefore **rotate by replacement**:

- a [`time_rotating`](https://registry.terraform.io/providers/hashicorp/time/latest/docs/resources/rotating)
  resource flips its `id` once the rotation window elapses,
- `replace_triggered_by` forces `nile_database_credential` to be recreated on
  the next apply, producing a fresh one-time password,
- the dependent `infisical_secret` resources pick up the new value and update
  Infisical automatically.

## Prerequisites

- A Nile workspace and API token (`NILE_API_TOKEN`).
- An Infisical account (or self-hosted instance) with a **project**.
- An Infisical **Machine Identity** with **Universal Auth**:

  1. In the Infisical dashboard: *Org Settings → Machine Identities →
     Universal Auth* → create one.
  2. Note the **Client ID** and **Client Secret** (the secret is shown only
     once).
  3. Attach a role that permits **secrets read/write** for the target project
     and environment (e.g. the *Project Admin* or a custom role with
     `secrets:read`/`secrets:edit`).

The Nile-side and the Infisical-side Machine Identity credentials are exactly
the kind of secrets this pattern is meant to keep out of your repo — store
them in your CI secret store (or, bootstrapped once, in Infisical itself).

## Files

| File          | Purpose                                                                    |
| ------------- | -------------------------------------------------------------------------- |
| `versions.tf` | Required providers: `nile`, `Infisical/infisical`, `hashicorp/time`       |
| `providers.tf`| Provider configuration (Nile token, Infisical Universal Auth)              |
| `variables.tf`| All inputs, including `credential_rotation_days`                           |
| `main.tf`     | Database, rotating credential, and the Infisical secrets                   |
| `outputs.tf`  | Credential ID, DB host, next rotation time (password only as `sensitive`)  |

## Usage

```shell
cd examples/infisical-secret-rotation
terraform init

terraform apply \
  -var="nile_api_token=$NILE_API_TOKEN" \
  -var="infisical_client_id=$INFISICAL_CLIENT_ID" \
  -var="infisical_client_secret=$INFISICAL_CLIENT_SECRET" \
  -var="infisical_project_id=<your-project-id>" \
  -var="db_username=<postgres-role>" # optional, enables NILE_DATABASE_URL
```

Credentials can also be provided via environment variables
(`NILE_API_TOKEN`, `INFISICAL_AUTH_METHOD=universal`,
`INFISICAL_UNIVERSAL_AUTH_CLIENT_ID`, `INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET`)
— then only `infisical_project_id` remains as a variable (ideal for a
`terraform.tfvars` file).

### What happens on each apply

| Situation                                   | Plan / behaviour                                                              |
| ------------------------------------------- | ----------------------------------------------------------------------------- |
| Rotation window not elapsed                 | No credential change; secrets in sync                                         |
| `rotation_days` elapsed (e.g. day 31)       | `time_rotating.id` changed → credential is **replaced**, secrets updated      |
| `terraform taint nile_database_credential.app` | Manual ad-hoc rotation on the next apply                                   |
| Secret value edited by hand in Infisical    | Drift detection overwrites it with the Terraform-managed value                |

> Rotation only happens when a `terraform apply` runs. Schedule periodic
> applies (CI pipeline, cron, or an [Atlantis][atlantis]-style automation) so
> the rotation window is actually enforced.

## How applications consume the secrets

Anything that already speaks Infisical works out of the box:

**Terraform (other stacks) via the `infisical_secrets` data source:**

```hcl
data "infisical_secrets" "nile" {
  env_slug     = "dev"
  project_id   = "<project-id>"
  folder_path  = "/nile"
}

resource "aws_lambda_function" "app" {
  # ...
  environment {
    variables = {
      DATABASE_URL = [for s in data.infisical_secrets.nile.secrets : s.value if s.name == "NILE_DATABASE_URL"][0]
    }
  }
}
```

**CLI export** for local development:

```shell
infisical secrets --env=dev --path=/nile run -- your-command
```

**Kubernetes**: the [Infisical Kubernetes Operator / Agent] injects the
secrets into pods and keeps them updated on rotation, without restarts.

**Fan-out to AWS Secrets Manager** (for stacks that cannot use the Infisical
SDK; requires an `infisical_app_connection_aws` and the AWS provider):

```hcl
resource "infisical_secret_sync_aws_secrets_manager" "nile" {
  name          = "nile-app-db-sync"
  project_id    = var.infisical_project_id
  environment   = var.infisical_env_slug
  secret_path   = var.infisical_folder_path
  connection_id = infisical_app_connection_aws.aws.id

  sync_options = {
    initial_sync_behavior = "overwrite-destination"
  }

  destination_config = {
    aws_region                      = "eu-central-1"
    mapping_behavior                = "many-to-one"
    aws_secrets_manager_secret_name = "nile/app-database"
  }
}
```

## Zero-gap rotation (blue/green credentials)

With the single-credential setup above, there is a short window during
rotation in which consumers still hold the old password while the old
credential is already deleted. If you cannot tolerate that:

1. Manage **two** `nile_database_credential` resources (`blue`, `green`) with
   staggered `time_rotating` windows.
2. Write the *active* credential to the secrets and keep the standby one
   valid (the Nile rotate API's `delayOldSecretsExpirationHours` shows the
   general idea: old secrets stay valid for a grace period).
3. Toggle the active credential in Infisical and only retire the old one
   after consumers have re-read the secrets.

## Alternative: let Infisical rotate the credentials itself

If you prefer server-side rotation without Terraform in the loop, Infisical
can rotate Postgres credentials on its own schedule using an **App
Connection**:

```hcl
resource "infisical_app_connection_postgres" "nile_db" {
  name   = "nile-app-db"
  method = "username-and-password"

  credentials = {
    host       = nile_database_credential.app.db_host
    port       = 5432
    database   = var.database_name
    username   = var.db_username
    password   = nile_database_credential.app.password
    ssl_enabled = true
  }
}

resource "infisical_secret_rotation_postgres_credentials" "nile_db" {
  name                  = "nile-db-credential-rotation"
  project_id            = var.infisical_project_id
  environment           = var.infisical_env_slug
  secret_path           = var.infisical_folder_path
  connection_id         = infisical_app_connection_postgres.nile_db.id
  username1             = var.db_username
  auto_rotation_enabled = true
  rotation_interval     = 30 # days
}
```

|                          | Terraform-driven (this example)          | Infisical-native rotation                     |
| ------------------------ | ---------------------------------------- | --------------------------------------------- |
| Rotation trigger         | `terraform apply` (schedule via CI)      | Infisical scheduler, no apply needed          |
| State                    | Password in Terraform state (sensitive)  | Terraform never sees the rotated password     |
| Drift control            | Full — Terraform owns the credential     | Terraform should not manage that credential   |
| Requires                 | periodic applies                         | App Connection + existing DB users            |

## Caveats

- **Plaintext in state:** both the Nile password and the Infisical secret
  values are stored in Terraform state. Protect the state backend
  (encryption, strict access) and prefer short rotation intervals.
- **One-time password semantics:** if the password is ever lost from state
  (or the API returns none at create time), the only fix is to replace the
  credential.
- The connection-URL secret is only created when `db_username` is set,
  because the Nile credentials API does not return the username.
- `infisical_secret` values are updated **after** the credential swap; see
  the zero-gap section above if that window matters to you.

## Clean up

```shell
terraform destroy
```

This deletes the Infisical secrets, the Nile credential and (careful!) the
Nile database itself.

[atlantis]: https://www.runatlantis.io
[Infisical Kubernetes Operator / Agent]: https://infisical.com/docs/documentation/getting-started/kubernetes
