---
name: SQL Database Permission Resource (fabricext_sql_database_permission)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric SQL Database item-level sharing and permissions
targets:
  - ../internal/provider/sql_database_permission_resource.go
  - ../internal/provider/sql_database_permission_resource_test.go
---

# SQL Database Permission Resource (`fabricext_sql_database_permission`)

## HCL Contract

```hcl
resource "fabricext_sql_database_permission" "example" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_name = "operational_orders_db"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "Group"     # Optional: User, Group, ServicePrincipal, ServicePrincipalProfile (default: Group)
  role_type         = "read_data" # Required: read, read_data, read_spark, write, reshare
}
```

`[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport`

## Schema, Plan Modifiers & Validators

- `workspace_id`, `sql_database_name`, `principal_id`, and `principal_type` carry `RequiresReplace()` plan modifiers; `id` and `sql_database_id` carry `UseStateForUnknown()`
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport`
- `role_type` validates `OneOf("read", "read_data", "read_spark", "write", "reshare")` mapping to Fabric SQL Database permissions (`["Read"]`, `["Read", "ReadData"]`, `["Read", "ReadAll", "SubscribeOneLakeEvents"]`, `["Read", "Write"]`, `["Read", "Reshare"]`)
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_ValidationErrors`

## CRUD Lifecycle, Downgrade Revocation, Disappears & ImportState

- `Create` resolves `sql_database_name` to `sql_database_id` (`itemType: "SQLDatabase"`), grants the mapped permissions, and sets composite `id = "{workspace_id}/{sql_database_id}/{principal_type}/{principal_id}"`
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport`
- `Update` revokes removed permissions before granting target permissions on role changes (e.g. `"read_spark"` $\rightarrow$ `"read"` or `"write"` $\rightarrow$ `"read_data"`)
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport`
- `Read` calls `resp.State.RemoveResource(ctx)` if the SQL Database or principal permission is deleted out-of-band
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_Disappears`
- `ImportState` parses `{workspace_id}/{sql_database_id}/{principal_type}/{principal_id}` and resolves `sql_database_name` via `GetItemByID`
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport`
