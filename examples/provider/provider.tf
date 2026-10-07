terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.5"
    }
  }
}

# Authenticate automatically via the native Entra ID credential chain:
# (Static Token -> Client Certificate -> Client Secret -> Azure DevOps OIDC -> Workload Identity -> Managed Identity -> Azure Developer CLI -> Azure CLI).
provider "fabricext" {}
