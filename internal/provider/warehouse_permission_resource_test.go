// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestAccWarehousePermissionResource_CRUDDowngradeAndImport(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "sales_analytics_wh",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create with role_type = "read"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "sales_analytics_wh"
  principal_id   = %q
  principal_type = "Group"
  role_type      = "read"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "id", wsID+"/"+whID+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "warehouse_id", whID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "role_type", "read"),
				),
			},
			// 2. Upgrade in-place to role_type = "write"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "sales_analytics_wh"
  principal_id   = %q
  principal_type = "Group"
  role_type      = "write"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "role_type", "write"),
					func(_ *terraform.State) error {
						perms := srv.GetPrincipalPermissions(wsID, "Warehouse", whID, principalID)
						if !slices.Contains(perms, "Write") || !slices.Contains(perms, "Read") {
							return fmt.Errorf("expected [Read Write] on server, got %v", perms)
						}
						return nil
					},
				),
			},
			// 3. Downgrade in-place to role_type = "read" (must revoke Write)
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "sales_analytics_wh"
  principal_id   = %q
  principal_type = "Group"
  role_type      = "read"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "role_type", "read"),
					func(_ *terraform.State) error {
						perms := srv.GetPrincipalPermissions(wsID, "Warehouse", whID, principalID)
						if !slices.Equal(perms, []string{"Read"}) {
							return fmt.Errorf("expected excess Write permission to be revoked leaving [Read], got %v", perms)
						}
						return nil
					},
				),
			},
			// 4. Upgrade to role_type = "reshare" and back to "read"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "sales_analytics_wh"
  principal_id   = %q
  principal_type = "Group"
  role_type      = "reshare"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "role_type", "reshare"),
					func(_ *terraform.State) error {
						perms := srv.GetPrincipalPermissions(wsID, "Warehouse", whID, principalID)
						if !slices.Contains(perms, "Reshare") || !slices.Contains(perms, "Read") {
							return fmt.Errorf("expected [Read Reshare] on server, got %v", perms)
						}
						return nil
					},
				),
			},
			// 5. ImportState verification
			{
				ResourceName:      "fabricext_warehouse_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccWarehousePermissionResource_Disappears(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "sales_analytics_wh",
		Type:        "Warehouse",
	})

	cfg := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "sales_analytics_wh"
  principal_id   = %q
  role_type      = "read"
}
`, wsID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.ClearPrincipalPermissions(wsID, "Warehouse", whID, principalID)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.RemoveItem(wsID, whID)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccWarehousePermissionResource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_warehouse_permission" "invalid_role" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "33333333-3333-3333-3333-333333333333"
  role_type      = "admin"
}
`,
				ExpectError: regexp.MustCompile(`Attribute role_type value must be one of`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_warehouse_permission" "invalid_uuid" {
  workspace_id   = "not-a-uuid"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "33333333-3333-3333-3333-333333333333"
  role_type      = "read"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_warehouse_permission" "invalid_principal_type" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  warehouse_name = "sales_analytics_wh"
  principal_id   = "33333333-3333-3333-3333-333333333333"
  principal_type = "InvalidType"
  role_type      = "read"
}
`,
				ExpectError: regexp.MustCompile(`Attribute principal_type value must be one of`),
			},
		},
	})
}

func TestAccWarehousePermissionResource_DirectIDReference(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "direct_wh",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "direct" {
  workspace_id = %q
  warehouse_id = %q
  principal_id = %q
  role_type    = "read"
}
`, wsID, whID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.direct", "id", wsID+"/"+whID+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.direct", "warehouse_id", whID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.direct", "warehouse_name", "direct_wh"),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.direct", "principal_type", "Group"),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.direct", "role_type", "read"),
				),
			},
			{
				ResourceName:      "fabricext_warehouse_permission.direct",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
