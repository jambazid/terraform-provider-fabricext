# Terraform Provider for Fabricext

| <img src="assets/terraform-logo.svg" width="260" alt="Terraform Logo"> | <img src="assets/fabric-logo.svg" width="56" alt="Microsoft Fabric Logo"> |
| :---: | :---: |

> [!WARNING]
> `fabricext` is an experimental, pre-stable provider published to validate automated packaging, supply-chain provenance, and early Fabric permission architectures. Resources, attributes, and behaviour may change in any minor release prior to `v1.0.0`. Test every configuration in a non-production workspace first. For standard Microsoft Fabric production infrastructure, use the official [`microsoft/fabric`](https://registry.terraform.io/providers/microsoft/fabric/latest) provider.

## Overview

`terraform-provider-fabricext` is a Terraform Plugin Framework (Protocol v6) provider for declaratively managing Microsoft Fabric item-level permissions, OneLake Data Access Roles, and security lifecycles:

| Resource / Data Source | Microsoft Fabric Item Type | Capabilities | Status |
| :--- | :--- | :--- | :--- |
| `fabricext_warehouse_permission` | `Warehouse` | Declaratively manages direct item-level sharing permissions (`Read`, `ReadWrite`, `Reshare`, `Explore`) with in-place drift reconciliation. | Active |
| `fabricext_sql_database_permission` | `SQLDatabase` | Declaratively manages direct item-level sharing permissions (`Read`, `ReadWrite`, `Reshare`, `Explore`) with in-place drift reconciliation. | Active |
| `fabricext_lakehouse_permission` | `Lakehouse` | Declaratively manages direct item-level sharing permissions (`Read`, `ReadWrite`, `Reshare`, `Explore`) with in-place drift reconciliation. | Active |
| `fabricext_item_share_gate` | `Warehouse`, `Lakehouse`, `SQLDatabase` | Waiter resource verifying effective direct and inherited item permissions via the official Fabric Admin REST API. | Active |
| `fabricext_lakehouse_data_access_role` | `Lakehouse` | Manages OneLake Data Access Roles (`/dataAccessRoles`) with per-Lakehouse mutex locking and `If-Match` ETag Read-Modify-Write concurrency control. | Active |
| `fabricext_lakehouse_onelake_security` | `Lakehouse` | Validates OneLake security enablement and manages the `DefaultReader` role lifecycle. | Active |
| `data.fabricext_item` | `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `SemanticModel` | Resolves `(workspace_id, display_name, type)` to canonical item UUIDs (`id`) using a paginated, type-isolated lookup cache. | Active |

~> **Note:** Item-level permissions (`fabricext_{warehouse,sql_database,lakehouse}_permission`) configure direct item shares via Power BI metadata access endpoints. Permissions drift is detected and reconciled in-place. For tenant-wide verification, `fabricext_item_share_gate` verifies applied permissions through the official Fabric Admin REST API.

## Quick Start

```hcl
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.7.0"
    }
  }
}

provider "fabricext" {}

# Discover existing Fabric items by display name:
data "fabricext_item" "bronze_lh" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  display_name = "raw_bronze_lh"
  type         = "Lakehouse"
}

# Declaratively share the Lakehouse item:
resource "fabricext_lakehouse_permission" "bronze_share" {
  workspace_id   = data.fabricext_item.bronze_lh.workspace_id
  lakehouse_id   = data.fabricext_item.bronze_lh.id
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  permissions    = ["Read"]
}

# Verify effective permissions via the Fabric Admin API gate:
resource "fabricext_item_share_gate" "bronze_gate" {
  workspace_id         = data.fabricext_item.bronze_lh.workspace_id
  item_id              = data.fabricext_item.bronze_lh.id
  principal_id         = "11111111-1111-1111-1111-111111111111"
  principal_type       = "Group"
  expected_permissions = ["Read"]

  depends_on = [fabricext_lakehouse_permission.bronze_share]
}

# Verify OneLake security enablement and purge open DefaultReader role:
resource "fabricext_lakehouse_onelake_security" "bronze_lh" {
  workspace_id               = data.fabricext_item.bronze_lh.workspace_id
  lakehouse_id               = data.fabricext_item.bronze_lh.id
  remove_default_reader_role = true
}

# Declaratively provision fine-grained OneLake Data Access Role:
resource "fabricext_lakehouse_data_access_role" "bronze_readers" {
  workspace_id   = data.fabricext_item.bronze_lh.workspace_id
  lakehouse_name = data.fabricext_item.bronze_lh.display_name
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"

  depends_on = [
    fabricext_lakehouse_onelake_security.bronze_lh,
    fabricext_item_share_gate.bronze_gate,
  ]
}
```

## Authentication

The provider supports Microsoft Entra ID authentication with parameter parity to the official `microsoft/fabric` provider:

| Authentication Method | Required Provider Attributes / Environment Variables |
| :--- | :--- |
| **Azure CLI (Default / Local)** | `use_cli = true` (or empty provider block when logged into `az login`) |
| **Service Principal (Secret)** | `client_id`, `client_secret`, `tenant_id` (or `ARM_CLIENT_SECRET`) |
| **Service Principal (Cert)** | `client_id`, `client_certificate_file_path` / `client_certificate_password`, `tenant_id` |
| **Managed Identity (MSI)** | `use_msi = true`, optional `client_id` for user-assigned MSI |
| **Workload Identity (OIDC)** | `use_oidc = true`, `oidc_token` or `oidc_token_file_path`, `client_id`, `tenant_id` |
| **Sovereign Clouds** | `environment = "public" \| "usgovernment" \| "china"` |

See the [Provider Documentation](docs/index.md) for full authentication examples.

## Permissions Module

An in-repo reusable HCL module (`modules/permissions`) is provided to declaratively manage Lakehouse, Warehouse, and SQL Database permissions alongside OneLake Data Access Roles and security lifecycles across workspaces in a single invocation. See [modules/permissions/README.md](modules/permissions/README.md) for documentation and examples.

## Development and Verification

All tools and tasks are managed through [mise](https://mise.jdx.dev/):

```bash
mise install          # Install all pinned toolchains (with 7-day release cooldown)
mise run init         # Initialize Tessl rules and install prek git hooks
mise run check        # Run lint, scan, specs:check, test:unit, test:acc, docs:check, and spec:verify
mise run install-local # Install provider binary into ~/.terraform.d/plugins local mirror
```

## Documentation

- [Architecture & Design Specification (`DESIGN.md`)](DESIGN.md)
- [Official Provider Comparison & Migration Guide (`guides/official_provider_comparison.md`)](guides/official_provider_comparison.md)
- [Product Roadmap & Official Provider Parity (`ROADMAP.md`)](ROADMAP.md)
- [Contributing Guide (`CONTRIBUTING.md`)](CONTRIBUTING.md)
- [Security Policy (`SECURITY.md`)](SECURITY.md)
