---
name: Fabric SQL Database Permission Resource (fabricext_sql_database_permission)
description: Declarative management of direct item-level permissions on Microsoft Fabric SQL Databases
targets:
  - ../internal/provider/sql_database_permission_resource.go
  - ../internal/provider/sql_database_permission_resource_test.go
---

# Fabric SQL Database Permission (`fabricext_sql_database_permission`)

## HCL Contract

```hcl
resource "fabricext_sql_database_permission" "example" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_id   = "00000000-0000-0000-0000-000000000002"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "User"
  permissions       = ["Read", "ReadData", "Execute", "Reshare"]
}
```

## Lifecycle & Validation Behavior

- Creates direct item-level permissions on Microsoft Fabric SQL Databases with display name or ID resolution and verifies composite ID import
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDAndImport`
- Detects item deletion in Fabric during Read and cleanly removes resource from state to plan re-creation without error diagnostics
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_DriftDetectionAndRecreation`
- Validates item type during Read and Create and reports actionable diagnostic error when target item type is not SQLDatabase
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_TypeMismatchError`
