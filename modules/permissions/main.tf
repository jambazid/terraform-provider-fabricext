# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

locals {
  uuid_regex = "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"

  warehouse_permissions = {
    for entry in flatten([
      for wh_key, roles in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        for role_type, principals in coalesce(roles, {}) : [
          for p in coalesce(principals, []) : {
            key            = "${wh_key}/${coalesce(p.type, "Group")}/${p.id}"
            warehouse_name = can(regex(local.uuid_regex, wh_key)) ? null : wh_key
            warehouse_id   = can(regex(local.uuid_regex, wh_key)) ? wh_key : null
            role_type      = role_type
            principal_id   = p.id
            principal_type = coalesce(p.type, "Group")
          }
        ]
      ]
    ]) : entry.key => entry
  }

  sql_database_permissions = {
    for entry in flatten([
      for db_key, roles in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        for role_type, principals in coalesce(roles, {}) : [
          for p in coalesce(principals, []) : {
            key               = "${db_key}/${coalesce(p.type, "Group")}/${p.id}"
            sql_database_name = can(regex(local.uuid_regex, db_key)) ? null : db_key
            sql_database_id   = can(regex(local.uuid_regex, db_key)) ? db_key : null
            role_type         = role_type
            principal_id      = p.id
            principal_type    = coalesce(p.type, "Group")
          }
        ]
      ]
    ]) : entry.key => entry
  }

  lakehouse_permissions = {
    for entry in flatten([
      for lh_key, roles in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for role_name, spec in coalesce(roles, {}) : {
          key                 = "${lh_key}/${role_name}"
          lakehouse_name      = can(regex(local.uuid_regex, lh_key)) ? null : lh_key
          lakehouse_id        = can(regex(local.uuid_regex, lh_key)) ? lh_key : null
          role_name           = role_name
          kind                = coalesce(spec.kind, "Policy")
          paths               = spec.paths
          actions             = spec.paths != null ? coalesce(spec.actions, toset(["Read"])) : null
          principal_ids       = spec.principal_ids
          principal_type      = spec.principal_ids != null ? coalesce(spec.principal_type, "Group") : null
          decision_rules      = coalesce(spec.decision_rules, [])
          entra_members       = coalesce(spec.entra_members, [])
          fabric_item_members = coalesce(spec.fabric_item_members, [])
        }
      ]
    ]) : entry.key => entry
  }
}

resource "fabricext_warehouse_permission" "this" {
  for_each = local.warehouse_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  warehouse_name = each.value.warehouse_name
  warehouse_id   = each.value.warehouse_id
  principal_id   = each.value.principal_id
  principal_type = each.value.principal_type
  role_type      = each.value.role_type
}

resource "fabricext_sql_database_permission" "this" {
  for_each = local.sql_database_permissions

  workspace_id      = var.fabric_permissions_matrix.workspace_id
  sql_database_name = each.value.sql_database_name
  sql_database_id   = each.value.sql_database_id
  principal_id      = each.value.principal_id
  principal_type    = each.value.principal_type
  role_type         = each.value.role_type
}

resource "fabricext_lakehouse_permission" "this" {
  for_each = local.lakehouse_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  lakehouse_name = each.value.lakehouse_name
  lakehouse_id   = each.value.lakehouse_id
  role_name      = each.value.role_name
  kind           = each.value.kind

  # Simple mode attributes
  paths          = each.value.paths
  actions        = each.value.actions
  principal_ids  = each.value.principal_ids
  principal_type = each.value.principal_type

  # Advanced mode blocks
  dynamic "decision_rule" {
    for_each = each.value.decision_rules
    content {
      paths   = decision_rule.value.paths
      actions = coalesce(decision_rule.value.actions, toset(["Read"]))
      effect  = coalesce(decision_rule.value.effect, "Permit")

      dynamic "row_constraint" {
        for_each = coalesce(decision_rule.value.row_constraints, [])
        content {
          table_path = row_constraint.value.table_path
          predicate  = row_constraint.value.predicate
        }
      }

      dynamic "column_constraint" {
        for_each = coalesce(decision_rule.value.column_constraints, [])
        content {
          table_path = column_constraint.value.table_path
          columns    = column_constraint.value.columns
          action     = coalesce(column_constraint.value.action, "Read")
          effect     = coalesce(column_constraint.value.effect, "Permit")
        }
      }
    }
  }

  dynamic "entra_member" {
    for_each = each.value.entra_members
    content {
      object_id   = entra_member.value.object_id
      object_type = coalesce(entra_member.value.object_type, "Group")
      tenant_id   = entra_member.value.tenant_id
    }
  }

  dynamic "fabric_item_member" {
    for_each = each.value.fabric_item_members
    content {
      source_path = fabric_item_member.value.source_path
      item_access = coalesce(fabric_item_member.value.item_access, toset(["ReadAll"]))
    }
  }
}
