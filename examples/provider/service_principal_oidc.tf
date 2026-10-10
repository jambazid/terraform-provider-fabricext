# Authenticate using Workload Identity Federation (OIDC) in CI/CD (e.g., GitHub Actions).
# Note: When omitting oidc_token, the runner environment must supply AZURE_FEDERATED_TOKEN_FILE.
provider "fabricext" {
  client_id = "00000000-0000-0000-0000-000000000001"
  tenant_id = "00000000-0000-0000-0000-000000000000"
  use_oidc  = true
}
