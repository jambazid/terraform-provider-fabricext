---
page_title: "Authenticating with a Service Principal and OpenID Connect (OIDC)"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using Workload Identity Federation (OIDC) in GitHub Actions and Kubernetes.
---

# Authenticating with a Service Principal and OpenID Connect (OIDC)

This guide explains how to authenticate the `fabricext` provider without long-lived credentials using Microsoft Entra Workload Identity Federation (OIDC) in CI/CD platforms like GitHub Actions.

## GitHub Actions Example

When using the official `azure/login` action, GitHub Actions sets `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, and `AZURE_FEDERATED_TOKEN_FILE`. The `fabricext` provider automatically detects these environment variables and authenticates via OIDC:

```yaml
jobs:
  terraform:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4
      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
      - run: terraform apply
```

## Explicit Configuration

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.5"
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
```
