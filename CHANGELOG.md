# Changelog

All notable changes to `terraform-provider-fabricext` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## v0.1.2 - October 07, 2026

### 🪲 Fixed

* Resolve concurrency group deadlock between tag workflow and caller reusable release workflow by explicitly namespacing concurrency groups (`tag-*` and `release-*`).

## v0.1.1 - October 07, 2026

### 🪲 Fixed

* Fix permissions inheritance in tag workflow when calling release reusable workflow by granting contents, id-token, and attestations write permissions.

## v0.1.0 - October 07, 2026

### ✨ Added

* Initial release of `terraform-provider-fabricext` (`fabricext`), providing declarative Microsoft Fabric item-level permissions management for Warehouses (`fabricext_warehouse_permission`), Lakehouses (`fabricext_lakehouse_permission`), and SQL Databases (`fabricext_sql_database_permission`), item discovery (`fabricext_item`), and a workspace permissions matrix module (`modules/permissions`).
* Add full authentication parity with the official Microsoft Fabric provider (`registry.terraform.io/microsoft/fabric`), supporting all 8 official authentication mechanisms (Static Token, Client Certificate, Client Secret, Azure DevOps OIDC, Workload Identity, Managed Identity, Azure Developer CLI, and Azure CLI) alongside sovereign cloud environments (`public`, `usgovernment`, `china`), auxiliary tenant tokens, and file path credential inputs. Add comprehensive authentication examples, Registry documentation guides, automated CI test coverage reporting with Step Summaries and PR comments, and Changie pre-commit validation.

### 🪲 Fixed

* Fix GPG secret shadowing in tag.yaml and release.yaml by removing empty caller secret passthroughs, add pre-release unprivileged verification gate, wire FABRIC_SKIP_CREDENTIALS_VALIDATION environment variable, strictly validate sovereign cloud environments, ensure item-aware principal_type validation, add null-safe coalesce guards and lakehouse role validations to modules/permissions, align example Terraform version constraints to >= 1.6.0, and fix schema attribute names and module sources in documentation.

### 🚨 Security

* Harden release automation and supply-chain governance: scope GHA secrets explicitly in tag.yaml avoiding secrets inheritance, add SemVer validation and pre-release verification gate to release.yaml, pin GoReleaser to locked version, add Terraform ecosystem to Dependabot, validate UUIDs and normalize principal_type in ImportState, and enforce role collapsing precedence hierarchy.
