---
page_title: "Use Cases: Power BI Identity Flow & Direct Lake vs. DirectQuery"
subcategory: "Use Cases"
description: |-
  How Microsoft Fabric security controls propagate to Power BI reports and semantic models, contrasting DirectQuery over Warehouse TDS endpoints with Direct Lake over Lakehouse Parquet files.
---

# Use Cases: Power BI Identity Flow & Direct Lake vs. DirectQuery

A frequent architectural question in Microsoft Fabric enterprise deployments is how item and engine permissions propagate into Power BI reports, semantic models, and Direct Lake queries.

This guide explains the end-to-end identity flow and security enforcement mechanisms across Fabric Warehouses and Lakehouses.

---

## Warehouses vs. Lakehouses: Power BI Query Modes

A critical architectural distinction governs reporting in Microsoft Fabric:

```text
+─────────────────────────────────────────+     +─────────────────────────────────────────+
│ Fabric Warehouse                        │     │ Fabric Lakehouse                        │
│ ├─ Storage: OneLake Delta (Parquet)     │     │ ├─ Storage: OneLake Delta (Parquet)     │
│ ├─ Engine: SQL Analytics (TDS 1433)     │     │ ├─ Engine: OneLake Storage Security     │
│ ├─ Direct Lake: Supported on Base Data  │     │ ├─ Direct Lake: Supported Natively      │
│ └─ Security Enforcement: T-SQL RBAC     │     │ └─ Security Enforcement: OneLake Roles  │
│    (Falls back to DirectQuery on RLS)   │        (Direct Parquet query via VertiPaq)    │
+─────────────────────────────────────────+     +─────────────────────────────────────────+
```

Both Microsoft Fabric Warehouses and Lakehouses persist data in OneLake in open Delta Lake (Parquet) format with V-Order optimization, and both support Direct Lake default semantic models.

However, a fundamental architectural difference exists in **how security is enforced**:

- **Lakehouses** enforce security at the storage layer using **OneLake Data Access Roles**. Power BI's semantic model engine (Analysis Services / VertiPaq) reads Parquet files directly from OneLake while OneLake evaluates storage-level RLS predicates, CLS column constraints, and path filters under Single Sign-On (SSO).
- **Warehouses** enforce security at the query engine layer using T-SQL RBAC over Tabular Data Stream (TDS, port 1433).
- **Automatic Fallback on SQL Security**: When database-level security—such as T-SQL Row-Level Security (RLS), Column-Level Security (CLS), or Object-Level Security (OLS)—is configured on a Warehouse, Power BI's VertiPaq engine **automatically falls back from Direct Lake to DirectQuery over TDS** so that SQL security policies evaluate dynamically against the querying user's Entra ID identity.

---

## Scenario A: Power BI Querying a Warehouse (DirectQuery with SSO)

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

!> **Warning:** **Power BI Service SSO Configuration Required**: When configuring a semantic model connected to a Warehouse or SQL Analytics endpoint in the Power BI Service, administrators **must check the box**: *"Report viewers can only access this data source with their own Power BI identities using Direct Query"* (Single Sign-On / SSO). If this setting is not enabled, Power BI executes all report queries using the fixed credentials of the dataset owner or gateway service account, completely bypassing user-level T-SQL grants and causing an unintended privilege exposure.

### Identity & Enforcement Rules

1. **Token Delegation**: Power BI's underlying semantic model engine (Analysis Services) passes the active end-user's Microsoft Entra bearer token directly to the Warehouse TDS endpoint (port 1433).
2. **Engine Evaluation**: The Warehouse evaluates the user's security group memberships against database-level permissions (`GRANT SELECT ON SCHEMA::finance`).
3. **Restricted Schemas**: If the report visual references a table in `hr.salaries` and the user only has access to `finance.*`, **the individual visual fails with a permissions error** while the rest of the report renders successfully.
4. **Row-Level Security**: Any T-SQL Security Policies (predicate functions) defined on the Warehouse tables execute dynamically for the querying user.

---

## Scenario B: Power BI Querying a Lakehouse (Direct Lake with OneLake SSO)

When a Power BI semantic model connects to a Fabric Lakehouse using Direct Lake mode:

```text
Analyst               Power BI Direct Lake (Analysis Services) OneLake Storage Security
   │                                  │                                  │
   │─── 1. Opens Report ─────────────►│                                  │
   │                                  │─── 2. Read Delta Parquet ───────►│ (Direct storage query)
   │                                  │    (SSO Token Delegation)        │
   │                                  │                                  │─── 3. Evaluate OneLake Roles
   │                                  │                                  │    - Path Filters (/Tables/*)
   │                                  │                                  │    - OneLake RLS Predicates
   │                                  │                                  │    - OneLake Column Masks
   │                                  │◄── 4. Permitted Parquet Columns ─│
   │◄── 5. Render Visual ─────────────│
```

### Identity & Enforcement Rules

1. **Storage-Level Access**: Analysis Services reads Delta Lake Parquet metadata and columnar files directly from OneLake storage without passing through the SQL query engine.
2. **OneLake Role Enforcement**: OneLake evaluates the user's Entra identity against the Lakehouse's **OneLake Data Access Roles** (configured via `fabricext_lakehouse_permission`).
3. **Fallback to DirectQuery**: If a query encounters operations unsupported by Direct Lake (or if complex security rules mandate query-engine translation), Power BI seamlessly falls back to DirectQuery over the Lakehouse SQL Analytics Endpoint, where T-SQL RBAC governs access.

---

## Scenario C: Fixed Identity (Service Principal) with Dataset RLS/OLS

If the semantic model uses a **Fixed Identity** (such as a Service Principal or shared credentials) rather than Single Sign-On:

1. **Data Engine Opacity**: The underlying Warehouse or Lakehouse sees only the Service Principal's identity. All queries run with the broad privileges granted to that Service Principal.
2. **Application-Layer Defense**: In this model, you **must** configure security directly inside the Power BI semantic model:
   - **Power BI Row-Level Security (RLS)**: Define DAX filter rules on tables (e.g. `[Region] = USERPRINCIPALNAME()`).
   - **Object-Level Security (OLS)**: Secure sensitive tables or columns so unauthorized users cannot reference them.
   - Map Power BI roles to Microsoft Entra security groups in the Power BI service.

---

## Comparison Summary Matrix

| Dimension | Warehouse (DirectQuery SSO) | Lakehouse (Direct Lake SSO) | Fixed Identity (Service Principal) |
| :--- | :--- | :--- | :--- |
| **Data Engine** | SQL TDS Engine (Port 1433) | Analysis Services + OneLake Storage | Semantic Model VertiPaq Engine |
| **Identity Passed** | User's Entra Bearer Token | User's Entra Bearer Token | Service Principal / Shared Credential |
| **Security Layer** | T-SQL RBAC, SQL RLS, SQL CLS | OneLake Data Access Roles | Power BI Dataset RLS / OLS |
| **Direct Lake Support** | Falls back to DirectQuery on RLS/CLS | Supported Natively | Supported (Fixed Identity Mode) |
| **Fallback Path** | N/A (Executes over TDS) | DirectQuery over SQL Analytics Endpoint | N/A (Cached / Fixed Model) |
| **`fabricext` Role** | `fabricext_warehouse_permission` (`role_type = "read"`) | `fabricext_lakehouse_permission` (OneLake Roles) | Manages Service Principal permissions |

---

## Related Guides

- **[Use Cases Overview](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_overview)**: Master comparison matrix and 5-layer perimeter overview.
- **[Security Controls Inventory & Interaction Matrix](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_controls_and_interactions)**: Precedence rules and override behaviors across tiers.
- **[Warehouse Schema Isolation & Declarative RBAC](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_warehouse_schema_isolation)**: Isolating schemas using `role_type = "read"` and T-SQL.
- **[Lakehouse OneLake Security & Data Access Roles](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_lakehouse_onelake_security)**: Granular path filters and storage-level RLS/CLS.
