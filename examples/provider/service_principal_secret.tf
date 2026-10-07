terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.4"
    }
  }
}

# Authenticate using a Microsoft Entra ID Service Principal with a Client Secret.
# We recommend passing sensitive values via environment variables:
# FABRIC_TENANT_ID, FABRIC_CLIENT_ID, and FABRIC_CLIENT_SECRET.
provider "fabricext" {
  tenant_id     = var.tenant_id
  client_id     = var.client_id
  client_secret = var.client_secret
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}

variable "client_secret" {
  type        = string
  sensitive   = true
  description = "Microsoft Entra ID Service Principal client secret."
}
