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

variable "workspace_slug" {
  type    = string
  default = "my-workspace"
}

resource "nile_database" "app" {
  workspace_slug = var.workspace_slug
  name           = "app-database"
  region         = "AWS_US_WEST_2"
}

output "database_host" {
  value = nile_database.app.db_host
}
