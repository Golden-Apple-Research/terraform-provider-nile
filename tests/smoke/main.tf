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

variable "database_name" {
  type    = string
  default = "app-database"
}

variable "instance_size" {
  type    = string
  default = "large"
}

provider "nile" {
  api_url = var.nile_api_url
}

# --- resources ---------------------------------------------------------------

resource "nile_database" "app" {
  workspace_slug = "test-workspace"
  name           = var.database_name
  region         = "AWS_US_WEST_2"
}

resource "nile_database_compute_instance" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  instance_name  = "primary"
  instance_size  = var.instance_size
}

resource "nile_database_credential" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  tenant_id      = "acme"
}

resource "nile_developer_invite" "qa" {
  workspace_slug = "test-workspace"
  email          = "qa@example.com"
  programmatic   = true
}

# --- data sources ------------------------------------------------------------

# Existing paginated compute instance list (seeded database).
data "nile_database_compute_instances" "test" {
  workspace_slug = "test-workspace"
  database_name  = "test-database"
}

data "nile_databases" "all" {
  workspace_slug = "test-workspace"

  # Re-read the list after the managed database changes (for example on a
  # rename), so the data source never shows stale names.
  depends_on = [nile_database.app]
}

data "nile_database" "app" {
  workspace_slug = nile_database.app.workspace_slug
  name           = nile_database.app.name
}

data "nile_database_credentials" "app" {
  workspace_slug = nile_database.app.workspace_slug
  database_name  = nile_database.app.name
  tenant_id      = "acme"

  # The list is empty until the credential resource above has created one.
  depends_on = [nile_database_credential.app]
}

data "nile_regions" "all" {
  workspace_slug = "test-workspace"
}

data "nile_compute_types" "all" {
  workspace_slug = "test-workspace"
}

data "nile_workspace" "test" {
  slug = "test-workspace"
}

data "nile_workspaces" "all" {}

data "nile_workspace_developers" "test" {
  workspace_slug = "test-workspace"
}

data "nile_workspace_invites" "test" {
  workspace_slug = "test-workspace"
}

data "nile_workspace_subscription" "test" {
  workspace_slug = "test-workspace"
}

data "nile_workspace_subscription_history" "test" {
  workspace_slug = "test-workspace"
}

data "nile_workspace_compute_usage" "test" {
  workspace_slug = "test-workspace"
}

data "nile_database_uptime_insights" "test" {
  workspace_slug = nile_database.app.workspace_slug
  database       = nile_database.app.name
}

data "nile_database_error_insights" "test" {
  workspace_slug = nile_database.app.workspace_slug
  database       = nile_database.app.name
}

data "nile_database_query_performance_insights" "test" {
  workspace_slug = nile_database.app.workspace_slug
  database       = nile_database.app.name
}

data "nile_workspace_billing_readiness" "test" {
  workspace_slug = "test-workspace"
}

data "nile_workspace_billing_totals" "test" {
  workspace_slug = "test-workspace"
  month          = "2025-06"
}

data "nile_developer" "me" {}

# --- outputs -----------------------------------------------------------------

output "compute_instance_count" {
  value = length(data.nile_database_compute_instances.test.instances)
}

output "compute_instance_ids" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.id]
}

output "compute_instance_names" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.name]
}

output "compute_instance_statuses" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.status]
}

output "compute_instance_sizes" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.size]
}

output "compute_instance_created_ats" {
  value = [for i in data.nile_database_compute_instances.test.instances : i.created_at]
}

output "first_raw" {
  value = jsondecode(data.nile_database_compute_instances.test.instances[0].raw_json)
}

output "database_name" {
  value = nile_database.app.name
}

output "database_status" {
  value = nile_database.app.status
}

output "database_region" {
  value = nile_database.app.region
}

output "instance_id" {
  value = nile_database_compute_instance.app.id
}

output "instance_status" {
  value = nile_database_compute_instance.app.status
}

output "instance_size" {
  value = nile_database_compute_instance.app.instance_size
}

output "instance_memory" {
  value = nile_database_compute_instance.app.memory
}

output "instance_hourly_cost" {
  value = nile_database_compute_instance.app.hourly_cost
}

output "credential_id" {
  value = nile_database_credential.app.id
}

output "credential_password" {
  value     = nile_database_credential.app.password
  sensitive = true
}

output "invite_id" {
  value = nile_developer_invite.qa.id
}

output "invite_code" {
  value     = nile_developer_invite.qa.code
  sensitive = true
}

output "invite_state" {
  value = nile_developer_invite.qa.verification_state
}

output "databases_count" {
  value = length(data.nile_databases.all.databases)
}

output "databases_names" {
  value = [for d in data.nile_databases.all.databases : d.name]
}

output "looked_up_database_status" {
  value = data.nile_database.app.status
}

output "credentials_count" {
  value = length(data.nile_database_credentials.app.credentials)
}

output "regions" {
  value = data.nile_regions.all.regions
}

output "compute_types" {
  value = [for t in data.nile_compute_types.all.compute_types : t.compute_size]
}

output "workspace_slug" {
  value = data.nile_workspace.test.slug
}

output "workspaces_count" {
  value = length(data.nile_workspaces.all.workspaces)
}

output "developers_count" {
  value = length(data.nile_workspace_developers.test.developers)
}

output "invites_count" {
  value = length(data.nile_workspace_invites.test.invites)
}

output "subscription_level" {
  value = data.nile_workspace_subscription.test.level
}

output "subscription_history_count" {
  value = length(data.nile_workspace_subscription_history.test.subscriptions)
}

output "compute_usage_vcpu" {
  value = data.nile_workspace_compute_usage.test.periods[0].total_vcpu_hours
}

output "uptime_percentage" {
  value = data.nile_database_uptime_insights.test.summary.uptime_percentage
}

output "error_count" {
  value = data.nile_database_error_insights.test.points[0].error_count
}

output "p99_latency" {
  value = data.nile_database_query_performance_insights.test.points[0].thoth_p99_latency_ms
}

output "billing_status" {
  value = data.nile_workspace_billing_readiness.test.status
}

output "billing_compute" {
  value = data.nile_workspace_billing_totals.test.totals["compute"]
}

output "developer_email" {
  value = data.nile_developer.me.email
}
