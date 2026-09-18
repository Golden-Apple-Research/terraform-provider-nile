output "credential_id" {
  description = "ID of the current (possibly just rotated) Nile credential."
  value       = nile_database_credential.app.id
}

output "db_host" {
  description = "PostgreSQL host of the Nile database."
  value       = nile_database_credential.app.db_host
}

output "next_rotation" {
  description = "RFC3339 timestamp when the rotation window elapses. The rotation itself runs on the next apply after this time."
  value       = time_rotating.credential_rotation.rotation_rfc3339
}

output "infisical_secret_names" {
  description = "Infisical secrets that receive the rotated values."
  value = compact([
    infisical_secret.nile_db_password.name,
    infisical_secret.nile_db_host.name,
    infisical_secret.nile_db_api_host.name,
    var.db_username != "" ? infisical_secret.nile_database_url[0].name : "",
  ])
}

# Demo output only — the password is sensitive. Pull it from Infisical in
# real deployments instead of printing it.
output "credential_password" {
  value     = nile_database_credential.app.password
  sensitive = true
}
