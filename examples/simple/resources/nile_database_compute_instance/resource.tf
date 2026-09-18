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
  name           = "app_database"
  region         = "AWS_US_WEST_2"
}

resource "nile_database_compute_instance" "primary" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  instance_name  = "primary"
  instance_size  = "large"
}

output "instance_status" {
  value = nile_database_compute_instance.primary.status
}
