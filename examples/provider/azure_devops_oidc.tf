terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.5"
    }
  }
}

# Authenticate using Azure DevOps Workload Identity Federation in Azure Pipelines.
# In an Azure Pipelines task, SYSTEM_ACCESSTOKEN and service connection ID can be passed:
provider "fabricext" {
  tenant_id                          = var.tenant_id
  client_id                          = var.client_id
  azure_devops_service_connection_id = var.azure_devops_service_connection_id
  oidc_request_token                 = var.system_access_token

  # Or allow the provider to automatically read FABRIC_AZURE_DEVOPS_SERVICE_CONNECTION_ID and SYSTEM_ACCESSTOKEN from the environment.
}

variable "tenant_id" {
  type        = string
  description = "Microsoft Entra ID tenant UUID."
}

variable "client_id" {
  type        = string
  description = "Microsoft Entra ID Service Principal application (client) UUID."
}

variable "azure_devops_service_connection_id" {
  type        = string
  description = "The Azure DevOps Service Connection ID using Workload Identity Federation."
}

variable "system_access_token" {
  type        = string
  sensitive   = true
  description = "The $(System.AccessToken) provided by Azure Pipelines."
}
