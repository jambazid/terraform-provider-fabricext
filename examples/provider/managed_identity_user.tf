# Authenticate using an Azure User-Assigned Managed Identity.
provider "fabricext" {
  use_msi   = true
  client_id = "00000000-0000-0000-0000-000000000001" # Client ID of the User-Assigned MSI
}
