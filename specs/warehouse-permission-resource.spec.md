---
name: Fabric Warehouse Permission Resource (fabricext_warehouse_permission)
description: Declarative management of direct item-level permissions on Microsoft Fabric Warehouses
targets:
  - ../internal/provider/warehouse_permission_resource.go
  - ../internal/provider/warehouse_permission_resource_test.go
---

# Fabric Warehouse Permission (`fabricext_warehouse_permission`)

## HCL Contract

```hcl
resource "fabricext_warehouse_permission" "example" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_id   = "00000000-0000-0000-0000-000000000002"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}
```

## Lifecycle & Validation Behavior

- Creates direct item-level permissions on Microsoft Fabric Warehouses with display name or ID resolution and verifies composite ID import
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDAndImport`
- Detects item deletion in Fabric during Read and cleanly removes resource from state to plan re-creation without error diagnostics
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_DriftDetectionAndRecreation`
- Validates item type during Read and Create and reports actionable diagnostic error when target item type is not Warehouse
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_TypeMismatchError`
- Supports optional recipient email notification (`notify = true`) and custom message delivery, and fails validation if `message` is configured with `notify = false` or if `permissions` does not include `Read`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_NotificationAndValidation`
