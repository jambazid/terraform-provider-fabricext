---
page_title: "Authenticating with a Service Principal and Client Secret"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using a Microsoft Entra ID Service Principal with a Client Secret.
---

# Service Principal Secret Authentication

This guide explains how to authenticate the `fabricext` provider using a Microsoft Entra ID Service Principal with a Client Secret.

## Prerequisites

1. An Azure App Registration with permissions granted in Microsoft Fabric (under **Tenant Settings > Developer Settings > Service principals can use Fabric APIs**).
2. The Tenant ID (directory ID), Client ID (application ID), and a generated Client Secret.

## Example Configuration

```terraform
# Authenticate using a Microsoft Entra Service Principal with a Client Secret.
provider "fabricext" {
  client_id     = "00000000-0000-0000-0000-000000000001"
  client_secret = "your-client-secret-here"
  tenant_id     = "00000000-0000-0000-0000-000000000000"
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
