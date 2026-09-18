terraform {
  required_version = ">= 1.5"

  required_providers {
    nile = {
      source  = "golden-apple-research/nile"
      version = "~> 0.1"
    }
    infisical = {
      source  = "Infisical/infisical"
      version = "~> 0.19"
    }
    # time_rotating drives the credential rotation schedule.
    time = {
      source  = "hashicorp/time"
      version = "~> 0.12"
    }
  }
}
