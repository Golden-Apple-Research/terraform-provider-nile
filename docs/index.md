---
page_title: "Nile Provider"
---

# Nile Provider

The Nile provider lets you manage and query resources in the [Nile](https://www.thenile.dev) control plane from Terraform.

The provider currently exposes a data source for listing dedicated compute instances attached to a Nile database.

## Example Usage

```terraform
terraform {
  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
  }
}

provider "nile" {
  # api_token can also be set with NILE_API_TOKEN.
  api_token = var.nile_api_token

  # api_url is optional and defaults to https://global.thenile.dev.
  # It can also be set with NILE_API_URL.
}

variable "nile_api_token" {
  type      = string
  sensitive = true
}
```

## Authentication

The provider authenticates with a Nile bearer token. Configure it using either the `api_token` provider argument or the `NILE_API_TOKEN` environment variable.

The API base URL can be configured using either the `api_url` provider argument or the `NILE_API_URL` environment variable. If neither is set, the provider uses `https://global.thenile.dev`.

## Argument Reference

- `api_token` - (Optional, Sensitive) Bearer token used to authenticate against the Nile API. May also be set with `NILE_API_TOKEN`.
- `api_url` - (Optional) Base URL of the Nile API. Defaults to `https://global.thenile.dev`. May also be set with `NILE_API_URL`.

## Data Sources

- [`nile_database_compute_instances`](data-sources/database_compute_instances) - Lists dedicated compute instances attached to a Nile database.
