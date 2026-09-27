---
name: Lakehouse Permission Resource (fabricext_lakehouse_permission)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric Lakehouse OneLake Data Access Roles
targets:
  - ../internal/provider/lakehouse_permission_resource.go
  - ../internal/provider/lakehouse_permission_resource_test.go
---

# Lakehouse Permission Resource (`fabricext_lakehouse_permission`)

## HCL Contract

```hcl
resource "fabricext_lakehouse_permission" "example" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"
}
```

`[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_CRUDAndImport`

## Schema, Plan Modifiers & Validators

- `workspace_id`, `lakehouse_name`, and `role_name` carry `RequiresReplace()` plan modifiers; `id` and `lakehouse_id` carry `UseStateForUnknown()`
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_CRUDAndImport`
- `paths` and `principal_ids` use `SetAttribute` (`ElementType: types.StringType`) with `SizeAtLeast(1)`; `actions` defaults to `["Read"]`
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_ValidationErrors`

## Concurrency-Safe CRUD Lifecycle, Disappears & ImportState

- Multiple `fabricext_lakehouse_permission` resources targeting the same Lakehouse via `for_each` execute safely in parallel without clobbering sibling roles or `DefaultReader`
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_ParallelForEachAndETagConflict`
- `Read` removes the resource from Terraform state (`resp.State.RemoveResource(ctx)`) if the Lakehouse or target `role_name` is deleted out-of-band
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_Disappears`
- `ImportState` parses `{workspace_id}/{lakehouse_id}/{role_name}` and populates `lakehouse_name`, `paths`, `actions`, and `principal_ids`
  `[@test] ../internal/provider/lakehouse_permission_resource_test.go::TestAccLakehousePermissionResource_CRUDAndImport`
