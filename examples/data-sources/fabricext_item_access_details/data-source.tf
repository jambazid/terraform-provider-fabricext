data "fabricext_item_access_details" "sales_warehouse" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  item_id      = "00000000-0000-0000-0000-000000000002"
  type         = "Warehouse"
}

output "access_details" {
  description = "Raw principal access entries on the warehouse."
  value       = data.fabricext_item_access_details.sales_warehouse.access_details
}

output "principal_permissions" {
  description = "Lookup map from principal ID to granted permissions set."
  value       = data.fabricext_item_access_details.sales_warehouse.principal_permissions
}
