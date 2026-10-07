terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.2"
    }
  }
}

# Authenticate using a User-Assigned Managed Identity (MSI) specifying its Client ID.
provider "fabricext" {
  use_msi   = true
  client_id = var.managed_identity_client_id
}

variable "managed_identity_client_id" {
  type        = string
  description = "Client (Application) ID of the User-Assigned Managed Identity."
}
