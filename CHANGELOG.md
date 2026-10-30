# Changelog

All notable changes to `terraform-provider-fabricext` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## v0.7.0 - October 30, 2026

### 💥 Breaking

* Unify `modules/permissions` matrix structure: compose item-level permissions and OneLake Data Access Roles under a unified `$.lakehouses` key, key principal permissions by Entra ID UUID maps across all workloads, and add `verify_item_shares` Admin API gate lever.

### ✨ Added

* Restore declarative item-level permission resources `fabricext_warehouse_permission`, `fabricext_sql_database_permission`, and `fabricext_lakehouse_permission` backed by Power BI metadata access endpoints with in-place drift reconciliation, `notify` flag and `message` routing. Add Power BI credentials support, dual-scope Entra token resolution, and in-memory mock server `pbimock`.

### 💫 Changed

* Uplift Dependabot configuration: exclude `.mise/locks/**` tool lockfiles from dependency scans, group Go module and Terraform updates, and automatically attach the `skip-changelog` label.

### 🪲 Fixed

* Harmonize Power BI item sharing wire protocol and OpenAPI contract with upstream runtime behavior: support root artifact access entities, Entra object ID resolution, bidirectional bitmask mappings, and prevent bitwise OR retention during permission updates with notifications.

### 🚨 Security

* Uplift toolchain dependencies to eliminate 17 open Dependabot security alerts, enforce Python 3.14 across tooling environments, and pin Trivy 0.74.0 for stable provenance verification.

## 0.6.0 - October 15, 2026

### ✨ Added

* Add `fabricext_item_access_details` data source and `fabricext_item_share_gate` verification waiter resource backed by the Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`), enabling declarative verification of out-of-band item shares for Warehouses, SQL Databases, and Lakehouses.

## 0.5.0 - October 15, 2026

### 💥 Breaking

* Rename resource `fabricext_lakehouse_permission` to `fabricext_lakehouse_data_access_role` across all provider schemas, tests, examples, and `modules/permissions`. Remove deprecated resources `fabricext_warehouse_permission` and `fabricext_sql_database_permission` as scheduled for v0.5.0.

### 📚 Documentation

* Align roadmap and documentation for post-v0.4.0 scope: update candidate extensions (UDF, dbt jobs, shortcuts, capacity governance), reflect v0.5.0 scheduled removal target for tombstoned resources, and enforce concise heading standards across project markdown.
* Reorganize documentation guides under subcategory Guides, remove Use Cases: page title prefixes, adopt open-ended connectivity tool phrasing, incorporate Atlas alongside SQL migration tools, and streamline guides for concision.

## 0.4.0 - October 14, 2026

### ⚰️ Deprecated

* Purge active item-sharing implications for Warehouse and SQL Database across documentation, guides, matrix composition, permissions module, and runnable HCL examples in preparation for v0.4.0 tombstoning

### 📚 Documentation

* Document honest technical comparison narrative against official provider, highlighting automated OneLake DefaultReader zero-trust purging, RFC 9110 ETag concurrency, and hermetic offline contract testing

## v0.3.0 - October 14, 2026

### ✨ Added

* Add fabricext_lakehouse_onelake_security resource for OneLake Data Access Security enablement verification and DefaultReader role management

### 💫 Changed

* Add explicit depends_on ordering for all lakehouses in permissions module and examples

### ⚰️ Deprecated

* Tombstone fabricext_warehouse_permission and fabricext_sql_database_permission resources due to non-existent Fabric REST endpoints, and remove repository-authored OpenAPI overlay.

### 🪲 Fixed

* Implement OneLake Data Access Role pagination via continuationToken, enforce RFC 9110 ETag quoting normalisation, and add client-side and mock validation rejecting write actions combined with row or column constraints.

### 📚 Documentation

* Document permissions module staged features, RLS/CLS, and full matrix geometry; add Registry Fabric logo and filter semantic PR body edits
* Add enterprise Access Control Playbook, Registry citations register, experimental provider notice, and deprecation documentation for warehouse and sql database item sharing

## v0.2.1 - October 09, 2026

### 📚 Documentation

* Add comprehensive enterprise access control and security guides under the "Use Cases" documentation category covering the 6-tier perimeter stack, Warehouse schema isolation via role_type = "read", Entra ID Object ID binding in T-SQL, OneLake Data Access Roles, Power BI Direct Lake vs. DirectQuery identity propagation, and declarative Atlas schema-as-code integration.
* Streamline the Terraform Registry landing page into a lean, single-example introduction matching the official provider structure, moving detailed material into dedicated guides.
* Add dedicated documentation guides for the Entra ID authentication and credential chain (`guides/authentication.md`) and declarative permission matrix composition (`guides/matrix_composition.md`).
* Redesign comparison tables in `guides/official_provider_comparison.md` to eliminate horizontal scrollbars and ensure responsive presentation on the Terraform Registry.

### 🚨 Security

* Bump `golang.org/x/net` from `v0.58.0` to `v0.60.0` to resolve `CVE-2026-78669` (HTTP/2 SETTINGS DoS).

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
