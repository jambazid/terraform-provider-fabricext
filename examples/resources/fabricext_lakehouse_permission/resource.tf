# Grant direct item-level sharing permissions on a Microsoft Fabric Lakehouse
resource "fabricext_lakehouse_permission" "curated_analysts" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "curated_lakehouse"
  principal_id   = "11111111-1111-1111-1111-111111111111"
  principal_type = "Group"
  permissions    = ["Read", "ReadData"]
}

# Grant direct permissions by Lakehouse UUID (recommended for rename immunity)
resource "fabricext_lakehouse_permission" "pipeline_service_principal" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_id   = "44444444-4444-4444-4444-444444444444"
  principal_id   = "55555555-5555-5555-5555-555555555555"
  principal_type = "ServicePrincipal"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}

