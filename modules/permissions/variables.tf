# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

variable "fabric_permissions_matrix" {
  description = <<-EOT
Declarative permission matrix mapping Microsoft Fabric Warehouses, SQL Databases, and Lakehouse OneLake Data Access Roles to Microsoft Entra principals within a workspace.

Object Attributes:
- `workspace_id` - (Required, string) The 36-character UUID of the target Microsoft Fabric workspace.
- `warehouses` - (Optional, map of maps) Map of Warehouse identifier (display name or UUID) to role maps (`read`, `write`, `reshare`), where each role maps to a list of principal objects:
  - `id` - (Required, string) Microsoft Entra ID Object ID of the principal.
  - `type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ServicePrincipalProfile`. Default: `Group`.
- `sql_databases` - (Optional, map of maps) Map of SQL Database identifier (display name or UUID) to role maps (`read`, `read_data`, `read_spark`, `write`, `reshare`), where each role maps to a list of principal objects:
  - `id` - (Required, string) Microsoft Entra ID Object ID of the principal.
  - `type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ServicePrincipalProfile`. Default: `Group`.
- `manage_lakehouse_security` - (Optional, bool) Whether to manage OneLake Data Access Security enablement verification for Lakehouses in the matrix. Default: `true`.
- `remove_lakehouse_default_reader_role` - (Optional, bool) Whether to purge the default `DefaultReader` role created by Fabric upon enabling OneLake security. Default: `true`.
- `lakehouses` - (Optional, map of maps) Map of Lakehouse identifier (display name or UUID) to role definitions. Each role supports either Simple Mode or Advanced Mode:
  - Simple Mode Attributes (mutually exclusive with Advanced Mode):
    - `paths` - (Optional, set of string) Storage paths within the Lakehouse (e.g. `["/Tables/customers"]`).
    - `actions` - (Optional, set of string) Storage action permissions: `Read`, `Write`, or `ReadWrite`. Default: `["Read"]`.
    - `principal_ids` - (Optional, set of string) Set of Microsoft Entra ID Object IDs.
    - `principal_type` - (Optional, string) Principal type for all principals in `principal_ids`: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
  - Advanced Mode Attributes (mutually exclusive with Simple Mode):
    - `kind` - (Optional, string) Role kind: `Policy`. Default: `Policy`.
    - `decision_rules` - (Optional, list of objects) Granular path rules with optional RLS and CLS:
      - `paths` - (Required, set of string) Target storage paths within the Lakehouse.
      - `actions` - (Optional, set of string) Storage action permissions (`Read`, `Write`, `ReadWrite`). Default: `["Read"]`.
      - `effect` - (Optional, string) Access effect: `Permit`. Default: `Permit`.
      - `row_constraints` - (Optional, list of objects) Row-Level Security (RLS) SQL predicate filters:
        - `table_path` - (Required, string) Table path within `/Tables/...`.
        - `predicate` - (Required, string) T-SQL Boolean expression filter (e.g. `Region = 'EMEA'`).
      - `column_constraints` - (Optional, list of objects) Column-Level Security (CLS) column allowlists:
        - `table_path` - (Required, string) Table path within `/Tables/...`.
        - `columns` - (Required, set of string) Allowed column names.
        - `action` - (Optional, string) Action: `Read`. Default: `Read`.
        - `effect` - (Optional, string) Effect: `Permit`. Default: `Permit`.
    - `entra_members` - (Optional, list of objects) Heterogeneous Microsoft Entra ID members:
      - `object_id` - (Required, string) Microsoft Entra ID Object ID.
      - `object_type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
      - `tenant_id` - (Optional, string) Entra ID Tenant ID for cross-tenant principals.
    - `fabric_item_members` - (Optional, list of objects) Dynamic item-permission members (member virtualization):
      - `source_path` - (Required, string) Source path of the Fabric item holding the permissions in format `{workspace_id}/{item_id}`.
      - `item_access` - (Optional, set of string) Required item access permissions (`ReadAll`). Default: `["ReadAll"]`.
EOT
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

    # Lakehouse OneLake Security enablement options
    manage_lakehouse_security            = optional(bool, true)
    remove_lakehouse_default_reader_role = optional(bool, true)

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
            # Simple mode: paths and principal_ids must be non-empty, and advanced blocks must NOT be declared
            (
              spec.paths != null && length(spec.paths) > 0 &&
              spec.principal_ids != null && length(spec.principal_ids) > 0 &&
              length(coalesce(spec.decision_rules, [])) == 0 &&
              length(coalesce(spec.entra_members, [])) == 0 &&
              length(coalesce(spec.fabric_item_members, [])) == 0 &&
              contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(spec.principal_type, "Group")) &&
              alltrue([for a in coalesce(spec.actions, ["Read"]) : contains(["Read", "Write", "ReadWrite"], a)])
            ) ||
            # Advanced mode: at least one decision_rule and at least one member, and simple attributes must NOT be declared
            (
              (spec.paths == null || length(spec.paths) == 0) &&
              (spec.principal_ids == null || length(spec.principal_ids) == 0) &&
              length(coalesce(spec.decision_rules, [])) > 0 &&
              (length(coalesce(spec.entra_members, [])) > 0 || length(coalesce(spec.fabric_item_members, [])) > 0) &&
              alltrue(flatten([
                for r in coalesce(spec.decision_rules, []) : [
                  for a in coalesce(r.actions, ["Read"]) : contains(["Read", "Write", "ReadWrite"], a)
                ]
              ]))
            )
          )
        )
      ]
    ]))
    error_message = "Lakehouse roles must have valid alphanumeric names (starting with a letter) and declare either valid simple mode attributes (paths, principal_ids) or advanced mode blocks (decision_rules, entra_members, fabric_item_members)."
  }
}
