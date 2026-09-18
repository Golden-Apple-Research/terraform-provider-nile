terraform {
  required_providers {
    nile = {
      source = "mal-2/nile"
    }
  }
}

provider "nile" {
  api_url = "http://127.0.0.1:18080"
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

output "statuses" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.status]
}

output "first_raw" {
  value = jsondecode(data.nile_database_compute_instances.test.instances[0].raw_json)
}
