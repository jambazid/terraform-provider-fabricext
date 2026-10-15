# Fabric Permissions Module

This module flattens and declaratively provisions workspace-wide Microsoft Fabric Lakehouse OneLake Data Access Roles and security lifecycles using a single structured matrix definition.

## Overview

- **Matrix Flattening**: Express multiple permissions across many items and principals without repeating boilerplate resource blocks.
- **Atomic Management**: Creates individual `fabricext_lakehouse_data_access_role` and `fabricext_lakehouse_onelake_security` resources under the hood, ensuring isolated updates and clean state tracking. *(Note: `warehouses` and `sql_databases` input blocks are deprecated and no longer create resources, as Microsoft Fabric does not offer public item-sharing REST APIs for those services).*

## Prerequisites

~> **Important:** **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

~> **Note:** **Lakehouse Security Sequencing**: When managing Lakehouses, the module automatically coordinates `fabricext_lakehouse_onelake_security` to verify security enablement and purge `DefaultReader` before creating OneLake Data Access Roles, enforcing execution order via an explicit `depends_on` across all Lakehouses.

## Staged Usage Examples

### Stage 1: Basic Permissions

Manage standard table access across Lakehouses using simple mode:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

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

Filter rows for specific principals using T-SQL Boolean predicates in `row_constraints`:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

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

Restrict column access to an explicit allowlist using `column_constraints`:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

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

Combine users, groups, service principals, and managed identities on the same role:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

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

Dynamically derive role membership from users holding item permissions on another Fabric item (member virtualization) using `fabric_item_members`:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

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

Control OneLake security verification and `DefaultReader` role purging behavior:

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=0.6.0"

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

## Matrix Object Geometry

The `fabric_permissions_matrix` variable accepts a structured object with the following schema hierarchy:

### Top-Level Attributes

| Attribute | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `workspace_id` | `string` | yes | - | 36-character UUID of the Microsoft Fabric workspace. |
| `warehouses` | `map(map(list(object)))` | no | `{}` | (Deprecated) Map of Warehouse identifiers (names or UUIDs) to role maps (`read`, `write`, `reshare`). Retained as a no-op returning an empty map `{}`. |
| `sql_databases` | `map(map(list(object)))` | no | `{}` | (Deprecated) Map of SQL Database identifiers (names or UUIDs) to role maps (`read`, `read_data`, `read_spark`, `write`, `reshare`). Retained as a no-op returning an empty map `{}`. |
| `lakehouses` | `map(map(object))` | no | `{}` | Map of Lakehouse identifiers (names or UUIDs) to role definition objects. |
| `manage_lakehouse_security` | `bool` | no | `true` | Automatically provisions `fabricext_lakehouse_onelake_security` per unique Lakehouse. |
| `remove_lakehouse_default_reader_role` | `bool` | no | `true` | Purges the open `DefaultReader` role created by Fabric upon enabling OneLake security. |

### Warehouse & SQL Principals

Each principal entry in `warehouses` and `sql_databases` role lists supports:

| Field | Type | Required | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `id` | `string` | yes | - | Microsoft Entra ID Object ID of the principal. |
| `type` | `string` | no | `"Group"` | Entra ID principal type: `User`, `Group`, `ServicePrincipal`, or `ServicePrincipalProfile`. |

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
| fabricext | `>= 0.5.0` |

## Inputs

| Name | Description | Type | Required |
| :--- | :--- | :--- | :--- |
| `fabric_permissions_matrix` | Declarative permission matrix mapping Warehouses, SQL Databases, and Lakehouse roles to Entra ID principals. | `object(...)` | yes |

## Outputs

| Name | Description |
| :--- | :--- |
| `warehouse_permission_ids` | (Deprecated) Empty map `{}` retained for interface compatibility; Warehouse item sharing was tombstoned in v0.3.0 and scheduled for removal in v0.5.0. |
| `sql_database_permission_ids` | (Deprecated) Empty map `{}` retained for interface compatibility; SQL Database item sharing was tombstoned in v0.3.0 and scheduled for removal in v0.5.0. |
| `lakehouse_data_access_role_ids` | Map of flattened Lakehouse Data Access Role keys to composite resource IDs. |
| `lakehouse_permission_ids` | (Deprecated) Map of flattened Lakehouse Data Access Role keys to composite resource IDs. Kept as alias for backwards compatibility. |
| `lakehouse_onelake_security_ids` | Map of unique Lakehouse keys to composite OneLake security resource IDs. |
