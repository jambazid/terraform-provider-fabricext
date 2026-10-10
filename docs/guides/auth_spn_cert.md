---
page_title: "Authenticating with a Service Principal and Client Certificate"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using a Microsoft Entra ID Service Principal with a Client Certificate (.pfx / .p12).
---

# Service Principal Certificate Authentication

This guide explains how to authenticate the `fabricext` provider using a Microsoft Entra ID Service Principal with an X.509 Client Certificate bundle (PKCS#12 format: `.pfx` or `.p12`).

## Prerequisites

1. An Azure App Registration with your certificate uploaded under **Certificates & secrets**.
2. The PKCS#12 bundle (`.pfx` or `.p12`) containing the private key and public certificate, either as a file path or base64-encoded string.

## Example Configuration

```terraform
# Authenticate using a Microsoft Entra Service Principal with a Client Certificate (PKCS#12 / PFX).
provider "fabricext" {
  client_id                    = "00000000-0000-0000-0000-000000000001"
  tenant_id                    = "00000000-0000-0000-0000-000000000000"
  client_certificate_file_path = "/path/to/certificate.pfx"
  client_certificate_password  = "cert-password"
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
