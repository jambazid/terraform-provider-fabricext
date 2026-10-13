# Verify OneLake security enablement and purge DefaultReader role for zero-trust least-privilege
resource "fabricext_lakehouse_onelake_security" "raw_bronze" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"

  # Purge the auto-created DefaultReader role so only explicitly assigned roles grant access
  remove_default_reader_role = true
}

# Preserve DefaultReader role if legacy ReadAll access across all tables is intentionally desired
resource "fabricext_lakehouse_onelake_security" "shared_silver" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "22222222-2222-2222-2222-222222222222"

  remove_default_reader_role = false
}

