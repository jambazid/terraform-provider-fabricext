terraform {
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.0"
    }
  }
}

# Authenticate automatically via the native Entra ID credential chain
# (Static Token -> Client Secret -> Workload Identity OIDC -> Managed Identity -> Azure CLI).
provider "fabricext" {}
