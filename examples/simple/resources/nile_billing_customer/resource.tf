provider "nile" {
  api_url = var.nile_api_url
}

resource "nile_billing_customer" "research" {
  workspace_slug = nile_workspace.research.slug
}
