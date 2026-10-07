terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.1"
    }
  }
}

# Authenticate against a sovereign cloud environment (e.g. Azure US Government or China).
# The provider automatically targets the sovereign endpoint and audience token scope:
# - usgovernment: https://api.fabric.microsoft.us/.default
# - china: https://api.fabric.microsoft.cn/.default
provider "fabricext" {
  environment = "usgovernment"

  # Or let the provider read FABRIC_ENVIRONMENT="usgovernment"
}
