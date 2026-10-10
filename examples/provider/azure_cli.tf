# Authenticate using an interactive Azure CLI session (`az login`).
# The provider automatically uses credentials from the active az CLI session.
provider "fabricext" {
  use_cli = true

  # Optional tenant ID filter
  # tenant_id = "00000000-0000-0000-0000-000000000000"
}
