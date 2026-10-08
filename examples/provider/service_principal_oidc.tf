terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.0"
    }
  }
}

# Authenticate using Workload Identity Federation (OpenID Connect / OIDC) in CI/CD (e.g. GitHub Actions).
# When using azure/login in GitHub Actions, the provider can resolve credentials automatically via
# AZURE_FEDERATED_TOKEN_FILE or through explicit attributes:
provider "fabricext" {
  use_oidc  = true
  tenant_id = var.tenant_id
  client_id = var.client_id

  # Explicit OIDC token or file path if not relying on AZURE_FEDERATED_TOKEN_FILE:
  # oidc_token = var.oidc_jwt_token
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}
