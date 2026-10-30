---
page_title: "Declarative Matrix Composition"
subcategory: "Guides"
description: |-
  How to declaratively manage workspace-level permission matrices across Lakehouses, Warehouses, and SQL Databases using native HCL for_each and the reusable permissions module.
---

# Declarative Matrix Composition

Managing item-level permissions and OneLake Data Access Roles across multiple users, groups, and Fabric items requires a scalable approach to infrastructure as code.

This guide outlines architectural patterns for composing permissions matrices across Microsoft Fabric workspaces, contrasting zero-module native Terraform HCL with the in-repo reusable permissions wrapper module (`modules/permissions`).

---

## Design Principles

Following HashiCorp Provider Design Principles, each active `fabricext_*` resource manages a single, atomic component:

- `fabricext_warehouse_permission`: Manages direct item-level sharing permissions on a Warehouse.
- `fabricext_sql_database_permission`: Manages direct item-level sharing permissions on a SQL Database.
- `fabricext_lakehouse_permission`: Manages direct item-level sharing permissions on a Lakehouse.
- `fabricext_item_share_gate`: Verifies effective direct and inherited item permissions via the official Fabric Admin REST API.
- `fabricext_lakehouse_data_access_role`: Manages a single OneLake Data Access Role on a Lakehouse.
- `fabricext_lakehouse_onelake_security`: Manages OneLake Data Access Security enablement verification and `DefaultReader` role purging.
- `fabricext_item`: (Data Source) Resolves Microsoft Fabric items by workspace and display name.

This atomic resource model decouples individual permission bindings from one another. An issue granting permissions to one item does not block or destroy grants on unrelated items.

---

## Direct Resources vs Module

Choose the deployment pattern that matches your organization's operating model:

| Dimension | Direct Resources (`fabricext_*`) | Reusable Permissions Module (`modules/permissions`) |
| :--- | :--- | :--- |
| **Primary Use Case** | Decentralized domain teams, single-item pipelines, custom wrappers | Centralized platform governance, multi-item access matrices |
| **Configuration Model** | Individual resource blocks per item and principal | Unified hierarchical HCL object or YAML/JSON matrix file |
| **Dependency Management** | Explicit manual `depends_on` between security and roles | Automated sequencing: security verification before role creation |
| **Share Verification** | Standalone `fabricext_item_share_gate` blocks wired as needed | Automatic chaining when `verify_item_shares = true` |
| **Blast Radius** | Scoped strictly to the individual resource block | Scoped to the module matrix definition |

### When to Use Direct Resources

1. **Decentralized Item Ownership**: When domain teams provision their own Warehouses or Lakehouses and manage their own security alongside item definitions.
2. **Third-Party Module Integration**: When embedding Fabric permissions into larger application modules (e.g. provisioning an app-specific SQL Database and its Service Principal share together).
3. **Fine-Grained Lifecycle Tuning**: When specific resources require unique polling timeouts, custom ignore_changes, or custom provider aliases.

### When to Use the Module

1. **Centralized Workspace Governance**: When platform security engineers manage access across many Warehouses, SQL Databases, and Lakehouses from a single definition.
2. **Multi-Role Lakehouse Matrices**: When Lakehouses define complex combinations of simple paths, RLS predicates, CLS column lists, and mixed Entra/item members.
3. **Turnkey Zero-Trust Enablement**: When OneLake security must be automatically verified and `DefaultReader` purged before roles are created across all Lakehouses.

---

## Workspace Role Prerequisites

~> **Important:** **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

~> **Important:** **Share Verification Privileges**: The default `verify_item_shares = true` setting chains `fabricext_item_share_gate` to query effective permissions via the Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`). Calling the Admin API strictly requires the executing identity to hold the **Fabric Administrator** tenant role or be a Service Principal enrolled in the tenant setting *"Service principals can access read-only admin APIs"* with delegated scope `Tenant.Read.All` or `Tenant.ReadWrite.All`. Teams operating with workspace-scoped permissions only should set `verify_item_shares = false`.

---

## Lifecycle & Key Invariants

When using Terraform `for_each` to declare multiple OneLake role resources, the map key definition directly controls how Terraform plans resource updates.

Keying `for_each` by `{lakehouse_name}/{role_name}` ensures that role updates (such as updating paths, actions, or members) execute **in-place** via RFC 9110 `If-Match` ETag read-modify-write rather than destroying and recreating the role:

```text
Map Key Structure:
  "<lakehouse_name>/<role_name>"

Example:
  "curated_lakehouse/SalesReaders"
```

---

## Zero-Module Native HCL

You can manage an entire workspace permissions matrix without external modules by declaring a local map and passing it to `for_each`:

```hcl
locals {
  workspace_id = "00000000-0000-0000-0000-000000000001"

  lakehouse_roles = {
    "raw_lakehouse/RawReaders" = {
      lakehouse_name = "raw_lakehouse"
      role_name      = "RawReaders"
      paths          = ["/Tables/customers", "/Files/landing"]
      actions        = ["Read"]
      principal_ids  = ["11111111-1111-1111-1111-111111111111"]
      principal_type = "Group"
    }
    "curated_lakehouse/CuratedReaders" = {
      lakehouse_name = "curated_lakehouse"
      role_name      = "CuratedReaders"
      paths          = ["/Tables/curated"]
      actions        = ["Read"]
      principal_ids  = ["22222222-2222-2222-2222-222222222222"]
      principal_type = "User"
    }
  }

  lakehouses = toset([
    "raw_lakehouse",
    "curated_lakehouse",
  ])
}

resource "fabricext_lakehouse_onelake_security" "security" {
  for_each = local.lakehouses

  workspace_id               = local.workspace_id
  lakehouse_name             = each.value
  remove_default_reader_role = true
}

resource "fabricext_lakehouse_data_access_role" "roles" {
  for_each = local.lakehouse_roles

  workspace_id   = local.workspace_id
  lakehouse_name = each.value.lakehouse_name
  role_name      = each.value.role_name
  paths          = each.value.paths
  actions        = each.value.actions
  principal_ids  = each.value.principal_ids
  principal_type = each.value.principal_type

  depends_on = [
    fabricext_lakehouse_onelake_security.security,
  ]
}
```

---

## Reusable Permissions Module

For teams managing complex permission sets across multiple Lakehouses, Warehouses, and SQL Databases, the in-repo [`modules/permissions`](https://github.com/jambazid/terraform-provider-fabricext/tree/main/modules/permissions) module flattens multi-dimensional matrices into atomic provider resources.

The module supports a gradual progression from flat permission sets to granular row-level and column-level security.

### Stage 1: Basic Permissions

Provisions standard Lakehouse storage paths using simple mode:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      raw_lakehouse = {
        data_access_roles = {
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
}
```

### Stage 2: Row-Level Security

Restricts Lakehouse queries to authorized rows using T-SQL Boolean predicates in `row_constraints`. Members of the role can only view rows matching the predicate:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        data_access_roles = {
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
}
```

### Stage 3: Column-Level Security

Limits Lakehouse column access using an explicit allowlist in `column_constraints`. Columns omitted from `columns` are masked and inaccessible to role members:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        data_access_roles = {
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
}
```

### Stage 4: Heterogeneous Principals

In simple mode, roles require uniform principal types. In advanced mode (`entra_members`), users, groups, service principals, and managed identities can be combined on the same role:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      gold_lakehouse = {
        data_access_roles = {
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
}
```

### Stage 5: Item Membership

Dynamically derives role membership from users holding item permissions on another Fabric item (member virtualization) using `fabric_item_members`:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      curated_lakehouse = {
        data_access_roles = {
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
}
```

~> **Note:** **Storage Actions vs. Item Access**: `decision_rules.actions` (`Read`, `Write`) defines the **storage-plane access granted** on the Lakehouse files. In contrast, `fabric_item_members.item_access` (`ReadAll`, `Read`) defines the **prerequisite control-plane permission** a user must hold on `source_path` to qualify for role membership.

### Stage 6: Security Lifecycle

By default, the module coordinates `fabricext_lakehouse_onelake_security` before role creation to verify that OneLake Data Access Security is enabled and purge the default open `DefaultReader` role.

You can configure this behavior or opt out via matrix flags:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    # Enabled by default: verifies security enablement and purges open DefaultReader role
    manage_lakehouse_security            = true
    remove_lakehouse_default_reader_role = true

    lakehouses = {
      secure_lakehouse = {
        data_access_roles = {
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
}
```

### Stage 7: Unified Permissions

Item-level permissions across Warehouses, SQL Databases, and Lakehouses can be declared alongside OneLake Data Access Roles:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    # Automatically verify direct item permissions via Admin API gate
    verify_item_shares = true

    warehouses = {
      analytics_wh = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            permissions    = ["Read"]
            principal_type = "Group"
          }
        }
      }
    }

    sql_databases = {
      orders_db = {
        permissions = {
          "22222222-2222-2222-2222-222222222222" = {
            permissions    = ["Read", "Write"]
            principal_type = "ServicePrincipal"
          }
        }
      }
    }

    lakehouses = {
      curated_lh = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            permissions    = ["Read"]
            principal_type = "Group"
          }
        }

        data_access_roles = {
          CuratedReaders = {
            paths          = ["/Tables/curated"]
            actions        = ["Read"]
            principal_ids  = ["11111111-1111-1111-1111-111111111111"]
            principal_type = "Group"
          }
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
| `warehouses` | `map(object)` | no | `{}` | Map of Warehouse identifiers (names or UUIDs) to item configuration with `permissions` map keyed by principal UUID. |
| `sql_databases` | `map(object)` | no | `{}` | Map of SQL Database identifiers (names or UUIDs) to item configuration with `permissions` map keyed by principal UUID. |
| `lakehouses` | `map(object)` | no | `{}` | Unified map of Lakehouse identifiers (names or UUIDs) configuring `permissions`, `data_access_roles`, and per-item `manage_onelake_security`. |
| `verify_item_shares` | `bool` | no | `true` | Validates applied item permissions through `fabricext_item_share_gate` before outputs complete. |
| `manage_lakehouse_security` | `bool` | no | `true` | Automatically provisions `fabricext_lakehouse_onelake_security` per Lakehouse when roles are configured. |
| `remove_lakehouse_default_reader_role` | `bool` | no | `true` | Purges the open `DefaultReader` role created by Fabric upon enabling OneLake security. |
| `notify` | `bool` | no | `false` | Optional flag to send email notifications for item shares across all workloads. |
| `message` | `string` | no | `null` | Optional custom email notification message sent when notify is true. |

### Permissions Object Schema

Each principal entry in `warehouses`, `sql_databases`, and `lakehouses.*.permissions` maps is keyed by the principal's Microsoft Entra Object ID (UUID):

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `permissions` | `set(string)` | yes | - | Direct item access permissions: must include `"Read"`; allowed: `"Read"`, `"Write"`, `"ReadData"`, `"ReadAll"`, `"Reshare"`, `"Execute"`. |
| `principal_type` | `string` | no | `"Group"` | Entra ID principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. |
| `notify` | `bool` | no | `null` | Optional per-grant override for sending email notifications (inherits top-level `var.notify`). |
| `message` | `string` | no | `null` | Optional per-grant custom email notification message when effective `notify` is true. |

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

## Migrating to the Module

When refactoring existing standalone provider resources into `modules/permissions`, use Terraform `moved` blocks to transfer state without destroying and recreating remote permissions. The module constructs predictable `for_each` instance keys based on item keys, lowercase principal UUIDs, and role names:

```hcl
# 1. Warehouse Permissions
moved {
  from = fabricext_warehouse_permission.sales_readers
  to   = module.workspace_permissions.fabricext_warehouse_permission.this["sales_analytics_wh/11111111-1111-1111-1111-111111111111"]
}

# 2. SQL Database Permissions
moved {
  from = fabricext_sql_database_permission.orders_readers
  to   = module.workspace_permissions.fabricext_sql_database_permission.this["operational_orders_db/11111111-1111-1111-1111-111111111111"]
}

# 3. Lakehouse Item Permissions
moved {
  from = fabricext_lakehouse_permission.raw_readers
  to   = module.workspace_permissions.fabricext_lakehouse_permission.this["raw_lakehouse/11111111-1111-1111-1111-111111111111"]
}

# 4. Lakehouse Data Access Roles
moved {
  from = fabricext_lakehouse_data_access_role.raw_readers
  to   = module.workspace_permissions.fabricext_lakehouse_data_access_role.this["raw_lakehouse/RawReaders"]
}

# 5. Lakehouse OneLake Security
moved {
  from = fabricext_lakehouse_onelake_security.raw_lakehouse
  to   = module.workspace_permissions.fabricext_lakehouse_onelake_security.this["raw_lakehouse"]
}

# 6. Item Share Verification Gate (if enabled)
moved {
  from = fabricext_item_share_gate.sales_readers
  to   = module.workspace_permissions.fabricext_item_share_gate.warehouse["sales_analytics_wh/11111111-1111-1111-1111-111111111111"]
}
```

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Access Control Playbook](access_control_playbook.md)** | Start Here routing table and 11 verified scenario execution cards. |
| **[Enterprise Access Control Overview](use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Warehouse Schema Isolation](use_case_warehouse_schema_isolation.md)** | Least-privilege `read` connectivity, Entra ID naming, and SQL RBAC. |
| **[Lakehouse OneLake Security](use_case_lakehouse_onelake_security.md)** | Data Access Roles, path filters, storage RLS/CLS, and shortcuts. |
| **[Official Provider Comparison](official_provider_comparison.md)** | Architectural differences, dual-provider coexistence, and state migration. |
