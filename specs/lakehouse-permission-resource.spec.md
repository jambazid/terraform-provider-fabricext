---
name: Fabric Lakehouse Permission Resource (fabricext_lakehouse_permission)
description: Declarative management of direct item-level sharing permissions on Microsoft Fabric Lakehouses
targets:
  - ../internal/provider/lakehouse_permission_resource.go
  - ../internal/provider/lakehouse_permission_resource_test.go
---

# Fabric Lakehouse Permission (`fabricext_lakehouse_permission`)

## HCL Contract

```hcl
resource "fabricext_lakehouse_permission" "example" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_id   = "00000000-0000-0000-0000-000000000002"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}
```

## Lifecycle & Validation Behavior

- Creates direct item-level sharing permissions on Microsoft Fabric Lakehouses with display name or ID resolution and verifies composite ID import
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_CRUDAndImport`
- Detects item deletion in Fabric during Read and cleanly removes resource from state to plan re-creation without error diagnostics
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_DriftDetectionAndRecreation`
- Validates item type during Read and Create and reports actionable diagnostic error when target item type is not Lakehouse
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_TypeMismatchError`
