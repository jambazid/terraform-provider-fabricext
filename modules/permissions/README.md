# Fabric Permissions Module

This module flattens and declaratively provisions workspace-wide Microsoft Fabric item-level permissions (Warehouses, SQL Databases, Lakehouses) and OneLake Data Access Roles using a single structured matrix definition.

## Overview

- **Unified Matrix**: Manage direct item-level sharing permissions and Lakehouse OneLake Data Access Roles within a single coherent configuration.
- **Cross-Plane Verification**: Chains `fabricext_item_share_gate` after item permissions to verify effective ACLs via the Fabric Admin API before downstream operations run.
- **Stateful Drift Self-Healing**: Detects portal drift during refresh and converges back to declared configuration in place.
- **Atomic Management**: Creates individual `fabricext_*_permission`, `fabricext_lakehouse_data_access_role`, `fabricext_lakehouse_onelake_security`, and `fabricext_item_share_gate` resources under the hood.

~> **Note:** Item-level permission resources call internal Power BI service endpoints because Microsoft Fabric does not yet offer a v1 REST write API for item sharing. Endpoint behavior may change without notice; pin provider versions and validate upgrades in non-production workspaces first.

## Prerequisites

~> **Important:** **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

~> **Important:** **Share Verification Privileges**: The default `verify_item_shares = true` setting chains `fabricext_item_share_gate` to query effective permissions via the Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`). Calling the Admin API strictly requires the executing identity to hold the **Fabric Administrator** tenant role or be a Service Principal enrolled in the tenant setting *"Service principals can access read-only admin APIs"* with delegated scope `Tenant.Read.All` or `Tenant.ReadWrite.All`. Teams operating with workspace-scoped permissions only should set `verify_item_shares = false`.

~> **Note:** **Lakehouse Security Sequencing**: When managing Lakehouses, the module automatically coordinates `fabricext_lakehouse_onelake_security` to verify security enablement and purge `DefaultReader` before creating OneLake Data Access Roles, enforcing execution order via an explicit `depends_on` across all Lakehouses.

## Staged Usage Examples

### Stage 1: Basic Permissions

Manage direct item permissions and standard table access across Lakehouses using simple mode:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id          = "00000000-0000-0000-0000-000000000001"
    verify_item_shares    = true
    share_gate_match_type = "subset"

    warehouses = {
      sales_analytics_wh = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            type        = "Group"
            permissions = ["Read", "ReadData"]
          }
        }
      }
    }

    sql_databases = {
      operational_orders_db = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            type        = "Group"
            permissions = ["Read", "ReadData"]
          }
        }
      }
    }

    lakehouses = {
      raw_lakehouse = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            type        = "Group"
            permissions = ["Read"]
          }
        }
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

Filter rows for specific principals using T-SQL Boolean predicates in `row_constraints`:

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

Restrict column access to an explicit allowlist using `column_constraints`:

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

Combine users, groups, service principals, and managed identities on the same role:

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

Dynamically derive role membership from users holding item permissions on another Fabric item (member virtualization) using `fabric_item_members`:

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
                paths   = ["/Tables/shared_dimension"]
                actions = ["Read"]
              }
            ]
            fabric_item_members = [
              {
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

~> **Note:** **Storage Actions vs. Item Access**: `decision_rules.actions` (`Read`, `Write`) defines the storage-plane access granted on the Lakehouse files. In contrast, `fabric_item_members.item_access` (`ReadAll`, `Read`) defines the prerequisite control-plane permission a user must hold on `source_path` to qualify for role membership.

### Stage 6: Security Lifecycle

Control OneLake security verification and `DefaultReader` role purging behavior:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.7.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

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

## Module Inputs

| Input | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `fabric_permissions_matrix` | `object` | yes | - | Declarative permission matrix mapping Fabric workloads to Entra principals. |
| `notify` | `bool` | no | `false` | Module-level flag to send email notifications for item shares via `/metadata/access/notify`. |
| `message` | `string` | no | `null` | Module-level custom notification message. Requires `notify = true`. |

## Matrix Object Geometry

The `fabric_permissions_matrix` variable accepts a structured object with the following schema hierarchy:

### Top-Level Attributes

| Attribute | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `workspace_id` | `string` | yes | - | 36-character UUID of the Microsoft Fabric workspace. |
| `verify_item_shares` | `bool` | no | `true` | Cross-plane Admin API verification sanity lever via `fabricext_item_share_gate`. |
| `share_gate_match_type` | `string` | no | `"subset"` | Verification match type: `subset` or `exact`. |
| `share_gate_timeout` | `string` | no | `"5m"` | Polling timeout bound for verification gate. |
| `warehouses` | `map(object)` | no | `{}` | Map of Warehouse identifiers (names or UUIDs) to warehouse configurations. |
| `sql_databases` | `map(object)` | no | `{}` | Map of SQL Database identifiers (names or UUIDs) to SQL database configurations. |
| `lakehouses` | `map(object)` | no | `{}` | Map of Lakehouse identifiers (names or UUIDs) to unified Lakehouse configurations. |
| `manage_lakehouse_security` | `bool` | no | `true` | Automatically provisions `fabricext_lakehouse_onelake_security` per unique Lakehouse. |
| `remove_lakehouse_default_reader_role` | `bool` | no | `true` | Purges the open `DefaultReader` role created by Fabric upon enabling OneLake security. |

### Warehouse Configuration

Entries in `warehouses` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `verify_item_shares` | `bool` | no | `null` | Per-item override for Admin API share verification. |
| `permissions` | `map(object)` | no | `{}` | Map of Entra ID UUIDs to permissions (`type`, `permissions`, `notify`, `message`). Must include `"Read"`. |

### SQL Database Configuration

Entries in `sql_databases` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `verify_item_shares` | `bool` | no | `null` | Per-item override for Admin API share verification. |
| `permissions` | `map(object)` | no | `{}` | Map of Entra ID UUIDs to permissions (`type`, `permissions`, `notify`, `message`). Must include `"Read"`. |

### Lakehouse Configuration

Entries in `lakehouses` support:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `verify_item_shares` | `bool` | no | `null` | Per-item override for Admin API share verification. |
| `manage_onelake_security` | `bool` | no | `null` | Per-Lakehouse override for OneLake security management. |
| `remove_default_reader_role` | `bool` | no | `null` | Per-Lakehouse override for default reader purging. |
| `permissions` | `map(object)` | no | `{}` | Direct item-level share permissions (`type`, `permissions`, `notify`, `message`). |
| `data_access_roles` | `map(object)` | no | `{}` | Map of role names to Simple Mode or Advanced Mode role definitions. |

### Lakehouse Simple Mode

Mutually exclusive with Advanced Mode. Used for flat path access with uniform principal types:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `paths` | `set(string)` | yes | - | Storage paths within the Lakehouse (e.g. `["/Tables/customers"]`). |
| `actions` | `set(string)` | no | `["Read"]` | Storage action permissions: `Read`, `Write`, or `ReadWrite`. |
| `principal_ids` | `set(string)` | yes | - | Set of Microsoft Entra ID Object IDs. |
| `principal_type` | `string` | no | `"Group"` | Principal type for all IDs: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. |

### Lakehouse Advanced Mode

Mutually exclusive with Simple Mode. Used for granular RLS, CLS, and mixed membership:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `kind` | `string` | no | `"Policy"` | Role kind. Microsoft Fabric currently supports `Policy`. |
| `decision_rules` | `list(object)` | yes | `[]` | Granular storage path rules with optional RLS and CLS constraints. |
| `entra_members` | `list(object)` | no | `[]` | Heterogeneous Microsoft Entra ID principal members. |
| `fabric_item_members` | `list(object)` | no | `[]` | Dynamic item-permission members (member virtualization). |

### Decision Rules

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

### Member Types

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

## Requirements

| Name | Version |
| :--- | :--- |
| terraform | `>= 1.6.0` |
| fabricext | `>= 0.7.0` |

## Inputs

| Name | Description | Type | Required |
| :--- | :--- | :--- | :--- |
| `fabric_permissions_matrix` | Declarative permission matrix mapping Warehouses, SQL Databases, and Lakehouse roles to Entra ID principals. | `object(...)` | yes |
| `notify` | Optional boolean flag to send email notifications for item share permissions across all workloads in the matrix. | `bool` | no |
| `message` | Optional custom email notification message sent when notify is true. | `string` | no |

## Outputs

| Name | Description |
| :--- | :--- |
| `warehouse_permission_ids` | Map of flattened Warehouse permission keys to composite resource IDs. |
| `sql_database_permission_ids` | Map of flattened SQL Database permission keys to composite resource IDs. |
| `lakehouse_permission_ids` | Map of flattened Lakehouse item-level permission keys to composite resource IDs. |
| `lakehouse_data_access_role_ids` | Map of flattened Lakehouse Data Access Role keys to composite resource IDs. |
| `lakehouse_onelake_security_ids` | Map of unique Lakehouse keys to composite OneLake security resource IDs. |
| `item_share_gate_ids` | Map of chained item share verification gate keys to composite resource IDs. |
| `gate_ids` | Alias for `item_share_gate_ids` providing a concise map of chained share verification gate keys to resource IDs. |

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
