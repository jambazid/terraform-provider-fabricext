---
name: OneLake Security Resource (fabricext_lakehouse_onelake_security)
description: Declarative Terraform Plugin Framework resource for Lakehouse OneLake Data Access Security enablement verification and DefaultReader role management
targets:
  - ../internal/provider/lakehouse_onelake_security_resource.go
  - ../internal/provider/lakehouse_onelake_security_resource_test.go
---

# OneLake Security Resource

## HCL Contract

```hcl
resource "fabricext_lakehouse_onelake_security" "example" {
  workspace_id               = "00000000-0000-0000-0000-000000000001"
  lakehouse_id               = "22222222-2222-2222-2222-222222222222"
  remove_default_reader_role = true
}
```

## Enablement Verification

- `Create` calls `GET /v1/workspaces/{workspace_id}/items/{lakehouse_id}/dataAccessRoles`
- When security is enabled, sets `enabled = true` and populates `default_reader_role_exists`
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_basic`
- When security is not enabled (`OneLakeSecurityNotEnabledError`), halts immediately with an actionable diagnostic directing operators to the Fabric portal
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_unenabledFailsLoud`

## DefaultReader Lifecycle

- When `remove_default_reader_role = true` and `DefaultReader` is present, purges it during creation via mutex-serialized, ETag-guarded Read-Modify-Write
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_purgeDefault`
- When `remove_default_reader_role = false`, preserves `DefaultReader` in remote roles
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_retainDefault`
- When `remove_default_reader_role = true` but `DefaultReader` is already absent, succeeds idempotently without error
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_purgeWhenAlreadyAbsent`
- When `remove_default_reader_role = true` and `DefaultReader` reappears out-of-band, `Update` purges `DefaultReader` to self-heal drift
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_updatePurge`

## Reversion Prevention

- `ModifyPlan` blocks reverting `remove_default_reader_role` from `true` to `false` with a diagnostic error because Fabric provides no API to restore purged `DefaultReader` roles
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_blockReversion`

## Dual Identifier Reconciliation

- Resource accepts either `lakehouse_id` (UUID) or `lakehouse_name` (display name), enforcing that at least one identifier is configured via `ValidateConfig`
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_ValidationErrors`
- `ModifyPlan` calls `reconcileItemIdentifiersPlan` so renames cleanly reset stale computed IDs to unknown
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_renameIdentifier`

## Drift Detection

- `Read` inspects remote roles and updates `default_reader_role_exists` to reflect out-of-band role additions or deletions
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_updatePurge`
- If the Lakehouse is deleted (404) or security is unenabled (`OneLakeSecurityNotEnabledError`), `Read` calls `resp.State.RemoveResource(ctx)` cleanly so Terraform plans recreation
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_driftRemoval`

## Delete Semantics

- `Delete` emits an informational warning diagnostic that Fabric cannot disable OneLake security once activated, removing the resource cleanly from Terraform state
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_deleteWarning`

## Import Semantics

- `ImportState` imports `{workspace_id}/{lakehouse_id}`, validates enablement, and sets `remove_default_reader_role = false` if `DefaultReader` exists, or `true` if absent
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_import`

## Dependency Ordering

- Downstream `fabricext_lakehouse_permission` resources declare an explicit `depends_on` referencing `fabricext_lakehouse_onelake_security` across all Lakehouses to ensure enablement verification and `DefaultReader` purging execute before role mutations
  `[@test] ../internal/provider/lakehouse_onelake_security_resource_test.go::TestAccLakehouseOnelakeSecurity_basic`
