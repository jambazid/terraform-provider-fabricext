---
page_title: "Lakehouse OneLake Security"
subcategory: "Guides"
description: |-
  Comprehensive guide to managing Microsoft Fabric Lakehouse OneLake Data Access Roles with fabricext, including simple vs. advanced modes, path filters, storage-level RLS/CLS, and shortcut delegation.
---

# Lakehouse OneLake Security

Microsoft Fabric Lakehouses store data in OneLake using open Delta Lake (Parquet) format. Unlike Warehouses, which enforce security through the T-SQL query engine, Lakehouses enforce security through **OneLake Data Access Roles**.

This guide explains how [`fabricext_lakehouse_data_access_role`](../resources/lakehouse_data_access_role.md) manages OneLake Data Access Roles declaratively.

~> **Note:** **Storage Roles vs. Catalog Sharing**: The `fabricext_lakehouse_data_access_role` resource configures storage-layer OneLake Data Access Roles. In Microsoft Fabric, Lakehouse item discovery in the workspace catalog is governed by workspace roles or the Fabric Portal.

---

## OneLake Security Architecture

OneLake Data Access Roles follow a **centralized policy, distributed compute engine enforcement model**. Security policies are authored and stored centrally in OneLake metadata, and enforced at query time by authorized compute engines:

```text
+--------------------------------------------------------------------------+
| Analytics Compute Engines (Enforcement Layer)                            |
| ├─ Apache Spark (Compiles policies into physical Spark execution plan)   |
| ├─ Power BI Analysis Services (Enforces policies under Direct Lake Single Sign-On / SSO)  |
| └─ SQL Analytics Endpoint (Enforces policies over Tabular Data Stream / TDS queries)      |
+--------------------------------------------------------------------------+
                                     │
                                     ▼ Retrieves effective role policies
+--------------------------------------------------------------------------+
| OneLake Storage Security Engine (Centralized Policy Definition)          |
| ├─ Path Filters: Folder / Table Boundaries (/Tables/{tableName})         |
| ├─ Action Scopes: Read, ReadWrite                                       |
| ├─ Storage-Level Row-Level Security (RLS Predicates)                     |
| └─ Storage-Level Column-Level Security (CLS Masks)                       |
+--------------------------------------------------------------------------+
                                     │
                                     ▼ Reads permitted data
+--------------------------------------------------------------------------+
| OneLake Underlying Storage (Delta Lake Parquet Files)                    |
+--------------------------------------------------------------------------+
```

When an analyst queries a Lakehouse through Spark or Power BI Direct Lake, the compute engine retrieves the user's effective OneLake role rules and compiles the row predicates and column masks into its query execution plan. External storage clients lacking OneLake security support are blocked from accessing secured tables.

---

## Configuration Modes Comparison

The `fabricext_lakehouse_data_access_role` resource supports two operational modes:

| Feature | Simple Mode (`paths`, `actions`, `principal_ids`) | Advanced Mode (`decision_rule`, `entra_member`, `fabric_item_member`) |
| :--- | :--- | :--- |
| **Primary Use Case** | Broad folder or table access with uniform actions across one or more Entra principals. | Fine-grained path boundaries, storage-level RLS row constraints, CLS column constraints, and shortcut delegation. |
| **Path Filtering** | Specified via `paths` attribute (e.g. `["/Tables/sales"]` or `["*"]`). | Configured per `decision_rule.paths` set (`/Tables/{tableName}`). |
| **Action Scopes** | `actions` (`Read`, `ReadWrite`). Defaults to `["Read"]`. | Configured per `decision_rule.actions` (`Read`, `ReadWrite`). |
| **Member Assignment** | `principal_ids` (Set of UUIDs) + uniform `principal_type`. | `entra_member` blocks (`object_id`, `object_type`, `tenant_id`). Supports mixed Microsoft Entra ID types. |
| **Row-Level Security** | Not supported. | Configurable via `decision_rule.row_constraint` (`table_path`, `predicate`). |
| **Column Security** | Not supported. | Configurable via `decision_rule.column_constraint` (`table_path`, `columns`, `action`, `effect`). |
| **Item Membership** | Not supported. | Dynamic member virtualization via `fabric_item_member` (`source_path`, `item_access`). |
| **Mutual Exclusivity** | Must specify `paths` and `principal_ids`; omit advanced blocks. | Must omit simple attributes; specify `decision_rule` and member blocks. |

~> **Note:** **Member Type Uniformity**: In simple mode (`principal_ids`), Microsoft Fabric OneLake Data Access Roles apply a single uniform `principal_type` (`Group`, `User`, or `ServicePrincipal`) per resource. In advanced mode (`entra_member`), heterogeneous member types (Users, Groups, Service Principals, and Managed Identities) are natively supported within the same role.

---

## Security Enablement & Purging

Before OneLake Data Access Roles take effect, security must be initialized on the Lakehouse:

1. **Portal Enablement**: In the Microsoft Fabric web portal, open the Lakehouse ribbon and select **Manage OneLake security**.
2. **Resource Verification**: Declare [`fabricext_lakehouse_onelake_security`](../resources/lakehouse_onelake_security.md) in Terraform to confirm enablement.
3. **Purging DefaultReader**: When OneLake security is enabled, Fabric auto-creates a `DefaultReader` role granting read access across all tables to anyone with workspace `ReadAll`. Leaving this role active undermines fine-grained RLS and CLS permissions. By default, `remove_default_reader_role = true` automatically removes this role.

```hcl
# 1. Initialize OneLake security and purge DefaultReader
resource "fabricext_lakehouse_onelake_security" "bronze" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = "raw_bronze_lh"

  remove_default_reader_role = true
}

# 2. Grant explicit fine-grained roles
resource "fabricext_lakehouse_data_access_role" "bronze_readers" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_name = fabricext_lakehouse_onelake_security.bronze.lakehouse_name
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"

  depends_on = [
    fabricext_lakehouse_onelake_security.bronze,
  ]
}
```

---

## Authoring OneLake Roles

### Simple Mode Example

For straightforward table access assigned to an Entra security group:

```hcl
resource "fabricext_lakehouse_onelake_security" "sales" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "00000000-0000-0000-0000-000000000002"

  remove_default_reader_role = true
}

resource "fabricext_lakehouse_data_access_role" "sales_analysts_simple" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_id   = fabricext_lakehouse_onelake_security.sales.lakehouse_id
  role_name      = "SalesTableReaders"
  paths          = ["/Tables/sales"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"

  depends_on = [
    fabricext_lakehouse_onelake_security.sales,
  ]
}
```

### Advanced Mode Configuration

For granular security restricting access to `/Tables/sales` with regional row filtering and explicit column restrictions:

```hcl
resource "fabricext_lakehouse_onelake_security" "sales" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "00000000-0000-0000-0000-000000000002"

  remove_default_reader_role = true
}

resource "fabricext_lakehouse_data_access_role" "sales_analysts_advanced" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = fabricext_lakehouse_onelake_security.sales.lakehouse_id
  role_name    = "SalesRegionalAuditors"

  entra_member {
    object_id   = "11111111-1111-1111-1111-111111111111"
    object_type = "Group"
  }

  decision_rule {
    effect  = "Permit"
    paths   = ["/Tables/sales"]
    actions = ["Read"]

    row_constraint {
      table_path = "/Tables/sales"
      predicate  = "Region = 'EMEA'"
    }

    column_constraint {
      table_path = "/Tables/sales"
      columns    = ["CustomerKey", "SalesAmount", "Region"]
      action     = "Read"
      effect     = "Permit"
    }
  }

  depends_on = [
    fabricext_lakehouse_onelake_security.sales,
  ]
}
```

### Item Membership Example

To dynamically grant role membership to users who hold permissions on another Fabric item (such as a downstream consumer Lakehouse or shortcut container), configure `fabric_item_member` on the data provider Lakehouse:

```hcl
resource "fabricext_lakehouse_onelake_security" "shared" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "00000000-0000-0000-0000-000000000002"

  remove_default_reader_role = true
}

resource "fabricext_lakehouse_data_access_role" "cross_workspace_readers" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = fabricext_lakehouse_onelake_security.shared.lakehouse_id
  role_name    = "CrossWorkspaceAccess"

  # Dynamically inherits role membership from principals holding
  # "ReadAll" permissions on consumer Lakehouse in Workspace B:
  fabric_item_member {
    # Format: "<consumer_workspace_id>/<consumer_item_id>"
    source_path = "00000000-0000-0000-0000-000000000002/33333333-3333-3333-3333-333333333333"
    item_access = ["ReadAll"]
  }

  decision_rule {
    effect  = "Permit"
    paths   = ["/Tables/shared_sales"]
    actions = ["Read"]
  }

  depends_on = [
    fabricext_lakehouse_onelake_security.shared,
  ]
}
```

~> **Note:** **Storage Actions vs. Item Access**: `decision_rules.actions` (`Read`, `ReadWrite`) defines the **storage-plane access granted** on the Lakehouse files. In contrast, `fabric_item_members.item_access` (`ReadAll`, `Read`) defines the **prerequisite control-plane permission** a user must hold on `source_path` to qualify for role membership.

---

## Concurrency & Mutex Semantics

Updating OneLake Data Access Roles involves a `GET` → `PUT` Read-Modify-Write (RMW) cycle with `If-Match` HTTP ETag headers.

Under parallel Terraform execution (`for_each`), concurrent requests mutating roles on the same Lakehouse can result in HTTP 412 (`Precondition Failed`). The `fabricext` provider protects this lifecycle with:

1. **Per-Lakehouse Mutex Locking**: Serializes parallel Terraform resource executions mutating roles on the same Lakehouse within the provider process.
2. **Exponential Backoff on ETag Mismatch**: Automatically retries on HTTP 412 conflicts, refreshing the current ETag and reapplying modifications safely.

---

## Related Guides

| Guide | Core Focus |
| :--- | :--- |
| **[Access Control Playbook](./access_control_playbook.md)** | Start Here routing table and 11 verified scenario execution cards. |
| **[Enterprise Access Control Overview](./use_case_overview.md)** | Master comparison matrix and 6-tier perimeter overview. |
| **[Security Controls & Interactions](./use_case_controls_and_interactions.md)** | Precedence rules and override behaviors across tiers. |
| **[Warehouse Schema Isolation](./use_case_warehouse_schema_isolation.md)** | Isolating schemas using `permissions = ["Read"]` and T-SQL RBAC. |
| **[Power BI Identity Flow](./use_case_powerbi_identity_propagation.md)** | End-to-end token flow to semantic models and query fallback. |
