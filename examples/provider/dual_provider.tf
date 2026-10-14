terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabric = {
      source  = "microsoft/fabric"
      version = "~> 1.0"
    }
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.3.0"
    }
  }
}

# Both providers authenticate against the same tenant using shared environment variables
# (FABRIC_CLIENT_ID, FABRIC_CLIENT_SECRET, FABRIC_TENANT_ID) or native Azure CLI sessions.
provider "fabric" {}
provider "fabricext" {}

# Provision the Lakehouse using Microsoft's official provider:
resource "fabric_lakehouse" "sales" {
  workspace_id = var.workspace_id
  display_name = "sales_lh"
}

# Declaratively manage OneLake security lifecycle and Data Access Roles using fabricext:
resource "fabricext_lakehouse_onelake_security" "sales" {
  workspace_id               = fabric_lakehouse.sales.workspace_id
  lakehouse_id               = fabric_lakehouse.sales.id
  remove_default_reader_role = true
}

resource "fabricext_lakehouse_permission" "analysts" {
  workspace_id   = fabric_lakehouse.sales.workspace_id
  lakehouse_id   = fabric_lakehouse.sales.id
  role_name      = "SalesAnalysts"
  paths          = ["/Tables/sales"]
  actions        = ["Read"]
  principal_ids  = [var.analysts_group_id]
  principal_type = "Group"

  depends_on = [
    fabricext_lakehouse_onelake_security.sales,
  ]
}

variable "workspace_id" {
  type        = string
  description = "Microsoft Fabric workspace UUID."
}

variable "analysts_group_id" {
  type        = string
  description = "Microsoft Entra ID group UUID."
}
