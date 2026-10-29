# Grant direct item-level sharing permissions on a Microsoft Fabric Warehouse
resource "fabricext_warehouse_permission" "analytics_analysts" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  permissions    = ["Read", "ReadData"]
}

# Grant direct permissions by Warehouse UUID (recommended for rename immunity)
resource "fabricext_warehouse_permission" "etl_service_principal" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_id   = "22222222-2222-2222-2222-222222222222"
  principal_id   = "33333333-3333-3333-3333-333333333333"
  principal_type = "ServicePrincipal"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}

