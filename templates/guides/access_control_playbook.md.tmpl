---
page_title: "Access Control Playbook"
subcategory: "Guides"
description: |-
  A practitioner guide to enterprise Microsoft Fabric access control patterns, mapping 11 common organizational personas and security perimeters to verified Terraform, portal, and T-SQL configurations.
---

# Access Control Playbook

Microsoft Fabric unifies compute engines, storage, and governance across Lakehouses, Warehouses, SQL Databases, and Power BI semantic models. Because Fabric spans multiple control planes, configuring least-privilege access requires coordinating workspace roles, item-level sharing, storage-plane OneLake security roles, and database-level T-SQL permissions.

This playbook provides verified, step-by-step guidance for 11 distinct enterprise access patterns.

~> **Note:** For foundational concept overviews and control precedence rules, refer to [Overview](use_case_overview.md) and [Controls & Interactions](use_case_controls_and_interactions.md).

---

## Start Here Routing Table

Find your target scenario based on the engine, the identity type, and the desired isolation boundary:

| # | Engine / Target | Identity / Persona | Isolation Boundary | Recommended Mechanism | Card Link |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **P1** | Warehouse | Analyst | Single schema read | Portal share (Read) + T-SQL `GRANT SELECT ON SCHEMA` | [Jump to P1](#p1-warehouse-schema-isolation) |
| **P2** | Warehouse | Sensitive table | Single table read | Portal share (Read) + T-SQL `GRANT SELECT ON table` | [Jump to P2](#p2-warehouse-table-isolation) |
| **P3** | Lakehouse | Analyst (Spark / Direct Lake) | Table subset + RLS/CLS | `fabricext_lakehouse_data_access_role` OneLake role | [Jump to P3](#p3-lakehouse-onelake-security) |
| **P4** | Lakehouse / Warehouse | Business user | Power BI report only | Fixed-identity Direct Lake semantic model | [Jump to P4](#p4-power-bi-report-only-consumers) |
| **P5** | Lakehouse | Cross-workspace team | Lakehouse shortcut | `ReadAll` item share on target lakehouse | [Jump to P5](#p5-cross-workspace-shortcuts) |
| **P6** | Warehouse | Ingestion pipeline (SPN) | Staging schema write | Portal share (Read) + T-SQL `INSERT`/`UPDATE`/`ADMINISTER DATABASE BULK OPERATIONS` | [Jump to P6](#p6-warehouse-pipeline-ingestion) |
| **P7** | SQL Database | Application backend | Database read/write | Portal share + T-SQL database roles | [Jump to P7](#p7-sql-database-application-backend) |
| **P8** | Workspace | Platform engineer | Admin / deployment | Workspace `Admin` or `Member` via `microsoft/fabric` | [Jump to P8](#p8-workspace-platform-engineering) |
| **P9** | Workspace | BI developer | Semantic model build | Workspace `Contributor` or `Viewer` + semantic model `Build` | [Jump to P9](#p9-bi-developer-semantic-model-authoring) |
| **P10** | Tenant / Workspace | External partner | B2B guest read | Entra B2B Guest + item share / OneLake role | [Jump to P10](#p10-external-partner-b2b-guest-access) |
| **P11** | Fabric Platform | CI/CD Runner | Automated deployment | Entra SPN + Workspace role via `microsoft/fabric` | [Jump to P11](#p11-cicd-runner-deployment-pipeline) |

---

## P1: Warehouse Schema Isolation

- **Goal**: Grant an analyst read-only access to a single schema (e.g. `finance`) while hiding all other schemas.
- **Provider Support**: Out of scope for automated Terraform item-sharing (`fabricext_warehouse_permission` was removed in `v0.5.0` because Microsoft Fabric does not offer a public item-sharing REST API). Connectivity must be granted via the portal; authorization is handled in T-SQL.
- **Prerequisites**: Microsoft Entra ID Security Group for analysts; Fabric Warehouse created.
- **Workspace Role**: None (`None`). Assigning `Admin`, `Member`, or `Contributor` grants `CONTROL` (`db_owner`), bypassing all internal SQL grants. Assigning `Viewer` grants workspace-wide `ReadData` (`db_datareader`), exposing all tables across all warehouses.
- **Terraform Configuration**: None. Manage warehouse provisioning in `microsoft/fabric`.
- **Portal & T-SQL Steps**:
  1. In the Fabric web portal, locate the Warehouse, click **Share**, enter the Entra Group name, and check **Read** only (do not check "Read all data using SQL").
  2. Connect to the Warehouse TDS endpoint (port 1433) using any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, or Python) as a workspace Admin.
  3. Execute T-SQL grants:
     ```sql
     -- In Fabric Data Warehouse, users are created implicitly upon GRANT
     GRANT SELECT ON SCHEMA::finance TO [SEC-Fabric-Finance-Analysts];
     ```
- **Managed Elsewhere**: Item share in Fabric Portal UI; T-SQL grants via CI/CD migration script (e.g., Atlas, Flyway, Liquibase, or sqlcmd).
- **Verify**:
  ```bash
  sqlcmd -S <guid>.datawarehouse.fabric.microsoft.com -d <warehouse_name> -G -Q "SELECT TOP 5 * FROM finance.revenue;"
  sqlcmd -S <guid>.datawarehouse.fabric.microsoft.com -d <warehouse_name> -G -Q "SELECT TOP 5 * FROM hr.salaries;" # Fails with Msg 229
  ```
- **Undo**:
  ```sql
  REVOKE SELECT ON SCHEMA::finance FROM [SEC-Fabric-Finance-Analysts];
  ```
- **Traps**:
  - Do not run `CREATE USER [name] FROM EXTERNAL PROVIDER`; Fabric Data Warehouse does not support `CREATE USER` and returns an error.
  - Do not assign workspace `Viewer`; it grants `CONNECT` and `ReadData` on all warehouses in the workspace.
- **Why Least Privilege**: Decoupling connectivity (portal Read) from data authorization (T-SQL schema grant) prevents unauthorized exposure of adjacent domains within the same database engine.

---

## P2: Warehouse Table Isolation

- **Goal**: Grant read access to a specific table within a shared schema (e.g. `sales.public_metrics`).
- **Provider Support**: Out of scope for automated Terraform item-sharing; configured via Fabric Portal UI + T-SQL.
- **Prerequisites**: Fabric Warehouse provisioned; Entra security group.
- **Workspace Role**: None (`None`).
- **Terraform Configuration**: None.
- **Portal & T-SQL Steps**:
  1. Share the Warehouse via the Fabric portal with **Read** connectivity only.
  2. Connect to the Warehouse TDS endpoint (port 1433) using any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, or Python).
  3. Execute T-SQL table grants:
     ```sql
     GRANT SELECT ON OBJECT::sales.public_metrics TO [SEC-Fabric-Sales-Auditors];
     ```
- **Managed Elsewhere**: Item share in Fabric Portal UI; T-SQL permissions via SQL deployment tools (e.g., Atlas, Flyway, Liquibase, or sqlcmd).
- **Verify**:
  ```bash
  sqlcmd -S <guid>.datawarehouse.fabric.microsoft.com -d <warehouse_name> -G -Q "SELECT TOP 5 * FROM sales.public_metrics;"
  ```
- **Undo**:
  ```sql
  REVOKE SELECT ON OBJECT::sales.public_metrics FROM [SEC-Fabric-Sales-Auditors];
  ```
- **Traps**:
  - Object grants require explicit schema access if schema-level permissions were denied.
- **Why Least Privilege**: Prevents exposure of sensitive peer tables in the same schema without requiring physical warehouse segmentation.

---

## P3: Lakehouse OneLake Security

- **Goal**: Grant read access to specific Delta tables and folders in a Lakehouse with Row-Level Security (RLS) and Column-Level Security (CLS).
- **Provider Support**: Fully supported via `fabricext_lakehouse_data_access_role` and `fabricext_lakehouse_onelake_security`.
- **Prerequisites**: OneLake Data Access Security enabled in Lakehouse via Fabric web portal (**Manage OneLake security**).
- **Workspace Role**: `Viewer` or `None` (with item `Read` permission). Workspace `Admin`, `Member`, and `Contributor` bypass OneLake security roles completely.
- **Terraform Configuration**:
  ```hcl
  resource "fabricext_lakehouse_onelake_security" "main" {
    workspace_id               = var.workspace_id
    lakehouse_id               = var.lakehouse_id
    remove_default_reader_role = true
  }

  resource "fabricext_lakehouse_data_access_role" "uk_analysts" {
    depends_on   = [fabricext_lakehouse_onelake_security.main]
    workspace_id = var.workspace_id
    lakehouse_id = var.lakehouse_id
    role_name    = "UKAnalysts"

    decision_rules = [
      {
        effect      = "Permit"
        actions     = ["Read"]
        source_path = "Tables/silver_sales"
      }
    ]

    row_constraints = [
      {
        table_name    = "silver_sales"
        filter_clause = "Country = 'UK'"
      }
    ]

    column_constraints = [
      {
        table_name    = "silver_sales"
        column_names  = ["SaleID", "Date", "Country", "Amount"]
        column_effect = "Permit"
      }
    ]

    members = [
      {
        id   = var.uk_analysts_group_id
        type = "Group"
      }
    ]
  }
  ```
- **Portal & T-SQL Steps**: OneLake Data Access Security must be enabled in the web portal once per Lakehouse before applying Terraform.
- **Managed Elsewhere**: Initial portal enablement toggle.
- **Verify**:
  Execute PySpark in a notebook as a member of `UKAnalysts`:
  ```python
  df = spark.read.table("silver_sales")
  assert set(df.columns) == {"SaleID", "Date", "Country", "Amount"}
  assert df.filter("Country != 'UK'").count() == 0
  ```
- **Undo**:
  Destroy the Terraform resource `fabricext_lakehouse_data_access_role.uk_analysts`.
- **Traps**:
  - If `remove_default_reader_role` is not set to `true`, the auto-generated `DefaultReader` role grants full table reads to all `ReadAll` holders, defeating custom RLS/CLS.
  - RLS/CLS cannot be applied to roles with `ReadWrite` action; `fabricext` rejects this at plan time.
  - Role names must be alphanumeric and length $\le 128$ characters.
- **Why Least Privilege**: OneLake security roles enforce storage-level access across Spark, SQL Analytics Endpoints, and Power BI Direct Lake without maintaining duplicate access control engines.

---

## P4: Power BI Reports

- **Goal**: Allow business stakeholders to view Power BI reports without exposing underlying Lakehouse Delta files or Warehouse tables.
- **Provider Support**: Out of scope for `fabricext`. Managed via Power BI service and `microsoft/fabric`.
- **Prerequisites**: Fabric Workspace; Lakehouse or Warehouse; Power BI semantic model.
- **Workspace Role**: None (`None`). Do not assign report consumers any workspace role.
- **Terraform Configuration**: Manage report and app permissions via Power BI APIs or `microsoft/fabric`.
- **Portal & Architecture Steps**:
  1. Build a custom Power BI semantic model using **Direct Lake** in Fixed Identity mode (where the dataset owner or Service Principal connects to the storage).
  2. Publish the report to an organizational Power BI App.
  3. Grant report consumers access to the Power BI App without granting `Build` permission on the semantic model.
- **Managed Elsewhere**: Power BI App audience configuration in portal.
- **Verify**:
  Confirm that business users can render the report visuals, but cannot discover the Warehouse or Lakehouse items in the OneLake Data Hub.
- **Undo**:
  Remove the user or group from the Power BI App access list.
- **Traps**:
  - Do not grant `Build` permission or share the underlying workspace; users could then connect Excel or external tools directly to the source.
- **Why Least Privilege**: Withholds data plane storage credentials from end users while serving aggregate business insights through governed semantic models.

---

## P5: Cross Workspace Shortcuts

- **Goal**: Enable Lakehouse A in Workspace A to consume Delta tables from Lakehouse B in Workspace B via OneLake shortcuts without duplicating data.
- **Provider Support**: Target Lakehouse OneLake security managed via `fabricext_lakehouse_data_access_role`.
- **Prerequisites**: Lakehouse A and Lakehouse B created.
- **Workspace Role**: `Viewer` or `None` on Workspace B.
- **Terraform Configuration**:
  ```hcl
  # In Workspace B (Target Lakehouse):
  # Grant virtualized member access derived from item ReadAll sharing
  resource "fabricext_lakehouse_data_access_role" "cross_workspace_readers" {
    workspace_id = var.target_workspace_id
    lakehouse_id = var.target_lakehouse_id
    role_name    = "CrossWorkspaceReaders"

    decision_rules = [
      {
        effect      = "Permit"
        actions     = ["Read"]
        source_path = "Tables/shared_dimension"
      }
    ]

    fabric_item_members = [
      {
        item_access = ["ReadAll"]
      }
    ]
  }
  ```
- **Portal & T-SQL Steps**: Create the OneLake-to-OneLake shortcut in Lakehouse A pointing to `Tables/shared_dimension` in Lakehouse B.
- **Managed Elsewhere**: Shortcut definition in Lakehouse A.
- **Verify**:
  Query the shortcut in Lakehouse A via Spark or SQL endpoint; confirm successful read.
- **Undo**:
  Delete the shortcut in Lakehouse A or remove the role in Lakehouse B.
- **Traps**:
  - You cannot define OneLake security permissions directly on a OneLake-to-OneLake shortcut; permissions on the folder containing the shortcut combine with permissions defined on the target path.
- **Why Least Privilege**: Allows centralized master data management without creating fragmented, out-of-sync ETL copies.

---

## P6: Warehouse Pipeline Ingestion

- **Goal**: Allow an automated ETL pipeline (Service Principal) to ingest data into a staging schema in a Warehouse without conferring database-wide admin rights.
- **Provider Support**: Out of scope for `fabricext`. Configured via Fabric Portal UI + T-SQL.
- **Prerequisites**: Microsoft Entra Service Principal; Fabric Warehouse.
- **Workspace Role**: None (`None`). Assigning Contributor grants `CONTROL` on the entire warehouse.
- **Terraform Configuration**: None.
- **Portal & T-SQL Steps**:
  1. Share the Warehouse via the Fabric portal with the Service Principal with **Read** access.
  2. Execute T-SQL grants over TDS using any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, or Python):
     ```sql
     GRANT INSERT, UPDATE, DELETE ON SCHEMA::staging TO [ETL-Ingestion-SPN];
     GRANT ADMINISTER DATABASE BULK OPERATIONS TO [ETL-Ingestion-SPN]; -- Required for COPY INTO
     ```
- **Managed Elsewhere**: T-SQL schema migrations (e.g., Atlas, Flyway, Liquibase, or sqlcmd).
- **Verify**:
  Execute ingestion pipeline via Service Principal bearer token; confirm writes to `staging` succeed while writes to `production` fail.
- **Undo**:
  `REVOKE INSERT, UPDATE, DELETE ON SCHEMA::staging FROM [ETL-Ingestion-SPN];`
- **Traps**:
  - `COPY INTO` requires `ADMINISTER DATABASE BULK OPERATIONS` data plane permission when the principal does not hold `CONTROL`.
- **Why Least Privilege**: Prevents automated ingestion pipelines from overwriting production consumption tables or altering database schemas.

---

## P7: SQL Database Application Backend

- **Goal**: Allow an application backend service principal to read and write tables in a Microsoft Fabric SQL Database.
- **Provider Support**: Out of scope for automated Terraform item-sharing (`fabricext_sql_database_permission` was removed in `v0.5.0`). Configured via Fabric Portal Share quick action + T-SQL.
- **Prerequisites**: Fabric SQL Database; Microsoft Entra Service Principal.
- **Workspace Role**: None (`None`).
- **Terraform Configuration**: None.
- **Portal & T-SQL Steps**:
  1. Share the SQL Database in the Fabric portal using the **Share** quick action with minimum permissions.
  2. Connect to the SQL Database as an Entra admin user using any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, or Python) and create the database user:
     ```sql
     -- When connected as an Entra user with Directory Readers:
     CREATE USER [app-backend-spn] FROM EXTERNAL PROVIDER;

     -- Alternatively, when connected via pipeline service principal:
     -- CREATE USER [app-backend-spn] WITH SID = <spn_sid>, TYPE = E;

     ALTER ROLE db_datareader ADD MEMBER [app-backend-spn];
     ALTER ROLE db_datawriter ADD MEMBER [app-backend-spn];
     ```
- **Managed Elsewhere**: Fabric portal Share quick action; T-SQL schema migrations (e.g., Atlas, Flyway, Liquibase, or sqlcmd).
- **Verify**:
  Connect using the application connection string via any ODBC or JDBC client (such as Atlas, sqlcmd, SSMS, Azure Data Studio, or Python); verify read and write operations.
- **Undo**:
  `DROP USER [app-backend-spn];`
- **Traps**:
  - Propagation of item-level sharing for Fabric SQL Databases can take up to 2 hours.
- **Why Least Privilege**: Keeps operational OLTP application accounts isolated from workspace administrative privileges.

---

## P8: Workspace Platform Engineering

- **Goal**: Manage workspace administrative and lifecycle permissions for platform engineering teams.
- **Provider Support**: Out of scope for `fabricext`. Managed via the official `microsoft/fabric` provider (`fabric_workspace_role_assignment`).
- **Prerequisites**: Microsoft Entra Security Group for Platform Engineers.
- **Workspace Role**: `Admin` or `Member`.
- **Terraform Configuration**:
  ```hcl
  resource "fabric_workspace_role_assignment" "platform_engineers" {
    workspace_id = var.workspace_id
    principal = {
      id   = var.platform_engineers_group_id
      type = "Group"
    }
    role = "Member"
  }
  ```
- **Portal & T-SQL Steps**: None.
- **Managed Elsewhere**: Core workspace governance in `microsoft/fabric`.
- **Verify**:
  Inspect workspace access in Fabric Admin portal or via REST API.
- **Undo**:
  Destroy `fabric_workspace_role_assignment.platform_engineers`.
- **Traps**:
  - Workspace `Admin` confers complete control over all workspace items and bypasses all OneLake and SQL permissions.
- **Why Least Privilege**: Restricts broad workspace-level roles strictly to infrastructure and platform custodians.

---

## P9: Semantic Model Authoring

- **Goal**: Allow business intelligence developers to author and publish semantic models against curated Lakehouse tables.
- **Provider Support**: OneLake table access managed via `fabricext_lakehouse_data_access_role`; workspace roles managed via `microsoft/fabric`.
- **Prerequisites**: Curated Lakehouse; Entra security group.
- **Workspace Role**: `Contributor` or `Viewer` + `Build` permission on semantic models.
- **Terraform Configuration**:
  ```hcl
  # In production workspaces, assign Viewer role and define explicit OneLake security roles
  resource "fabricext_lakehouse_data_access_role" "bi_readers" {
    workspace_id = var.workspace_id
    lakehouse_id = var.lakehouse_id
    role_name    = "BIDevelopers"

    decision_rules = [
      {
        effect      = "Permit"
        actions     = ["Read"]
        source_path = "Tables/curated_*"
      }
    ]

    members = [
      {
        id   = var.bi_developers_group_id
        type = "Group"
      }
    ]
  }
  ```
- **Portal & T-SQL Steps**: None.
- **Managed Elsewhere**: Workspace role assignment via `microsoft/fabric`.
- **Verify**:
  Confirm BI developer can connect Power BI Desktop to curated tables and cannot read raw or staging folders.
- **Undo**:
  Destroy `fabricext_lakehouse_data_access_role.bi_readers`.
- **Traps**:
  - Assigning Contributor in the production workspace bypasses OneLake security roles. Use dedicated development workspaces for authors.
- **Why Least Privilege**: Protects raw data ingestion perimeters while enabling self-service reporting on certified datasets.

---

## P10: External B2B Guests

- **Goal**: Securely share specific Lakehouse tables with an external partner Entra ID B2B guest account.
- **Provider Support**: Fully supported via `fabricext_lakehouse_data_access_role`.
- **Prerequisites**: Fabric tenant settings configured to allow external B2B guest sharing; Guest user invited to Entra tenant.
- **Workspace Role**: None (`None`).
- **Terraform Configuration**:
  ```hcl
  resource "fabricext_lakehouse_data_access_role" "partner_readers" {
    workspace_id = var.workspace_id
    lakehouse_id = var.lakehouse_id
    role_name    = "PartnerAuditors"

    decision_rules = [
      {
        effect      = "Permit"
        actions     = ["Read"]
        source_path = "Tables/audit_export"
      }
    ]

    members = [
      {
        id   = var.partner_guest_user_object_id
        type = "User"
      }
    ]
  }
  ```
- **Portal & T-SQL Steps**: Share Lakehouse item with partner guest in Fabric portal (**Read** permission only).
- **Managed Elsewhere**: Tenant B2B export and sharing settings in Fabric Admin portal.
- **Verify**:
  External guest signs into Fabric portal; verifies visibility of `Tables/audit_export` only.
- **Undo**:
  Destroy `fabricext_lakehouse_data_access_role.partner_readers`.
- **Traps**:
  - B2B guests must be invited into the resource tenant; use the guest user's Object ID in the resource tenant.
- **Why Least Privilege**: Limits external access strictly to targeted export tables without exposing workspace internal assets.

---

## P11: CI/CD Runner Deployment Pipeline

- **Goal**: Enable an automated GitHub Actions or Azure DevOps pipeline Service Principal to deploy Fabric workspace items.
- **Provider Support**: Out of scope for `fabricext`. Managed via `microsoft/fabric`.
- **Prerequisites**: Service Principal tenant setting enabled in Fabric Admin Portal ("Allow service principals to use Fabric APIs").
- **Workspace Role**: `Contributor` or `Admin` across deployment pipeline stages.
- **Terraform Configuration**:
  ```hcl
  resource "fabric_workspace_role_assignment" "cicd_spn" {
    workspace_id = var.workspace_id
    principal = {
      id   = var.cicd_spn_object_id
      type = "ServicePrincipal"
    }
    role = "Contributor"
  }
  ```
- **Portal & T-SQL Steps**: None.
- **Managed Elsewhere**: Fabric Admin portal tenant setting for Service Principals.
- **Verify**:
  Execute deployment pipeline step; confirm successful API authentication and asset deployment.
- **Undo**:
  Destroy `fabric_workspace_role_assignment.cicd_spn`.
- **Traps**:
  - Fabric APIs will reject Service Principal calls with 401/403 if the tenant setting is disabled in the Admin portal.
- **Why Least Privilege**: Scopes automated deployment identities to Contributor privileges in specific workspace environments.
