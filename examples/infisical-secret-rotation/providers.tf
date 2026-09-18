provider "nile" {
  # Can also be set via the NILE_API_TOKEN env var.
  api_token = var.nile_api_token
}

provider "infisical" {
  # Only needed for self-hosted Infisical; defaults to https://app.infisical.com.
  host = var.infisical_host

  # Universal Auth (Machine Identity) credentials. Can also be supplied via
  # env vars:
  #   INFISICAL_AUTH_METHOD="universal"
  #   INFISICAL_UNIVERSAL_AUTH_CLIENT_ID / ..._CLIENT_SECRET
  auth = {
    universal = {
      client_id     = var.infisical_client_id
      client_secret = var.infisical_client_secret
    }
  }
}
