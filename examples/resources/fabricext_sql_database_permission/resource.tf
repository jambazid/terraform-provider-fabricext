# To safely remove existing fabricext_sql_database_permission resources from Terraform state
# without attempting external API calls or deleting Fabric resources, use Terraform 1.7+ removed blocks:
removed {
  from = fabricext_sql_database_permission.orders_readers

  lifecycle {
    destroy = false
  }
}

removed {
  from = fabricext_sql_database_permission.direct_id_readers

  lifecycle {
    destroy = false
  }
}
