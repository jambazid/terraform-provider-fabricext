---
page_title: "Use Cases: Security Controls Inventory & Interaction Matrix"
subcategory: "Use Cases"
description: |-
  Detailed inventory of Microsoft Fabric security controls across 6 tiers, including precedence rules, override behaviors, and conflict resolution between workspace roles, item shares, and SQL permissions.
---

# Use Cases: Security Controls Inventory & Interaction Matrix

Microsoft Fabric evaluates access across six distinct security tiers. Understanding which tier takes precedence and how controls interact is essential for robust least-privilege design.

---

## The Controls Inventory

Security controls in Microsoft Fabric span identity, capacity, workspace, item, storage engine, and reporting planes:

| Tier | Control Type | Primary Governance Plane | Description |
| :--- | :--- | :--- | :--- |
| **Tier 1: Identity** | Entra ID Groups & Principals | Azure Entra ID / Graph | Authenticates users, service principals, and managed identities; establishes group memberships. |
| **Tier 2: Tenant & Capacity** | Tenant Switch Overrides & Domains | Fabric Admin Portal | Controls external sharing toggles, OneLake access API switches, and capacity assignment boundaries. |
| **Tier 3: Workspace Boundary** | Workspace Roles | Fabric Workspace API / `microsoft/fabric` | Roles: `Admin`, `Member`, `Contributor`, `Viewer`. Determines administrative and collaboration scope. |
| **Tier 4: Item Perimeter** | Item Shares & Permissions | Fabric Item Permission API / `fabricext` | Roles: `read` (CONNECT to SQL endpoint), `read_data` (SQL DB broad access), `write`, `reshare`, OneLake Data Access Roles. Controls item discovery and gateway connectivity. |
| **Tier 5: Data Engine RBAC** | T-SQL RBAC & OneLake Storage Roles | SQL TDS Engine / OneLake Storage Engine | T-SQL `GRANT`/`DENY` on Schemas/Tables/Views, SQL RLS/CLS, OneLake path filters, and row/column constraints over Tabular Data Stream (TDS, port 1433). |
| **Tier 6: Consumption & BI** | Semantic Models & Power BI Apps | Power BI Analysis Services | Direct Lake mode, DirectQuery fallback over TDS, Entra SSO token delegation, and dataset-level RLS/OLS. |

---

## Control Precedence & Conflict Resolution Matrix

When multiple controls apply to a principal simultaneously, Microsoft Fabric evaluates access according to strict precedence rules:

| Condition / Conflict | Evaluation Precedence | Net Effective Access | Architecture Rule |
| :--- | :--- | :--- | :--- |
| **Workspace `Member` vs. T-SQL `DENY`** | Workspace Role **overrides** T-SQL RBAC. | **Full Access** (`db_owner`) | Workspace `Admin`, `Member`, and `Contributor` automatically hold administrative privileges on all items in the workspace. |
| **Workspace `Viewer` vs. T-SQL `GRANT`** | T-SQL RBAC **governs** data query. | **Granular Access** | `Viewer` grants item visibility; SQL engine filters data according to T-SQL grants. |
| **SQL DB `read_data` vs. T-SQL `DENY`** | Item `read_data` **bypasses** SQL schema isolation. | **Broad Data Access** | Fabric grants a synthetic read token across all tables when `read_data` is present on a SQL Database item. |
| **Warehouse `read` vs. T-SQL Default-Deny** | T-SQL engine **governs** object access. | **Strict Schema Isolation** | `read` provides TDS `CONNECT` only; ungranted schemas remain completely hidden and inaccessible. |
| **OneLake Role vs. Direct Lake** | OneLake Security **filters** storage reads. | **Filtered Parquet Access** | Power BI's semantic model engine (Analysis Services) respects OneLake path filters and RLS/CLS when SSO is active. |
| **Warehouse T-SQL RLS vs. Direct Lake** | T-SQL Security **triggers fallback**. | **DirectQuery over TDS** | Direct Lake automatically falls back to DirectQuery over TDS to enforce SQL-level security predicates. |
| **Dataset RLS vs. Underlying SQL RLS** | **Both** evaluated in series (Intersection). | **Most Restrictive Subset** | Power BI filters data in the semantic model; SQL engine filters data at query time via SSO. |

---

## Workspace Role Administrative Inheritance

!> **Warning:** **Workspace Role Permission Inheritance**: Assigning a user or security group to the **Admin**, **Member**, or **Contributor** role on a Fabric workspace automatically confers administrative rights (`db_owner`) on all Warehouses and Lakehouses within that workspace:
- T-SQL `DENY SELECT` on sensitive schemas or tables has **no effect**.
- SQL Row-Level Security (RLS) predicates are completely **bypassed**.
- OneLake Data Access Roles have **no effect**.

### Mitigation Strategies: Enforcing Least Privilege

To enforce granular object-level or schema-level security:

1. **Keep Data Consumers Out of Workspace Roles**: Data analysts and reporting users should have **no role assignment** on the parent workspace. Grant access exclusively at the item level using `fabricext_warehouse_permission` (`role_type = "read"`) or `fabricext_lakehouse_permission`.
2. **Optional Workspace `Viewer` Role**: If users require broad browsing of workspace artifact lists, assign the **Viewer** role. Unlike Admin/Member/Contributor, the `Viewer` role does not grant `db_owner` and respects engine-level T-SQL grants.
3. **Split-Workspace Topology for Data Engineers**: If engineers require Contributor permissions to build ETL pipelines or notebooks, separate the ETL authoring workspace from the production storage workspace. Grant pipeline identities access to the warehouse item via `fabricext`, leaving the developers without administrative roles on the data-bearing workspace.

---

## Identity Federation & Principal Support

Microsoft Fabric enforces distinct principal type rules depending on the target resource:

| Principal Type | Warehouse (`fabricext_warehouse_permission`) | SQL Database (`fabricext_sql_database_permission`) | Lakehouse (`fabricext_lakehouse_permission`) |
| :--- | :--- | :--- | :--- |
| **User** | Supported (`"User"`) | Supported (`"User"`) | Supported (`"User"`) |
| **Group** *(Recommended)* | Supported (`"Group"`) | Supported (`"Group"`) | Supported (`"Group"`) |
| **ServicePrincipal** | Supported (`"ServicePrincipal"`) | Supported (`"ServicePrincipal"`) | Supported (`"ServicePrincipal"`) |
| **ManagedIdentity** | Supported via Group | Supported via Group | Supported (`"ManagedIdentity"`) |
| **ServicePrincipalProfile** | Supported (`"ServicePrincipalProfile"`) | Supported (`"ServicePrincipalProfile"`) | Not Supported by Lakehouse API |

~> **Note:** **OneLake Role Member Type Uniformity**: OneLake Data Access Roles require uniform member types within each role. To manage mixed user types within a single Lakehouse role, assign access to a Microsoft Entra ID security `Group` containing those users and service principals.

---

## Related Guides

- **[Use Cases Overview](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_overview)**: Master comparison matrix and 5-layer perimeter overview.
- **[Warehouse Schema Isolation & Declarative RBAC](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_warehouse_schema_isolation)**: Isolating schemas using `role_type = "read"` and T-SQL.
- **[Lakehouse OneLake Security & Data Access Roles](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_lakehouse_onelake_security)**: Granular path filters and storage-level RLS/CLS.
- **[Power BI Identity Flow: Direct Lake vs. DirectQuery](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_powerbi_identity_propagation)**: End-to-end token flow to semantic models.
