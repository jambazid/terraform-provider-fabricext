---
page_title: "Provider: Fabric Extensions (fabricext)"
description: |-
  The Fabric Extensions provider (registry.terraform.io/jambazid/fabricext) is a purpose-built provider for declaratively managing Microsoft Fabric Lakehouse OneLake Data Access Roles and security lifecycles (fabricext_*).
---

# Fabric Extensions Provider

<p align="center">
  <img src="https://raw.githubusercontent.com/jambazid/terraform-provider-fabricext/main/assets/fabric-logo.svg" width="72" height="72" alt="Microsoft Fabric Logo" />
</p>

!> **Warning:** `fabricext` is an experimental, pre-stable provider published to validate automated packaging, supply-chain provenance, and early Fabric permission architectures. Resources, attributes, and behaviour may change in any minor release prior to `v1.0.0`. Test every configuration in a non-production workspace first. For standard Microsoft Fabric production infrastructure, use the official [`microsoft/fabric`](https://registry.terraform.io/providers/microsoft/fabric/latest) provider.

The **Fabric Extensions (fabricext)** provider (`registry.terraform.io/jambazid/fabricext`) is an experimental community provider for declaratively managing Microsoft Fabric OneLake Data Access Roles and security governance (`fabricext_*`). Published pre-v1.0.0 for automated packaging and supply-chain verification.

## Example Usage

```terraform
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.7.1"
    }
  }
}

# Authenticate automatically via the native Entra ID credential chain:
# (Static Token -> Client Certificate -> Client Secret -> Azure DevOps OIDC -> Workload Identity -> Managed Identity -> Azure Developer CLI -> Azure CLI).
provider "fabricext" {}
```

## Authentication

The provider supports Microsoft Entra ID authentication via the official Azure Go SDK (`azidentity`). Authentication methods are evaluated automatically in priority order. Sensitive credentials should always be supplied via environment variables or workload identity rather than embedded in configuration files.

For setup instructions and credential evaluation details, see the dedicated authentication guides:

- **[Authentication Overview & Credential Chain](guides/authentication.md)**: Full evaluation order table (Priority 1–8), sovereign cloud audience scopes, and local CLI workflows.
- **[Azure CLI](guides/authentication.md#azure-cli)**: Default interactive developer workstation login.
- **[Azure Developer CLI (`azd`)](guides/authentication.md#azure-developer-cli)**: Developer CLI login integration.
- **[Service Principal with Secret](guides/auth_spn_secret.md)**: Standard Entra ID app registration authentication.
- **[Service Principal with Certificate](guides/auth_spn_cert.md)**: PKCS#12 / PFX certificate authentication.
- **[Workload Identity Federation (OIDC)](guides/auth_spn_oidc.md)**: Passwordless federation for GitHub Actions and external CI/CD.
- **[Azure DevOps Workload Identity](guides/auth_azure_devops.md)**: Passwordless Azure DevOps service connections.
- **[Managed Identity (MSI)](guides/auth_msi.md)**: System-Assigned and User-Assigned Managed Identity on Azure compute.

## Permissions Management

The provider delivers declarative item-level sharing for Warehouses, SQL Databases, and Lakehouses (`fabricext_warehouse_permission`, `fabricext_sql_database_permission`, `fabricext_lakehouse_permission`), alongside Lakehouse OneLake Data Access Roles (`fabricext_lakehouse_data_access_role`) and security lifecycle governance (`fabricext_lakehouse_onelake_security`).

The companion verification waiter [`fabricext_item_share_gate`](resources/item_share_gate.md) provides cross-plane validation confirming applied permissions propagate to Fabric Admin effective ACLs.

To manage permissions across an entire workspace, you can compose them using native Terraform `for_each` maps or use the in-repo [`modules/permissions`](https://github.com/jambazid/terraform-provider-fabricext/tree/main/modules/permissions) wrapper module.

For architectural patterns, in-place lifecycle keying invariants, and code examples, see the **[Declarative Matrix Composition Guide](guides/matrix_composition.md)**.


## Guides & Documentation

For comprehensive guides on access control architecture, cross-engine identity propagation, and official provider coexistence:

| Guide | Core Focus |
| :--- | :--- |
| **[Access Control Playbook](guides/access_control_playbook.md)** | Start Here routing matrix and 11 least-privilege scenario cards. |
| **[Enterprise Access Control Overview](guides/use_case_overview.md)** | Master comparison matrix and 6-tier security perimeter stack. |
| **[Security Controls & Interactions](guides/use_case_controls_and_interactions.md)** | Tier inventory, precedence rules, and workspace role bypass rules. |
| **[Warehouse Schema Isolation](guides/use_case_warehouse_schema_isolation.md)** | Least-privilege `read` connectivity, Entra ID naming, and SQL RBAC. |
| **[Lakehouse OneLake Security](guides/use_case_lakehouse_onelake_security.md)** | Data Access Roles, path filters, storage RLS/CLS, and shortcuts. |
| **[Power BI Identity Flow](guides/use_case_powerbi_identity_propagation.md)** | DirectQuery TDS vs Direct Lake Parquet SSO token propagation and fallback. |
| **[Official Provider Comparison](guides/official_provider_comparison.md)** | Architectural differences, dual-provider coexistence, and state migration. |

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `access_token` (String, Sensitive) Static Microsoft Entra ID bearer token for the Microsoft Fabric API (`https://api.fabric.microsoft.com/.default`). Can also be sourced from `FABRIC_ACCESS_TOKEN`.
- `auxiliary_tenant_ids` (List of String) Auxiliary Tenant IDs for multi-tenant token acquisition. Can also be sourced from comma-separated `FABRIC_AUXILIARY_TENANT_IDS`, `AZURE_AUXILIARY_TENANT_IDS`, or `ARM_AUXILIARY_TENANT_IDS`.
- `azure_devops_service_connection_id` (String) Azure DevOps Service Connection ID that uses Workload Identity Federation. Can also be sourced from `FABRIC_AZURE_DEVOPS_SERVICE_CONNECTION_ID`, `ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID`, or `ARM_OIDC_AZURE_SERVICE_CONNECTION_ID`.
- `client_certificate` (String, Sensitive) Base64 encoded PKCS#12 certificate bundle (.pfx / .p12). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE`, `AZURE_CLIENT_CERTIFICATE`, or `ARM_CLIENT_CERTIFICATE`.
- `client_certificate_file_path` (String) Path to a PKCS#12 certificate file (.pfx / .p12, raw binary or base64 text). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_FILE_PATH`, `AZURE_CLIENT_CERTIFICATE_FILE_PATH`, `ARM_CLIENT_CERTIFICATE_FILE_PATH`, `FABRIC_CLIENT_CERTIFICATE_PATH`, `AZURE_CLIENT_CERTIFICATE_PATH`, or `ARM_CLIENT_CERTIFICATE_PATH`.
- `client_certificate_password` (String, Sensitive) Password associated with the PKCS#12 client certificate. Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_PASSWORD`, `AZURE_CLIENT_CERTIFICATE_PASSWORD`, or `ARM_CLIENT_CERTIFICATE_PASSWORD`.
- `client_id` (String) Microsoft Entra ID application (client) UUID. Can also be sourced from `FABRIC_CLIENT_ID`, `AZURE_CLIENT_ID`, or `ARM_CLIENT_ID`.
- `client_id_file_path` (String) Path to a file containing the Entra ID application (client) UUID. Can also be sourced from `FABRIC_CLIENT_ID_FILE_PATH`, `AZURE_CLIENT_ID_FILE_PATH`, or `ARM_CLIENT_ID_FILE_PATH`.
- `client_secret` (String, Sensitive) Microsoft Entra ID Service Principal client secret. Can also be sourced from `FABRIC_CLIENT_SECRET`, `AZURE_CLIENT_SECRET`, or `ARM_CLIENT_SECRET`.
- `client_secret_file_path` (String) Path to a file containing the Service Principal client secret. Can also be sourced from `FABRIC_CLIENT_SECRET_FILE_PATH`, `AZURE_CLIENT_SECRET_FILE_PATH`, or `ARM_CLIENT_SECRET_FILE_PATH`.
- `endpoint` (String) Base URL for the Microsoft Fabric REST API. Defaults to `https://api.fabric.microsoft.com` (or sovereign cloud equivalent). Can also be sourced from `FABRIC_ENDPOINT`.
- `environment` (String) Cloud environment to target (`public`, `usgovernment`, `china`, case-insensitive). Defaults to `public`. Can also be sourced from `FABRIC_ENVIRONMENT`, `AZURE_ENVIRONMENT`, or `ARM_ENVIRONMENT`.
- `oidc_request_token` (String, Sensitive) Bearer token for requesting an ID token from an OIDC provider. Can also be sourced from `FABRIC_OIDC_REQUEST_TOKEN`, `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, `SYSTEM_ACCESSTOKEN`, or `ARM_OIDC_REQUEST_TOKEN`.
- `oidc_request_url` (String) URL for requesting an ID token from an OIDC provider. Can also be sourced from `FABRIC_OIDC_REQUEST_URL`, `ACTIONS_ID_TOKEN_REQUEST_URL`, `SYSTEM_OIDCREQUESTURI`, or `ARM_OIDC_REQUEST_URL`.
- `oidc_token` (String, Sensitive) OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN`, `AZURE_OIDC_TOKEN`, or `ARM_OIDC_TOKEN`.
- `oidc_token_file_path` (String) Path to a file containing the OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN_FILE_PATH`, `AZURE_OIDC_TOKEN_FILE_PATH`, `ARM_OIDC_TOKEN_FILE_PATH`, or `AZURE_FEDERATED_TOKEN_FILE`.
- `powerbi_access_token` (String, Sensitive) Static Microsoft Entra ID bearer token scoped to Power BI (`https://analysis.windows.net/powerbi/api/.default`). Can also be sourced from `FABRIC_POWERBI_ACCESS_TOKEN`.
- `powerbi_endpoint` (String) Regional Power BI cluster endpoint (e.g. `https://wabi-west-europe-redirect.analysis.windows.net`) or bootstrap portal. When configured with a regional cluster URL, bootstrap discovery is bypassed. Can also be sourced from `FABRIC_POWERBI_ENDPOINT`.
- `request_timeout` (String) Per-request HTTP timeout as a Go duration string (for example, `60s`). Defaults to `60s`. Can also be sourced from `FABRIC_REQUEST_TIMEOUT` or `FABRIC_TIMEOUT`.
- `skip_credentials_validation` (Boolean) Skip eager credential validation during `Configure()`. Defaults to `false`. Can also be sourced from `FABRIC_SKIP_CREDENTIALS_VALIDATION`.
- `tenant_id` (String) Microsoft Entra ID tenant UUID. Can also be sourced from `FABRIC_TENANT_ID`, `AZURE_TENANT_ID`, or `ARM_TENANT_ID`.
- `tenant_id_file_path` (String) Path to a file containing the Entra ID tenant UUID. Can also be sourced from `FABRIC_TENANT_ID_FILE_PATH`, `AZURE_TENANT_ID_FILE_PATH`, or `ARM_TENANT_ID_FILE_PATH`.
- `use_cli` (Boolean) Allow fallback to the local Azure CLI (`az login`) session. Defaults to `true`. Can also be sourced from `FABRIC_USE_CLI`, `AZURE_USE_CLI`, or `ARM_USE_CLI`.
- `use_dev_cli` (Boolean) Allow fallback to the Azure Developer CLI (`azd auth login`) session. Can also be sourced from `FABRIC_USE_DEV_CLI`, `AZURE_USE_DEV_CLI`, or `ARM_USE_DEV_CLI`.
- `use_msi` (Boolean) Enable Azure Managed Identity authentication. Can also be sourced from `FABRIC_USE_MSI`, `AZURE_USE_MSI`, or `ARM_USE_MSI`.
- `use_oidc` (Boolean) Enable Microsoft Entra Workload Identity (OIDC) authentication. When enabled, requires OIDC request tokens or assertion files. Can also be sourced from `FABRIC_USE_OIDC`, `AZURE_USE_OIDC`, or `ARM_USE_OIDC`.
