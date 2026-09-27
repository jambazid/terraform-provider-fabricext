# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

locals {
  warehouse_permissions = {
    for entry in flatten([
      for wh_name, roles in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        for role_type, principals in coalesce(roles, {}) : [
          for p in coalesce(principals, []) : {
            key            = "${wh_name}/${coalesce(p.type, "Group")}/${p.id}"
            warehouse_name = wh_name
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
      for db_name, roles in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        for role_type, principals in coalesce(roles, {}) : [
          for p in coalesce(principals, []) : {
            key               = "${db_name}/${coalesce(p.type, "Group")}/${p.id}"
            sql_database_name = db_name
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
      for lh_name, roles in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for role_name, spec in coalesce(roles, {}) : {
          key            = "${lh_name}/${role_name}"
          lakehouse_name = lh_name
          role_name      = role_name
          paths          = spec.paths
          actions        = coalesce(spec.actions, toset(["Read"]))
          principal_ids  = spec.principal_ids
          principal_type = coalesce(spec.principal_type, "Group")
        }
      ]
    ]) : entry.key => entry
  }
}

resource "fabricext_warehouse_permission" "this" {
  for_each = local.warehouse_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  warehouse_name = each.value.warehouse_name
  principal_id   = each.value.principal_id
  principal_type = each.value.principal_type
  role_type      = each.value.role_type
}

resource "fabricext_sql_database_permission" "this" {
  for_each = local.sql_database_permissions

  workspace_id      = var.fabric_permissions_matrix.workspace_id
  sql_database_name = each.value.sql_database_name
  principal_id      = each.value.principal_id
  principal_type    = each.value.principal_type
  role_type         = each.value.role_type
}

resource "fabricext_lakehouse_permission" "this" {
  for_each = local.lakehouse_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  lakehouse_name = each.value.lakehouse_name
  role_name      = each.value.role_name
  paths          = each.value.paths
  actions        = each.value.actions
  principal_ids  = each.value.principal_ids
  principal_type = each.value.principal_type
}
