terraform {
  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
  }
}

provider "nile" {
  # The token can also be supplied through NILE_API_TOKEN.
  api_token = var.nile_api_token
}

variable "nile_api_token" {
  type      = string
  sensitive = true
}

resource "nile_database" "app" {
  workspace_slug = "my-workspace"
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}

resource "nile_database_credential" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  # tenant_id    = "acme"
}

output "credential_password" {
  value     = nile_database_credential.app.password
  sensitive = true
}
