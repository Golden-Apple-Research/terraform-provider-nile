provider "nile" {
  api_url = var.nile_api_url
}

resource "nile_workspace_subscription" "paid" {
  workspace_slug = nile_workspace.research.slug
  level          = "paid"
}
