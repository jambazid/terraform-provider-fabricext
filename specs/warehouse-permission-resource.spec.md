---
name: Warehouse Permission Resource (fabricext_warehouse_permission)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric Warehouse item-level permissions (Deprecated)
targets:
  - ../internal/provider/warehouse_permission_resource.go
  - ../internal/provider/warehouse_permission_resource_test.go
---

# Warehouse Permission Resource (`fabricext_warehouse_permission`)

## Deprecation Notice

- `fabricext_warehouse_permission` is deprecated and will be removed in `v0.4.0`. Microsoft Fabric does not provide REST API routes for item-level sharing on Warehouses.
- Practitioners must configure permissions using Fabric portal sharing and in-database T-SQL controls (`GRANT`/`DENY`/`REVOKE`).
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestWarehousePermissionResource_TombstoneLifecycle`

## Schema & Plan Modifiers

- The resource schema defines `DeprecationMessage` alerting practitioners during plan and apply.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestWarehousePermissionResource_TombstoneLifecycle`
- `workspace_id`, `warehouse_id`, and `principal_id` validate UUID format at plan time; `role_type` validates `OneOf("read", "write", "reshare")`; `principal_type` defaults to `"Group"` and validates `OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile")`.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_ValidationErrors`

## Tombstoned Lifecycle

- `Create` and `Update` return an error diagnostic explaining that item-level sharing endpoints do not exist in Fabric, failing without issuing HTTP calls.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_DeprecatedCreateFails`
- `Read` preserves prior state in memory without making network requests.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestWarehousePermissionResource_TombstoneLifecycle`
- `Delete` removes the resource from state and emits an informational warning diagnostic without making network calls.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestWarehousePermissionResource_TombstoneLifecycle`
- `ImportState` returns an error diagnostic directing practitioners to portal sharing and T-SQL.
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestWarehousePermissionResource_TombstoneLifecycle`
