###############################################################################
# Nile: database + credential that rotates on a schedule
###############################################################################

resource "nile_database" "app" {
  workspace_slug = var.workspace_slug
  name           = var.database_name
  region         = var.database_region
}

# Replaces itself once the rotation window elapses. The next `terraform
# apply` then sees a changed `id` and rotates the credential below.
resource "time_rotating" "credential_rotation" {
  rotation_days = var.credential_rotation_days
}

# Every configurable attribute of nile_database_credential forces
# replacement, and the API returns the password exactly once — so "rotation"
# in Terraform means: replace the credential, keep the new password in state,
# and let the Infisical secrets below pick it up automatically.
resource "nile_database_credential" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  # tenant_id = "acme" # scope the credential to a tenant if needed

  lifecycle {
    # Rotate by replacement: a new rotation period (or `terraform taint`)
    # forces this resource to be recreated with a fresh password.
    replace_triggered_by = [time_rotating.credential_rotation.id]

    # Keep the old credential valid until the replacement exists. Note that
    # the Infisical secrets are updated only after the swap, so consumers
    # briefly still hold the old password — see the README for a zero-gap
    # blue/green variant.
    create_before_destroy = true
  }
}

###############################################################################
# Infisical: publish the rotated values as secrets
###############################################################################

resource "infisical_secret" "nile_db_password" {
  name         = "NILE_DB_PASSWORD"
  value        = nile_database_credential.app.password
  workspace_id = var.infisical_project_id
  env_slug     = var.infisical_env_slug
  folder_path  = var.infisical_folder_path
}

resource "infisical_secret" "nile_db_host" {
  name         = "NILE_DB_HOST"
  value        = nile_database_credential.app.db_host
  workspace_id = var.infisical_project_id
  env_slug     = var.infisical_env_slug
  folder_path  = var.infisical_folder_path
}

resource "infisical_secret" "nile_db_api_host" {
  name         = "NILE_DB_API_HOST"
  value        = nile_database_credential.app.api_host
  workspace_id = var.infisical_project_id
  env_slug     = var.infisical_env_slug
  folder_path  = var.infisical_folder_path
}

# Optional: a ready-to-use Postgres connection string. Requires
# var.db_username, since the Nile API does not return it.
resource "infisical_secret" "nile_database_url" {
  count = var.db_username != "" ? 1 : 0

  name = "NILE_DATABASE_URL"
  value = format(
    "postgresql://%s:%s@%s/%s",
    var.db_username,
    nile_database_credential.app.password,
    nile_database_credential.app.db_host,
    var.database_name,
  )
  workspace_id = var.infisical_project_id
  env_slug     = var.infisical_env_slug
  folder_path  = var.infisical_folder_path
}
