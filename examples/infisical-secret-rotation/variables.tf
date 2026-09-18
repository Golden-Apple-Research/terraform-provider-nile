# --- Nile ---

variable "nile_api_token" {
  type        = string
  sensitive   = true
  description = "Bearer token for the Nile API (or set NILE_API_TOKEN)."
}

variable "workspace_slug" {
  type        = string
  default     = "my-workspace"
  description = "Nile workspace that owns the database."
}

variable "database_name" {
  type        = string
  default     = "app-database"
  description = "Name of the Nile database."
}

variable "database_region" {
  type        = string
  default     = "AWS_US_WEST_2"
  description = "Region the Nile database is provisioned in."
}

# --- Rotation ---

variable "credential_rotation_days" {
  type        = number
  default     = 30
  description = <<-EOT
    Rotate the Nile database credential every N days. Rotation happens on the
    next `terraform apply` after the window elapses, so schedule periodic
    applies (CI/cron) if you want the rotation to actually run on time.
  EOT
}

# --- Infisical ---

variable "infisical_host" {
  type        = string
  default     = "https://app.infisical.com"
  description = "Infisical API URL. Only change this for self-hosted instances."
}

variable "infisical_client_id" {
  type        = string
  sensitive   = true
  description = "Machine Identity client ID (Universal Auth)."
}

variable "infisical_client_secret" {
  type        = string
  sensitive   = true
  description = "Machine Identity client secret (Universal Auth)."
}

variable "infisical_project_id" {
  type        = string
  description = "Infisical project ID (shown as Workspace ID in the dashboard)."
}

variable "infisical_env_slug" {
  type        = string
  default     = "dev"
  description = "Infisical environment slug the secrets are written to."
}

variable "infisical_folder_path" {
  type        = string
  default     = "/nile"
  description = "Infisical folder the secrets are written to. Use / for the root folder."
}

# --- Connection string assembly ---

variable "db_username" {
  type        = string
  default     = ""
  description = <<-EOT
    Postgres role used when assembling NILE_DATABASE_URL. The Nile credentials
    API returns the password and hosts but not the username; take it from the
    Nile dashboard (or your DB login). Leave empty to skip the URL secret.
  EOT
}
