# Changelog

All notable changes to `terraform-provider-fabricext` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## v0.2.1 - October 08, 2026

### 📚 Documentation

* Add comprehensive enterprise access control and security guides under the "Use Cases" documentation category covering the 6-tier perimeter stack, Warehouse schema isolation via role_type = "read", Entra ID Object ID binding in T-SQL, OneLake Data Access Roles, Power BI Direct Lake vs. DirectQuery identity propagation, and declarative Atlas schema-as-code integration.

## v0.2.0 - October 08, 2026

### ✨ Added

* Add OneLake Data Access Security parity (RLS/CLS dual-mode) to fabricext_lakehouse_permission supporting both simple flat string lists and advanced decision_rule, entra_member, and fabric_item_member blocks.
* Add direct item UUID reference attributes (warehouse_id, sql_database_id, lakehouse_id) across all permission resources and uplift modules/permissions to support direct item IDs and advanced Lakehouse security matrices.

### 🪲 Fixed

* Harden plan modification to prevent false replacement plans on identifier transitions, add nil client defensive guards, scope role collapsing by item type, add randomized ETag retry jitter, and synchronize CI coverage reporting.

### 📚 Documentation

* Add comprehensive upstream provider comparison guide and zero-downtime migration playbooks alongside official provider warning admonitions on Lakehouse permissions.

## v0.1.5 - October 07, 2026

### 📚 Documentation

* Align item permission reconciliation request sequence and payload semantics with Go implementation, disentangle upstream issue citations, add conceptual migration pseudocode callouts, update Registry guide links to canonical URLs, and clean schema frontmatter description metadata.

## v0.1.4 - October 07, 2026

### 📚 Documentation

* Tailor Terraform Registry documentation with native callout sigils and accessible ASCII architecture diagrams while preserving canonical GitHub guide with interactive Mermaid diagrams.

## v0.1.3 - October 07, 2026

### 🪲 Fixed

* Inherit environment secrets in `tag.yaml` reusable workflow call to unblock GPG import and release publishing.

### 📚 Documentation

* Simplify documentation headings across README, SECURITY, CONTRIBUTING, and ROADMAP, and add vector brand assets.

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
