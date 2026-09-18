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

resource "nile_developer_invite" "qa" {
  workspace_slug = "my-workspace"
  email          = "qa@example.com"
  programmatic   = true
}

output "invite_code" {
  value     = nile_developer_invite.qa.code
  sensitive = true
}
