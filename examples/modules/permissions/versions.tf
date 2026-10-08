# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.0"
    }
  }
}
