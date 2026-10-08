---
page_title: "Use Cases: Warehouse Schema Isolation & Declarative RBAC"
subcategory: "Use Cases"
description: |-
  Technical guide to implementing granular schema isolation in Microsoft Fabric Warehouses using fabricext for item-level connectivity and T-SQL for database RBAC, including Entra ID Object ID binding and declarative schema-as-code automation.
---

# Use Cases: Warehouse Schema Isolation & Declarative RBAC

In enterprise data platforms, multi-tenant databases frequently host datasets across confidential business domains—such as `finance`, `hr`, and `sales`—within the same data warehouse.

This guide demonstrates how to achieve strict schema isolation in Microsoft Fabric Warehouses: granting an identity access to a single schema (e.g. `finance`) while completely hiding and denying access to all other schemas.

---

## The Schema Isolation Problem

By default in Microsoft Fabric:
- Assigning a user or group to a workspace role (`Admin`, `Member`, `Contributor`) confers `db_owner` on all Warehouses in the workspace, bypassing internal database permissions.
- Data analysts requiring access to specific schemas should not hold administrative workspace roles.

To restrict a group to schema `finance` only, you must decouple **item connectivity** from **internal database authorization**.

---

## Technical Mechanics: Role Types in `fabricext`

The `fabricext_warehouse_permission` resource manages item-level sharing for Warehouses:

| Role Type (`role_type`) | Fabric API Action | SQL Engine Effect | Schema Visibility |
| :--- | :--- | :--- | :--- |
| **`read`** *(Recommended)* | `Read` | Grants database connectivity (`CONNECT`) over Tabular Data Stream (TDS, port 1433). | **Strictly Restricted**: User sees only schemas and objects explicitly granted in T-SQL. |
| **`write`** | `Write` | Grants permission to modify item metadata and write tables. | Full read/write access. |
| **`reshare`** | `Reshare` | Grants permission to share the warehouse with other principals. | Shareable access. |

~> **Important:** **Decoupling Connectivity from Data Access**: Specifying `role_type = "read"` in `fabricext_warehouse_permission` is the foundation of schema-level security. It grants database connectivity (`CONNECT`) while leaving the database engine's default-deny security model intact. If updating an existing grant in Terraform from `"write"` to `"read"`, `fabricext` automatically revokes the excess permissions, safely locking down the warehouse in-place.

*(Note: The broad `read_data` item token applies exclusively to Fabric SQL Databases via `fabricext_sql_database_permission`, not Warehouses).*

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

## Microsoft Entra ID Identity Binding: Display Names vs. Object IDs

A frequent practitioner question is how Microsoft Entra ID identities map between Terraform configurations and T-SQL database statements.

### 1. In Terraform (`fabricext` Provider)

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

### 2. In the T-SQL Engine

When provisioning database users over the TDS connection endpoint:

```sql
CREATE USER [SEC-Fabric-Finance-Analysts] FROM EXTERNAL PROVIDER;
```

`[SEC-Fabric-Finance-Analysts]` is the **Microsoft Entra Security Group display name** (or UserPrincipalName for individual users). Upon execution, the SQL engine queries Microsoft Graph to resolve the display name to its internal Entra Object ID and binary Security Identifier (SID).

~> **Note:** **T-SQL Name Resolution**: In T-SQL `CREATE USER [name] FROM EXTERNAL PROVIDER`, `name` must match the Microsoft Entra Display Name or UPN. Passing a raw UUID string inside brackets `[<guid>]` will fail unless the Entra principal's display name literally matches that UUID string.

### Recommended Dual-Plane Pattern

| Plane | Tool | Identity Identifier | Rationale |
| :--- | :--- | :--- | :--- |
| **Item Gate** | Terraform (`fabricext`) | Microsoft Entra **Object ID (UUID)** | Immutable; impervious to Entra group renames in Azure Portal. |
| **SQL Engine** | T-SQL / Schema Operator | Microsoft Entra **Display Name** | Human-readable in DBA query plans, SSMS Object Explorer, and audit logs. |

---

## Enterprise RBAC Automation: Integrating Terraform with Schema Operators

While `fabricext` manages the Fabric item boundary (`CONNECT`), managing internal database schemas, tables, roles, and `GRANT SELECT` statements across dozens of Warehouses is typically automated using a declarative database schema management tool or migration framework (such as [Atlas](https://atlasgo.io/), Flyway, Liquibase, or the Terraform `mssql` provider).

### The End-to-End Governance Model

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
| - CREATE USER [SEC-Fabric-Finance-Analysts] FROM EXTERNAL PROVIDER             |
| - CREATE SCHEMA finance; CREATE SCHEMA hr; CREATE SCHEMA sales;               |
| - GRANT SELECT ON SCHEMA::finance TO [SEC-Fabric-Finance-Analysts];            |
+-------------------------------------------------------------------------------+
```

### Conceptual Blueprint: Terraform + `fabricext` + Atlas

In this architecture, Terraform provisions the Entra group, Fabric workspace, and warehouse item. `fabricext` grants item connectivity to the Entra group, and a declarative schema operator configures internal database security:

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

# --- 3. Declarative SQL RBAC via Atlas (Conceptual Pseudocode) ---
# Note: This is generic conceptual pseudocode representing a database
# schema-as-code operator managing SQL objects over the TDS endpoint.
resource "atlas_resource" "warehouse_rbac" {
  target_database = fabric_warehouse.corporate_dw.display_name
  schema_file     = "./schemas/warehouse_security.hcl"

  depends_on = [
    fabricext_warehouse_permission.finance_access
  ]
}
```

---

## Fabric Console Experience for Non-Workspace Members

When an Entra security group receives `read` via `fabricext_warehouse_permission` without workspace membership:

1. **OneLake Data Hub**: The Warehouse appears under the **"Shared with me"** tab and in the OneLake Data Hub catalog.
2. **Workspace Navigation**: The user **cannot** see the parent workspace in the workspace flyout menu.
3. **Web Query Editor**: Clicking the Warehouse opens the Web Query Editor. The object explorer renders only the schemas and tables that the user's Entra credentials have SQL permissions to view (metadata hiding). Schemas without `GRANT` permissions do not appear in the tree.
4. **Connection Endpoints**: The user can copy the TDS connection string (`*.datawarehouse.fabric.microsoft.com`) and connect via SSMS, Azure Data Studio, VS Code, Python, or DBeaver.

---

## Related Guides

- **[Use Cases Overview](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_overview)**: Master comparison matrix and 5-layer perimeter overview.
- **[Security Controls Inventory & Interaction Matrix](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_controls_and_interactions)**: Precedence rules and override behaviors across tiers.
- **[Lakehouse OneLake Security & Data Access Roles](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_lakehouse_onelake_security)**: Granular path filters and storage-level RLS/CLS.
- **[Power BI Identity Flow: Direct Lake vs. DirectQuery](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_powerbi_identity_propagation)**: End-to-end token flow to semantic models.
