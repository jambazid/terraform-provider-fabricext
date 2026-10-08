---
page_title: "Provider: Fabric Extensions (fabricext)"
description: |-
  The Fabric Extensions provider (registry.terraform.io/jambazid/fabricext) is a purpose-built provider for declaratively managing Microsoft Fabric item-level sharing and data-access permissions (fabricext_*).
---

# Fabric Extensions Provider

~> **Warning:** **Pre-Alpha & Community Extension Notice**: `fabricext` (`v0.x`) is a purpose-built community provider for declaratively managing Microsoft Fabric item-level sharing and OneLake Data Access Roles (`fabricext_warehouse_permission`, `fabricext_sql_database_permission`, `fabricext_lakehouse_permission`, and `fabricext_item`) until equivalent resources land in Microsoft's official [`microsoft/fabric`](https://registry.terraform.io/providers/microsoft/fabric/latest) provider. Workspace provisioning and item lifecycles should be managed via `microsoft/fabric`.

The **Fabric Extensions (fabricext)** provider (`registry.terraform.io/jambazid/fabricext`) is a purpose-built provider for declaratively managing Microsoft Fabric item-level sharing and data-access permissions (`fabricext_*`) across Warehouses, SQL Databases, and Lakehouses until equivalent resources are available in Microsoft's official `microsoft/fabric` provider.

## Example Usage

### Automatic Credential Chain

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.1"
    }
  }
}

# Authenticate automatically via the native Entra ID credential chain:
# (Static Token -> Client Certificate -> Client Secret -> Azure DevOps OIDC -> Workload Identity -> Managed Identity -> Azure Developer CLI -> Azure CLI).
provider "fabricext" {}
```

### Azure CLI (Default Interactive)

```terraform
# Authenticate using an interactive Azure CLI session (`az login`).
# The provider automatically uses credentials from the active az CLI session.
provider "fabricext" {
  use_cli = true

  # Optional tenant ID filter
  # tenant_id = "00000000-0000-0000-0000-000000000000"
}
```

### Azure Developer CLI (`azd`)

```terraform
# Authenticate using Azure Developer CLI (`azd auth login`).
provider "fabricext" {
  use_dev_cli = true
}
```

### Service Principal with Client Secret

```terraform
# Authenticate using a Microsoft Entra Service Principal with a Client Secret.
provider "fabricext" {
  client_id     = "00000000-0000-0000-0000-000000000001"
  client_secret = "your-client-secret-here"
  tenant_id     = "00000000-0000-0000-0000-000000000000"
}
```

### Service Principal with Client Certificate

```terraform
# Authenticate using a Microsoft Entra Service Principal with a Client Certificate (PKCS#12 / PFX).
provider "fabricext" {
  client_id                    = "00000000-0000-0000-0000-000000000001"
  tenant_id                    = "00000000-0000-0000-0000-000000000000"
  client_certificate_file_path = "/path/to/certificate.pfx"
  client_certificate_password  = "cert-password"
}
```

### Workload Identity Federation

```terraform
# Authenticate using Workload Identity Federation (OIDC) in CI/CD (e.g., GitHub Actions).
provider "fabricext" {
  client_id = "00000000-0000-0000-0000-000000000001"
  tenant_id = "00000000-0000-0000-0000-000000000000"
  use_oidc  = true
}
```

### Managed Identity (System-Assigned)

```terraform
# Authenticate using an Azure System-Assigned Managed Identity.
provider "fabricext" {
  use_msi = true
}
```

### Managed Identity (User-Assigned)

```terraform
# Authenticate using an Azure User-Assigned Managed Identity.
provider "fabricext" {
  use_msi   = true
  client_id = "00000000-0000-0000-0000-000000000001" # Client ID of the User-Assigned MSI
}
```

### Azure DevOps Workload Identity Federation

```terraform
# Authenticate using Azure DevOps Workload Identity Federation (Service Connection OIDC).
provider "fabricext" {
  client_id                          = "00000000-0000-0000-0000-000000000001"
  tenant_id                          = "00000000-0000-0000-0000-000000000000"
  azure_devops_service_connection_id = "00000000-0000-0000-0000-000000000002"
}
```

### Dual-Provider Coexistence (`microsoft/fabric` + `fabricext`)

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabric = {
      source  = "microsoft/fabric"
      version = "~> 1.0"
    }
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.1"
    }
  }
}

# Both providers authenticate against the same tenant using shared environment variables
# (FABRIC_CLIENT_ID, FABRIC_CLIENT_SECRET, FABRIC_TENANT_ID) or native Azure CLI sessions.
provider "fabric" {}
provider "fabricext" {}

# Provision the Warehouse using Microsoft's official provider:
resource "fabric_warehouse" "sales" {
  workspace_id = var.workspace_id
  display_name = "sales_wh"
}

# Declaratively manage item-level sharing using fabricext:
resource "fabricext_warehouse_permission" "analysts" {
  workspace_id   = fabric_warehouse.sales.workspace_id
  warehouse_name = fabric_warehouse.sales.display_name
  principal_id   = var.analysts_group_id
  principal_type = "Group"
  role_type      = "read"
}

variable "workspace_id" {
  type        = string
  description = "Microsoft Fabric workspace UUID."
}

variable "analysts_group_id" {
  type        = string
  description = "Microsoft Entra ID group UUID."
}
```

### Sovereign Cloud Environments

```terraform
# Configure for Microsoft Azure Government (US Government) or China cloud.
provider "fabricext" {
  environment = "usgovernment" # Options: "public" (default), "usgovernment", "china"
  use_cli     = true
}
```

## Authentication and Credential Chain

The provider implements the complete Microsoft Entra ID authentication credential chain using Microsoft's official Go authentication SDK (`github.com/Azure/azure-sdk-for-go/sdk/azidentity`). Credentials are evaluated in the following deterministic order:

| Priority | Credential Source | Configuration Attributes / Environment Variables | Guide |
| :--- | :--- | :--- | :--- |
| **1** | **Static Access Token** | `access_token` attribute or `FABRIC_ACCESS_TOKEN` | *Testing / Pre-minted* |
| **2** | **Client Certificate** | `client_certificate`, `client_certificate_file_path`, `client_certificate_password` or `FABRIC_CLIENT_CERTIFICATE_*` | [Guide](guides/auth_spn_cert.md) |
| **3** | **Client Secret** | `client_id`, `client_secret`, `tenant_id` (plus `*_file_path` variants) or `FABRIC_*` / `AZURE_*` / `ARM_*` | [Guide](guides/auth_spn_secret.md) |
| **4** | **Azure DevOps OIDC** | `azure_devops_service_connection_id`, `oidc_request_token` or `SYSTEM_ACCESSTOKEN` | [Guide](guides/auth_azure_devops.md) |
| **5** | **Workload Identity (OIDC)** | `use_oidc = true`, `oidc_token`, `oidc_token_file_path` or `AZURE_FEDERATED_TOKEN_FILE` | [Guide](guides/auth_spn_oidc.md) |
| **6** | **Managed Identity (MSI)** | `use_msi = true`, optional `client_id` for User-Assigned MSI or `FABRIC_USE_MSI` | [Guide](guides/auth_msi.md) |
| **7** | **Azure Developer CLI** | `use_dev_cli = true` or `FABRIC_USE_DEV_CLI=true` | *Local `azd`* |
| **8** | **Azure CLI (`az login`)** | `use_cli = true` (default) or `FABRIC_USE_CLI` | *Interactive `az`* |

### Sovereign Clouds & Token Scopes

| Cloud Environment | `environment` Attribute | Microsoft Fabric Audience Scope | Default Endpoint |
| :--- | :--- | :--- | :--- |
| **Public** (Default) | `"public"` | `https://api.fabric.microsoft.com/.default` | `https://api.fabric.microsoft.com` |
| **US Government** | `"usgovernment"` | `https://api.fabric.microsoft.us/.default` | `https://api.fabric.microsoft.us` |
| **China** | `"china"` | `https://api.fabric.microsoft.cn/.default` | `https://api.fabric.microsoft.cn` |

## Declarative Matrix Composition

~> **Important:** **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

Following HashiCorp Provider Design Principles, each `fabricext_*` resource manages a single atomic permission binding or OneLake role. Keying `for_each` by `{item_name}/{principal_type}/{principal_id}` (omitting `role_type` from the map key) ensures that role changes update the existing resource in-place rather than causing Terraform to destroy and recreate the permission. You can declaratively manage an entire workspace's permissions matrix either with **Zero-Module Native HCL (`for_each`)** or with the in-repo **`modules/permissions`** wrapper module:

```hcl
locals {
  workspace_id = "00000000-0000-0000-0000-000000000001"

  warehouse_grants = {
    "sales_analytics_wh/Group/11111111-1111-1111-1111-111111111111" = {
      warehouse_name = "sales_analytics_wh"
      role_type      = "read"
      principal_id   = "11111111-1111-1111-1111-111111111111"
      principal_type = "Group"
    }
  }
}

resource "fabricext_warehouse_permission" "grants" {
  for_each = local.warehouse_grants

  workspace_id   = local.workspace_id
  warehouse_name = each.value.warehouse_name
  principal_id   = each.value.principal_id
  principal_type = each.value.principal_type
  role_type      = each.value.role_type
}
```

### Reusable Permissions Module

You can also use the in-repo companion HCL module to flatten matrices across Warehouses, SQL Databases, and Lakehouses in one place:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.2.1"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    warehouses = {
      sales_analytics_wh = {
        read = [
          {
            id   = "11111111-1111-1111-1111-111111111111"
            type = "Group"
          }
        ]
      }
    }
  }
}
```

*(Note: In local or monorepo development checkouts, you can also use `source = "./modules/permissions"`).*

~> **Note:** **Lakehouse Member Type Uniformity**: In simple mode (`principal_ids`), Microsoft Fabric OneLake Data Access Roles require uniform `principal_type` per role resource. In advanced mode (`entra_member` blocks), heterogeneous member types (Users, Groups, Service Principals, and Managed Identities) are natively supported on the same role.

## Use Cases & Security Architecture

For detailed architectural deep dives on enterprise security perimeters, schema isolation, and cross-engine identity flows, refer to the **Use Cases** guides:

- **[Use Cases Overview](guides/use_case_overview.md)**: Master comparison matrix and 6-tier perimeter overview.
- **[Security Controls & Interactions](guides/use_case_controls_and_interactions.md)**: Controls inventory, evaluation precedence, and conflict resolution rules.
- **[Warehouse Schema Isolation](guides/use_case_warehouse_schema_isolation.md)**: Granular schema isolation via `role_type = "read"` and T-SQL, Microsoft Entra ID Display Name resolution, and declarative schema-as-code integration.
- **[Lakehouse OneLake Security](guides/use_case_lakehouse_onelake_security.md)**: OneLake Data Access Roles, path filters, storage RLS/CLS, and shortcut delegation.
- **[Power BI Identity Flow](guides/use_case_powerbi_identity_propagation.md)**: DirectQuery TDS vs Direct Lake Parquet SSO identity propagation.

## Migration & Official Provider Coexistence

For detailed architectural trade-offs, state migration instructions, and resource mapping between `fabricext` and `microsoft/fabric`, see the [Official Provider Comparison Guide](guides/official_provider_comparison.md).

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `access_token` (String, Sensitive) Static Microsoft Entra ID bearer token for the Microsoft Fabric API (`https://api.fabric.microsoft.com/.default`). Can also be sourced from `FABRIC_ACCESS_TOKEN`.
- `auxiliary_tenant_ids` (List of String) Auxiliary Tenant IDs for multi-tenant token acquisition. Can also be sourced from comma-separated `FABRIC_AUXILIARY_TENANT_IDS`.
- `azure_devops_service_connection_id` (String) Azure DevOps Service Connection ID that uses Workload Identity Federation. Can also be sourced from `FABRIC_AZURE_DEVOPS_SERVICE_CONNECTION_ID`.
- `client_certificate` (String, Sensitive) Base64 encoded PKCS#12 certificate bundle (.pfx / .p12). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE`, `AZURE_CLIENT_CERTIFICATE`, or `ARM_CLIENT_CERTIFICATE`.
- `client_certificate_file_path` (String) Path to a PKCS#12 certificate file (.pfx / .p12). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_PATH`, `AZURE_CLIENT_CERTIFICATE_PATH`, or `ARM_CLIENT_CERTIFICATE_PATH`.
- `client_certificate_password` (String, Sensitive) Password associated with the PKCS#12 client certificate. Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_PASSWORD`, `AZURE_CLIENT_CERTIFICATE_PASSWORD`, or `ARM_CLIENT_CERTIFICATE_PASSWORD`.
- `client_id` (String) Microsoft Entra ID application (client) UUID. Can also be sourced from `FABRIC_CLIENT_ID`, `AZURE_CLIENT_ID`, or `ARM_CLIENT_ID`.
- `client_id_file_path` (String) Path to a file containing the Entra ID application (client) UUID.
- `client_secret` (String, Sensitive) Microsoft Entra ID Service Principal client secret. Can also be sourced from `FABRIC_CLIENT_SECRET`, `AZURE_CLIENT_SECRET`, or `ARM_CLIENT_SECRET`.
- `client_secret_file_path` (String) Path to a file containing the Service Principal client secret.
- `endpoint` (String) Base URL for the Microsoft Fabric REST API. Defaults to `https://api.fabric.microsoft.com` (or sovereign cloud equivalent). Can also be sourced from `FABRIC_ENDPOINT`.
- `environment` (String) Cloud environment to target (`public`, `usgovernment`, `china`). Defaults to `public`. Can also be sourced from `FABRIC_ENVIRONMENT` or `ARM_ENVIRONMENT`.
- `oidc_request_token` (String, Sensitive) Bearer token for requesting an ID token from an OIDC provider. Can also be sourced from `SYSTEM_ACCESSTOKEN`, `FABRIC_OIDC_REQUEST_TOKEN`, or `ARM_OIDC_REQUEST_TOKEN`.
- `oidc_request_url` (String) URL for requesting an ID token from an OIDC provider. Can also be sourced from `SYSTEM_OIDCREQUESTURI`, `FABRIC_OIDC_REQUEST_URL`, or `ARM_OIDC_REQUEST_URL`.
- `oidc_token` (String, Sensitive) OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN` or `ARM_OIDC_TOKEN`.
- `oidc_token_file_path` (String) Path to a file containing the OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN_FILE_PATH` or `ARM_OIDC_TOKEN_FILE_PATH`.
- `request_timeout` (String) Per-request HTTP timeout as a Go duration string (for example, `60s`). Defaults to `60s`. Can also be sourced from `FABRIC_REQUEST_TIMEOUT`.
- `skip_credentials_validation` (Boolean) Skip eager credential validation during `Configure()`. Defaults to `false`. Can also be sourced from `FABRIC_SKIP_CREDENTIALS_VALIDATION`.
- `tenant_id` (String) Microsoft Entra ID tenant UUID. Can also be sourced from `FABRIC_TENANT_ID`, `AZURE_TENANT_ID`, or `ARM_TENANT_ID`.
- `tenant_id_file_path` (String) Path to a file containing the Entra ID tenant UUID.
- `use_cli` (Boolean) Allow fallback to the local Azure CLI (`az login`) session. Defaults to `true`. Can also be sourced from `FABRIC_USE_CLI`, `AZURE_USE_CLI`, or `ARM_USE_CLI`.
- `use_dev_cli` (Boolean) Allow fallback to the Azure Developer CLI (`azd auth login`) session. Can also be sourced from `FABRIC_USE_DEV_CLI`.
- `use_msi` (Boolean) Enable Azure Managed Identity authentication. Can also be sourced from `FABRIC_USE_MSI`, `AZURE_USE_MSI`, or `ARM_USE_MSI`.
- `use_oidc` (Boolean) Enable Microsoft Entra Workload Identity (OIDC) authentication. Can also be sourced from `FABRIC_USE_OIDC`, `AZURE_USE_OIDC`, or `ARM_USE_OIDC`.
