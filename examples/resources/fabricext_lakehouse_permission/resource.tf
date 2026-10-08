# Simple flat mode (basic table and folder path access)
resource "fabricext_lakehouse_permission" "bronze_readers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"
}

# Advanced structured mode (Row-Level Security, Column-Level Security, and heterogeneous members)
resource "fabricext_lakehouse_permission" "emea_analysts" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "22222222-2222-2222-2222-222222222222"
  role_name    = "EmeaAnalysts"

  decision_rule {
    paths   = ["/Tables/customers"]
    actions = ["Read"]
    effect  = "Permit"

    row_constraint {
      table_path = "/Tables/customers"
      predicate  = "Region = 'EMEA'"
    }

    column_constraint {
      table_path = "/Tables/customers"
      columns    = ["customer_id", "email", "country"]
      action     = "Read"
      effect     = "Permit"
    }
  }

  entra_member {
    object_id   = "11111111-1111-1111-1111-111111111111"
    object_type = "Group"
  }

  entra_member {
    object_id   = "33333333-3333-3333-3333-333333333333"
    object_type = "User"
  }
}
