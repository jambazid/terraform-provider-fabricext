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
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/pbimock"
)

func TestAccLakehousePermissionResource_CRUDAndImport(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "curated_lakehouse",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create with lakehouse_name and subset permissions
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_name = "curated_lakehouse"
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "id", fmt.Sprintf("%s/%s/User/%s", wsID, lhID, principalID)),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "lakehouse_id", lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "lakehouse_name", "curated_lakehouse"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "permissions.#", "2"),
				),
			},
			// 2. Update permissions in-place (expand to ReadAll, Reshare)
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}
`, wsID, lhID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "permissions.#", "4"),
				),
			},
			// 3. ImportState verification
			{
				ResourceName:      "fabricext_lakehouse_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakehousePermissionResource_DriftDetectionAndRecreation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "bronze_lakehouse",
		Type:        "Lakehouse",
	})

	cfg := testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
}
`, wsID, lhID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Initial creation, then simulate item deletion in Fabric
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "lakehouse_name", "bronze_lakehouse"),
					func(_ *terraform.State) error {
						fabricSrv.RemoveItem(wsID, lhID)
						return nil
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccLakehousePermissionResource_TypeMismatchError(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "55555555-5555-5555-5555-555555555555"
	principalID := "33333333-3333-3333-3333-333333333333"

	// Upsert as Warehouse, not Lakehouse
	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
}
`, wsID, whID, principalID),
				ExpectError: regexp.MustCompile(`Item Type Mismatch`),
			},
		},
	})
}

func TestAccLakehousePermissionResource_NotificationAndValidation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "notify_lakehouse",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Validation error when message is set without notify=true
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
  message        = "Invalid without notify"
}
`, wsID, lhID, principalID),
				ExpectError: regexp.MustCompile(`The 'message' attribute requires 'notify = true'`),
			},
			// 2. Validation error when permissions set does not contain Read
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["ReadData"]
}
`, wsID, lhID, principalID),
				ExpectError: regexp.MustCompile(`The 'permissions' set must contain 'Read'`),
			},
			// 3. Successful create with notify=true and message
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
  notify         = true
  message        = "Welcome to lakehouse access!"
}
`, wsID, lhID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "message", "Welcome to lakehouse access!"),
				),
			},
			// 4. Update message and permissions in-place
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll"]
  notify         = true
  message        = "Updated lakehouse access permissions."
}
`, wsID, lhID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "message", "Updated lakehouse access permissions."),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "permissions.#", "3"),
				),
			},
		},
	})
}
