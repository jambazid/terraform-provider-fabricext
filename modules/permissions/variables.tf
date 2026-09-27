# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

variable "fabric_permissions_matrix" {
  description = "Declarative permission matrix mapping Microsoft Fabric Warehouses, SQL Databases, and Lakehouse OneLake Data Access Roles to Microsoft Entra principals within a workspace."
  type = object({
    workspace_id = string
    warehouses = optional(map(map(list(object({
      id   = string
      type = optional(string, "Group")
    })))), {})
    sql_databases = optional(map(map(list(object({
      id   = string
      type = optional(string, "Group")
    })))), {})
    lakehouses = optional(map(map(object({
      paths          = set(string)
      actions        = optional(set(string), ["Read"])
      principal_ids  = set(string)
      principal_type = optional(string, "Group")
    }))), {})
  })

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", var.fabric_permissions_matrix.workspace_id))
    error_message = "workspace_id must be a valid 36-character UUID."
  }

  validation {
    condition = alltrue(flatten([
      for wh_name, roles in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        for role_type, _ in coalesce(roles, {}) : contains(["read", "write", "reshare"], role_type)
      ]
    ]))
    error_message = "Warehouse role_type keys must be one of: read, write, reshare."
  }

  validation {
    condition = alltrue(flatten([
      for db_name, roles in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        for role_type, _ in coalesce(roles, {}) : contains(["read", "read_data", "read_spark", "write", "reshare"], role_type)
      ]
    ]))
    error_message = "SQL Database role_type keys must be one of: read, read_data, read_spark, write, reshare."
  }

  validation {
    condition = alltrue(flatten([
      for lh_name, roles in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for role_name, spec in coalesce(roles, {}) : (
          spec != null &&
          can(regex("^[a-zA-Z][a-zA-Z0-9_]*$", role_name)) &&
          length(spec.paths) > 0 &&
          length(spec.principal_ids) > 0 &&
          contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(spec.principal_type, "Group")) &&
          alltrue([for a in coalesce(spec.actions, ["Read"]) : contains(["Read"], a)])
        )
      ]
    ]))
    error_message = "Lakehouse roles must have valid alphanumeric names (starting with a letter), non-empty paths and principal_ids, supported actions (Read), and valid principal_type (User, Group, ServicePrincipal, ManagedIdentity)."
  }
}

