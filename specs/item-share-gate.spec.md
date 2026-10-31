---
name: Fabric Item Share Verification Gate Resource (fabricext_item_share_gate)
description: Declarative verification waiter polling the Microsoft Fabric Admin API until an item share propagates to a named principal
targets:
  - ../internal/provider/item_share_gate_resource.go
  - ../internal/provider/item_share_gate_resource_test.go
---

# Fabric Item Share Verification Gate (`fabricext_item_share_gate`)

## HCL Contract

```hcl
resource "fabricext_item_share_gate" "example" {
  workspace_id         = "00000000-0000-0000-0000-000000000001"
  item_id              = "00000000-0000-0000-0000-000000000002"
  principal_id         = "11111111-1111-1111-1111-111111111111"
  principal_type       = "Group"
  expected_permissions = ["Read", "ReadData"]
  match_type           = "subset"

  timeouts {
    create = "60m"
    read   = "10m"
    update = "60m"
    delete = "1m"
  }
}
```

## Lifecycle & Validation Behavior

- Validates UUIDs for `workspace_id`, `item_id`, `principal_id`, validates `match_type` in `OneOf("subset", "exact")`, validates `principal_type` in `OneOf("User", "Group", "ServicePrincipal", "ManagedIdentity")`, and requires replacement on identity changes
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_ValidationErrors`
- Create polls the Fabric Admin API with backoff (5s initial backoff, 2s jitter floor, max 300s) until the named principal holds the expected permissions (subset or exact match) and records state
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_CreateAndReadSuccess`
- Create fails with an actionable diagnostic instructing the operator to complete the share in the Fabric portal if polling times out
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_TimeoutError`
- Read removes the resource from state without error diagnostics when the item or principal share disappears, enabling clean drift detection and destroy
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_DisappearsAndDrift`
- Delete is a pure state removal that performs zero network requests and never revokes shares
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_DeleteNoOp`
- ImportState verifies the existing item share using composite identifier `{workspace_id}/{item_id}/{principal_id}` and adopts state
  `[@test] ../internal/provider/item_share_gate_resource_test.go::TestAccItemShareGateResource_Import`
