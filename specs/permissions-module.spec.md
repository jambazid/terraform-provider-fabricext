---
name: Declarative Workspace Permissions Wrapper Module
description: Reusable in-repo Terraform HCL module flattening nested Warehouse, Lakehouse, and SQL Database permission matrices
targets:
  - ../modules/permissions/versions.tf
  - ../modules/permissions/variables.tf
  - ../modules/permissions/main.tf
  - ../modules/permissions/outputs.tf
---

# Declarative Workspace Permissions Wrapper Module (`modules/permissions`)

## Module Interface

```hcl
module "fabric_permissions" {
  source = "./modules/permissions"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"
    warehouses = {
      sales_analytics_wh = {
        read  = [{ id = "11111111-1111-1111-1111-111111111111", type = "Group" }]
        write = [{ id = "22222222-2222-2222-2222-222222222222", type = "ServicePrincipal" }]
      }
    }
    sql_databases = {
      operational_orders_db = {
        read_data = [{ id = "11111111-1111-1111-1111-111111111111", type = "Group" }]
      }
    }
    lakehouses = {
      raw_bronze_lh = {
        BronzeReaders = {
          paths          = ["/Tables/customers"]
          actions        = ["Read"]
          principal_ids  = ["11111111-1111-1111-1111-111111111111"]
          principal_type = "Group"
        }
      }
    }
  }
}
```

`[@test] ../internal/provider/provider_test.go`

## Matrix Flattening & Outputs

- Flattens `warehouses`, `sql_databases`, and `lakehouses` sub-maps via `flatten([...])` into deterministic composite `for_each` keys (`{item_name}/{principal_type}/{principal_id}` for Warehouses and SQL Databases so role updates occur in-place, and `{lakehouse_name}/{role_name}` for Lakehouses) over `fabricext_warehouse_permission`, `fabricext_sql_database_permission`, and `fabricext_lakehouse_permission`, handling omitted or empty sub-maps cleanly without error
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Exports `warehouse_permission_ids`, `sql_database_permission_ids`, and `lakehouse_permission_ids` outputs
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
