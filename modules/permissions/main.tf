# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

locals {
  uuid_regex = "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"

  warehouse_permissions = {
    for entry in flatten([
      for wh_key, wh in coalesce(var.fabric_permissions_matrix.warehouses, {}) : [
        for p_id, p in coalesce(wh.permissions, {}) : {
          key                = "${wh_key}/${lower(p_id)}"
          warehouse_name     = can(regex(local.uuid_regex, wh_key)) ? null : wh_key
          warehouse_id       = can(regex(local.uuid_regex, wh_key)) ? wh_key : null
          principal_id       = p_id
          principal_type     = coalesce(p.type, "Group")
          permissions        = coalesce(p.permissions, toset(["Read"]))
          verify_item_shares = coalesce(wh.verify_item_shares, var.fabric_permissions_matrix.verify_item_shares, true)
        }
      ]
    ]) : entry.key => entry
  }

  sql_database_permissions = {
    for entry in flatten([
      for db_key, db in coalesce(var.fabric_permissions_matrix.sql_databases, {}) : [
        for p_id, p in coalesce(db.permissions, {}) : {
          key                = "${db_key}/${lower(p_id)}"
          sql_database_name  = can(regex(local.uuid_regex, db_key)) ? null : db_key
          sql_database_id    = can(regex(local.uuid_regex, db_key)) ? db_key : null
          principal_id       = p_id
          principal_type     = coalesce(p.type, "Group")
          permissions        = coalesce(p.permissions, toset(["Read"]))
          verify_item_shares = coalesce(db.verify_item_shares, var.fabric_permissions_matrix.verify_item_shares, true)
        }
      ]
    ]) : entry.key => entry
  }

  lakehouse_item_permissions = {
    for entry in flatten([
      for lh_key, lh in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for p_id, p in coalesce(lh.permissions, {}) : {
          key                = "${lh_key}/${lower(p_id)}"
          lakehouse_name     = can(regex(local.uuid_regex, lh_key)) ? null : lh_key
          lakehouse_id       = can(regex(local.uuid_regex, lh_key)) ? lh_key : null
          principal_id       = p_id
          principal_type     = coalesce(p.type, "Group")
          permissions        = coalesce(p.permissions, toset(["Read"]))
          verify_item_shares = coalesce(lh.verify_item_shares, var.fabric_permissions_matrix.verify_item_shares, true)
        }
      ]
    ]) : entry.key => entry
  }

  unique_lakehouses = {
    for lh_key, lh in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : lh_key => {
      key                        = lh_key
      lakehouse_name             = can(regex(local.uuid_regex, lh_key)) ? null : lh_key
      lakehouse_id               = can(regex(local.uuid_regex, lh_key)) ? lh_key : null
      remove_default_reader_role = coalesce(lh.remove_default_reader_role, var.fabric_permissions_matrix.remove_lakehouse_default_reader_role, true)
    }
    if(coalesce(lh.manage_onelake_security, var.fabric_permissions_matrix.manage_lakehouse_security, true) && (length(coalesce(lh.data_access_roles, {})) > 0 || lh.manage_onelake_security == true))
  }

  lakehouse_data_access_roles = {
    for entry in flatten([
      for lh_key, lh in coalesce(var.fabric_permissions_matrix.lakehouses, {}) : [
        for role_name, spec in coalesce(lh.data_access_roles, {}) : {
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

# 1. Warehouses
resource "fabricext_warehouse_permission" "this" {
  for_each = local.warehouse_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  warehouse_name = each.value.warehouse_name
  warehouse_id   = each.value.warehouse_id
  principal_id   = each.value.principal_id
  principal_type = each.value.principal_type
  permissions    = each.value.permissions
}

resource "fabricext_item_share_gate" "warehouse" {
  for_each = { for k, v in local.warehouse_permissions : k => v if v.verify_item_shares }

  workspace_id         = var.fabric_permissions_matrix.workspace_id
  item_id              = fabricext_warehouse_permission.this[each.key].warehouse_id
  principal_id         = each.value.principal_id
  principal_type       = each.value.principal_type
  expected_permissions = fabricext_warehouse_permission.this[each.key].permissions
  match_type           = coalesce(var.fabric_permissions_matrix.share_gate_match_type, "subset")

  timeouts {
    create = coalesce(var.fabric_permissions_matrix.share_gate_timeout, "5m")
  }

  depends_on = [
    fabricext_warehouse_permission.this,
  ]
}

# 2. SQL Databases
resource "fabricext_sql_database_permission" "this" {
  for_each = local.sql_database_permissions

  workspace_id      = var.fabric_permissions_matrix.workspace_id
  sql_database_name = each.value.sql_database_name
  sql_database_id   = each.value.sql_database_id
  principal_id      = each.value.principal_id
  principal_type    = each.value.principal_type
  permissions       = each.value.permissions
}

resource "fabricext_item_share_gate" "sql_database" {
  for_each = { for k, v in local.sql_database_permissions : k => v if v.verify_item_shares }

  workspace_id         = var.fabric_permissions_matrix.workspace_id
  item_id              = fabricext_sql_database_permission.this[each.key].sql_database_id
  principal_id         = each.value.principal_id
  principal_type       = each.value.principal_type
  expected_permissions = fabricext_sql_database_permission.this[each.key].permissions
  match_type           = coalesce(var.fabric_permissions_matrix.share_gate_match_type, "subset")

  timeouts {
    create = coalesce(var.fabric_permissions_matrix.share_gate_timeout, "5m")
  }

  depends_on = [
    fabricext_sql_database_permission.this,
  ]
}

# 3. Lakehouses (Item Sharing)
resource "fabricext_lakehouse_permission" "this" {
  for_each = local.lakehouse_item_permissions

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  lakehouse_name = each.value.lakehouse_name
  lakehouse_id   = each.value.lakehouse_id
  principal_id   = each.value.principal_id
  principal_type = each.value.principal_type
  permissions    = each.value.permissions
}

resource "fabricext_item_share_gate" "lakehouse" {
  for_each = { for k, v in local.lakehouse_item_permissions : k => v if v.verify_item_shares }

  workspace_id         = var.fabric_permissions_matrix.workspace_id
  item_id              = fabricext_lakehouse_permission.this[each.key].lakehouse_id
  principal_id         = each.value.principal_id
  principal_type       = each.value.principal_type
  expected_permissions = fabricext_lakehouse_permission.this[each.key].permissions
  match_type           = coalesce(var.fabric_permissions_matrix.share_gate_match_type, "subset")

  timeouts {
    create = coalesce(var.fabric_permissions_matrix.share_gate_timeout, "5m")
  }

  depends_on = [
    fabricext_lakehouse_permission.this,
  ]
}

# 4. Lakehouses (OneLake Security Enablement)
resource "fabricext_lakehouse_onelake_security" "this" {
  for_each = local.unique_lakehouses

  workspace_id               = var.fabric_permissions_matrix.workspace_id
  lakehouse_name             = each.value.lakehouse_name
  lakehouse_id               = each.value.lakehouse_id
  remove_default_reader_role = each.value.remove_default_reader_role
}

# 5. Lakehouses (OneLake Data Access Roles)
resource "fabricext_lakehouse_data_access_role" "this" {
  for_each = local.lakehouse_data_access_roles

  workspace_id   = var.fabric_permissions_matrix.workspace_id
  lakehouse_name = each.value.lakehouse_name != null ? try(fabricext_lakehouse_onelake_security.this[each.value.lakehouse_name].lakehouse_name, each.value.lakehouse_name) : null
  lakehouse_id   = each.value.lakehouse_id != null ? try(fabricext_lakehouse_onelake_security.this[each.value.lakehouse_id].lakehouse_id, each.value.lakehouse_id) : null
  role_name      = each.value.role_name
  kind           = each.value.kind

  # Simple mode attributes
  paths          = each.value.paths
  actions        = each.value.actions
  principal_ids  = each.value.principal_ids
  principal_type = each.value.principal_type

  depends_on = [
    fabricext_lakehouse_onelake_security.this,
    fabricext_lakehouse_permission.this,
    fabricext_item_share_gate.lakehouse,
  ]

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
