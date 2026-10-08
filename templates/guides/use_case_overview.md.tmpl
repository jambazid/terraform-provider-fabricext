---
page_title: "Use Cases: Enterprise Access Control & Governance Overview"
subcategory: "Use Cases"
description: |-
  Overview of enterprise access control patterns in Microsoft Fabric, including the 5-layer security perimeter stack and common access goals vs. architectural solutions.
---

# Use Cases: Enterprise Access Control & Governance Overview

This architectural guide outlines how enterprise organizations implement least-privilege access control, granular schema isolation, and data governance in Microsoft Fabric using [`terraform-provider-fabricext`](https://registry.terraform.io/providers/jambazid/fabricext/latest).

---

## The 5-Layer Security Perimeter Stack

Security in Microsoft Fabric is governed by five decoupled, cascading evaluation perimeters. A principal must clear each layer sequentially to read an underlying data element:

```text
+--------------------------------------------------------------------------+
| Layer 1: Fabric Tenant & Domain Boundary                                 |
| - Capacity licensing & assignments                                       |
| - Tenant switch overrides & external sharing controls                     |
| - Microsoft Entra ID authentication & conditional access                 |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Layer 2: Workspace Boundary                                              |
| - Roles: Admin, Member, Contributor (db_owner / administrative rights)   |
| - Roles: Viewer (read-only discovery) or Non-Member (pure item share)    |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Layer 3: Item-Level Sharing Boundary (fabricext)                         |
| - Warehouse: 'read' (CONNECT to SQL endpoint), 'write', 'reshare'        |
| - SQL Database: 'read', 'read_data', 'write', 'reshare'                  |
| - Lakehouse: OneLake Data Access Roles (path & action scopes)            |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Layer 4: Data Engine Enforcement                                         |
| ├─ Warehouse / SQL DB: T-SQL Permissions Engine                          |
| │  - GRANT / DENY on Schema, Table, View                                 |
| │  - T-SQL Row-Level Security (RLS) & Column-Level Security (CLS)        |
| └─ Lakehouse: OneLake Storage Security Engine                            |
|    - Path attribute filters (/Tables/sales/*)                            |
|    - OneLake RLS row constraints & CLS column constraints                |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Layer 5: Semantic & BI Consumption (Power BI)                            |
| - Direct Lake on OneLake Parquet (High-performance direct queries)       |
| - DirectQuery Fallback over TDS (Evaluates T-SQL RLS/CLS dynamically)    |
| - Semantic Model: Power BI Row-Level & Object-Level Security (RLS/OLS)   |
+--------------------------------------------------------------------------+
```

!> **Warning:** **Workspace Role Permission Inheritance**: Users assigned **Admin**, **Member**, or **Contributor** roles on a Fabric workspace are automatically granted `db_owner` / administrative privileges on all Warehouses and Lakehouses in that workspace. T-SQL `DENY` statements, SQL Row-Level Security, and OneLake Data Access Roles **have no effect** on Admins, Members, or Contributors. To enforce schema, object, or row-level security, users should have **no workspace role** (pure item share) or at most the **Viewer** workspace role.

---

## Common Access Goals & Security Solutions

The following matrix maps enterprise access control requirements to their Microsoft Fabric architectural solutions, highlighting where `terraform-provider-fabricext` enables declarative Infrastructure as Code (IaC):

| Access Goal / Scenario | Recommended Solution | Mechanism & Boundary | `fabricext` Provider Role |
| :--- | :--- | :--- | :--- |
| **Broad Workspace Access**<br/>User needs to read or collaborate on all items across a workspace. | Assign **Viewer** (read-only) or **Contributor** (read/write) workspace role. | **Layer 2 (Workspace)**<br/>Workspace role assignment via Fabric Portal or `microsoft/fabric`. | **Out of Scope**<br/>Handled by official `fabric_workspace_role_assignment`. |
| **Warehouse Schema Isolation**<br/>User needs access to schema `finance` only; all other schemas (`hr`, `sales`) must be denied. | Grant item **`read`** (connectivity only) + create Entra DB user + grant T-SQL schema permissions. | **Layer 3 (Item Share) + Layer 4 (SQL Engine)**<br/>Item share grants Tabular Data Stream (TDS) `CONNECT`; default-deny SQL engine restricts data access. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`) grants TDS connection without granting `db_owner` rights. |
| **SQL Database Broad Access**<br/>User needs read access across all tables in a Fabric SQL Database. | Grant item-level **`read_data`** share on the SQL database item. | **Layer 3 (Item Share)**<br/>Item permission grants connectivity and broad data access across all tables. | **Direct**<br/>`fabricext_sql_database_permission` (`role_type = "read_data"`). |
| **Object-Level Security (OLS)**<br/>User can access specific tables or views within a schema, but sensitive tables are hidden. | Item **`read`** + T-SQL `GRANT SELECT ON table/view`. | **Layer 4 (SQL Engine)**<br/>SQL metadata hiding ensures unpermitted tables are invisible in `sys.tables`. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`). |
| **SQL Row/Column Security (RLS/CLS)**<br/>Warehouse user should only see regional rows or cannot see credit card columns. | Item **`read`** + T-SQL Security Policies (predicate function) & `DENY SELECT ON table(col)`. | **Layer 4 (SQL Engine)**<br/>T-SQL execution engine filters rows/columns at query runtime. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`). |
| **Lakehouse Path & Folder Access**<br/>Lakehouse user needs access to specific Delta folders or table paths without SQL engine. | Configure OneLake Data Access Role with path attribute scopes (`paths = ["/Tables/sales"]`). | **Layer 3 & 4 (OneLake Storage)**<br/>OneLake security engine evaluates path filters directly against storage calls. | **Direct**<br/>`fabricext_lakehouse_permission` (simple mode with `paths` and `actions = ["Read"]`). |
| **Lakehouse Storage RLS & CLS**<br/>Lakehouse user needs row-filtering or column-masking on Delta tables. | Configure OneLake Data Access Role with `row_constraint` and `column_constraint`. | **Layer 4 (OneLake Engine)**<br/>Storage-level RLS/CLS enforced by Analysis Services and OneLake. | **Direct**<br/>`fabricext_lakehouse_permission` (advanced mode with `decision_rule`). |
| **Cross-Workspace Data Sharing**<br/>Derive OneLake role membership from users who have item permissions on another Fabric item. | Configure Lakehouse role containing `fabric_item_member` referencing the source workspace and item path. | **Layer 3 & 4 (OneLake Security)**<br/>OneLake dynamically inherits role membership from the source item permissions. | **Direct**<br/>`fabricext_lakehouse_permission` (`fabric_item_member` blocks). |
| **Power BI Semantic Model Consumption**<br/>Power BI report users must only see data permitted by their Entra identity. | Use **Direct Lake mode with SSO**; falls back to DirectQuery over TDS when SQL RLS/CLS is active. | **Layer 5 (Consumption)**<br/>Power BI's semantic model engine (Analysis Services) flows the user's active Entra token to the data engine. | **Underlying Engine Gate**<br/>`fabricext_*` manages item connectivity and OneLake Data Access Roles. |
| **Enterprise RBAC & Schema Automation**<br/>Automate both item-level gates and internal database RBAC across environments. | Combine Terraform with declarative database schema-as-code operators (e.g. Atlas, Flyway, or Liquibase). | **Layers 1–4 Integrated**<br/>Terraform provisions Entra & Fabric gates; schema operator configures T-SQL objects. | **Prerequisite Gateway**<br/>Provides the prerequisite connectivity gate (`CONNECT`) enabling database-level operators. |

---

## Detailed Use Case Guides

For in-depth architectural analysis, sequence diagrams, and runnable Terraform configurations, explore the dedicated deep-dive guides:

- **[Security Controls Inventory & Interaction Matrix](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_controls_and_interactions)**: Complete inventory across all 6 tiers and evaluation precedence rules between workspace roles, item shares, and SQL `DENY` statements.
- **[Warehouse Schema Isolation & Declarative RBAC](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_warehouse_schema_isolation)**: Step-by-step pattern for isolating schemas using `role_type = "read"`, Entra ID display name vs. Object ID UUID binding in T-SQL, and declarative schema-as-code automation.
- **[Lakehouse OneLake Security & Data Access Roles](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_lakehouse_onelake_security)**: Managing OneLake Data Access Roles with simple vs. advanced mode, path filters, storage RLS/CLS, and shortcut delegation.
- **[Power BI Identity Flow: Direct Lake vs. DirectQuery](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_powerbi_identity_propagation)**: Understanding how Entra ID security group tokens flow from Power BI reports through Analysis Services to Fabric Warehouses and Lakehouses, including DirectQuery fallback.

~> **Note:** A companion guide with interactive graphical Mermaid flowcharts is available on GitHub: [Granular Access Control Architecture Guide](https://github.com/jambazid/terraform-provider-fabricext/blob/main/guides/granular_access_control.md).
