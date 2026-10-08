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
# Authenticate using an Azure System-Assigned Managed Identity.
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
# Authenticate using an Azure User-Assigned Managed Identity.
provider "fabricext" {
  use_msi   = true
  client_id = "00000000-0000-0000-0000-000000000001" # Client ID of the User-Assigned MSI
}
```

Or set environment variables:

```bash
export FABRIC_USE_MSI="true"
export FABRIC_CLIENT_ID="<client-id-of-user-assigned-msi>"
```
