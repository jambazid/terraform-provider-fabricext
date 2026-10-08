# Referencing SQL Database by display name
resource "fabricext_sql_database_permission" "orders_readers" {
  workspace_id      = "00000000-0000-0000-0000-000000000001"
  sql_database_name = "operational_orders_db"
  principal_id      = "11111111-1111-1111-1111-111111111111"
  principal_type    = "Group"
  role_type         = "read_data"
}

# Referencing SQL Database by direct UUID (ideal for upstream resource chaining)
resource "fabricext_sql_database_permission" "direct_id_readers" {
  workspace_id    = "00000000-0000-0000-0000-000000000001"
  sql_database_id = "33333333-3333-3333-3333-333333333333"
  principal_id    = "11111111-1111-1111-1111-111111111111"
  principal_type  = "Group"
  role_type       = "read_spark"
}
