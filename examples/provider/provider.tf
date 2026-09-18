terraform {
  required_providers {
    nile = {
      source  = "mal-2/nile"
      version = "~> 0.1"
    }
  }
}

provider "nile" {
  # api_token can also be set via the NILE_API_TOKEN env var.
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

variable "database_name" {
  type    = string
  default = "my-database"
}

data "nile_database_compute_instances" "current" {
  workspace_slug = var.workspace_slug
  database_name  = var.database_name
}

output "compute_instances" {
  description = "Dedicated compute instances attached to the database."
  value       = data.nile_database_compute_instances.current.instances
}
