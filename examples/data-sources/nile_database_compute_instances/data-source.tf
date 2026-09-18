# List all dedicated compute instances of a database
data "nile_database_compute_instances" "all" {
  workspace_slug = "my-workspace"
  database_name  = "my-database"
}

# Only instances that were active during a specific time window
data "nile_database_compute_instances" "window" {
  workspace_slug = "my-workspace"
  database_name  = "my-database"
  start          = "2025-01-01T00:00:00Z"
  end            = "2025-02-01T00:00:00Z"
}

output "instance_ids" {
  value = [for inst in data.nile_database_compute_instances.all.instances : inst.id]
}

output "instance_count" {
  value = length(data.nile_database_compute_instances.all.instances)
}
