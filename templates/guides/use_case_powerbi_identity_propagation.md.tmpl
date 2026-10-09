---
page_title: "Use Cases: Power BI Identity Flow"
subcategory: "Use Cases"
description: |-
  How Microsoft Fabric security controls propagate to Power BI reports and semantic models, contrasting DirectQuery over Warehouse TDS endpoints with Direct Lake over Lakehouse Parquet files.
---

# Power BI Identity Flow

A frequent architectural question in Microsoft Fabric enterprise deployments is how item and engine permissions propagate into Power BI reports, semantic models, and Direct Lake queries.

This guide explains the end-to-end identity flow and security enforcement mechanisms across Fabric Warehouses and Lakehouses.

---

## Warehouse vs Lakehouse Modes

Warehouses and Lakehouses both store open Delta Parquet data in OneLake, but diverge in security enforcement:

```text
+─────────────────────────────────────────+     +─────────────────────────────────────────+
│ Fabric Warehouse                        │     │ Fabric Lakehouse                        │
├─────────────────────────────────────────┤     ├─────────────────────────────────────────┤
│ ├─ Storage: OneLake Delta (Parquet)     │     │ ├─ Storage: OneLake Delta (Parquet)     │
│ ├─ Engine: Synapse Data Warehouse (TDS) │     │ ├─ Engine: OneLake Security Policy      │
│ ├─ Direct Lake: Supported on Base Data  │     │ ├─ Direct Lake: Supported Natively      │
│ └─ Security Enforcement: T-SQL RBAC     │     │ └─ Security Enforcement: OneLake Roles  │
│    (Falls back to DirectQuery on RLS)   │        (DirectLakeOnly mode; no fallback)     │
+─────────────────────────────────────────+     +─────────────────────────────────────────+
```

Key architectural differences:
- **Lakehouse**: Enforced by OneLake storage rules under Direct Lake Single Sign-On (SSO). Operates in `DirectLakeOnly` mode; does not fall back to DirectQuery over SQL endpoints.
- **Warehouse**: Enforced by T-SQL Role-Based Access Control (RBAC); falls back from Direct Lake to DirectQuery over Tabular Data Stream (TDS, port 1433) whenever SQL-level security (Row-Level Security, Column-Level Security) is configured.

---

## Warehouse DirectQuery SSO

When a Power BI semantic model connects to a Fabric Warehouse using DirectQuery with Microsoft Entra Single Sign-On (SSO):

```text
Analyst               Power BI Service (Analysis Services)   Warehouse SQL Engine (TDS)
   │                                  │                                  │
   │─── 1. Opens Report ─────────────►│                                  │
   │    (Entra JWT Token)             │─── 2. Query with User Token ────►│ (Delegated Entra Token)
   │                                  │    (Port 1433 / TDS)             │
   │                                  │                                  │─── 3. Evaluate T-SQL RBAC
   │                                  │                                  │    - Check Schema Grants
   │                                  │                                  │    - Apply SQL RLS Predicates
   │                                  │                                  │    - Apply Column Masks
   │                                  │◄── 4. Filtered Query Results ────│
   │◄── 5. Render Visual ─────────────│
```

!> **Warning:** **Power BI Service SSO Configuration Required**: When configuring a semantic model connected to a Warehouse or SQL Analytics endpoint in the Power BI Service (**Workspace > Semantic Model > Settings > Gateway and cloud connections > Data source credentials > Edit credentials > Advanced**), administrators **must check the box**: *"Report viewers can only access this data source with their own Power BI identities using Direct Query"* (Single Sign-On / SSO).

**Dual Consequence of DirectQuery Fallback**:
- **Without SSO**: If Single Sign-On is not enabled, Power BI executes all report queries using the fixed credentials of the dataset owner. Because dataset creators typically hold workspace Contributor or Admin roles (`db_owner`), **queries run as `db_owner`, completely bypassing the end-user's T-SQL RLS predicates and schema isolation** (unintended query execution under elevated dataset-owner privileges).
- **With SSO**: When Single Sign-On is enabled, every report viewer must hold an item-level `read` permission (`fabricext_warehouse_permission` granting TDS `CONNECT`) AND database-level T-SQL grants. Users granted report access in Power BI without an underlying item share will experience visual errors (`Cannot connect to the data source`).

### Identity & Enforcement Rules

1. **Token Delegation**: Power BI's underlying semantic model engine (Analysis Services / VertiPaq) passes the active end-user's Microsoft Entra JSON Web Token (JWT) directly to the Warehouse TDS endpoint (port 1433).
2. **Engine Evaluation**: The Warehouse evaluates the user's security group memberships against database-level permissions (`GRANT SELECT ON SCHEMA::finance`).
3. **Restricted Schemas**: If the report visual references a table in `hr.salaries` and the user only has access to `finance.*`, **the individual visual fails with a permissions error** while the rest of the report renders successfully.
4. **Row-Level Security**: Any T-SQL Security Policies (predicate functions) defined on Warehouse tables execute dynamically for the querying user.

---

## Lakehouse Direct Lake SSO

When a Power BI semantic model connects to a Fabric Lakehouse using Direct Lake mode:

```text
Analyst               Power BI Direct Lake (Analysis Services) OneLake Storage Security
   │                                  │                                  │
   │─── 1. Opens Report ─────────────►│                                  │
   │                                  │─── 2. Read Delta Parquet ───────►│ (Direct storage query)
   │                                  │    (SSO Token Delegation)        │
   │                                  │                                  │─── 3. Evaluate OneLake Roles
   │                                  │                                  │    - Path Filters (/Tables/{table})
   │                                  │                                  │    - OneLake RLS Predicates
   │                                  │                                  │    - OneLake Column Masks
   │                                  │◄── 4. Permitted Parquet Columns ─│
   │◄── 5. Render Visual ─────────────│
```

!> **Warning:** **Direct Lake Identity Delegation & Fixed-Identity Risks**: In Microsoft Fabric, Direct Lake semantic models operate under Single Sign-On (SSO) by default, passing the active viewer's Microsoft Entra ID token to evaluate OneLake Data Access Roles. However, if the semantic model connection is modified to use a **Fixed Identity** (such as a shared connection or model owner credentials in **Semantic Model Settings > Gateway and Cloud Connections**), Analysis Services queries OneLake as that fixed identity (often an administrative account), **bypassing all OneLake Data Access Roles** configured via `fabricext_lakehouse_permission` for report viewers. Maintain Single Sign-On on the Direct Lake connection whenever granular OneLake roles must govern end-user data visibility.

### OneLake Enforcement Rules

1. **Storage-Level Access**: Analysis Services reads Delta Lake Parquet metadata and columnar files directly from OneLake storage without passing through a SQL query engine.
2. **OneLake Role Enforcement**: OneLake evaluates the user's Entra identity against the Lakehouse's **OneLake Data Access Roles** (configured via `fabricext_lakehouse_permission`).
3. **No DirectQuery Fallback on OneLake**: Direct Lake on OneLake operates exclusively in `DirectLakeOnly` mode. If a query encounters unsupported operations, guardrail violations, or security incompatibilities, **the query fails with an error**. DirectQuery fallback is not supported in this mode because the model binds directly to storage files rather than a relational TDS endpoint.

---

## Fixed Identity Direct Lake

If the semantic model uses a **Fixed Identity** (such as a Service Principal or shared credentials) rather than Single Sign-On:

1. **Data Engine Opacity**: The underlying Warehouse or Lakehouse sees only the Service Principal's identity. All queries run with the broad privileges granted to that Service Principal.
2. **Application-Layer Defense**: In this model, you **must** configure security directly inside the Power BI semantic model:
   - **Power BI Row-Level Security (RLS)**: Define Data Analysis Expressions (DAX) filter rules on tables (e.g. `[Region] = USERPRINCIPALNAME()`).
   - **Object-Level Security (OLS)**: Secure sensitive tables or columns so unauthorized users cannot reference them.
   - Map Power BI roles to Microsoft Entra security groups in the Power BI service.

---

## Comparison Summary Matrix

| Dimension | Warehouse (DirectQuery SSO) | Lakehouse (Direct Lake SSO) | Fixed Identity (Service Principal) |
| :--- | :--- | :--- | :--- |
| **Data Engine** | SQL TDS Engine (Port 1433) | Analysis Services + OneLake Storage | Semantic Model VertiPaq Engine |
| **Identity Passed** | User's Entra Bearer Token | User's Entra Bearer Token | Service Principal / Shared Credential |
| **Security Layer** | T-SQL RBAC, SQL RLS, SQL CLS | OneLake Data Access Roles | Power BI Dataset RLS / OLS |
| **Direct Lake Support** | Falls back to DirectQuery on RLS/CLS | Supported Natively (`DirectLakeOnly`) | Supported (Fixed Identity Mode) |
| **Fallback Path** | N/A (Executes over TDS) | **None** (Fails if unsupported) | N/A (Cached / Fixed Model) |
| **`fabricext` Role** | `fabricext_warehouse_permission` (`role_type = "read"`) | `fabricext_lakehouse_permission` (OneLake Roles) | Manages Service Principal permissions |

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Use Cases Overview](./use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](./use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Warehouse Schema Isolation](./use_case_warehouse_schema_isolation.md)** | Isolating schemas using `role_type = "read"` and T-SQL RBAC. |
| **[Lakehouse OneLake Security](./use_case_lakehouse_onelake_security.md)** | Granular path filters and storage-level RLS/CLS. |
