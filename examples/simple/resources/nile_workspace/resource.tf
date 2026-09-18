provider "nile" {
  api_url = var.nile_api_url
}

resource "nile_workspace" "research" {
  name = "Research"
}
