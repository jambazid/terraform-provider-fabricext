# Roadmap

> **Mission Statement:** `terraform-provider-fabricext` (`registry.terraform.io/jambazid/fabricext`) provides declarative Terraform Plugin Framework extensions for Microsoft Fabric security governance, Lakehouse OneLake Data Access Roles, and advanced platform capabilities not currently covered by the official provider.
>
> **Pre-Stable Status (`v0.x`):** All resources in this repository are experimental and pre-stable (`v0.x`). Pin provider versions (`~> 0.7.2`) in `required_providers`.

---

## Current Scope

| Artifact | Type | Fabric Item Type | Underlying Fabric API Surface | Status |
| :--- | :--- | :--- | :--- | :--- |
| `fabricext_warehouse_permission` | Resource | `Warehouse` | Power BI metadata item-sharing API (`POST`/`PUT /metadata/access` with bijective bitmasks & 10s read-back) | Active (`v0.7.0`) |
| `fabricext_sql_database_permission` | Resource | `SQLDatabase` | Power BI metadata item-sharing API (`POST`/`PUT /metadata/access` with bijective bitmasks & 10s read-back) | Active (`v0.7.0`) |
| `fabricext_lakehouse_permission` | Resource | `Lakehouse` | Power BI metadata item-sharing API (`POST`/`PUT /metadata/access` with bijective bitmasks & 10s read-back) | Active (`v0.7.0`) |
| `fabricext_item_share_gate` | Resource | `Warehouse`, `Lakehouse`, `SQLDatabase` | Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`) | Active (`v0.6.0`) |
| `fabricext_item_access_details` | Data Source | `Warehouse`, `Lakehouse`, `SQLDatabase` | Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`) | Active (`v0.6.0`) |
| `fabricext_lakehouse_data_access_role` | Resource | `Lakehouse` | OneLake Data Access Security (`GET` & `PUT /dataAccessRoles` with per-item mutex & `If-Match` ETag RMW) | Active (`v0.5.0`) |
| `fabricext_lakehouse_onelake_security` | Resource | `Lakehouse` | OneLake Data Access Security Enablement Verification & `DefaultReader` Purge | Active (`v0.3.x`) |
| `fabricext_item` | Data Source | `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `SemanticModel` | Workspace Items (`GET /v1/workspaces/{workspaceId}/items?type={type}`) | Active (`v0.1.x`) |
| `modules/permissions` | HCL Module | `Lakehouse`, `Warehouse`, `SQLDatabase` | Flattens unified permissions matrix into `for_each` resource bindings with chained `fabricext_item_share_gate` verification | Active (`v0.7.0`) |

---

## Candidate Future Resources

In alignment with the Fabric API surface gap analysis ([`DESIGN.md §12`](DESIGN.md)), the following extension candidates represent high-value additions where upstream support in `microsoft/fabric` is absent:

1. **User Data Functions (`fabricext_user_data_function`)**:
   - Declarative management for serverless Python functions with connected Lakehouse/Warehouse data sources, PyPI dependencies, and private `.whl` code packages (`/v1/workspaces/{workspaceId}/userDataFunctions`).
2. **Data Build Tool Jobs (`fabricext_data_build_tool_job`)**:
   - Declarative management for native Fabric dbt job schedules, parameters, and connected data sources (`/v1/workspaces/{workspaceId}/dataBuildToolJobs`).
3. **Lakehouse Shortcuts (`fabricext_lakehouse_shortcut`)**:
   - Declarative lifecycle for OneLake-to-OneLake, ADLS Gen2, and Amazon S3 shortcuts (`/v1/workspaces/{workspaceId}/items/{lakehouseId}/shortcuts`).
4. **Fabric Capacity Governance (`fabricext_capacity_assignment`)**:
   - Automated workspace-to-capacity assignment, capacity migration, and governance controls (`/v1/capacities`).
5. **Preview Single-Role OneLake Endpoints**:
   - Once Microsoft promotes the single-role OneLake endpoints (`CreateOrUpdateSingleDataAccessRole`, `GetDataAccessRole`, `DeleteDataAccessRole`) from preview to GA in `platform/swagger.json`, `FabricClient` will cut over to single-role CRUD without changing the public HCL schema.

---

## Deprecation and Migration Policy

Each `fabricext_*` resource follows an explicit deprecation and migration lifecycle:

1. **Upstream Parity Trigger**: When `microsoft/fabric` releases a GA resource covering the same Fabric API surface (for example, official OneLake data-access-role resources).
2. **Deprecation Notice**: The corresponding `fabricext_*` resource sets `DeprecationMessage` in its Plugin Framework schema (emitting a plan warning) and provides an official migration guide.
3. **State Detachment vs Migration**:
   - **Direct Item Permissions (Warehouses, SQL Databases, Lakehouses)**: Managed declaratively via Power BI metadata APIs with drift detection and cross-plane Admin API verification.
   - **Upstream Migration**: Once native resources land in `microsoft/fabric`, state will be cleanly migrated using `removed` blocks in conjunction with `import` blocks pointing to the official provider.
4. **Comparative Analysis & Playbooks**: See [`docs/guides/official_provider_comparison.md`](docs/guides/official_provider_comparison.md) for architectural comparisons and Terraform 1.7+ state detachment and migration playbooks.
