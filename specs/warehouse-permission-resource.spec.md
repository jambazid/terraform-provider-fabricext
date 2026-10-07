---
name: Warehouse Permission Resource (fabricext_warehouse_permission)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric Warehouse item-level permissions
targets:
  - ../internal/provider/warehouse_permission_resource.go
  - ../internal/provider/warehouse_permission_resource_test.go
---

# Warehouse Permission Resource (`fabricext_warehouse_permission`)

## HCL Contract

```hcl
resource "fabricext_warehouse_permission" "example" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group" # Optional: User, Group, ServicePrincipal, ServicePrincipalProfile (default: Group)
  role_type      = "read"  # Required: read, write, reshare
}
```

`[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDDowngradeAndImport`

## Schema, Plan Modifiers & Validators

- `workspace_id`, `warehouse_name`, `warehouse_id`, `principal_id`, and `principal_type` carry `RequiresReplace()` plan modifiers; `id` carries `UseStateForUnknown()`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDDowngradeAndImport`
- `warehouse_name` and `warehouse_id` are optional attributes where at least one must be specified; if `warehouse_id` is supplied, `warehouse_name` is resolved automatically via `GetItemByID`, and if `warehouse_name` is supplied, `warehouse_id` is resolved via `GetItemByName`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_DirectIDReference`
- `workspace_id`, `warehouse_id`, and `principal_id` validate UUID format at plan time; `role_type` validates `OneOf("read", "write", "reshare")`; `principal_type` defaults to `"Group"` and validates `OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile")`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_ValidationErrors`

## CRUD Lifecycle, Disappears & ImportState

- `Create` resolves `warehouse_name` to `warehouse_id` (or verifies `warehouse_id`), grants the mapped permissions (`"read"` $\rightarrow$ `["Read"]`, `"write"` $\rightarrow$ `["Read", "Write"]`, `"reshare"` $\rightarrow$ `["Read", "Reshare"]`), and sets composite `id = "{workspace_id}/{warehouse_id}/{principal_type}/{principal_id}"`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDDowngradeAndImport`
- `Update` supports both upgrades (`"read"` $\rightarrow$ `"write"`) and downgrades (`"write"` $\rightarrow$ `"read"`), granting target permissions before revoking removed permissions to prevent downtime
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDDowngradeAndImport`
- `Read` removes the resource from Terraform state (`resp.State.RemoveResource(ctx)`) if the Warehouse or principal permission has been deleted out-of-band
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_Disappears`
- `ImportState` parses `{workspace_id}/{warehouse_id}/{principal_type}/{principal_id}` and resolves `warehouse_name` via `GetItemByID`
  `[@test] ../internal/provider/warehouse_permission_resource_test.go::TestAccWarehousePermissionResource_CRUDDowngradeAndImport`
