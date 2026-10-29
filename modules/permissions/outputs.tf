# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

output "warehouse_permission_ids" {
  description = "Map of flattened Warehouse permission keys to composite resource IDs."
  value       = { for k, v in fabricext_warehouse_permission.this : k => v.id }
  depends_on = [
    fabricext_item_share_gate.warehouse,
  ]
}

output "sql_database_permission_ids" {
  description = "Map of flattened SQL Database permission keys to composite resource IDs."
  value       = { for k, v in fabricext_sql_database_permission.this : k => v.id }
  depends_on = [
    fabricext_item_share_gate.sql_database,
  ]
}

output "lakehouse_permission_ids" {
  description = "Map of flattened Lakehouse item-level permission keys to composite resource IDs."
  value       = { for k, v in fabricext_lakehouse_permission.this : k => v.id }
  depends_on = [
    fabricext_item_share_gate.lakehouse,
  ]
}

output "lakehouse_data_access_role_ids" {
  description = "Map of flattened Lakehouse Data Access Role keys to composite resource IDs."
  value       = { for k, v in fabricext_lakehouse_data_access_role.this : k => v.id }
}

output "lakehouse_onelake_security_ids" {
  description = "Map of unique Lakehouse keys to composite OneLake security resource IDs."
  value       = { for k, v in fabricext_lakehouse_onelake_security.this : k => v.id }
}

output "item_share_gate_ids" {
  description = "Map of chained item share verification gate keys to composite resource IDs."
  value = merge(
    { for k, v in fabricext_item_share_gate.warehouse : "warehouse/${k}" => v.id },
    { for k, v in fabricext_item_share_gate.sql_database : "sql_database/${k}" => v.id },
    { for k, v in fabricext_item_share_gate.lakehouse : "lakehouse/${k}" => v.id },
  )
}

output "gate_ids" {
  description = "Alias for item_share_gate_ids providing a concise map of chained share verification gate keys to resource IDs."
  value = merge(
    { for k, v in fabricext_item_share_gate.warehouse : "warehouse/${k}" => v.id },
    { for k, v in fabricext_item_share_gate.sql_database : "sql_database/${k}" => v.id },
    { for k, v in fabricext_item_share_gate.lakehouse : "lakehouse/${k}" => v.id },
  )
}
