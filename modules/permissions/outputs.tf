# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

output "warehouse_permission_ids" {
  description = "Empty map retained for interface compatibility; warehouse item sharing is deprecated."
  value       = {}
}

output "sql_database_permission_ids" {
  description = "Empty map retained for interface compatibility; SQL database item sharing is deprecated."
  value       = {}
}

output "lakehouse_data_access_role_ids" {
  description = "Map of flattened Lakehouse Data Access Role keys to composite resource IDs."
  value       = { for k, v in fabricext_lakehouse_data_access_role.this : k => v.id }
}

output "lakehouse_permission_ids" {
  description = "Deprecated alias for lakehouse_data_access_role_ids retained for backwards compatibility."
  value       = { for k, v in fabricext_lakehouse_data_access_role.this : k => v.id }
}

output "lakehouse_onelake_security_ids" {
  description = "Map of unique Lakehouse keys to composite OneLake security resource IDs."
  value       = { for k, v in fabricext_lakehouse_onelake_security.this : k => v.id }
}
