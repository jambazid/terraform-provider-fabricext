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

## Design Principles

Following HashiCorp Provider Design Principles, each `fabricext_*` resource manages a single, atomic permission binding or OneLake role:

- `fabricext_warehouse_permission`: Manages permissions for a single principal on a Warehouse.
- `fabricext_sql_database_permission`: Manages permissions for a single principal on a SQL Database.
- `fabricext_lakehouse_permission`: Manages a single OneLake Data Access Role on a Lakehouse.
- `fabricext_lakehouse_onelake_security`: Manages OneLake Data Access Security enablement verification and `DefaultReader` role purging.

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

For teams managing complex permission sets across multiple item types, the in-repo [`modules/permissions`](https://github.com/jambazid/terraform-provider-fabricext/tree/main/modules/permissions) module flattens multi-dimensional matrices into atomic provider resources.

The module supports a gradual progression from flat permission sets to granular row-level and column-level security.

### Stage 1: Basic Permissions

Provisions standard read/write permissions on Warehouses, SQL Databases, and Lakehouse storage paths using simple mode:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

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
        write = [
          {
            id   = "22222222-2222-2222-2222-222222222222"
            type = "ServicePrincipal"
          }
        ]
      }
    }

    sql_databases = {
      orders_db = {
        read_data = [
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

### Stage 2: Row-Level Security

Restricts Lakehouse queries to authorized rows using T-SQL Boolean predicates in `row_constraints`. Members of the role can only view rows matching the predicate:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        EmeaAnalysts = {
          decision_rules = [
            {
              paths   = ["/Tables/sales"]
              actions = ["Read"]
              effect  = "Permit"
              row_constraints = [
                {
                  table_path = "/Tables/sales"
                  predicate  = "Region = 'EMEA'"
                }
              ]
            }
          ]
          entra_members = [
            {
              object_id   = "11111111-1111-1111-1111-111111111111"
              object_type = "Group"
            }
          ]
        }
      }
    }
  }
}
```

### Stage 3: Column-Level Security

Limits Lakehouse column access using an explicit allowlist in `column_constraints`. Columns omitted from `columns` are masked and inaccessible to role members:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        PublicDataViewers = {
          decision_rules = [
            {
              paths   = ["/Tables/customers"]
              actions = ["Read"]
              effect  = "Permit"
              column_constraints = [
                {
                  table_path = "/Tables/customers"
                  columns    = ["customer_id", "country", "signup_date"]
                  action     = "Read"
                  effect     = "Permit"
                }
              ]
            }
          ]
          entra_members = [
            {
              object_id   = "11111111-1111-1111-1111-111111111111"
              object_type = "Group"
            }
          ]
        }
      }
    }
  }
}
```

### Stage 4: Heterogeneous Principals

In simple mode, roles require uniform principal types. In advanced mode (`entra_members`), users, groups, service principals, and managed identities can be combined on the same role:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      gold_lakehouse = {
        ReportingConsumers = {
          decision_rules = [
            {
              paths   = ["/Tables/reporting"]
              actions = ["Read"]
            }
          ]
          entra_members = [
            {
              object_id   = "11111111-1111-1111-1111-111111111111"
              object_type = "Group"
            },
            {
              object_id   = "22222222-2222-2222-2222-222222222222"
              object_type = "User"
            },
            {
              object_id   = "33333333-3333-3333-3333-333333333333"
              object_type = "ServicePrincipal"
            },
            {
              object_id   = "44444444-4444-4444-4444-444444444444"
              object_type = "ManagedIdentity"
            }
          ]
        }
      }
    }
  }
}
```

### Stage 5: Item Membership

Dynamically derives role membership from users holding item permissions on another Fabric item (member virtualization) using `fabric_item_members`:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        SharedDimensionReaders = {
          decision_rules = [
            {
              # Storage-plane access granted on this curated Lakehouse:
              paths   = ["/Tables/shared_dimension"]
              actions = ["Read"]
            }
          ]
          fabric_item_members = [
            {
              # Prerequisite Fabric item permission required for role membership:
              # Users holding "ReadAll" on the consumer item are dynamically included.
              source_path = "99999999-9999-9999-9999-999999999999/88888888-8888-8888-8888-888888888888"
              item_access = ["ReadAll"]
            }
          ]
        }
      }
    }
  }
}
```

~> **Note:** **Storage Actions vs. Item Access**: `decision_rules.actions` (`Read`, `Write`) defines the **storage-plane access granted** on the Lakehouse files. In contrast, `fabric_item_members.item_access` (`ReadAll`, `Read`) defines the **prerequisite control-plane permission** a user must hold on `source_path` to qualify for role membership.

### Stage 6: Security Lifecycle

By default, the module coordinates `fabricext_lakehouse_onelake_security` before role creation to verify that OneLake Data Access Security is enabled and purge the default open `DefaultReader` role.

You can configure this behavior or opt out via matrix flags:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.3.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    # Enabled by default: verifies security enablement and purges open DefaultReader role
    manage_lakehouse_security            = true
    remove_lakehouse_default_reader_role = true

    lakehouses = {
      secure_lakehouse = {
        SecureReaders = {
          paths          = ["/Tables/confidential"]
          actions        = ["Read"]
          principal_ids  = ["11111111-1111-1111-1111-111111111111"]
          principal_type = "Group"
        }
      }
    }
  }
}
```

---

## Matrix Object Geometry

The `fabric_permissions_matrix` variable accepts a structured object with the following schema hierarchy:

### Top-Level Attributes

| Attribute | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `workspace_id` | `string` | yes | - | 36-character UUID of the Microsoft Fabric workspace. |
| `warehouses` | `map(map(list(object)))` | no | `{}` | Map of Warehouse identifiers (names or UUIDs) to role maps (`read`, `write`, `reshare`). |
| `sql_databases` | `map(map(list(object)))` | no | `{}` | Map of SQL Database identifiers (names or UUIDs) to role maps (`read`, `read_data`, `read_spark`, `write`, `reshare`). |
| `lakehouses` | `map(map(object))` | no | `{}` | Map of Lakehouse identifiers (names or UUIDs) to role definition objects. |
| `manage_lakehouse_security` | `bool` | no | `true` | Automatically provisions `fabricext_lakehouse_onelake_security` per unique Lakehouse. |
| `remove_lakehouse_default_reader_role` | `bool` | no | `true` | Purges the open `DefaultReader` role created by Fabric upon enabling OneLake security. |

### Principal Object Schema

Each principal entry in `warehouses` and `sql_databases` role lists supports:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `id` | `string` | yes | - | Microsoft Entra ID Object ID of the principal. |
| `type` | `string` | no | `"Group"` | Entra ID principal type: `User`, `Group`, `ServicePrincipal`, or `ServicePrincipalProfile`. |

### Simple Mode Attributes

Used for flat path access with uniform principal types (mutually exclusive with Advanced Mode):

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `paths` | `set(string)` | yes | - | Storage paths within the Lakehouse (e.g. `["/Tables/customers"]`). |
| `actions` | `set(string)` | no | `["Read"]` | Storage action permissions: `Read`, `Write`, or `ReadWrite`. |
| `principal_ids` | `set(string)` | yes | - | Set of Microsoft Entra ID Object IDs. |
| `principal_type` | `string` | no | `"Group"` | Principal type for all IDs: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. |

### Advanced Mode Attributes

Used for granular RLS, CLS, and mixed membership (mutually exclusive with Simple Mode):

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `kind` | `string` | no | `"Policy"` | Role kind. Microsoft Fabric currently supports `Policy`. |
| `decision_rules` | `list(object)` | yes | `[]` | Granular storage path rules with optional RLS and CLS constraints. |
| `entra_members` | `list(object)` | no | `[]` | Heterogeneous Microsoft Entra ID principal members. |
| `fabric_item_members` | `list(object)` | no | `[]` | Dynamic item-permission members (member virtualization). |

### Decision Rule Schema

Each entry in `decision_rules` supports:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `paths` | `set(string)` | yes | - | Storage paths within the Lakehouse targeted by this rule. |
| `actions` | `set(string)` | no | `["Read"]` | Storage action permissions: `Read`, `Write`, or `ReadWrite`. |
| `effect` | `string` | no | `"Permit"` | Rule effect. Fabric OneLake security currently supports `Permit`. |
| `row_constraints` | `list(object)` | no | `[]` | Row-Level Security (RLS) predicate filters. |
| `column_constraints` | `list(object)` | no | `[]` | Column-Level Security (CLS) column allowlists. |

### Row & Column Constraints

Entries within `row_constraints` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `table_path` | `string` | yes | - | Table storage path (e.g. `"/Tables/sales"`). |
| `predicate` | `string` | yes | - | T-SQL Boolean filter expression (e.g. `"Region = 'EMEA'"`). |

Entries within `column_constraints` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `table_path` | `string` | yes | - | Table storage path (e.g. `"/Tables/customers"`). |
| `columns` | `set(string)` | yes | - | Set of allowed column names accessible to members. |
| `action` | `string` | no | `"Read"` | Action permission: `Read`. |
| `effect` | `string` | no | `"Permit"` | Constraint effect: `Permit`. |

### Member Type Schemas

Entries within `entra_members` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `object_id` | `string` | yes | - | Microsoft Entra ID Object ID of the principal. |
| `object_type` | `string` | no | `"Group"` | Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. |
| `tenant_id` | `string` | no | `null` | Microsoft Entra ID Tenant ID for cross-tenant principals. |

Entries within `fabric_item_members` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `source_path` | `string` | yes | - | Source path of the Fabric item holding permissions in format `{workspace_id}/{item_id}`. |
| `item_access` | `set(string)` | no | `["ReadAll"]` | Required item access permissions: `ReadAll`. |

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Use Cases Overview](use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Warehouse Schema Isolation](use_case_warehouse_schema_isolation.md)** | Least-privilege `read` connectivity, Entra ID naming, and SQL RBAC. |
| **[Lakehouse OneLake Security](use_case_lakehouse_onelake_security.md)** | Data Access Roles, path filters, storage RLS/CLS, and shortcuts. |
| **[Official Provider Comparison](official_provider_comparison.md)** | Architectural differences, dual-provider coexistence, and state migration. |
