---
page_title: "Use Cases: Enterprise Access Control Overview"
subcategory: "Use Cases"
description: |-
  Overview of enterprise access control patterns in Microsoft Fabric, including the 6-tier security perimeter stack and common access goals vs. architectural solutions.
---

# Enterprise Access Control Overview

This architectural guide outlines how enterprise organizations implement least-privilege access control, granular schema isolation, and data governance in Microsoft Fabric using the `fabricext` Terraform provider.

---

## Security Perimeter Stack

Security in Microsoft Fabric is governed by six decoupled, cascading evaluation perimeters. A principal must clear each tier sequentially to access an underlying data element:

```text
+--------------------------------------------------------------------------+
| Tier 1: Identity & Directory Perimeter (Microsoft Entra ID)              |
| - User accounts (UserPrincipalName / UPN), Security Groups (display name)|
| - Multi-factor authentication (MFA) & Conditional Access Policies        |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Tier 2: Tenant & Capacity Perimeter (Fabric Admin)                       |
| - Capacity licensing & assignments                                       |
| - Tenant switch overrides & external sharing toggles                     |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Tier 3: Workspace Perimeter                                              |
| - Roles: Admin, Member, Contributor (db_owner / administrative rights)   |
| - Roles: Viewer (read-only discovery) or Non-Member (pure item share)    |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Tier 4: Item Perimeter (fabricext Sharing & Discovery)                   |
| - Warehouse: 'read' (CONNECT to Tabular Data Stream / TDS endpoint)      |
| - SQL Database: 'read', 'read_data', 'read_spark', 'write', 'reshare'    |
| - Lakehouse: OneLake Data Access Roles (path & action scopes)            |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Tier 5: Engine & Storage RBAC (Role-Based Access Control)                |
| ├─ Warehouse / SQL DB: T-SQL Permissions Engine                          |
| │  - GRANT / DENY on Schema, Table, View over TDS (port 1433)            |
| │  - T-SQL Row-Level Security (RLS) & Column-Level Security (CLS)        |
| └─ Lakehouse: OneLake Storage Security Engine (Centralized Policy)       |
|    - Table path filters (/Tables/{tableName})                            |
|    - OneLake storage RLS row constraints & CLS column constraints        |
|    - Distributed enforcement by authorized compute engines at query time |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| Tier 6: Semantic & BI Consumption (Power BI)                             |
| - Direct Lake on OneLake (High-performance Parquet reads, DirectLakeOnly)|
| - Direct Lake on SQL Endpoint (Falls back to DirectQuery over TDS on RLS)|
| - DirectQuery over TDS (Evaluates T-SQL RLS/CLS under Entra ID SSO)      |
| - Semantic Model: Power BI Row-Level & Object-Level Security (RLS/OLS)   |
+--------------------------------------------------------------------------+
```

!> **Warning:** **Workspace Role Permission Inheritance & Cross-Item Isolation**: Users assigned **Admin**, **Member**, or **Contributor** roles on a Fabric workspace are automatically granted `db_owner` / administrative privileges on all Warehouses and Lakehouses in that workspace. T-SQL `DENY` statements, SQL Row-Level Security, and OneLake Data Access Roles **have no effect** on Admins, Members, or Contributors. While the **Viewer** workspace role respects T-SQL permissions, it grants read access across *all* items in the workspace (including inspecting notebooks, pipelines, and unconfigured Lakehouses). To enforce strict least privilege and cross-item isolation, principals should have **no workspace role** (pure item share via `fabricext`).

---

## Access Goals & Solutions

The following matrix maps enterprise access control requirements to their Microsoft Fabric architectural solutions, highlighting where `terraform-provider-fabricext` enables declarative Infrastructure as Code (IaC):

| Access Goal / Scenario | Recommended Solution | Mechanism & Boundary | `fabricext` Provider Role |
| :--- | :--- | :--- | :--- |
| **Broad Workspace Access**<br/>User needs to read or collaborate on all items across a workspace. | Assign **Viewer** (read-only) or **Contributor** (read/write) workspace role. | **Tier 3 (Workspace)**<br/>Workspace role assignment via Fabric Portal or `microsoft/fabric`. | **Out of Scope**<br/>Handled by official `fabric_workspace_role_assignment`. |
| **Warehouse Schema Isolation**<br/>User needs access to schema `finance` only; all other schemas (`hr`, `sales`) must be denied. | Grant item **`read`** (connectivity only) + create Entra DB user + grant T-SQL schema permissions. | **Tier 4 (Item Share) + Tier 5 (SQL Engine)**<br/>Item share grants Tabular Data Stream (TDS, port 1433) `CONNECT`; default-deny SQL engine restricts data access. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`) grants TDS connection without granting `db_owner` rights. *(Note: Warehouses support `ReadData` platform-wide; `fabricext` restricts to `read` to enforce schema isolation).* |
| **SQL Database Broad Access**<br/>User needs read access across all tables in a Fabric SQL Database. | Grant item-level **`read_data`** share on the SQL database item. | **Tier 4 (Item Share)**<br/>Item permission grants connectivity and broad data access across all tables in the operational Online Transaction Processing (OLTP) database. | **Direct**<br/>`fabricext_sql_database_permission` (`role_type = "read_data"`). |
| **SQL Database Spark Analytics**<br/>Spark notebooks require read access to OneLake mirrored database data. | Grant item-level **`read_spark`** share on the SQL database item. | **Tier 4 (Item Share) + Tier 5 (Storage)**<br/>Grants Spark access to near-real-time mirrored Parquet data and event subscriptions. | **Direct**<br/>`fabricext_sql_database_permission` (`role_type = "read_spark"`). |
| **Object-Level Security (OLS)**<br/>User can access specific tables or views within a schema, but sensitive tables are hidden. | Item **`read`** + T-SQL `GRANT SELECT ON table/view`. | **Tier 5 (SQL Engine)**<br/>SQL metadata hiding ensures unpermitted tables are invisible in `sys.tables`. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`). |
| **SQL Row/Column Security (RLS/CLS)**<br/>Warehouse user should only see regional rows or cannot see credit card columns. | Item **`read`** + T-SQL Security Policies (predicate function) & `DENY SELECT ON table(col)`. | **Tier 5 (SQL Engine)**<br/>T-SQL execution engine filters rows/columns at query runtime. | **Item Connectivity Gate**<br/>`fabricext_warehouse_permission` (`role_type = "read"`). |
| **Lakehouse Path & Folder Access**<br/>Lakehouse user needs access to specific Delta tables without SQL engine. | Configure OneLake Data Access Role with path attribute scopes (`paths = ["/Tables/sales"]`). | **Tier 4 & 5 (OneLake Storage)**<br/>OneLake security evaluates path filters against storage operations (`actions = ["Read"]`). | **Direct**<br/>`fabricext_lakehouse_permission` (simple mode with `paths` and `actions = ["Read"]`). |
| **Lakehouse Storage RLS & CLS**<br/>Lakehouse user needs row-filtering or column-masking on Delta tables. | Configure OneLake Data Access Role with `row_constraint` and `column_constraint`. | **Tier 5 (OneLake Engine)**<br/>Centralized policy enforced at query time by authorized engines (Spark, Analysis Services). | **Direct**<br/>`fabricext_lakehouse_permission` (advanced mode with `decision_rule`). |
| **Cross-Workspace Data Sharing**<br/>Derive OneLake role membership from users who have item permissions on another Fabric item. | Configure Lakehouse role containing `fabric_item_member` referencing the source workspace and item path. | **Tier 4 & 5 (OneLake Security)**<br/>OneLake dynamically inherits role membership from the source item permissions. | **Direct**<br/>`fabricext_lakehouse_permission` (`fabric_item_member` blocks). |
| **Power BI Semantic Model Consumption**<br/>Power BI report users must only see data permitted by their Entra identity. | Use **Direct Lake with Single Sign-On (SSO)** or DirectQuery over TDS. | **Tier 6 (Consumption)**<br/>Power BI Analysis Services delegates user's active Entra token to data engine. | **Underlying Engine Gate**<br/>`fabricext_*` manages item connectivity and OneLake Data Access Roles. |
| **Enterprise RBAC & Schema Automation**<br/>Automate both item-level gates and internal database RBAC across environments. | Combine Terraform with declarative database schema-as-code operators (e.g. Atlas, Flyway, or Liquibase). | **Tiers 1–5 Integrated**<br/>Terraform provisions Entra & Fabric gates; schema operator configures T-SQL objects via Entra token. | **Prerequisite Gateway**<br/>Provides prerequisite connectivity gate (`CONNECT`) enabling database-level operators. |

---

## Detailed Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Security Controls & Interactions](./use_case_controls_and_interactions.md)** | Tier inventory, precedence rules, and workspace role bypass rules. |
| **[Warehouse Schema Isolation](./use_case_warehouse_schema_isolation.md)** | Least-privilege `read` connectivity, Entra ID naming, and SQL RBAC. |
| **[Lakehouse OneLake Security](./use_case_lakehouse_onelake_security.md)** | Data Access Roles, path filters, storage RLS/CLS, and shortcuts. |
| **[Power BI Identity Flow](./use_case_powerbi_identity_propagation.md)** | DirectQuery vs. Direct Lake SSO token propagation and fallback. |

~> **Note:** A companion guide with interactive graphical Mermaid flowcharts is available on GitHub: [Granular Access Control Architecture Guide](https://github.com/jambazid/terraform-provider-fabricext/blob/main/guides/granular_access_control.md).
