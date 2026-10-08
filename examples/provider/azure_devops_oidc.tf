# Authenticate using Azure DevOps Workload Identity Federation (Service Connection OIDC).
provider "fabricext" {
  client_id                          = "00000000-0000-0000-0000-000000000001"
  tenant_id                          = "00000000-0000-0000-0000-000000000000"
  azure_devops_service_connection_id = "00000000-0000-0000-0000-000000000002"
}
