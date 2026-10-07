terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.5"
    }
  }
}

# Authenticate using the Azure Developer CLI (`azd auth login`).
provider "fabricext" {
  use_dev_cli = true

  # Optional tenant ID
  # tenant_id = "00000000-0000-0000-0000-000000000000"
}
