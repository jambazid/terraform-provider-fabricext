terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.0"
    }
  }
}

# Authenticate using a System-Assigned Managed Identity (MSI) on Azure VMs, Container Apps, or Azure DevOps agents.
provider "fabricext" {
  use_msi = true
}
