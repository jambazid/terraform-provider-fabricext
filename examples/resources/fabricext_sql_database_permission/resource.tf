# Grant direct item-level sharing permissions on a Microsoft Fabric SQL Database
resource "fabricext_sql_database_permission" "orders_analysts" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_name = "operational_orders_db"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "Group"
  permissions       = ["Read", "ReadData"]
}

# Grant direct permissions by SQL Database UUID (recommended for rename immunity)
resource "fabricext_sql_database_permission" "app_managed_identity" {
  workspace_id    = "00000000-0000-0000-0000-000000000001"
  sql_database_id = "33333333-3333-3333-3333-333333333333"
  principal_id    = "44444444-4444-4444-4444-444444444444"
  principal_type  = "ManagedIdentity"
  permissions     = ["Read", "ReadData", "Execute", "Reshare"]
}

