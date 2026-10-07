# Microsoft Fabric Permissions Matrix Module (`modules/permissions`)

This module flattens and declaratively provisions workspace-wide item permissions across Microsoft Fabric Warehouses, SQL Databases, and Lakehouse OneLake Data Access Roles using a single structured matrix definition.

## Features

- **Matrix Flattening**: Express multiple permissions across many items and principals without repeating boilerplate resource blocks.
- **Strict Validation**: Validates UUIDs and allowed role types (`read`, `write`, `reshare`, `read_data`, `read_spark`) at plan time.
- **Atomic Management**: Creates individual `fabricext_warehouse_permission`, `fabricext_sql_database_permission`, and `fabricext_lakehouse_permission` resources under the hood, ensuring isolated updates and clean state tracking.

## Prerequisites

> [!IMPORTANT]
> **Workspace Role Prerequisites**: The executing identity (user, service principal, or managed identity) must have **Admin** or **Member** permissions on the target Microsoft Fabric workspace to manage item-level permissions and OneLake Data Access Roles. Principals with only **Contributor** or **Viewer** workspace roles cannot grant or revoke item permissions.

> [!NOTE]
> **Lakehouse Member Type Uniformity**: Microsoft Fabric OneLake Data Access Roles require uniform `principal_type` per role resource. When assigning access to mixed principal types (e.g. both users and service principals) for the same role, assign them via an Entra ID security `Group`.

## Usage

```hcl
module "workspace_permissions" {
  source = "github.com/jambazid/terraform-provider-fabricext//modules/permissions?ref=v0.1.2"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    warehouses = {
      sales_dw = {
        read = [
          {
            id   = "11111111-1111-1111-1111-111111111111"
            type = "Group"
          }
        ]
        write = [
          {
            id   = "22222222-2222-2222-2222-222222222222"
            type = "User"
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

*(Note: In local or monorepo development checkouts, you can also use `source = "./modules/permissions"`).*

## Requirements

| Name | Version |
| :--- | :--- |
| terraform | `>= 1.6.0` |
| fabricext | `>= 0.1.0` |

## Inputs

| Name | Description | Type | Required |
| :--- | :--- | :--- | :--- |
| `fabric_permissions_matrix` | Declarative permission matrix mapping Warehouses, SQL Databases, and Lakehouse roles to Entra ID principals. | `object(...)` | yes |

## Outputs

| Name | Description |
| :--- | :--- |
| `warehouse_permission_ids` | Map of flattened Warehouse permission keys to composite resource IDs. |
| `sql_database_permission_ids` | Map of flattened SQL Database permission keys to composite resource IDs. |
| `lakehouse_permission_ids` | Map of flattened Lakehouse Data Access Role keys to composite resource IDs. |
