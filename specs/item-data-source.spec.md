---
name: Fabric Item Data Source (fabricext_item)
description: Singular read-only data source resolving Microsoft Fabric item display names to canonical UUIDs
targets:
  - ../internal/provider/item_data_source.go
  - ../internal/provider/item_data_source_test.go
---

# Fabric Item Data Source (`fabricext_item`)

## HCL Contract

```hcl
data "fabricext_item" "example" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  display_name = "sales_analytics_wh"
  type         = "Warehouse"
}
```

`[@test] ../internal/provider/item_data_source_test.go::TestAccItemDataSource_LookupAndTypeIsolation`

## Lookup & Validation Behavior

- Resolves `(workspace_id, display_name, type)` to the matching Fabric item `id` without side effects
  `[@test] ../internal/provider/item_data_source_test.go::TestAccItemDataSource_LookupAndTypeIsolation`
- Validates `workspace_id` as a UUID and `type` as `OneOf("Warehouse", "Lakehouse", "SQLDatabase", "SQLEndpoint", "SemanticModel")`
  `[@test] ../internal/provider/item_data_source_test.go::TestAccItemDataSource_ValidationErrors`
- Returns a clear diagnostic error when zero items match `(workspace_id, display_name, type)`
  `[@test] ../internal/provider/item_data_source_test.go::TestAccItemDataSource_NotFoundError`
