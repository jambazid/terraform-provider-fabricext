---
name: SQL Database Permission Resource (fabricext_sql_database_permission)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric SQL Database item-level permissions (Deprecated)
targets:
  - ../internal/provider/sql_database_permission_resource.go
  - ../internal/provider/sql_database_permission_resource_test.go
---

# SQL Database Permission Resource (`fabricext_sql_database_permission`)

## Deprecation Notice

- `fabricext_sql_database_permission` is deprecated and will be removed in `v0.4.0`. Microsoft Fabric does not provide REST API routes for item-level sharing on SQL Databases.
- Practitioners must configure permissions using Fabric portal Quick Actions and in-database SQL roles or Purview policies.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestSQLDatabasePermissionResource_TombstoneLifecycle`

## Schema & Plan Modifiers

- The resource schema defines `DeprecationMessage` alerting practitioners during plan and apply.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestSQLDatabasePermissionResource_TombstoneLifecycle`
- `workspace_id`, `sql_database_id`, and `principal_id` validate UUID format at plan time; `role_type` validates `OneOf("read", "read_data", "read_spark", "write", "reshare")`; `principal_type` defaults to `"Group"` and validates `OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile")`.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_ValidationErrors`

## Tombstoned Lifecycle

- `Create` and `Update` return an error diagnostic explaining that item-level sharing endpoints do not exist in Fabric, failing without issuing HTTP calls.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestAccSQLDatabasePermissionResource_DeprecatedCreateFails`
- `Read` preserves prior state in memory without making network requests.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestSQLDatabasePermissionResource_TombstoneLifecycle`
- `Delete` removes the resource from state and emits an informational warning diagnostic without making network calls.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestSQLDatabasePermissionResource_TombstoneLifecycle`
- `ImportState` returns an error diagnostic directing practitioners to portal Quick Actions and SQL roles.
  `[@test] ../internal/provider/sql_database_permission_resource_test.go::TestSQLDatabasePermissionResource_TombstoneLifecycle`
