---
page_title: "Warehouse Schema Isolation"
subcategory: "Guides"
description: |-
  Technical guide to implementing granular schema isolation in Microsoft Fabric Warehouses using Fabric web portal for item connectivity and T-SQL for database RBAC, including Entra ID identity binding and declarative schema-as-code automation.
---

# Warehouse Schema Isolation

In enterprise data platforms, multi-tenant databases frequently host datasets across confidential business domains—such as `finance`, `hr`, and `sales`—within the same data warehouse.

This guide demonstrates how to achieve strict schema isolation in Microsoft Fabric Warehouses: granting an identity access to a single schema (e.g. `finance`) while completely hiding and denying access to all other schemas.

~> **Note:** **Declarative Item Sharing**: Item-level sharing for Warehouses can be declaratively managed in Terraform using `fabricext_warehouse_permission` and verified via `fabricext_item_share_gate`. Fine-grained data access inside the warehouse is implemented using in-database T-SQL permissions (`GRANT`/`REVOKE`/`DENY`). Refer to the [Access Control Playbook](./access_control_playbook.md) for full scenario walkthroughs.

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

Item-level connectivity is declaratively provisioned in Terraform using `fabricext_warehouse_permission`:

```hcl
resource "fabricext_warehouse_permission" "finance_access" {
  workspace_id   = var.workspace_id
  warehouse_id   = var.warehouse_id
  principal_id   = var.finance_analysts_group_id
  principal_type = "Group"
  permissions    = ["Read"]
}

# Optional verification gate ensuring Fabric Admin API sees the effective grant:
resource "fabricext_item_share_gate" "finance_access" {
  workspace_id         = var.workspace_id
  item_id              = var.warehouse_id
  principal_id         = var.finance_analysts_group_id
  principal_type       = "Group"
  expected_permissions = ["Read"]

  depends_on = [fabricext_warehouse_permission.finance_access]
}
```

~> **Important:** **Service Principal Object ID vs Client ID**: When granting item permissions to a Service Principal (`azuread_service_principal`), always supply the Enterprise Application Directory **Object ID** (`azuread_service_principal.*.object_id`) to `principal_id`, **never** the Application / Client ID (`azuread_service_principal.*.client_id`). Microsoft Fabric metadata services index permissions strictly by directory object UUID.

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
| **Item Gate** | `fabricext_warehouse_permission` or Portal | Microsoft Entra **Object ID (UUID)** (IaC) / **UPN** (Portal) | Declarative `fabricext_warehouse_permission` or portal share grants item connectivity (`CONNECT`). |
| **SQL Engine** | T-SQL / Schema Operator | Microsoft Entra **Display Name / UPN** | Human-readable in DBA query plans, SSMS Object Explorer, and audit logs. |

---

## Declarative RBAC Automation

While item connectivity is established declaratively via `fabricext_warehouse_permission` (or the Fabric web portal), managing internal database schemas, tables, roles, and `GRANT SELECT` statements across dozens of Warehouses is typically automated using a declarative database schema management tool or migration framework (such as [Atlas](https://atlasgo.io/), Flyway, Liquibase, or sqlcmd).

### End-to-End Governance Model

```text
+-------------------------------------------------------------------------------+
| 1. Infrastructure Orchestration (Terraform & fabricext)                       |
| - azuread_group: Creates Entra ID Security Group                              |
| - microsoft/fabric: Provisions Workspace and Warehouse items                  |
| - fabricext_warehouse_permission: Declarative 'Read' share (CONNECT)          |
| - fabricext_item_share_gate: Verifies effective ACLs via Fabric Admin API    |
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

~> **Important:** **TDS Authentication & Engine Invariants**: Fabric Warehouse TDS endpoints (port 1433) **only support Microsoft Entra ID authentication**; standard SQL username/password logins are unsupported. The CI/CD identity running the schema operator must authenticate via Entra ID (CLI session, service principal, or workload identity) and hold administrative privileges (`db_owner` / workspace Contributor or Admin). Furthermore, Fabric Warehouse operates as a distributed Massively Parallel Processing (MPP) query engine: schema-as-code operators must execute in autocommit mode without issuing `USE [database];` statements (database switching over TDS is unsupported) or wrapping DDL in unsupported multi-statement transactions. Standard Terraform `mssql` providers that issue `USE` or multi-statement transactions will fail against Fabric Warehouse endpoints.

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

# --- 2. Declarative Item-Level Gate via fabricext ---
resource "fabricext_warehouse_permission" "finance_share" {
  workspace_id   = fabric_workspace.analytics.id
  warehouse_id   = fabric_warehouse.corporate_dw.id
  principal_id   = azuread_group.finance_analysts.object_id
  principal_type = "Group"
  permissions    = ["Read"]
}

resource "fabricext_item_share_gate" "finance_gate" {
  workspace_id         = fabric_workspace.analytics.id
  item_id              = fabric_warehouse.corporate_dw.id
  principal_id         = azuread_group.finance_analysts.object_id
  principal_type       = "Group"
  expected_permissions = ["Read"]

  depends_on = [fabricext_warehouse_permission.finance_share]
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
    fabricext_item_share_gate.finance_gate
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
