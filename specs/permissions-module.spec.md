---
name: Declarative Workspace Permissions Wrapper Module
description: Reusable in-repo Terraform HCL module flattening nested Warehouse, Lakehouse, and SQL Database permission matrices
targets:
  - ../modules/permissions/versions.tf
  - ../modules/permissions/variables.tf
  - ../modules/permissions/main.tf
  - ../modules/permissions/outputs.tf
---

# Declarative Permissions Wrapper Module

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

- Flattens `lakehouses` sub-maps via `flatten([...])` into deterministic composite `for_each` keys (`{lakehouse_name}/{role_name}`) over `fabricext_lakehouse_data_access_role`, handling omitted or empty sub-maps cleanly without error
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Supports direct item UUID keys as well as display names across `lakehouses`, and maps advanced Lakehouse `decision_rules` (with `row_constraints` and `column_constraints`), `entra_members`, and `fabric_item_members` cleanly to underlying resources
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixWithRLSandCLS`
- Enforces explicit `depends_on = [fabricext_lakehouse_onelake_security.this]` on `fabricext_lakehouse_data_access_role.this` across all configured Lakehouses so that OneLake security verification and `DefaultReader` role purging complete before any role is provisioned
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Exports `lakehouse_data_access_role_ids`, `lakehouse_onelake_security_ids`, and backward-compatible alias maps
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
