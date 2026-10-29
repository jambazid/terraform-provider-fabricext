---
name: Declarative Workspace Permissions Wrapper Module
description: Reusable in-repo Terraform HCL module flattening unified Warehouse, Lakehouse, and SQL Database permission matrices
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
      raw_bronze_lh = {
        permissions = {
          "11111111-1111-1111-1111-111111111111" = {
            type        = "Group"
            permissions = ["Read"]
          }
        }
        data_access_roles = {
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
}
```

`[@test] ../internal/provider/provider_test.go`

## Matrix Flattening & Outputs

- Flattens unified `lakehouses`, `warehouses`, and `sql_databases` sub-maps via `flatten([...])` into deterministic composite `for_each` keys over `fabricext_*_permission` and `fabricext_lakehouse_data_access_role`, handling omitted or empty sub-maps cleanly without error
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Chains `fabricext_item_share_gate` verification resources after item-level permissions when `verify_item_shares = true`, verifying effective permissions via the Fabric Admin API
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Allows callers without Fabric Admin permissions to bypass Admin API share gate verification via `verify_item_shares = false` while maintaining direct item permissions
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_VerifySharesDisabled`
- Supports direct item UUID keys as well as display names across `lakehouses`, and maps advanced Lakehouse `decision_rules` (with `row_constraints` and `column_constraints`), `entra_members`, and `fabric_item_members` cleanly to underlying resources
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixWithRLSandCLS`
- Supports opting out of OneLake security enablement management via `manage_lakehouse_security = false`
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_LakehouseSecurityOptOut`
- Threads top-level `notify` and `message` input variables with optional per-principal overrides into `fabricext_warehouse_permission`, `fabricext_sql_database_permission`, and `fabricext_lakehouse_permission`, enforcing that `message` requires `notify = true`
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_NotificationAndValidation`
- Enforces explicit `depends_on = [fabricext_lakehouse_onelake_security.this, fabricext_lakehouse_permission.this, fabricext_item_share_gate.lakehouse]` on `fabricext_lakehouse_data_access_role.this` across all configured Lakehouses
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
- Exports `warehouse_permission_ids`, `sql_database_permission_ids`, `lakehouse_permission_ids`, `lakehouse_data_access_role_ids`, `lakehouse_onelake_security_ids`, and `item_share_gate_ids`
  `[@test] ../internal/provider/provider_test.go::TestAccPermissionsModule_MatrixFlattening`
