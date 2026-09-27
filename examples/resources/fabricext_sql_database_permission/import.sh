#!/usr/bin/env bash
# Import an existing SQL Database permission using {workspace_id}/{sql_database_id}/{principal_type}/{principal_id}
terraform import fabricext_sql_database_permission.orders_readers "00000000-0000-0000-0000-000000000001/33333333-3333-3333-3333-333333333333/Group/11111111-1111-1111-1111-111111111111"
