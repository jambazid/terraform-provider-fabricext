terraform {
  required_version = ">= 1.6.0"
  required_providers {
    fabric = {
      source  = "microsoft/fabric"
      version = "~> 1.0"
    }
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.1.2"
    }
  }
}

# Both providers authenticate against the same tenant using shared environment variables
# (FABRIC_CLIENT_ID, FABRIC_CLIENT_SECRET, FABRIC_TENANT_ID) or native Azure CLI sessions.
provider "fabric" {}
provider "fabricext" {}

# Provision the Warehouse using Microsoft's official provider:
resource "fabric_warehouse" "sales" {
  workspace_id = var.workspace_id
  display_name = "sales_wh"
}

# Declaratively manage item-level sharing using fabricext:
resource "fabricext_warehouse_permission" "analysts" {
  workspace_id   = fabric_warehouse.sales.workspace_id
  warehouse_name = fabric_warehouse.sales.display_name
  principal_id   = var.analysts_group_id
  principal_type = "Group"
  role_type      = "read"
}

variable "workspace_id" {
  type        = string
  description = "Microsoft Fabric workspace UUID."
}

variable "analysts_group_id" {
  type        = string
  description = "Microsoft Entra ID group UUID."
}
