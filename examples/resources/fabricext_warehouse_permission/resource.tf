# To safely remove existing fabricext_warehouse_permission resources from Terraform state
# without attempting external API calls or deleting Fabric resources, use Terraform 1.7+ removed blocks:
removed {
  from = fabricext_warehouse_permission.analytics_readers

  lifecycle {
    destroy = false
  }
}

removed {
  from = fabricext_warehouse_permission.direct_id_writers

  lifecycle {
    destroy = false
  }
}
