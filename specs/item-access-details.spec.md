---
name: Fabric Item Access Details Data Source (fabricext_item_access_details)
description: Read-only projection data source querying effective permissions on a workspace item via the Microsoft Fabric Admin API
targets:
  - ../internal/provider/item_access_details_data_source.go
  - ../internal/provider/item_access_details_data_source_test.go
---

# Fabric Item Access Details Data Source (`fabricext_item_access_details`)

## HCL Contract

```hcl
data "fabricext_item_access_details" "example" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  item_id      = "00000000-0000-0000-0000-000000000002"
  type         = "Warehouse"
}
```

## Schema & Validation Behavior

- Requires valid UUIDs for `workspace_id` and `item_id`, and validates optional `type`
  `[@test] ../internal/provider/item_access_details_data_source_test.go::TestAccItemAccessDetailsDataSource_ValidationErrors`
- Queries `GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users` and projects raw principal access entries into `access_details`
  `[@test] ../internal/provider/item_access_details_data_source_test.go::TestAccItemAccessDetailsDataSource_BasicAndProjection`
- Computes `principal_permissions` as a convenience map keyed by principal ID mapping to the union of item permissions and additional permissions
  `[@test] ../internal/provider/item_access_details_data_source_test.go::TestAccItemAccessDetailsDataSource_BasicAndProjection`
- Returns a structured diagnostic error when the caller lacks Fabric Administrator permissions (HTTP 403 Forbidden)
  `[@test] ../internal/provider/item_access_details_data_source_test.go::TestAccItemAccessDetailsDataSource_AdminForbiddenError`
- Returns a clear diagnostic error when the workspace or item is not found (HTTP 404)
  `[@test] ../internal/provider/item_access_details_data_source_test.go::TestAccItemAccessDetailsDataSource_NotFoundError`
