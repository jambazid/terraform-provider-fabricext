---
page_title: "Authenticating using Managed Identity (MSI)"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using Azure System-Assigned or User-Assigned Managed Identity.
---

# Authenticating using Managed Identity (MSI)

This guide explains how to authenticate the `fabricext` provider using Azure Managed Identities on Azure Virtual Machines, Azure Container Apps, or Azure Kubernetes Service (AKS).

## System-Assigned Managed Identity

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.0"
    }
  }
}

# Authenticate using a System-Assigned Managed Identity (MSI) on Azure VMs, Container Apps, or Azure DevOps agents.
provider "fabricext" {
  use_msi = true
}
```

Or set the environment variable:

```bash
export FABRIC_USE_MSI="true"
```

## User-Assigned Managed Identity

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.0"
    }
  }
}

# Authenticate using a User-Assigned Managed Identity (MSI) specifying its Client ID.
provider "fabricext" {
  use_msi   = true
  client_id = var.managed_identity_client_id
}

variable "managed_identity_client_id" {
  type        = string
  description = "Client (Application) ID of the User-Assigned Managed Identity."
}
```

Or set environment variables:

```bash
export FABRIC_USE_MSI="true"
export FABRIC_CLIENT_ID="<client-id-of-user-assigned-msi>"
```
