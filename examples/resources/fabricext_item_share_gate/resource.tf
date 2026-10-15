resource "fabricext_item_share_gate" "warehouse_connect" {
  workspace_id         = "00000000-0000-0000-0000-000000000001"
  item_id              = "00000000-0000-0000-0000-000000000002"
  principal_id         = "11111111-1111-1111-1111-111111111111"
  principal_type       = "Group"
  expected_permissions = ["Read"]
  match_type           = "subset"

  timeouts {
    create = "60m"
    read   = "10m"
    update = "60m"
    delete = "1m"
  }
}
