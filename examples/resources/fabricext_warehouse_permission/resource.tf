# Referencing Warehouse by display name
resource "fabricext_warehouse_permission" "analytics_readers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  role_type      = "read"
}

# Referencing Warehouse by direct UUID (ideal for upstream resource chaining)
resource "fabricext_warehouse_permission" "direct_id_writers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_id   = "22222222-2222-2222-2222-222222222222"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  role_type      = "write"
}
