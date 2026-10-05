# Terraform Provider for Microsoft Fabric Item Sharing (`jambazid/fabricext`)

> [!WARNING]
> **Pre-Alpha & Stopgap Provider Notice**: `registry.terraform.io/jambazid/fabricextext` (`v0.x`) is a **pre-alpha, purpose-built stopgap provider** created to fill the item-level sharing and OneLake Data Access Role gap (`fabricext_warehouse_permission`, `fabricext_sql_database_permission`, `fabricext_lakehouse_permission`, and `fabricext_item`) until equivalent resources are released in Microsoft's official [`microsoft/fabric` Terraform provider](https://github.com/microsoft/terraform-provider-fabric). Breaking schema changes may occur between `0.x` minor versions, and individual `fabricext_*` resources will be deprecated with migration guides as official equivalents reach General Availability.

## Overview

`terraform-provider-fabricext` is a Terraform Plugin Framework (Protocol v6) provider for declaratively managing Microsoft Fabric item-level permissions and OneLake Data Access Roles across:

| Resource / Data Source | Microsoft Fabric Item Type | Capabilities |
| :--- | :--- | :--- |
| `fabricext_warehouse_permission` | `Warehouse` | Grants, updates (with downgrade revocation), reads, deletes, and imports item-level permissions (`read`, `write`, `reshare`). |
| `fabricext_sql_database_permission` | `SQLDatabase` | Grants, updates (with downgrade revocation), reads, deletes, and imports SQL Database permissions (`read`, `read_data`, `read_spark`, `write`, `reshare`). |
| `fabricext_lakehouse_permission` | `Lakehouse` | Manages OneLake Data Access Roles (`/dataAccessRoles`) with per-Lakehouse mutex locking and `If-Match` ETag Read-Modify-Write concurrency control. |
| `data.fabricext_item` | `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `SemanticModel` | Resolves `(workspace_id, display_name, type)` to canonical item UUIDs (`id`) using a paginated, type-isolated lookup cache. |

## Quick Start

```hcl
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    fabric = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.0"
    }
  }
}

provider "fabricext" {}

resource "fabricext_warehouse_permission" "analytics_readers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  role_type      = "read"
}

resource "fabricext_sql_database_permission" "orders_readers" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_name = "operational_orders_db"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "Group"
  role_type         = "read_data"
}

resource "fabricext_lakehouse_permission" "bronze_readers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"
}
```

## Local Development & Verification (`mise`)

All tools and tasks are managed through [mise](https://mise.jdx.dev/):

```bash
mise install          # Install all pinned toolchains (with 7-day release cooldown)
mise run init         # Initialize Tessl rules and install prek git hooks
mise run check        # Run lint, scan, specs:check, test:unit, test:acc, docs:check, and spec:verify
mise run install-local # Install provider binary into ~/.terraform.d/plugins local mirror
```

## Documentation & Governance

- [Architecture & Design Specification (`DESIGN.md`)](DESIGN.md)
- [Official Provider Comparison & Migration Guide (`docs/guides/official_provider_comparison.md`)](docs/guides/official_provider_comparison.md)
- [Product Roadmap & Official Provider Parity (`ROADMAP.md`)](ROADMAP.md)
- [Contributing Guide (`CONTRIBUTING.md`)](CONTRIBUTING.md)
- [Security Policy (`SECURITY.md`)](SECURITY.md)
