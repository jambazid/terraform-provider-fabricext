terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.2"
    }
  }
}

# Authenticate using a Microsoft Entra ID Service Principal with a Client Certificate (.pfx / .p12).
# Supports base64-encoded certificate strings or filesystem paths.
provider "fabricext" {
  tenant_id                    = var.tenant_id
  client_id                    = var.client_id
  client_certificate_file_path = "/path/to/certificate.pfx"
  client_certificate_password  = var.client_certificate_password

  # Alternatively, pass base64 encoded certificate data directly:
  # client_certificate = var.client_certificate_base64
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}

variable "client_certificate_password" {
  type        = string
  sensitive   = true
  description = "Password protecting the PKCS#12 certificate file."
}
