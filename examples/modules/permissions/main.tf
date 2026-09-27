# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

provider "fabricext" {}

module "permissions" {
  source = "../../../modules/permissions"

  fabric_permissions_matrix = {
    workspace_id = "00000000-0000-0000-0000-000000000001"

    warehouses = {
      sales_dw = {
        read = [
          {
            id   = "11111111-1111-1111-1111-111111111111"
            type = "Group"
          }
        ]
        write = [
          {
            id   = "22222222-2222-2222-2222-222222222222"
            type = "User"
          }
        ]
      }
    }

    sql_databases = {
      orders_db = {
        read_data = [
          {
            id   = "11111111-1111-1111-1111-111111111111"
            type = "Group"
          }
        ]
      }
    }

    lakehouses = {
      raw_lakehouse = {
        RawReaders = {
          paths          = ["/Tables/customers", "/Files/landing"]
          actions        = ["Read"]
          principal_ids  = ["11111111-1111-1111-1111-111111111111"]
          principal_type = "Group"
        }
      }
    }
  }
}
