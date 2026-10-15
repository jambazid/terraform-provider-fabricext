---
name: Lakehouse Data Access Role Resource (fabricext_lakehouse_data_access_role)
description: Declarative Terraform Plugin Framework resource for Microsoft Fabric Lakehouse OneLake Data Access Roles
targets:
  - ../internal/provider/lakehouse_data_access_role_resource.go
  - ../internal/provider/lakehouse_data_access_role_resource_test.go
---

# Lakehouse Data Access Role

## HCL Contract

### Simple Flat Mode

```hcl
resource "fabricext_lakehouse_data_access_role" "simple" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"
}
```

### Advanced Structured Mode

```hcl
resource "fabricext_lakehouse_data_access_role" "advanced" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_id   = "22222222-2222-2222-2222-222222222222"
  role_name      = "EmeaSalesAnalysts"
  kind           = "Policy"

  decision_rule {
    paths   = ["/Tables/sales"]
    actions = ["Read"]

    row_constraint {
      table_path = "/Tables/sales"
      predicate  = "Region = 'EMEA'"
    }

    column_constraint {
      table_path = "/Tables/sales"
      columns    = ["customer_id", "sale_amount"]
    }
  }

  entra_member {
    object_id   = "33333333-3333-3333-3333-333333333333"
    object_type = "Group"
  }

  fabric_item_member {
    source_path = "00000000-0000-0000-0000-000000000001/44444444-4444-4444-4444-444444444444"
    item_access = ["ReadAll"]
  }
}
```

`[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_CRUDAndImport`

## Schema, Plan Modifiers & Validators

- `workspace_id`, `lakehouse_name`, `lakehouse_id`, and `role_name` carry `RequiresReplace()` plan modifiers; `id`, `lakehouse_name`, and `lakehouse_id` carry `UseStateForUnknown()`
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_CRUDAndImport`
- `lakehouse_name` and `lakehouse_id` are optional attributes where at least one must be specified; if `lakehouse_id` is supplied, `lakehouse_name` is resolved automatically via `GetItemByID`, and if `lakehouse_name` is supplied, `lakehouse_id` is resolved via `GetItemByName`; if both are configured, `Create` validates that `lakehouse_name` matches the fetched display name; on replacement, `ModifyPlan` clears the unconfigured counterpart so stale state values from `UseStateForUnknown` do not cause replacement failures
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_MismatchedIdentifiers`
- `fabricext_lakehouse_data_access_role` provides a dual-mode schema: simple flat mode (`paths`, `actions`, `principal_ids`, `principal_type`) for uniform path access, and advanced structured mode (`decision_rule` with optional `row_constraint` and `column_constraint`, `entra_member` set, and `fabric_item_member` set) for fine-grained OneLake Data Access Security; `role_name` enforces length $\le 128$ characters; `actions` validates `OneOf("Read", "Write", "ReadWrite")` and emits a value-level deprecation warning for `Write` in favor of `ReadWrite`
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_SimpleAndAdvancedParity`
- Heterogeneous Entra ID principals (`User`, `Group`, `ServicePrincipal`, `ManagedIdentity`) with optional `tenant_id` and dynamic item membership (`fabric_item_member` with `{workspace_id}/{item_id}` UUID pair `source_path`) can be declared within the same OneLake Data Access Role; state refresh correlates prior member types to preserve configured object types when Fabric API omits `objectType`
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_MixedMembersAndShortcuts`
- Validation enforces that either simple mode attributes or advanced mode blocks are provided, rejecting configurations declaring both simple mode attributes (`paths`, `actions`, `principal_ids`, `principal_type`) and advanced mode blocks, missing both, declaring empty rules, combining `ReadWrite` action with row or column constraints, or configuring a `role_name` exceeding 128 characters
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_ValidationErrors`

## Concurrency-Safe Lifecycle

- Multiple `fabricext_lakehouse_data_access_role` resources targeting the same Lakehouse via `for_each` execute safely in parallel without clobbering sibling roles or `DefaultReader`
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_ParallelForEachAndETagConflict`
- `Read` removes the resource from Terraform state (`resp.State.RemoveResource(ctx)`) if the Lakehouse or target `role_name` is deleted out-of-band
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_Disappears`
- `ImportState` parses `{workspace_id}/{lakehouse_id}/{role_name}` and populates `lakehouse_name`, `paths`, `actions`, `principal_ids`, and structured rules
  `[@test] ../internal/provider/lakehouse_data_access_role_resource_test.go::TestAccLakehouseDataAccessRoleResource_CRUDAndImport`
