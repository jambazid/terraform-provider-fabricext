---
page_title: "Authenticating with a Service Principal and Client Certificate"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using a Microsoft Entra ID Service Principal with a Client Certificate (.pfx / .p12).
---

# Authenticating with a Service Principal and Client Certificate

This guide explains how to authenticate the `fabricext` provider using a Microsoft Entra ID Service Principal with an X.509 Client Certificate bundle (PKCS#12 format: `.pfx` or `.p12`).

## Prerequisites

1. An Azure App Registration with your certificate uploaded under **Certificates & secrets**.
2. The PKCS#12 bundle (`.pfx` or `.p12`) containing the private key and public certificate, either as a file path or base64-encoded string.

## Example Configuration

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.1"
    }
  }
}

# Authenticate using a Microsoft Entra ID Service Principal with a Client Certificate (.pfx / .p12).
# Supports base64-encoded certificate strings or filesystem paths.
provider "fabricext" {
  tenant_id                    = var.tenant_id
  client_id                    = var.client_id
  client_certificate_file_path = "/path/to/certificate.pfx"
  client_certificate_password  = var.client_certificate_password

  # Alternatively, pass base64 encoded certificate data directly:
  # client_certificate = var.client_certificate_base64
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}

variable "client_certificate_password" {
  type        = string
  sensitive   = true
  description = "Password protecting the PKCS#12 certificate file."
}
```

## Environment Variables

You can supply the certificate configuration via environment variables:

```bash
export FABRIC_TENANT_ID="00000000-0000-0000-0000-000000000000"
export FABRIC_CLIENT_ID="11111111-1111-1111-1111-111111111111"
export FABRIC_CLIENT_CERTIFICATE_PATH="/path/to/certificate.pfx"
export FABRIC_CLIENT_CERTIFICATE_PASSWORD="your-certificate-password"
```

Or base64-encoded:

```bash
export FABRIC_CLIENT_CERTIFICATE="$(base64 -w 0 /path/to/certificate.pfx)"
```
