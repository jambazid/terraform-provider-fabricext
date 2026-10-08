---
page_title: "Use Cases: Lakehouse OneLake Security & Data Access Roles"
subcategory: "Use Cases"
description: |-
  Comprehensive guide to managing Microsoft Fabric Lakehouse OneLake Data Access Roles with fabricext, including simple vs. advanced modes, path filters, storage-level RLS/CLS, and shortcut delegation.
---

# Use Cases: Lakehouse OneLake Security & Data Access Roles

Microsoft Fabric Lakehouses store data in OneLake using open Delta Lake (Parquet) format. Unlike Warehouses, which enforce security exclusively through the T-SQL query engine, Lakehouses enforce security directly at the **OneLake storage engine layer** through **OneLake Data Access Roles**.

This guide explains how [`fabricext_lakehouse_permission`](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/resources/lakehouse_permission) manages OneLake Data Access Roles declaratively.

---

## OneLake Security Architecture

OneLake Data Access Roles govern storage-level access across all Microsoft Fabric analytics engines:

```text
+--------------------------------------------------------------------------+
| Analytics Consumption Engines                                            |
| ├─ Apache Spark (Notebooks & Spark Jobs)                                  |
| ├─ Power BI Analysis Services (Direct Lake Mode)                         |
| └─ SQL Analytics Endpoint (TDS & T-SQL Queries)                          |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| OneLake Storage Security Engine (Data Access Roles)                      |
| ├─ Path Filters: Folder / Table Path Boundaries (/Tables/sales/*)        |
| ├─ Action Scopes: Read, ReadData, ReadAll                                |
| ├─ Storage-Level Row-Level Security (RLS Predicates)                     |
| └─ Storage-Level Column-Level Security (CLS Masks)                       |
+--------------------------------------------------------------------------+
                                     │
                                     ▼
+--------------------------------------------------------------------------+
| OneLake Underlying Storage (Delta Lake Parquet Files)                    |
+--------------------------------------------------------------------------+
```

When an analyst queries a Lakehouse through Spark or Power BI Direct Lake, OneLake evaluates the user's active Microsoft Entra token against the assigned OneLake role. This ensures that restrictions apply universally regardless of which engine is used.

---

## Simple Mode vs. Advanced Mode

The `fabricext_lakehouse_permission` resource supports two operational modes:

| Feature | Simple Mode (`paths`, `actions`, `principal_ids`) | Advanced Mode (`decision_rule`, `entra_member`, `fabric_item_member`) |
| :--- | :--- | :--- |
| **Primary Use Case** | Broad folder or table access with uniform actions across one or more Entra principals. | Fine-grained path boundaries, storage-level RLS row constraints, CLS column constraints, and shortcut delegation. |
| **Path Filtering** | Specified via `paths` attribute (e.g. `["/Tables/sales"]` or `["*"]`). | Configured per `decision_rule.paths` set. |
| **Action Scopes** | `actions` (`Read`, `Write`, `ReadWrite`). Defaults to `["Read"]`. | Configured per `decision_rule.actions` (`Read`, `Write`, `ReadWrite`). |
| **Member Assignment** | `principal_ids` (Set of UUIDs) + `principal_type`. | `entra_member` blocks (`object_id`, `object_type`, `tenant_id`). |
| **Row-Level Security** | Not supported. | Configurable via `decision_rule.row_constraint` (`table_path`, `predicate`). |
| **Column Security** | Not supported. | Configurable via `decision_rule.column_constraint` (`table_path`, `columns`, `action`, `effect`). |
| **Shortcut Delegation** | Not supported. | Dynamic membership inheritance via `fabric_item_member` (`source_path`, `item_access`). |
| **Mutual Exclusivity** | Must specify `paths` and `principal_ids`; omit advanced blocks. | Must omit simple attributes; specify `decision_rule` and member blocks. |

~> **Note:** **Lakehouse Member Type Uniformity**: Microsoft Fabric OneLake Data Access Roles require uniform `principal_type` per role resource. When assigning access to mixed principal types (e.g. both users and service principals) for the same role, assign them via an Entra ID security `Group`.

---

## Authoring Granular Lakehouse Permissions

### 1. Simple Mode Example

For straightforward table access assigned to an Entra security group:

```hcl
resource "fabricext_lakehouse_permission" "sales_analysts_simple" {
  workspace_id   = "00000000-0000-0000-0000-000000000001"
  lakehouse_id   = "00000000-0000-0000-0000-000000000002"
  role_name      = "SalesTableReaders"
  paths          = ["/Tables/sales"]
  actions        = ["Read"]
  principal_ids  = ["11111111-1111-1111-1111-111111111111"]
  principal_type = "Group"
}
```

### 2. Advanced Mode: Path Filters, RLS, and Column Security

For granular security restricting access to `/Tables/sales` with regional row filtering and explicit column restrictions:

```hcl
resource "fabricext_lakehouse_permission" "sales_analysts_advanced" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "00000000-0000-0000-0000-000000000002"
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
}
```

### 3. Cross-Workspace Shortcut Delegation

When a Lakehouse contains OneLake shortcuts referencing data in another workspace or item, use `fabric_item_member` to dynamically inherit access:

```hcl
resource "fabricext_lakehouse_permission" "cross_workspace_readers" {
  workspace_id = "00000000-0000-0000-0000-000000000001"
  lakehouse_id = "00000000-0000-0000-0000-000000000002"
  role_name    = "ShortcutDelegatedAccess"

  # Dynamically inherits role membership from principals who hold
  # "ReadAll" permissions on source Lakehouse B in Workspace B:
  fabric_item_member {
    source_path = "00000000-0000-0000-0000-000000000002/33333333-3333-3333-3333-333333333333"
    item_access = ["ReadAll"]
  }

  decision_rule {
    effect  = "Permit"
    paths   = ["/Tables/shared_sales"]
    actions = ["Read"]
  }
}
```

---

## Concurrency & Mutex Semantics

Updating OneLake Data Access Roles involves a `GET` $\rightarrow$ `PUT` Read-Modify-Write cycle with `If-Match` ETag headers.

Under parallel Terraform execution (`for_each`), concurrent requests mutating roles on the same Lakehouse can result in HTTP 412 (`Precondition Failed`). The `fabricext` provider protects this lifecycle with:

1. **Per-Lakehouse Mutex Locking**: Synchronizes goroutines modifying roles on the same Lakehouse.
2. **Exponential Backoff on ETag Mismatch**: Automatically retries on HTTP 412 conflicts, refreshing the current ETag and reapplying modifications safely.

---

## Related Guides

- **[Use Cases Overview](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_overview)**: Master comparison matrix and 5-layer perimeter overview.
- **[Security Controls Inventory & Interaction Matrix](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_controls_and_interactions)**: Precedence rules and override behaviors across tiers.
- **[Warehouse Schema Isolation & Declarative RBAC](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_warehouse_schema_isolation)**: Isolating schemas using `role_type = "read"` and T-SQL.
- **[Power BI Identity Flow: Direct Lake vs. DirectQuery](https://registry.terraform.io/providers/jambazid/fabricext/latest/docs/guides/use_case_powerbi_identity_propagation)**: End-to-end token flow to semantic models.
