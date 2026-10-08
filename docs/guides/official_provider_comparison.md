---
page_title: "Official Fabric Provider Comparison & Migration Guide"
subcategory: "Architecture & Migration"
description: |-
  Comparative architecture analysis between Microsoft's official Terraform provider (microsoft/fabric) and the jambazid/fabricext item-sharing stopgap provider, including dual-provider coexistence and Terraform 1.7+ state migration playbooks for Lakehouses, Warehouses, and SQL Databases.
---

# Official Fabric Provider Comparison

Comparative architecture analysis between Microsoft's official Terraform provider ([`microsoft/terraform-provider-fabric`](https://github.com/microsoft/terraform-provider-fabric), `registry.terraform.io/microsoft/fabric`) and this stopgap item-sharing provider (`registry.terraform.io/jambazid/fabricext`).

---

## Executive Summary

| Dimension | `microsoft/fabric` (Official) | `jambazid/fabricext` (Stopgap) | Cost | Impact | Risk |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Primary Scope** | Workspace & item lifecycle provisioning (`fabric_workspace`, `fabric_warehouse`, `fabric_lakehouse`, `fabric_sql_database`, `fabric_workspace_role_assignment`). | Declarative item-level sharing (`fabricext_warehouse_permission`, `fabricext_sql_database_permission`, `fabricext_lakehouse_permission`). | Low | High — fills upstream item-sharing gap for Warehouses and SQL Databases, and provides OneLake Data Access Roles for Lakehouses (clarifying [Issue #425](https://github.com/microsoft/terraform-provider-fabric/issues/425) where Lakehouse item-level sharing has no public API). | Low — zero resource-name collision (`fabricext_*` prefix). |
| **SDK & API Layer** | 100% bound to generated `microsoft/fabric-sdk-go` service clients. Cannot call REST endpoints absent from `microsoft/fabric-rest-api-specs`. | Uses `microsoft/fabric-sdk-go` models + `azidentity` with a targeted REST/RPC client (`internal/client/fabric_client.go`) for item permission and OneLake ETag RMW operations. | Low | High — unblocks `/permissions`, `/grantPermissions`, and `/revokePermissions` before upstream SDK generation lands. | Low — validated against vendored OpenAPI specs + overlay (`specs/openapi/`). |
| **OneLake Data Access Roles** | `fabric_onelake_data_access_security` (Preview-only, requires `preview = true`; affected by `object_type` drift in [Issue #1044](https://github.com/microsoft/terraform-provider-fabric/issues/1044)). | `fabricext_lakehouse_permission` (GA `GET`/`PUT /dataAccessRoles` with per-Lakehouse mutex, `If-Match` ETag RMW, dual-mode simple/advanced RLS & CLS, and direct ID support). | Low | High — safe under parallel `for_each` without preview-mode gating. | Low — shares identical `{workspace_id}/{lakehouse_id}/{role_name}` import ID for zero-destroy migration. |
| **Contract & Acceptance Testing** | Unit tests use compile-time `fabcore/fake` Go struct stubs; acceptance tests require live Azure/Fabric capacities. | All API client tests and `TF_ACC=1` acceptance tests communicating with Fabric endpoints validate wire payloads at runtime through `kin-openapi` (`openapi3filter`) against `specs/openapi/`. | Low | High — catches wire-level schema regressions offline in < 10s. | Low — `mise run specs:sync` detects upstream Swagger drift. |

---

## Upstream Gap Analysis

### Item Permissions Gap

Microsoft Fabric exposes item-level permission endpoints (`GET /permissions`, `POST /grantPermissions`, `POST /revokePermissions`) for Warehouses and SQL Databases, but the official provider does not currently offer resources managing these item-level permissions.

1. **Swagger & SDK Generation Bottleneck**:
   - `microsoft/terraform-provider-fabric` delegates all HTTP calls to `github.com/microsoft/fabric-sdk-go`.
   - `fabric-sdk-go` is strictly code-generated from [`microsoft/fabric-rest-api-specs`](https://github.com/microsoft/fabric-rest-api-specs).
   - Because `warehouse/swagger.json` and `sqlDatabase/swagger.json` do not yet include the `/permissions`, `/grantPermissions`, and `/revokePermissions` path definitions, `fabric-sdk-go` exposes no client methods for them, blocking the official provider.
2. **Additive Grant vs. Declarative Reconciliation**:
   - Fabric's `POST /grantPermissions` endpoint is **additive**: calling `grantPermissions` with `["Read"]` on a principal that currently holds `["Read", "Write", "Reshare"]` leaves `"Write"` and `"Reshare"` intact.
   - `jambazid/fabricext` bridges this gap via a formal OpenAPI overlay (`specs/openapi/overlays/item-permissions.json`) and set-difference reconciliation in `FabricClient.UpdateItemPermissions`:
     - Revokes removed permissions first (`current \ target`) under the Downgrade Revocation Law so downgrades never leave excess privileges.
     - Grants target permissions (`target`) to establish the complete desired state.

```text
Terraform (jambazid/fabricext)                     Fabric REST API (/permissions)
            │                                                      │
  [Plan: reshare (Read, Reshare)]                                  │
  [State: write (Read, Write)]                                     │
  [Diff: revoke Write, grant Read+Reshare]                         │
            │                                                      │
            ├─────── 1. POST /revokePermissions (["Write"]) ──────►│
            │◄────── 200 OK ───────────────────────────────────────┤
            │                                                      │
            ├─────── 2. POST /grantPermissions (["Read","Reshare"])►│
            │◄────── 200 OK ───────────────────────────────────────┤
            │                                                      │
            ├─────── 3. GET /permissions (refresh & verify) ──────►│
            │◄────── 200 OK (verified: ["Read", "Reshare"]) ───────┤
            ▼                                                      ▼
```

-> **Note:** An interactive graphical sequence diagram is rendered natively in the [GitHub Comparison Guide](https://github.com/jambazid/terraform-provider-fabricext/blob/main/guides/official_provider_comparison.md#additive-grant-vs-declarative-reconciliation).

### OneLake Roles Comparison

| Aspect | `microsoft/fabric` (`fabric_onelake_data_access_security`) | `jambazid/fabricext` (`fabricext_lakehouse_permission`) |
| :--- | :--- | :--- |
| **Maturity Gate** | Preview-only; fails at plan/apply unless `provider "fabric" { preview = true }` is set. | Available by default using the GA `GET` and `PUT /dataAccessRoles` endpoints (`platform/swagger.json`). |
| **Resource Granularity & Modes** | Single-mode nested `decision_rules` and `members` blocks. | Dual-mode: simple flat mode (`paths`, `actions`, `principal_ids`, `principal_type`) or advanced structured mode (`decision_rule`, `row_constraint`, `column_constraint`, `entra_member`, `fabric_item_member`). |
| **Item Identification** | Accepts `item_id` (UUID). | Accepts either `lakehouse_id` (UUID) or `lakehouse_name` (display name resolved via type-isolated cache). |
| **Row & Column Level Security (RLS/CLS)** | Supported via `row_constraints` and `column_constraints` in `decision_rules`. | Supported via `row_constraint` and `column_constraint` in `decision_rule` blocks (100% parity). |
| **Mixed Entra Members & Shortcuts** | Supported via `members.microsoft_entra_members` and `fabric_item_members`. | Supported via `entra_member` (heterogeneous types/tenants) and `fabric_item_member` (shortcut inheritance). |
| **Concurrent `for_each` Safety** | Direct API calls without cross-resource in-process mutex coordination. | Keyed per-Lakehouse `sync.Mutex` (`workspaceID + "/" + lakehouseID`) + `If-Match` ETag optimistic concurrency with bounded `412 Precondition Failed` retry. |
| **Read-Back `objectType` Bug ([Issue #1044](https://github.com/microsoft/terraform-provider-fabric/issues/1044))** | Fails with `Provider produced inconsistent result after apply` when the Fabric `GET /dataAccessRoles` API omits `objectType` in `microsoftEntraMembers`. | Preserves `state.PrincipalType` when the API response omits `objectType` on read-back (`internal/provider/lakehouse_permission_resource.go`). |
| **Import ID Format** | `{workspace_id}/{item_id}/{role_name}` | `{workspace_id}/{lakehouse_id}/{role_name}` (100% compatible). |

#### OneLake Roles vs Item Sharing

Practitioners sometimes conflate Fabric item-level sharing with OneLake Data Access Roles:

- **OneLake Data Access Roles** (Storage Layer): Both `microsoft/fabric` (`fabric_onelake_data_access_security`) and `jambazid/fabricext` (`fabricext_lakehouse_permission`) manage OneLake Data Access Roles via the OneLake Data Access Security API (`/v1/.../items/{id}/dataAccessRoles`). These roles govern fine-grained access to Parquet tables, folders, rows (RLS), and columns (CLS) in OneLake storage.
- **Lakehouse Item-Level Sharing** (Workspace Layer): Fabric item-level sharing (`ReadAll` workspace share grants at the Fabric item level) does not currently have a public REST API for Lakehouses, as discussed in [microsoft/terraform-provider-fabric#425](https://github.com/microsoft/terraform-provider-fabric/issues/425). Neither provider manages Lakehouse item-level sharing until Microsoft makes that API available.

---

## Authentication and Dual-Provider Coexistence

Practitioners frequently run `microsoft/fabric` (to provision workspaces and items) and `jambazid/fabricext` (to manage item permissions) side by side in the same root module.

### Credential and Environment Parity

`jambazid/fabricext` supports both standard Azure SDK (`AZURE_*`, `ARM_*`) and official Fabric provider (`FABRIC_*`) environment variables in `internal/credentials/chain.go` and `internal/provider/provider.go`, allowing a single set of CI/CD environment variables to authenticate both providers simultaneously:

| Attribute | `microsoft/fabric` Env Vars | `jambazid/fabricext` Env Vars | Notes |
| :--- | :--- | :--- | :--- |
| `endpoint` | `FABRIC_ENDPOINT` | `FABRIC_ENDPOINT` | Defaults to `https://api.fabric.microsoft.com` (normalized to `/v1` automatically). |
| `tenant_id` | `FABRIC_TENANT_ID` | `AZURE_TENANT_ID`, `ARM_TENANT_ID`, `FABRIC_TENANT_ID` | Entra ID tenant UUID. |
| `client_id` | `FABRIC_CLIENT_ID` | `AZURE_CLIENT_ID`, `ARM_CLIENT_ID`, `FABRIC_CLIENT_ID` | Service Principal / Managed Identity client ID. |
| `client_secret` | `FABRIC_CLIENT_SECRET` | `AZURE_CLIENT_SECRET`, `ARM_CLIENT_SECRET`, `FABRIC_CLIENT_SECRET` | Marked `Sensitive: true` and redacted via `Credentials.String()` / `GoString()`. |
| `use_oidc` | `FABRIC_USE_OIDC` | `AZURE_USE_OIDC`, `ARM_USE_OIDC`, `FABRIC_USE_OIDC` | Uses `azidentity.NewWorkloadIdentityCredential` in GitHub Actions / ADO. |
| `use_msi` | `FABRIC_USE_MSI` | `AZURE_USE_MSI`, `ARM_USE_MSI`, `FABRIC_USE_MSI` | Uses `azidentity.NewManagedIdentityCredential`. |
| `use_cli` | `FABRIC_USE_CLI` | `AZURE_USE_CLI`, `ARM_USE_CLI`, `FABRIC_USE_CLI` | Uses `azidentity.NewAzureCLICredential`. |
| `access_token` | *(Not supported)* | `FABRIC_ACCESS_TOKEN` | Static bearer token override; enables zero-network `TF_ACC=1` testing. |
| `request_timeout` | `FABRIC_TIMEOUT` | `FABRIC_REQUEST_TIMEOUT` | Per-request HTTP timeout (Go duration string, defaults to `"60s"`). |
| `skip_credentials_validation` | *(Not supported)* | `FABRIC_SKIP_CREDENTIALS_VALIDATION` | Skips eager token validation in `Configure()` when set to `true`. |

### Dual-Provider HCL Pattern

Because `jambazid/fabricext` resources accept human-readable item names (`warehouse_name`, `sql_database_name`, `lakehouse_name`) and resolve them via a paginated, type-isolated `(workspaceID, itemType, displayName)` cache, referencing `display_name` from a `microsoft/fabric` resource automatically establishes the Terraform dependency graph edge:

```hcl
terraform {
  required_version = ">= 1.7.0"
  required_providers {
    fabric = {
      source  = "microsoft/fabric"
      version = "~> 1.0"
    }
    fabricext = {
      source  = "jambazid/fabricext"
      version = "~> 0.2.1"
    }
  }
}

provider "fabric" {}
provider "fabricext" {}

resource "fabric_warehouse" "sales" {
  workspace_id = var.workspace_id
  display_name = "sales_wh"
}

resource "fabricext_warehouse_permission" "analysts" {
  provider       = fabricext
  workspace_id   = fabric_warehouse.sales.workspace_id
  warehouse_name = fabric_warehouse.sales.display_name
  principal_id   = var.analysts_group_oid
  principal_type = "Group"
  role_type      = "read"
}
```

---

## Schema and Implementation Idioms

| Pattern | `microsoft/fabric` Convention | `jambazid/fabricext` Convention | Trade-Off & Rationale |
| :--- | :--- | :--- | :--- |
| **Principal Modeling** | Single nested attribute: `principal = { id = "...", type = "Group" }`. | Flat attributes: `principal_id = "..."` and `principal_type = "Group"` (default `"Group"`). | **Cost**: None. **Impact**: Flat attributes simplify `for_each` matrix mapping (`modules/permissions`) and default to Entra Security Groups (`"Group"`). **Risk**: Documented in migration playbook below. |
| **Item Identification** | Requires `item_id` / `warehouse_id` UUID on every resource. | Accepts `warehouse_name` / `sql_database_name` / `lakehouse_name` (`Required`) and populates `*_id` (`Computed`, `UseStateForUnknown`). | **Cost**: 1 cached `GET /items?type={type}` call per workspace/type. **Impact**: Eliminates extra data-source boilerplate when sharing existing items by name. **Risk**: Item renames outside Terraform are detected on `Read` and trigger resource replacement (`RequiresReplace`). |
| **UUID Validation** | Custom Framework type `customtypes.UUID` (`google/uuid`). | `stringvalidator.RegexMatches` canonical UUID regex (`^[0-9a-fA-F]{8}-...$`). | **Cost**: Low. **Impact**: Equivalent plan-time validation with zero third-party custom-type coupling. **Risk**: Case-insensitive comparison handled via `strings.EqualFold` in `FabricClient`. |
| **404 Drift Recovery** | Calls `resp.State.RemoveResource(ctx)` when `Read` returns HTTP `404`. | Calls `resp.State.RemoveResource(ctx)` when `Read` returns HTTP 404 (`client.IsNotFound(err)`). | **100% Aligned** — both providers cleanly plan recreation when an item or role is deleted outside Terraform. |
| **Import State IDs** | Slash-delimited composite UUIDs (`workspace_id/item_id/...`). | Slash-delimited composite IDs (`{workspace_id}/{item_id}/{principal_type}/{principal_id}` and `{workspace_id}/{lakehouse_id}/{role_name}`). | **100% Aligned** — both `ImportState` and `Read` call `GetItemByID` to resolve and refresh the human-readable `*_name` attribute from the Fabric item's `displayName`. |

---

## Testing and Verification Comparison

```text
┌──────────────────────────────────────────────────────────────────────────────────┐
│ microsoft/terraform-provider-fabric (Official)                                   │
│   Unit Tests ──────────► fabcore/fake Go Struct Stubs (No HTTP/JSON validation)  │
│   Acceptance Tests ────► Live Azure & Fabric Capacity Required                   │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────────┐
│ jambazid/terraform-provider-fabricext (Community Extension)                      │
│   Unit & Acceptance ───► fabricmock ──► kin-openapi (openapi3filter)             │
│   (TF_ACC=1 Hermetic)   (In-Memory)     ├── Request & Response Schema Validation │
│                                         └── Vendored specs/openapi/*.json        │
└──────────────────────────────────────────────────────────────────────────────────┘
```

-> **Note:** An interactive graphical flowchart is rendered natively in the [GitHub Comparison Guide](https://github.com/jambazid/terraform-provider-fabricext/blob/main/guides/official_provider_comparison.md#testing-and-verification-comparison).

- **Why `kin-openapi` Contract Validation Matters**:
  - Go struct fakes (`fabcore/fake`) only test that the provider passes Go structs matching the current SDK version; they do not validate URL routing, query parameters, ETag headers (`ETag` / `If-Match`), or JSON wire serialization against the OpenAPI specification.
  - `internal/testutil/fabricmock/server.go` converts the vendored Swagger 2.0 specs (`specs/openapi/`) into OpenAPI v3 via `kin-openapi/openapi2conv` and validates every incoming HTTP request and outgoing HTTP response across `fabricmock`-backed unit tests and acceptance tests (`mise run test:acc` with `TF_ACC=1`).

---

## State Migration Playbook

When `microsoft/fabric` promotes OneLake Data Access Security to GA and adds native Warehouse/SQL Database item-permission resources, practitioners can migrate state with **zero permission revocation or downtime** using Terraform 1.7+ `removed` and `import` blocks.

-> **Tip:** **Retrieving Resolved Item UUIDs Before Migration**: Every `jambazid/fabricext` permission resource stores the resolved Fabric item UUID in state (`lakehouse_id`, `warehouse_id`, or `sql_database_id`). Before replacing a `fabricext_*` resource block, inspect state with `terraform state show` (or reference `data.fabricext_item`) to obtain the item UUID for your `import` block.

### Lakehouse Role Migration

Because `fabricext_lakehouse_permission` and `fabric_onelake_data_access_security` share the exact `{workspace_id}/{lakehouse_id}/{role_name}` composite import ID:

```hcl
# 1. Detach from jambazid/fabricext state without deleting the OneLake role in Fabric
removed {
  from = fabricext_lakehouse_permission.bronze_readers

  lifecycle {
    destroy = false
  }
}

# 2. Declare the target official microsoft/fabric resource
resource "fabric_onelake_data_access_security" "bronze_readers" {
  workspace_id = var.workspace_id
  item_id      = var.lakehouse_id
  role_name    = "BronzeReaders"
  # ... configure decision_rules and members matching BronzeReaders ...
}

# 3. Import the existing OneLake role into the official microsoft/fabric resource
import {
  to = fabric_onelake_data_access_security.bronze_readers
  id = "${var.workspace_id}/${var.lakehouse_id}/BronzeReaders"
}
```

### Warehouse Permission Migration

`fabricext_warehouse_permission` maps `role_type` (`read`, `write`, `reshare`) to Fabric item permissions (`["Read"]`, `["Read", "Write"]`, `["Read", "Reshare"]`). When `microsoft/fabric` ships native Warehouse item permission resources:

-> **Note:** **Illustrative Migration Pseudocode**: The `fabric_item_permission` resource block below is conceptual pseudocode illustrating the Terraform 1.7+ migration pattern. Upstream resource names, schemas, and import ID conventions have not yet been defined by Microsoft.

```hcl
# 1. Detach from jambazid/fabricext state without revoking the Warehouse permission in Fabric
removed {
  from = fabricext_warehouse_permission.analysts

  lifecycle {
    destroy = false
  }
}

# 2. Declare the target official microsoft/fabric item permission resource (hypothetical schema)
resource "fabric_item_permission" "analysts_warehouse" {
  workspace_id = var.workspace_id
  item_id      = var.warehouse_id
  # ... configure principal and permissions matching ["Read"] ...
}

# 3. Import into the official microsoft/fabric resource (syntax subject to upstream specification)
import {
  to = fabric_item_permission.analysts_warehouse
  id = "${var.workspace_id}/${var.warehouse_id}/${var.analysts_group_oid}"
}
```

### SQL Database Permission Migration

`fabricext_sql_database_permission` supports five granular SQL Database permission presets across the TDS SQL endpoint, Spark/OneLake mirror, and event subscriptions:

| `jambazid/fabricext` `role_type` | Underlying Fabric API Permissions | Scope |
| :--- | :--- | :--- |
| `read` | `["Read"]` | Item-level connection metadata without table read access. |
| `read_data` | `["Read", "ReadData"]` | Read data via the SQL Database TDS endpoint (`ReadData`). |
| `read_spark` | `["Read", "ReadAll", "SubscribeOneLakeEvents"]` | Read mirrored OneLake Parquet tables via Spark (`ReadAll`) and subscribe to OneLake events. |
| `write` | `["Read", "Write"]` | Read and modify SQL Database schema and data. |
| `reshare` | `["Read", "Reshare"]` | Read and re-share the SQL Database with additional principals. |

When `microsoft/fabric` ships native SQL Database item permission resources, detach the stopgap resource with `destroy = false` and import the existing SQL Database grant:

-> **Note:** **Illustrative Migration Pseudocode**: The `fabric_item_permission` resource block below is conceptual pseudocode illustrating the Terraform 1.7+ migration pattern. Upstream resource names, schemas, and import ID conventions have not yet been defined by Microsoft.

```hcl
# 1. Detach from jambazid/fabricext state without revoking the SQL Database permission in Fabric
removed {
  from = fabricext_sql_database_permission.orders_tds_readers

  lifecycle {
    destroy = false
  }
}

# 2. Declare the target official microsoft/fabric item permission resource (hypothetical schema)
resource "fabric_item_permission" "orders_tds_readers" {
  workspace_id = var.workspace_id
  item_id      = var.sql_database_id
  # ... configure principal and permissions matching ["Read", "ReadData"] ...
}

# 3. Import into the official microsoft/fabric resource (syntax subject to upstream specification)
import {
  to = fabric_item_permission.orders_tds_readers
  id = "${var.workspace_id}/${var.sql_database_id}/${var.analysts_group_oid}"
}
```
