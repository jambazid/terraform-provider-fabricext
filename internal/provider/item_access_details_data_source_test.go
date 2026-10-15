// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
	fabadmin "github.com/microsoft/fabric-sdk-go/fabric/admin"
)

func TestAccItemAccessDetailsDataSource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
data "fabricext_item_access_details" "invalid_ws" {
  workspace_id = "not-a-uuid"
  item_id      = "22222222-2222-2222-2222-222222222222"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
			{
				Config: testAccProviderConfig(srv) + `
data "fabricext_item_access_details" "invalid_item" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  item_id      = "invalid-uuid"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
		},
	})
}

func TestAccItemAccessDetailsDataSource_BasicAndProjection(t *testing.T) {
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

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
data "fabricext_item_access_details" "wh" {
  workspace_id = %q
  item_id      = %q
  type         = "Warehouse"
}
`, wsID, itemID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.#", "1"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.principal.id", principalID),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.principal.display_name", principalName),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.principal.type", "Group"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.principal.group_type", "SecurityGroup"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.item_access_details.permissions.#", "1"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.item_access_details.permissions.0", "Read"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.item_access_details.additional_permissions.#", "1"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", "access_details.0.item_access_details.additional_permissions.0", "ReadData"),
					resource.TestCheckResourceAttr("data.fabricext_item_access_details.wh", fmt.Sprintf("principal_permissions.%s.#", principalID), "2"),
				),
			},
		},
	})
}

func TestAccItemAccessDetailsDataSource_AdminForbiddenError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})
	srv.SetAdminForbidden(wsID, itemID, true)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
data "fabricext_item_access_details" "forbidden" {
  workspace_id = %q
  item_id      = %q
}
`, wsID, itemID),
				ExpectError: regexp.MustCompile(`Fabric Admin API Access Denied`),
			},
		},
	})
}

func TestAccItemAccessDetailsDataSource_NotFoundError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	missingItemID := "99999999-9999-9999-9999-999999999999"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
data "fabricext_item_access_details" "missing" {
  workspace_id = %q
  item_id      = %q
}
`, wsID, missingItemID),
				ExpectError: regexp.MustCompile(`not found in workspace`),
			},
		},
	})
}
