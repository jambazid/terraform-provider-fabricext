---
page_title: "Warehouse Schema Isolation"
subcategory: "Guides"
description: |-
  Technical guide to implementing granular schema isolation in Microsoft Fabric Warehouses using Fabric web portal for item connectivity and T-SQL for database RBAC, including Entra ID identity binding and declarative schema-as-code automation.
---

# Warehouse Schema Isolation

In enterprise data platforms, multi-tenant databases frequently host datasets across confidential business domains—such as `finance`, `hr`, and `sales`—within the same data warehouse.

This guide demonstrates how to achieve strict schema isolation in Microsoft Fabric Warehouses: granting an identity access to a single schema (e.g. `finance`) while completely hiding and denying access to all other schemas.

!> **Important:** **Item Sharing via Portal UI**: Declarative item sharing via REST (`fabricext_warehouse_permission`) was removed in provider v0.5.0 because Microsoft Fabric does not offer a public item-sharing REST endpoint for Warehouses. Item-level sharing must be configured using the Fabric web portal interface, and fine-grained data access should be implemented using in-database T-SQL permissions (`GRANT`/`REVOKE`/`DENY`). Refer to the [Access Control Playbook](./access_control_playbook.md) for full scenario walkthroughs.

---

## Schema Isolation Problem

By default in Microsoft Fabric:
- Assigning a user or group to a workspace role (`Admin`, `Member`, `Contributor`) confers `db_owner` (`CONTROL`) on all Warehouses in the workspace, bypassing internal database permissions.
- Data analysts requiring access to specific schemas should not hold administrative workspace roles.

To restrict a group to schema `finance` only, you must decouple **item connectivity** from **internal database authorization**.

---

## Role Types Mechanics

Item-level sharing for Warehouses provides the connectivity gate:

| Portal Permission Option | Fabric Action | SQL Engine Effect | Schema Visibility |
| :--- | :--- | :--- | :--- |
| **`Read`** *(Recommended)* | `Read` | Grants database connectivity (`CONNECT`) over Tabular Data Stream (TDS, port 1433). | **Strictly Restricted**: User sees only schemas and objects explicitly granted in T-SQL. |
| **`ReadData`** | `ReadData` | Confers `db_datareader` across all tables and schemas. | Full table read access across database. |
| **`ReadAll`** | `ReadAll` | Grants read access across OneLake shortcuts and Spark. | Storage and engine access. |

~> **Important:** **Decoupling Connectivity from Data Access**: Specifying **Read** in the Fabric portal share dialog is the foundation of schema-level security. It grants database connectivity (`CONNECT`) while leaving the database engine's default-deny security model intact.

~> **Note:** **Warehouse Permission Tokens**: In Microsoft Fabric Data Warehouse, users are created implicitly upon executing a T-SQL `GRANT` statement; running explicit `CREATE USER` statements is unsupported and returns an error.

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

~> **Important:** Declarative management via `fabricext_warehouse_permission` was removed in v0.5.0. Existing Terraform state can be safely detached using Terraform 1.7+ `removed` blocks:

```hcl
removed {
  from = fabricext_warehouse_permission.finance_access

  lifecycle {
    destroy = false
  }
}
```

Item connectivity should be granted via the Fabric web portal using the Microsoft Entra Object ID (UUID) or display name.

### In T-SQL Engine

In Microsoft Fabric Data Warehouse, users are created implicitly upon granting permissions. Running `CREATE USER ... FROM EXTERNAL PROVIDER` in a Warehouse is unsupported and returns an error:

```sql
-- For Microsoft Entra Security Groups: Use the exact Display Name in GRANT
GRANT SELECT ON SCHEMA::finance TO [SEC-Fabric-Finance-Analysts];

-- For Individual Users: Strictly require the UserPrincipalName (UPN) in GRANT
GRANT SELECT ON SCHEMA::finance TO [bob@contoso.com];

-- For B2B Guest Users: Use the transformed external UPN
GRANT SELECT ON SCHEMA::finance TO [external_user#EXT#@tenant.onmicrosoft.com];
```

~> **Note:** Explicit `CREATE USER [name] FROM EXTERNAL PROVIDER` is supported in **Microsoft Fabric SQL Database**, but not in Fabric Data Warehouse.

**Identity Resolution Rules & Caveats**:
- **Display Names vs. UPNs**: Microsoft Entra Security Groups resolve by `displayName`. Individual user accounts **strictly require the UserPrincipalName (UPN)**.
- **Why Bracketed UUIDs Fail**: The SQL engine matches Entra directory objects by name/UPN, not raw directory UUIDs.
- **Identifier Escaping**: Group display names containing closing brackets (e.g. `SG-Data[Finance]-US`) must escape the bracket as `]]` in T-SQL.

### Recommended Dual-Plane Pattern

| Plane | Tool | Identity Identifier | Rationale |
| :--- | :--- | :--- | :--- |
| **Item Gate** | Fabric Web Portal (REST unsupported) | Microsoft Entra **Display Name / UPN** | Interactive share dialog in Fabric Portal grants item connectivity (`CONNECT`). |
| **SQL Engine** | T-SQL / Schema Operator | Microsoft Entra **Display Name / UPN** | Human-readable in DBA query plans, SSMS Object Explorer, and audit logs. |

---

## Declarative RBAC Automation

While item connectivity is established via the Fabric web portal share dialog, managing internal database schemas, tables, roles, and `GRANT SELECT` statements across dozens of Warehouses is typically automated using a declarative database schema management tool or migration framework (such as [Atlas](https://atlasgo.io/), Flyway, Liquibase, or the Terraform `mssql` provider).

### End-to-End Governance Model

```text
+-------------------------------------------------------------------------------+
| 1. Infrastructure Orchestration (Terraform & Fabric Portal)                   |
| - azuread_group: Creates Entra ID Security Group                              |
| - microsoft/fabric: Provisions Workspace and Warehouse items                  |
| - Fabric Portal: Shares Warehouse item with 'Read' connectivity (CONNECT)     |
+-------------------------------------------------------------------------------+
                                      │
                                      ▼ TDS Connection Endpoint (Port 1433)
+-------------------------------------------------------------------------------+
| 2. Declarative Schema & RBAC Management (Atlas / Schema Operator)             |
| - Authenticates via Microsoft Entra token (TDS does not support SQL logins)   |
| - CI/CD Identity holds db_owner / workspace Contributor rights                |
| - CREATE SCHEMA finance; CREATE SCHEMA hr; CREATE SCHEMA sales;               |
| - GRANT SELECT ON SCHEMA::finance TO [SEC-Fabric-Finance-Analysts];            |
+-------------------------------------------------------------------------------+
```

### Architecture Blueprint

~> **Important:** **TDS Authentication & Privileges**: Fabric Warehouse TDS endpoints (port 1433) **only support Microsoft Entra ID authentication**; standard SQL username/password logins are unsupported. The CI/CD identity running the schema operator must authenticate via Entra ID (CLI session, service principal, or workload identity) and hold administrative privileges (`db_owner` / workspace Contributor or Admin) to create users, schemas, and grant permissions. Fabric Warehouse operates as a Massively Parallel Processing (MPP) distributed query engine with a specific T-SQL subset.

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

# --- 2. Item-Level Gate via Fabric Web Portal ---
# In the Fabric web portal, share the warehouse with SEC-Fabric-Finance-Analysts
# granting 'Read' connectivity (CONNECT) without administrative workspace rights.

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

When an Entra security group receives **Read** connectivity via the Fabric web portal without workspace membership:

1. **OneLake Data Hub**: The Warehouse appears under the **"Shared with me"** tab and in the OneLake Data Hub catalog.
2. **Workspace Navigation**: The user **cannot** see the parent workspace in the workspace flyout menu.
3. **Web Query Editor**: Clicking the Warehouse opens the Web Query Editor. The object explorer renders only the schemas and tables that the user's Entra credentials have SQL permissions to view (metadata hiding). Schemas without `GRANT` permissions do not appear in the tree.
4. **Connection Endpoints**: The user can copy the TDS connection string (`*.datawarehouse.fabric.microsoft.com`) and connect via any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, VS Code, Python, or DBeaver).

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Access Control Playbook](./access_control_playbook.md)** | Start Here routing table and 11 verified scenario execution cards. |
| **[Enterprise Access Control Overview](./use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](./use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Lakehouse OneLake Security](./use_case_lakehouse_onelake_security.md)** | Granular path filters and storage-level RLS/CLS. |
| **[Power BI Identity Flow](./use_case_powerbi_identity_propagation.md)** | End-to-end token flow to semantic models. |
