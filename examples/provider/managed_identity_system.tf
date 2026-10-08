# Authenticate using an Azure System-Assigned Managed Identity.
provider "fabricext" {
  use_msi = true
}
