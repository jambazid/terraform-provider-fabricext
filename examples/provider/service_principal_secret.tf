# Authenticate using a Microsoft Entra Service Principal with a Client Secret.
provider "fabricext" {
  client_id     = "00000000-0000-0000-0000-000000000001"
  client_secret = "your-client-secret-here"
  tenant_id     = "00000000-0000-0000-0000-000000000000"
}
