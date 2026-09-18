terraform {
  required_providers {
    nile = {
      source = "golden-apple-research/nile"
    }
  }
}

variable "nile_api_url" {
  type    = string
  default = "http://127.0.0.1:18080"
}

provider "nile" {
  api_url = var.nile_api_url
}

data "nile_database_compute_instances" "test" {
  workspace_slug = "test-workspace"
  database_name  = "test-database"
}

output "count" {
  value = length(data.nile_database_compute_instances.test.instances)
}

output "ids" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.id]
}

output "names" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.name]
}

output "statuses" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.status]
}

output "sizes" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.size]
}

output "created_ats" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.created_at]
}

output "first_raw" {
  value = jsondecode(data.nile_database_compute_instances.test.instances[0].raw_json)
}
