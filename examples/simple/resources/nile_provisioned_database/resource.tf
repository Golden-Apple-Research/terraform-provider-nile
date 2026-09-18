provider "nile" {
  api_url = var.nile_api_url
}

resource "nile_provisioned_database" "dedicated" {
  region = "AWS_EU_CENTRAL_1"
}

resource "nile_database" "claimed" {
  workspace_slug = nile_workspace.research.slug
  region         = "AWS_EU_CENTRAL_1"
  claim_code     = nile_provisioned_database.dedicated.claim_code
}
