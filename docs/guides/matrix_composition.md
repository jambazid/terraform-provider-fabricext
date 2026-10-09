---
page_title: "Declarative Matrix Composition"
subcategory: "Guides"
description: |-
  How to declaratively manage workspace-level permission matrices across Warehouses, SQL Databases, and Lakehouses using native HCL for_each and the reusable permissions module.
---

# Declarative Matrix Composition

Managing item-level permissions and OneLake Data Access Roles across multiple users, groups, and Fabric items requires a scalable approach to infrastructure as code.

This guide outlines architectural patterns for composing permissions matrices across Microsoft Fabric workspaces, contrasting zero-module native Terraform HCL with the in-repo reusable permissions wrapper module (`modules/permissions`).

---

## Architectural Design Principles

Following HashiCorp Provider Design Principles, each `fabricext_*` resource manages a single, atomic permission binding or OneLake role:

- `fabricext_warehouse_permission`: Manages permissions for a single principal on a Warehouse.
- `fabricext_sql_database_permission`: Manages permissions for a single principal on a SQL Database.
- `fabricext_lakehouse_permission`: Manages a single OneLake Data Access Role on a Lakehouse.

This atomic resource model decouples individual permission bindings from one another. An issue granting permissions to one item does not block or destroy grants on unrelated items.

---

## Workspace Role Prerequisites

~> **Important:** **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

---

## Lifecycle & Key Invariants

When using Terraform `for_each` to declare multiple permission resources, the map key definition directly controls how Terraform plans resource updates.

Keying `for_each` by `{item_name}/{principal_type}/{principal_id}` (omitting `role_type` from the key) ensures that role updates (such as upgrading `role_type` from `read` to `write`) execute **in-place** rather than destroying and recreating the resource:

```text
Map Key Structure:
  "<item_name>/<principal_type>/<principal_id>"

Example:
  "sales_analytics_wh/Group/11111111-1111-1111-1111-111111111111"
```

If `role_type` were included in the map key, changing the role would change the resource address, causing Terraform to schedule a destroy-before-create cycle that briefly leaves the principal with no permissions.

---

## Zero-Module Native HCL

You can manage an entire workspace permissions matrix without external modules by declaring a local map and passing it to `for_each`:

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
    "finance_reporting_wh/User/22222222-2222-2222-2222-222222222222" = {
      warehouse_name = "finance_reporting_wh"
      role_type      = "write"
      principal_id   = "22222222-2222-2222-2222-222222222222"
      principal_type = "User"
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

---

## Reusable Permissions Module

For teams managing complex permission sets across multiple item types, the in-repo [`modules/permissions`](https://github.com/jambazid/terraform-provider-fabricext/tree/main/modules/permissions) module flattens multi-dimensional matrices into atomic provider resources:

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

    lakehouses = {
      raw_lakehouse = {
        RawReaders = {
          paths          = ["/Tables/customers", "/Files/landing"]
          actions        = ["Read"]
          principal_ids  = ["11111111-1111-1111-1111-111111111111"]
          principal_type = "Group"
        }
      }
    }
  }
}
```

*(Note: In local or monorepo development checkouts, you can also use `source = "./modules/permissions"`).*

---

## Lakehouse Member Type Uniformity

~> **Note:** **Member Type Uniformity & Advanced Mode**: In simple mode (`principal_ids`), OneLake Data Access Roles require uniform `principal_type` per role. For heterogeneous member types on the same role, use advanced mode. In `modules/permissions`, advanced configuration uses plural map attributes (`entra_members`, `decision_rules`, `fabric_item_members`), which the module projects to singular resource blocks (`entra_member`, `decision_rule`, `fabric_item_member`) as detailed in the **[Lakehouse OneLake Security Guide](use_case_lakehouse_onelake_security.md)**.

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Use Cases Overview](use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Warehouse Schema Isolation](use_case_warehouse_schema_isolation.md)** | Least-privilege `read` connectivity, Entra ID naming, and SQL RBAC. |
| **[Lakehouse OneLake Security](use_case_lakehouse_onelake_security.md)** | Data Access Roles, path filters, storage RLS/CLS, and shortcuts. |
