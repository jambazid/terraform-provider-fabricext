# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

variable "fabric_permissions_matrix" {
  description = "Declarative permission matrix mapping Microsoft Fabric Warehouses, SQL Databases, and Lakehouse OneLake Data Access Roles to Microsoft Entra principals within a workspace."
  type = object({
    workspace_id = string

    # Warehouses: supports either display name OR UUID as map keys!
    warehouses = optional(map(map(list(object({
      id   = string
      type = optional(string, "Group")
    })))), {})

    # SQL Databases: supports either display name OR UUID as map keys!
    sql_databases = optional(map(map(list(object({
      id   = string
      type = optional(string, "Group")
    })))), {})

    # Lakehouses: supports either display name OR UUID as map keys, and both simple and advanced RLS/CLS rules!
    lakehouses = optional(map(map(object({
      # Simple mode attributes
      paths          = optional(set(string))
      actions        = optional(set(string), ["Read"])
      principal_ids  = optional(set(string))
      principal_type = optional(string, "Group")

      # Role metadata
      kind = optional(string, "Policy")

      # Advanced mode blocks
      decision_rules = optional(list(object({
        paths   = set(string)
        actions = optional(set(string), ["Read"])
        effect  = optional(string, "Permit")
        row_constraints = optional(list(object({
          table_path = string
          predicate  = string
        })), [])
        column_constraints = optional(list(object({
          table_path = string
          columns    = set(string)
          action     = optional(string, "Read")
          effect     = optional(string, "Permit")
        })), [])
      })), [])

      entra_members = optional(list(object({
        object_id   = string
        object_type = optional(string, "Group")
        tenant_id   = optional(string)
      })), [])

      fabric_item_members = optional(list(object({
        source_path = string
        item_access = optional(set(string), ["ReadAll"])
      })), [])
    }))), {})
  })

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", var.fabric_permissions_matrix.workspace_id))
    error_message = "workspace_id must be a valid 36-character UUID."
  }

  validation {
    condition = alltrue(flatten([
      for wh_key, roles in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        for role_type, _ in coalesce(roles, {}) : contains(["read", "write", "reshare"], role_type)
      ]
    ]))
    error_message = "Warehouse role_type keys must be one of: read, write, reshare."
  }

  validation {
    condition = alltrue(flatten([
      for db_key, roles in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        for role_type, _ in coalesce(roles, {}) : contains(["read", "read_data", "read_spark", "write", "reshare"], role_type)
      ]
    ]))
    error_message = "SQL Database role_type keys must be one of: read, read_data, read_spark, write, reshare."
  }

  validation {
    condition = alltrue(flatten([
      for lh_key, roles in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for role_name, spec in coalesce(roles, {}) : (
          spec != null &&
          can(regex("^[a-zA-Z][a-zA-Z0-9_]*$", role_name)) &&
          (
            # Simple mode: paths and principal_ids must be non-empty
            (
              spec.paths != null && length(spec.paths) > 0 &&
              spec.principal_ids != null && length(spec.principal_ids) > 0 &&
              contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(spec.principal_type, "Group")) &&
              alltrue([for a in coalesce(spec.actions, ["Read"]) : contains(["Read"], a)])
            ) ||
            # Advanced mode: at least one decision_rule and at least one entra_member or fabric_item_member
            (
              length(coalesce(spec.decision_rules, [])) > 0 &&
              (length(coalesce(spec.entra_members, [])) > 0 || length(coalesce(spec.fabric_item_members, [])) > 0)
            )
          )
        )
      ]
    ]))
    error_message = "Lakehouse roles must have valid alphanumeric names (starting with a letter) and declare either valid simple mode attributes (paths, principal_ids) or advanced mode blocks (decision_rules, entra_members, fabric_item_members)."
  }
}
