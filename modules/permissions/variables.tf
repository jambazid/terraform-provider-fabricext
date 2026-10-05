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
}
