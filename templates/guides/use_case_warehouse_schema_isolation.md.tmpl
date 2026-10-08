---
page_title: "Use Cases: Warehouse Schema Isolation"
subcategory: "Use Cases"
description: |-
  Technical guide to implementing granular schema isolation in Microsoft Fabric Warehouses using fabricext for item connectivity and T-SQL for database RBAC, including Entra ID identity binding and declarative schema-as-code automation.
---

# Warehouse Schema Isolation

In enterprise data platforms, multi-tenant databases frequently host datasets across confidential business domains—such as `finance`, `hr`, and `sales`—within the same data warehouse.

This guide demonstrates how to achieve strict schema isolation in Microsoft Fabric Warehouses: granting an identity access to a single schema (e.g. `finance`) while completely hiding and denying access to all other schemas.

---

## Schema Isolation Problem

By default in Microsoft Fabric:
- Assigning a user or group to a workspace role (`Admin`, `Member`, `Contributor`) confers `db_owner` on all Warehouses in the workspace, bypassing internal database permissions.
- Data analysts requiring access to specific schemas should not hold administrative workspace roles.

To restrict a group to schema `finance` only, you must decouple **item connectivity** from **internal database authorization**.

---

## Role Types Mechanics

The `fabricext_warehouse_permission` resource manages item-level sharing for Warehouses:

| Role Type (`role_type`) | Fabric API Action | SQL Engine Effect | Schema Visibility |
| :--- | :--- | :--- | :--- |
| **`read`** *(Recommended)* | `Read` | Grants database connectivity (`CONNECT`) over Tabular Data Stream (TDS, port 1433). | **Strictly Restricted**: User sees only schemas and objects explicitly granted in T-SQL. |
| **`write`** | `Write` | Grants permission to modify item metadata and write tables. | Full read/write access. |
| **`reshare`** | `Reshare` | Grants permission to share the warehouse with other principals. | Shareable access. |

~> **Important:** **Decoupling Connectivity from Data Access**: Specifying `role_type = "read"` in `fabricext_warehouse_permission` is the foundation of schema-level security. It grants database connectivity (`CONNECT`) while leaving the database engine's default-deny security model intact. If updating an existing grant in Terraform from `"write"` to `"read"`, `fabricext` automatically revokes the excess permissions, safely locking down the warehouse in-place.

~> **Note:** **Warehouse Permission Tokens**: Microsoft Fabric Warehouses natively support `ReadData` (`db_datareader`) and `ReadAll` at the platform level. `fabricext_warehouse_permission` intentionally restricts `role_type` to `read`, `write`, and `reshare` as an opinionated design choice to prevent accidental database-wide access grants and preserve fine-grained T-SQL schema isolation.

---

## Query Authorization Flow

When an identity connects to a Fabric Warehouse with item `read` permissions:

```text
Analyst (Entra Group)       Fabric Portal / Hub         Fabric Item Security       Warehouse SQL Engine (TDS)
         │                           │                            │                            │
         │─── 1. Access Data Hub ───►│                            │                            │
         │                           │─── 2. Query Shared Items ─►│                            │
         │                           │◄── 3. Item List (Read) ────│                            │
         │◄── 4. "Shared with me" ───│                            │                            │
         │                                                        │                            │
         │─── 5. Connect via TDS Endpoint (Port 1433) ────────────────────────────────────────►│
         │                                                        │                            │
         │                                                        │◄── 6. Validate Token ──────│
         │                                                        │─── 7. "Read" (CONNECT OK) ─►
         │                                                        │                            │
         │─── 8. SELECT * FROM hr.salaries; ──────────────────────────────────────────────────►│
         │◄── 9. Error 229: SELECT permission denied on object 'salaries', schema 'hr' ────────┤
         │                                                                                     │
         │─── 10. SELECT * FROM finance.revenue; ─────────────────────────────────────────────►│
         │◄── 11. 200 OK (Data returned via T-SQL GRANT) ──────────────────────────────────────┤
```

---

## Entra ID Identity Binding

A frequent practitioner question is how Microsoft Entra ID identities map between Terraform configurations and T-SQL database statements.

### In Terraform Providers

`fabricext_warehouse_permission` strictly requires the **36-character Microsoft Entra Object ID (UUID)**:

```hcl
resource "fabricext_warehouse_permission" "finance_access" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  warehouse_id   = "00000000-0000-0000-0000-000000000002"
  principal_id   = azuread_group.finance_analysts.object_id # Immutable UUID
  principal_type = "Group"
  role_type      = "read" # Connectivity only
}
```

The Microsoft Fabric REST API expects the Entra Object ID. Passing display names in `principal_id` will fail UUID validation.

### In T-SQL Engine

When provisioning database users over the TDS connection endpoint:

```sql
-- For Microsoft Entra Security Groups: Use the exact Display Name
CREATE USER [SEC-Fabric-Finance-Analysts] FROM EXTERNAL PROVIDER;

-- For Individual Users: Strictly require the UserPrincipalName (UPN)
CREATE USER [bob@contoso.com] FROM EXTERNAL PROVIDER;

-- For B2B Guest Users: Use the transformed external UPN
CREATE USER [external_user#EXT#@tenant.onmicrosoft.com] FROM EXTERNAL PROVIDER;
```

**Identity Resolution Rules & Caveats**:
- **Display Names vs. UPNs**: Entra Security Groups and Enterprise Applications resolve by `displayName`. Individual user accounts **strictly require the UserPrincipalName (UPN)**. Passing a user's display name (`CREATE USER [Bob Smith] FROM EXTERNAL PROVIDER`) fails with `Msg 33130`.
- **Why Bracketed UUIDs Fail**: When `CREATE USER [name] FROM EXTERNAL PROVIDER` runs, the SQL engine queries Microsoft Graph filtering strictly on `displayName eq '<name>'` (for groups) or `userPrincipalName eq '<name>'` (for users). It does not query `id eq '<guid>'`. Passing a raw UUID `[00000000-0000-...]` fails unless the directory object's display name or UPN literally matches that UUID string.
- **Duplicate Display Names & `WITH OBJECT_ID`**: Microsoft Entra ID permits duplicate group display names. If ambiguous display names exist in a tenant, SQL fails with `Msg 33131`. Resolve this using the official syntax:
  ```sql
  CREATE USER [Finance-Analysts-Alias] FROM EXTERNAL PROVIDER WITH OBJECT_ID = '11111111-1111-1111-1111-111111111111';
  ```
- **Identifier Escaping**: Group display names containing closing brackets (e.g. `SG-Data[Finance]-US`) must escape the bracket as `]]` in T-SQL (`CREATE USER [SG-Data[Finance]]-US] FROM EXTERNAL PROVIDER;`).

### Recommended Dual-Plane Pattern

| Plane | Tool | Identity Identifier | Rationale |
| :--- | :--- | :--- | :--- |
| **Item Gate** | Terraform (`fabricext`) | Microsoft Entra **Object ID (UUID)** | Immutable; impervious to Entra group renames in Azure Portal. |
| **SQL Engine** | T-SQL / Schema Operator | Microsoft Entra **Display Name / UPN** | Human-readable in DBA query plans, SSMS Object Explorer, and audit logs. |

---

## Declarative RBAC Automation

While `fabricext` manages the Fabric item boundary (`CONNECT`), managing internal database schemas, tables, roles, and `GRANT SELECT` statements across dozens of Warehouses is typically automated using a declarative database schema management tool or migration framework (such as [Atlas](https://atlasgo.io/), Flyway, Liquibase, or the Terraform `mssql` provider).

### End-to-End Governance Model

```text
+-------------------------------------------------------------------------------+
| 1. Infrastructure Orchestration (Terraform)                                   |
| - azuread_group: Creates Entra ID Security Group                              |
| - microsoft/fabric: Provisions Workspace and Warehouse items                  |
| - fabricext_warehouse_permission: Grants item 'read' connectivity (CONNECT)   |
+-------------------------------------------------------------------------------+
                                      │
                                      ▼ TDS Connection Endpoint (Port 1433)
+-------------------------------------------------------------------------------+
| 2. Declarative Schema & RBAC Management (Atlas / Schema Operator)             |
| - Authenticates via Microsoft Entra token (TDS does not support SQL logins)   |
| - CI/CD Identity holds db_owner / workspace Contributor rights                |
| - CREATE USER [SEC-Fabric-Finance-Analysts] FROM EXTERNAL PROVIDER             |
| - CREATE SCHEMA finance; CREATE SCHEMA hr; CREATE SCHEMA sales;               |
| - GRANT SELECT ON SCHEMA::finance TO [SEC-Fabric-Finance-Analysts];            |
+-------------------------------------------------------------------------------+
```

### Architecture Blueprint

~> **Important:** **TDS Authentication & Privileges**: Fabric Warehouse TDS endpoints (port 1433) **only support Microsoft Entra ID authentication**; standard SQL username/password logins are unsupported. The CI/CD identity running the schema operator must authenticate via Entra ID (CLI session, service principal, or workload identity) and hold administrative privileges (`db_owner` / workspace Contributor or Admin) to create users, schemas, and grant permissions. Fabric Warehouse operates as an MPP distributed query engine with a specific T-SQL subset.

```hcl
# --- 1. Identity & Workspace Provisioning ---
resource "azuread_group" "finance_analysts" {
  display_name     = "SEC-Fabric-Finance-Analysts"
  security_enabled = true
}

resource "fabric_workspace" "analytics" {
  display_name = "Enterprise Analytics"
}

resource "fabric_warehouse" "corporate_dw" {
  workspace_id = fabric_workspace.analytics.id
  display_name = "corporate_dw"
}

# --- 2. Item-Level Gate via fabricext ---
# Grants TDS connectivity (CONNECT) without administrative workspace rights
resource "fabricext_warehouse_permission" "finance_access" {
  workspace_id   = fabric_workspace.analytics.id
  warehouse_id   = fabric_warehouse.corporate_dw.id
  principal_id   = azuread_group.finance_analysts.object_id
  principal_type = "Group"
  role_type      = "read"
}

# --- 3. Declarative SQL RBAC via Schema Operator (Conceptual Example) ---
# Note: Generic conceptual pseudocode representing a database
# schema-as-code operator (e.g. Atlas, Flyway) managing SQL objects over TDS.
# The schema operator authenticates via Microsoft Entra token.
resource "atlas_schema" "warehouse_rbac" {
  # Connects over TDS (port 1433) using Microsoft Entra authentication
  url = "sqlserver://${fabric_warehouse.corporate_dw.properties.connection_info.endpoint_fqdn}:1433?database=${fabric_warehouse.corporate_dw.display_name}&fedauth=ActiveDirectoryDefault"
  src = file("${path.module}/schemas/warehouse_security.hcl")

  depends_on = [
    fabric_warehouse.corporate_dw
  ]
}
```

---

## Non-Member Console Experience

When an Entra security group receives `read` via `fabricext_warehouse_permission` without workspace membership:

1. **OneLake Data Hub**: The Warehouse appears under the **"Shared with me"** tab and in the OneLake Data Hub catalog.
2. **Workspace Navigation**: The user **cannot** see the parent workspace in the workspace flyout menu.
3. **Web Query Editor**: Clicking the Warehouse opens the Web Query Editor. The object explorer renders only the schemas and tables that the user's Entra credentials have SQL permissions to view (metadata hiding). Schemas without `GRANT` permissions do not appear in the tree.
4. **Connection Endpoints**: The user can copy the TDS connection string (`*.datawarehouse.fabric.microsoft.com`) and connect via SSMS, Azure Data Studio, VS Code, Python, or DBeaver.

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Use Cases Overview](./use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](./use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Lakehouse OneLake Security](./use_case_lakehouse_onelake_security.md)** | Granular path filters and storage-level RLS/CLS. |
| **[Power BI Identity Flow](./use_case_powerbi_identity_propagation.md)** | End-to-end token flow to semantic models. |
