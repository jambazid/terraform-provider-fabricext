---
page_title: "Authenticating with a Service Principal and Client Secret"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using a Microsoft Entra ID Service Principal with a Client Secret.
---

# Authenticating with a Service Principal and Client Secret

This guide explains how to authenticate the `fabricext` provider using a Microsoft Entra ID Service Principal with a Client Secret.

## Prerequisites

1. An Azure App Registration with permissions granted in Microsoft Fabric (under **Tenant Settings > Developer Settings > Service principals can use Fabric APIs**).
2. The Tenant ID (directory ID), Client ID (application ID), and a generated Client Secret.

## Example Configuration

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.3"
    }
  }
}

# Authenticate using a Microsoft Entra ID Service Principal with a Client Secret.
# We recommend passing sensitive values via environment variables:
# FABRIC_TENANT_ID, FABRIC_CLIENT_ID, and FABRIC_CLIENT_SECRET.
provider "fabricext" {
  tenant_id     = var.tenant_id
  client_id     = var.client_id
  client_secret = var.client_secret
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}

variable "client_secret" {
  type        = string
  sensitive   = true
  description = "Microsoft Entra ID Service Principal client secret."
}
```

## Environment Variables

We strongly recommend passing secrets via environment variables rather than embedding them in `.tf` files:

```bash
export FABRIC_TENANT_ID="00000000-0000-0000-0000-000000000000"
export FABRIC_CLIENT_ID="11111111-1111-1111-1111-111111111111"
export FABRIC_CLIENT_SECRET="your-client-secret-value"
```

The provider also recognizes standard Azure SDK aliases (`AZURE_TENANT_ID`, `ARM_TENANT_ID`, `AZURE_CLIENT_ID`, `ARM_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `ARM_CLIENT_SECRET`).
