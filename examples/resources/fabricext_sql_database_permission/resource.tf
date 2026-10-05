resource "fabricext_sql_database_permission" "orders_readers" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_name = "operational_orders_db"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "Group"
  role_type         = "read_data"
}
