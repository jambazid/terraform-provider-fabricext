# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

output "warehouse_permission_ids" {
  description = "Map of flattened Warehouse permission keys to composite resource IDs."
  value       = { for k, v in fabricext_warehouse_permission.this : k => v.id }
}

output "sql_database_permission_ids" {
  description = "Map of flattened SQL Database permission keys to composite resource IDs."
  value       = { for k, v in fabricext_sql_database_permission.this : k => v.id }
}

output "lakehouse_permission_ids" {
  description = "Map of flattened Lakehouse Data Access Role keys to composite resource IDs."
  value       = { for k, v in fabricext_lakehouse_permission.this : k => v.id }
}

output "lakehouse_onelake_security_ids" {
  description = "Map of unique Lakehouse keys to composite OneLake security resource IDs."
  value       = { for k, v in fabricext_lakehouse_onelake_security.this : k => v.id }
}
