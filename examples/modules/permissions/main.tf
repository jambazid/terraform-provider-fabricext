# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

provider "fabricext" {}

module "permissions" {
  source = "../../../modules/permissions"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    lakehouses = {
      raw_lakehouse = {
        RawReaders = {
          paths          = ["/Tables/customers", "/Files/landing"]
          actions        = ["Read"]
          principal_ids  = ["11111111-1111-1111-1111-111111111111"]
          principal_type = "Group"
        }
      }
      curated_lakehouse = {
        FinanceAnalysts = {
          decision_rules = [
            {
              paths   = ["/Tables/finance"]
              actions = ["Read"]
              effect  = "Permit"
              row_constraints = [
                {
                  table_path = "/Tables/finance"
                  predicate  = "Region = 'EMEA'"
                }
              ]
            }
          ]
          entra_members = [
            {
              object_id   = "22222222-2222-2222-2222-222222222222"
              object_type = "Group"
            }
          ]
        }
      }
    }
  }
}
