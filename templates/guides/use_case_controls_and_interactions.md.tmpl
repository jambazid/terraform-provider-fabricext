---
page_title: "Use Cases: Security Controls & Interactions"
subcategory: "Use Cases"
description: |-
  Detailed inventory of Microsoft Fabric security controls across 6 tiers, including precedence rules, override behaviors, and conflict resolution between workspace roles, item shares, and SQL permissions.
---

# Security Controls & Interactions

Microsoft Fabric evaluates access across six distinct security tiers. Understanding which tier takes precedence and how controls interact is essential for robust least-privilege design.

---

## The Controls Inventory

Security controls in Microsoft Fabric span identity, capacity, workspace, item, storage engine, and reporting planes:

| Tier | Control Type | Primary Governance Plane | Description |
| :--- | :--- | :--- | :--- |
| **Tier 1: Identity** | Entra ID Groups & Principals | Azure Entra ID / Graph | Authenticates users (UPN), security groups (display name), and service principals; establishes group memberships. |
| **Tier 2: Tenant & Capacity** | Tenant Switch Overrides & Domains | Fabric Admin Portal | Controls external sharing toggles, OneLake access API switches, and capacity assignment boundaries. |
| **Tier 3: Workspace Boundary** | Workspace Roles | Fabric Workspace API / `microsoft/fabric` | Roles: `Admin`, `Member`, `Contributor`, `Viewer`. Determines administrative and collaboration scope. |
| **Tier 4: Item Perimeter** | Item Shares & Permissions | Fabric Item Permission API / `fabricext` | Roles: `read` (CONNECT to TDS endpoint), `read_data` (SQL DB broad read), `read_spark` (SQL DB Spark analytics), `write`, `reshare`, OneLake Data Access Roles. Controls item discovery and gateway access. |
| **Tier 5: Data Engine RBAC** | T-SQL RBAC & OneLake Storage Roles | SQL TDS Engine / OneLake Storage Engine | T-SQL `GRANT`/`DENY` on Schemas/Tables/Views, SQL Row-Level Security (RLS), Column-Level Security (CLS), OneLake path filters, and row/column constraints over Tabular Data Stream (TDS, port 1433). |
| **Tier 6: Consumption & BI** | Semantic Models & Power BI Apps | Power BI Analysis Services | Direct Lake mode, DirectQuery fallback over TDS, Entra Single Sign-On (SSO) token delegation, and dataset-level RLS/Object-Level Security (OLS). |

---

## Control Precedence Matrix

When multiple controls apply to a principal simultaneously, Microsoft Fabric evaluates access according to strict precedence rules:

| Condition / Conflict | Evaluation Precedence | Net Effective Access | Architecture Rule |
| :--- | :--- | :--- | :--- |
| **Workspace `Member` vs. T-SQL `DENY`** | Workspace Role **overrides** T-SQL RBAC. | **Full Access** (`db_owner`) | Workspace `Admin`, `Member`, and `Contributor` automatically hold administrative privileges on all items in the workspace. |
| **Workspace `Viewer` vs. T-SQL `GRANT`** | T-SQL RBAC **governs** data query. | **Granular Access** | `Viewer` grants item visibility; SQL engine filters data according to T-SQL grants. |
| **SQL DB `read_data` vs. T-SQL `DENY`** | Item `read_data` **bypasses** SQL schema isolation. | **Broad Data Access** | Fabric grants a synthetic read token across all tables when `read_data` is present on a SQL Database item. |
| **Warehouse `read` vs. T-SQL Default-Deny** | T-SQL engine **governs** object access. | **Strict Schema Isolation** | `read` provides TDS `CONNECT` only; ungranted schemas remain completely hidden and inaccessible. |
| **OneLake Role vs. Direct Lake** | OneLake Security **filters** storage reads. | **Filtered Parquet Access** | Power BI Analysis Services respects OneLake path filters and RLS/CLS when SSO is active. |
| **Warehouse T-SQL RLS vs. Direct Lake** | T-SQL Security **triggers fallback**. | **DirectQuery over TDS** | Direct Lake on SQL endpoints automatically falls back to DirectQuery over TDS to enforce SQL-level security predicates. |
| **Dataset RLS vs. Underlying SQL RLS** | **Both** evaluated in series (Intersection). | **Most Restrictive Subset** | Power BI filters data in the semantic model; SQL engine filters data at query time via SSO. |

---

## Workspace Role Administrative Inheritance

!> **Warning:** **Workspace Role Permission Inheritance**: Assigning a user or security group to the **Admin**, **Member**, or **Contributor** role on a Fabric workspace automatically confers administrative rights (`db_owner`) on all Warehouses and Lakehouses within that workspace:
- T-SQL `DENY SELECT` on sensitive schemas or tables has **no effect**.
- SQL Row-Level Security (RLS) predicates are completely **bypassed**.
- OneLake Data Access Roles have **no effect**.

### Least-Privilege Mitigations

To enforce granular object-level or schema-level security:

1. **Keep Data Consumers Out of Workspace Roles**: Data analysts and reporting users should have **no role assignment** on the parent workspace. Grant access exclusively at the item level using `fabricext_warehouse_permission` (`role_type = "read"`) or `fabricext_lakehouse_permission`.
2. **Beware of Workspace `Viewer` Cross-Item Leakage**: While assigning the **Viewer** role respects T-SQL permissions on a specific Warehouse, it grants read and enumeration access across **all** items in that workspace (including other Warehouses, notebooks, pipelines, and unconfigured Lakehouses). To maintain strict cross-item isolation, principals should have **no workspace role** (pure item share).
3. **Split-Workspace Topology for Data Engineers**: If engineers require Contributor permissions to build ETL pipelines or notebooks, separate the ETL authoring workspace from the production storage workspace. Grant pipeline identities access to the warehouse item via `fabricext`, leaving the developers without administrative roles on the data-bearing workspace.

---

## Identity Federation & Principals

Microsoft Fabric enforces distinct principal type rules depending on the target resource:

| Principal Type | Warehouse (`fabricext_warehouse_permission`) | SQL Database (`fabricext_sql_database_permission`) | Lakehouse (`fabricext_lakehouse_permission`) |
| :--- | :--- | :--- | :--- |
| **User** | Supported (`"User"`) | Supported (`"User"`) | Supported (`"User"`) |
| **Group** *(Recommended)* | Supported (`"Group"`) | Supported (`"Group"`) | Supported (`"Group"`) |
| **ServicePrincipal** | Supported (`"ServicePrincipal"`) | Supported (`"ServicePrincipal"`) | Supported (`"ServicePrincipal"`) |
| **ManagedIdentity** | Supported via Group | Supported via Group | Supported (`"ManagedIdentity"`) |
| **ServicePrincipalProfile** | Supported (`"ServicePrincipalProfile"`) | Supported (`"ServicePrincipalProfile"`) | Not Supported by Lakehouse API |

~> **Note:** **Managed Identities in Warehouse & SQL Database**: The Microsoft Fabric Item Permissions API does not support direct assignment to Managed Identity principals. To grant access to a Managed Identity, add the Managed Identity to a Microsoft Entra ID Security Group and grant permissions to that Group.

~> **Note:** **OneLake Role Member Type Uniformity**: In simple mode (`principal_ids`), OneLake Data Access Roles require uniform member types within each role resource. In advanced mode (`entra_member` blocks), heterogeneous member types (Users, Groups, Service Principals, and Managed Identities) are natively supported on the same role.

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Use Cases Overview](./use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Warehouse Schema Isolation](./use_case_warehouse_schema_isolation.md)** | Isolating schemas using `role_type = "read"` and T-SQL RBAC. |
| **[Lakehouse OneLake Security](./use_case_lakehouse_onelake_security.md)** | Granular path filters and storage-level RLS/CLS. |
| **[Power BI Identity Flow](./use_case_powerbi_identity_propagation.md)** | End-to-end token flow to semantic models and query fallback. |
