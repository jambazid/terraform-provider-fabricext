---
name: registry-documentation
description: Behavioral specification and factual accuracy requirements for Terraform Registry documentation, Access Control Playbook, and guides.
targets:
  - ../templates/**
  - ../specs/citations.md
  - ../README.md
---

# Behavioral Specification: Registry Documentation & Access Control Playbook

## 1. Overview & Architectural Role

All documentation rendered for the Terraform Registry is generated from templates under `templates/` and resource/provider `MarkdownDescription` schemas via `tfplugindocs`. In accordance with repository laws:

1. `docs/**` is 100% owned by `tfplugindocs` and is never hand-edited.
2. Every heading across `templates/**` and `docs/**` must be 5 words or fewer.
3. GitHub-style alerts (`> [!NOTE]`, `> [!WARNING]`) and ```mermaid diagrams are prohibited in Registry templates and docs; Terraform Registry callouts (`~> **Note:**`, `!> **Warning:**`, `~> **Important:**`) and ASCII diagrams are used instead.

## 2. Experimental Status & Upstream Positioning

Because `fabricext` is a community extension operating in a pre-`v1.0.0` state to facilitate early packaging and supply-chain provenance:

- The experimental banner (`!> **Warning:**`) must appear prominently across `templates/index.md.tmpl`, resource templates, and `README.md`.
- Documentation clearly positions `fabricext` alongside Microsoft's official [`microsoft/fabric`](https://registry.terraform.io/providers/microsoft/fabric/latest) provider, recommending upstream for core workspace and item management.

## 3. Deprecated Resources & Factual Accuracy

- `fabricext_warehouse_permission` and `fabricext_sql_database_permission` templates lead with the deprecation notice explaining that Microsoft Fabric does not offer public item-sharing REST endpoints for Warehouses or SQL Databases.
- Fine-grained database access is ground in primary-source Microsoft Learn documentation:
  - Warehouses: In-database T-SQL permissions (`GRANT`/`DENY`/`REVOKE`).
  - SQL Databases: In-database T-SQL roles and permissions or Purview policies.
  - Lakehouses: OneLake Data Access Roles (`fabricext_lakehouse_permission`, `fabricext_lakehouse_onelake_security`).

## 4. Verification Requirements & Falsifiers

- `[@test] ../internal/provider/registry_docs_test.go::TestRegistryDocs_ExperimentalBannerPresent`
  Verifies that `templates/index.md.tmpl`, `README.md`, `modules/permissions/README.md`, and all resource templates contain the experimental banner.

- `[@test] ../internal/provider/registry_docs_test.go::TestRegistryDocs_DeprecationNoticePresent`
  Verifies that `templates/resources/warehouse_permission.md.tmpl` and `templates/resources/sql_database_permission.md.tmpl` include the formal deprecation notice.

- `[@test] ../internal/provider/registry_docs_test.go::TestRegistryDocs_AccessControlPlaybook`
  Verifies that `templates/guides/access_control_playbook.md.tmpl` exists, contains the Start Here routing table, and provides structured cards with verified verify queries.

- `[@test] ../internal/provider/registry_docs_test.go::TestRegistryDocs_HeadingLengthBound`
  Verifies that every markdown heading across `templates/**` does not exceed 5 words.
