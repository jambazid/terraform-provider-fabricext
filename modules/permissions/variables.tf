# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

variable "notify" {
  description = "Optional boolean flag to send email notifications for item share permissions across all workloads in the matrix. When true, permissions are granted via /metadata/access/notify. When false, permissions are granted silently via /metadata/access. Default: false."
  type        = bool
  default     = false
}

variable "message" {
  description = "Optional custom email notification message sent when notify is true. If notify is true and message is null or empty, the Power BI service default notification message is used. Setting message when notify is false is disallowed."
  type        = string
  default     = null

  validation {
    condition     = var.message == null || var.notify == true
    error_message = "The 'message' variable can only be set when 'notify' is true."
  }
}

variable "fabric_permissions_matrix" {
  description = <<-EOT
Declarative permission matrix mapping Microsoft Fabric Warehouses, SQL Databases, and Lakehouses (item-level sharing and OneLake Data Access Roles) to Microsoft Entra principals within a workspace.

Object Attributes:
- `workspace_id` - (Required, string) The 36-character UUID of the target Microsoft Fabric workspace.
- `verify_item_shares` - (Optional, bool) Cross-plane Admin API verification sanity lever. When true, chains `fabricext_item_share_gate` after item permissions to verify effective ACLs via Fabric Admin REST API. Default: `true`.
- `share_gate_match_type` - (Optional, string) Verification match type for `fabricext_item_share_gate`: `subset` or `exact`. Default: `subset`.
- `share_gate_timeout` - (Optional, string) Polling timeout bound for `fabricext_item_share_gate`. Default: `5m`.
- `warehouses` - (Optional, map of objects) Map of Warehouse identifier (display name or UUID) to warehouse configuration:
  - `verify_item_shares` - (Optional, bool) Per-item override for Admin API share verification. Default: inherits matrix-level setting.
  - `permissions` - (Optional, map of objects) Map of Microsoft Entra Object ID (UUID) to permission specification:
    - `type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
    - `permissions` - (Optional, set of string) Exact item permissions: must include `"Read"`; allowed: `"Read"`, `"ReadData"`, `"ReadAll"`, `"Reshare"`. Default: `["Read"]`.
- `sql_databases` - (Optional, map of objects) Map of SQL Database identifier (display name or UUID) to SQL database configuration:
  - `verify_item_shares` - (Optional, bool) Per-item override for Admin API share verification. Default: inherits matrix-level setting.
  - `permissions` - (Optional, map of objects) Map of Microsoft Entra Object ID (UUID) to permission specification:
    - `type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
    - `permissions` - (Optional, set of string) Exact item permissions: must include `"Read"`; allowed: `"Read"`, `"ReadData"`, `"Execute"`, `"Reshare"`. Default: `["Read"]`.
- `manage_lakehouse_security` - (Optional, bool) Default OneLake Data Access Security enablement verification for Lakehouses in the matrix. Default: `true`.
- `remove_lakehouse_default_reader_role` - (Optional, bool) Default setting for purging `DefaultReader` roles created upon enabling OneLake security. Default: `true`.
- `lakehouses` - (Optional, map of objects) Map of Lakehouse identifier (display name or UUID) to unified Lakehouse configuration:
  - `verify_item_shares` - (Optional, bool) Per-item override for Admin API share verification. Default: inherits matrix-level setting.
  - `manage_onelake_security` - (Optional, bool) Per-Lakehouse override for OneLake security management. Default: `null` (smart-gated based on matrix default and role presence).
  - `remove_default_reader_role` - (Optional, bool) Per-Lakehouse override for default reader purging. Default: `null` (inherits matrix default).
  - `permissions` - (Optional, map of objects) Map of Microsoft Entra Object ID (UUID) to direct item-level share permissions:
    - `type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
    - `permissions` - (Optional, set of string) Exact item permissions: must include `"Read"`; allowed: `"Read"`, `"ReadData"`, `"ReadAll"`, `"Reshare"`. Default: `["Read"]`.
  - `data_access_roles` - (Optional, map of objects) Map of OneLake Data Access Role names to role definitions:
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
        - `row_constraints` - (Optional, list of objects) Row-Level Security (RLS) SQL predicate filters.
        - `column_constraints` - (Optional, list of objects) Column-Level Security (CLS) column allowlists.
      - `entra_members` - (Optional, list of objects) Heterogeneous Microsoft Entra ID members:
        - `object_id` - (Required, string) Microsoft Entra ID Object ID.
        - `object_type` - (Optional, string) Principal type: `User`, `Group`, `ServicePrincipal`, or `ManagedIdentity`. Default: `Group`.
        - `tenant_id` - (Optional, string) Entra ID Tenant ID for cross-tenant principals.
      - `fabric_item_members` - (Optional, list of objects) Dynamic item-permission members.
EOT
  type = object({
    workspace_id          = string
    verify_item_shares    = optional(bool, true)
    share_gate_match_type = optional(string, "subset")
    share_gate_timeout    = optional(string, "5m")

    # Warehouses
    warehouses = optional(map(object({
      verify_item_shares = optional(bool)
      permissions = optional(map(object({
        type        = optional(string, "Group")
        permissions = optional(set(string), ["Read"])
        notify      = optional(bool)
        message     = optional(string)
      })), {})
    })), {})

    # SQL Databases
    sql_databases = optional(map(object({
      verify_item_shares = optional(bool)
      permissions = optional(map(object({
        type        = optional(string, "Group")
        permissions = optional(set(string), ["Read"])
        notify      = optional(bool)
        message     = optional(string)
      })), {})
    })), {})

    # Lakehouse OneLake Security matrix-level defaults
    manage_lakehouse_security            = optional(bool, true)
    remove_lakehouse_default_reader_role = optional(bool, true)

    # Lakehouses (unified: item permissions + OneLake data access roles)
    lakehouses = optional(map(object({
      verify_item_shares         = optional(bool)
      manage_onelake_security    = optional(bool)
      remove_default_reader_role = optional(bool)

      permissions = optional(map(object({
        type        = optional(string, "Group")
        permissions = optional(set(string), ["Read"])
        notify      = optional(bool)
        message     = optional(string)
      })), {})

      data_access_roles = optional(map(object({
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
      })), {})
    })), {})
  })

  validation {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", var.fabric_permissions_matrix.workspace_id))
    error_message = "workspace_id must be a valid 36-character UUID."
  }

  validation {
    condition     = contains(["subset", "exact"], coalesce(var.fabric_permissions_matrix.share_gate_match_type, "subset"))
    error_message = "share_gate_match_type must be either 'subset' or 'exact'."
  }

  validation {
    condition = alltrue(flatten([
      for wh_key, wh in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        length(distinct([for k in keys(coalesce(wh.permissions, {})) : lower(k)])) == length(keys(coalesce(wh.permissions, {}))),
        alltrue([
          for p_id, p in coalesce(wh.permissions, {}) : (
            can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", p_id)) &&
            contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(p.type, "Group")) &&
            contains(coalesce(p.permissions, ["Read"]), "Read") &&
            alltrue([for perm in coalesce(p.permissions, ["Read"]) : contains(["Read", "ReadData", "ReadAll", "Reshare"], perm)]) &&
            (p.message == null || coalesce(p.notify, var.notify) == true)
          )
        ])
      ]
    ]))
    error_message = "Warehouse permissions must be keyed by unique Microsoft Entra UUIDs, use a valid principal type, include 'Read', contain only allowed permissions (Read, ReadData, ReadAll, Reshare), and require notify=true when message is set."
  }

  validation {
    condition = alltrue(flatten([
      for db_key, db in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        length(distinct([for k in keys(coalesce(db.permissions, {})) : lower(k)])) == length(keys(coalesce(db.permissions, {}))),
        alltrue([
          for p_id, p in coalesce(db.permissions, {}) : (
            can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", p_id)) &&
            contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(p.type, "Group")) &&
            contains(coalesce(p.permissions, ["Read"]), "Read") &&
            alltrue([for perm in coalesce(p.permissions, ["Read"]) : contains(["Read", "ReadData", "Execute", "Reshare"], perm)]) &&
            (p.message == null || coalesce(p.notify, var.notify) == true)
          )
        ])
      ]
    ]))
    error_message = "SQL Database permissions must be keyed by unique Microsoft Entra UUIDs, use a valid principal type, include 'Read', contain only allowed permissions (Read, ReadData, Execute, Reshare), and require notify=true when message is set."
  }

  validation {
    condition = alltrue(flatten([
      for lh_key, lh in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        length(distinct([for k in keys(coalesce(lh.permissions, {})) : lower(k)])) == length(keys(coalesce(lh.permissions, {}))),
        alltrue([
          for p_id, p in coalesce(lh.permissions, {}) : (
            can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", p_id)) &&
            contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(p.type, "Group")) &&
            contains(coalesce(p.permissions, ["Read"]), "Read") &&
            alltrue([for perm in coalesce(p.permissions, ["Read"]) : contains(["Read", "ReadData", "ReadAll", "Reshare"], perm)]) &&
            (p.message == null || coalesce(p.notify, var.notify) == true)
          )
        ]),
        alltrue([
          for role_name, spec in coalesce(lh.data_access_roles, {}) : (
            spec != null &&
            can(regex("^[a-zA-Z][a-zA-Z0-9_]*$", role_name)) &&
            (
              # Simple mode
              (
                spec.paths != null && length(spec.paths) > 0 &&
                spec.principal_ids != null && length(spec.principal_ids) > 0 &&
                alltrue([for pid in coalesce(spec.principal_ids, []) : can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", pid))]) &&
                length(coalesce(spec.decision_rules, [])) == 0 &&
                length(coalesce(spec.entra_members, [])) == 0 &&
                length(coalesce(spec.fabric_item_members, [])) == 0 &&
                contains(["User", "Group", "ServicePrincipal", "ManagedIdentity"], coalesce(spec.principal_type, "Group")) &&
                alltrue([for a in coalesce(spec.actions, ["Read"]) : contains(["Read", "Write", "ReadWrite"], a)])
              ) ||
              # Advanced mode
              (
                (spec.paths == null || length(spec.paths) == 0) &&
                (spec.principal_ids == null || length(spec.principal_ids) == 0) &&
                length(coalesce(spec.decision_rules, [])) > 0 &&
                (length(coalesce(spec.entra_members, [])) > 0 || length(coalesce(spec.fabric_item_members, [])) > 0) &&
                alltrue([for m in coalesce(spec.entra_members, []) : can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", m.object_id))]) &&
                alltrue(flatten([
                  for r in coalesce(spec.decision_rules, []) : [
                    for a in coalesce(r.actions, ["Read"]) : contains(["Read", "Write", "ReadWrite"], a)
                  ]
                ]))
              )
            )
          )
        ])
      ]
    ]))
    error_message = "Lakehouse configuration must have unique permission principal UUIDs with valid permissions including 'Read', and data_access_roles must declare valid simple or advanced mode definitions."
  }
}
