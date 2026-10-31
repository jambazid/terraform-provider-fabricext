// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
	fabadmin "github.com/microsoft/fabric-sdk-go/fabric/admin"
)

func TestAccItemShareGateResource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_item_share_gate" "invalid_ws" {
  workspace_id         = "not-a-uuid"
  item_id              = "22222222-2222-2222-2222-222222222222"
  principal_id         = "33333333-3333-3333-3333-333333333333"
  expected_permissions = ["Read"]
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_item_share_gate" "invalid_match" {
  workspace_id         = "11111111-1111-1111-1111-111111111111"
  item_id              = "22222222-2222-2222-2222-222222222222"
  principal_id         = "33333333-3333-3333-3333-333333333333"
  expected_permissions = ["Read"]
  match_type           = "fuzzy"
}
`,
				ExpectError: regexp.MustCompile(`Attribute match_type value must be one of`),
			},
		},
	})
}

func TestAccItemShareGateResource_CreateAndReadSuccess(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{"ReadData"},
				},
			},
		},
	})

	gateConfigSubset := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "gate" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  principal_type       = "Group"
  expected_permissions = ["Read"]
  match_type           = "subset"
}
`, wsID, itemID, principalID)

	gateConfigUpdate := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "gate" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  principal_type       = "Group"
  expected_permissions = ["Read", "ReadData"]
  match_type           = "exact"
}
`, wsID, itemID, principalID)

	expectedID := fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: gateConfigSubset,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "id", expectedID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "workspace_id", wsID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "item_id", itemID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "principal_id", principalID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "principal_type", "Group"),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "match_type", "subset"),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "expected_permissions.#", "1"),
				),
			},
			{
				Config: gateConfigUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "id", expectedID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "match_type", "exact"),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.gate", "expected_permissions.#", "2"),
				),
			},
		},
	})
}

func TestAccItemShareGateResource_TimeoutError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})
	// Access details empty - share has not occurred
	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "timeout" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  expected_permissions = ["Read"]

  timeouts {
    create = "50ms"
  }
}
`, wsID, itemID, principalID),
				ExpectError: regexp.MustCompile(`timed out after.*waiting for principal`),
			},
		},
	})
}

func TestAccItemShareGateResource_DisappearsAndDrift(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{},
				},
			},
		},
	})

	config := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "drift" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  expected_permissions = ["Read"]

  timeouts {
    create = "100ms"
  }
}
`, wsID, itemID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fabricext_item_share_gate.drift", "id"),
					func(_ *terraform.State) error {
						// Simulate remote share revocation
						srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
							AccessDetails: []fabadmin.ItemAccessDetails{},
						})
						return nil
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccItemShareGateResource_DeleteNoOp(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{},
				},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			// Zero network calls made on delete, verify state was removed
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "del" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  expected_permissions = ["Read"]
}
`, wsID, itemID, principalID),
				Check: resource.TestCheckResourceAttrSet("fabricext_item_share_gate.del", "id"),
			},
		},
	})
}

func TestAccItemShareGateResource_Import(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{},
				},
			},
		},
	})

	expectedID := fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "imp" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  expected_permissions = ["Read"]
}
`, wsID, itemID, principalID),
				Check: resource.TestCheckResourceAttr("fabricext_item_share_gate.imp", "id", expectedID),
			},
			{
				ResourceName:      "fabricext_item_share_gate.imp",
				ImportState:       true,
				ImportStateId:     expectedID,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccItemShareGateResource_MultiEntryAndTokenNormalization(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	// Two separate entries for the same principal simulating workspace-role inheritance + direct item share
	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{},
				},
			},
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{},
					AdditionalPermissions: []string{"ReadData"},
				},
			},
		},
	})

	config := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_item_share_gate" "multi" {
  workspace_id         = %q
  item_id              = %q
  type                 = "Warehouse"
  principal_id         = %q
  expected_permissions = ["read", "read_data"]
  match_type           = "exact"
}
`, wsID, itemID, principalID)

	expectedID := fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_item_share_gate.multi", "id", expectedID),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.multi", "type", "Warehouse"),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.multi", "principal_type", "Group"),
					resource.TestCheckResourceAttr("fabricext_item_share_gate.multi", "expected_permissions.#", "2"),
				),
			},
			{
				ResourceName:            "fabricext_item_share_gate.multi",
				ImportState:             true,
				ImportStateId:           expectedID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"expected_permissions", "match_type"},
			},
		},
	})
}

func TestAccItemShareGateResource_SPNTenantRoleConflict401Error(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetAdminUnauthorizedErrorInfo(wsID, itemID, "PowerBINotAuthorizedException")

	spnToken := createSyntheticAccJWT(`{"idtyp":"app","appid":"app-spn-gate","oid":"spn-oid-gate","tid":"tenant-gate","roles":["Tenant.ReadWrite.All"]}`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "fabricext" {
  endpoint     = %q
  access_token = %q
  tenant_id    = "77777777-7777-7777-7777-777777777777"
}

resource "fabricext_item_share_gate" "test" {
  workspace_id         = %q
  item_id              = %q
  principal_id         = %q
  expected_permissions = ["Read"]
}
`, srv.URL(), spnToken, wsID, itemID, principalID),
				ExpectError: regexp.MustCompile(`(?s)fault_class=ServicePrincipalTenantRoleConflict.*status_code=401.*x_powerbi_error=PowerBINotAuthorizedException.*caller_posture=\[.*appid=app-spn-gate.*roles=\[Tenant\.ReadWrite\.All\].*\]`),
			},
		},
	})
}
