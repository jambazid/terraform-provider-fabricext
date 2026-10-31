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

func TestAccWarehousePermissionResource_CRUDAndImport(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "analytics_warehouse",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create with warehouse_name and subset permissions
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_name = "analytics_warehouse"
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "id", fmt.Sprintf("%s/%s/User/%s", wsID, whID, principalID)),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "warehouse_id", whID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "warehouse_name", "analytics_warehouse"),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "permissions.#", "2"),
				),
			},
			// 2. Update permissions in-place (expand to Reshare)
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll", "Reshare"]
}
`, wsID, whID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "permissions.#", "4"),
				),
			},
			// 3. ImportState verification
			{
				ResourceName:      "fabricext_warehouse_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccWarehousePermissionResource_DriftDetectionAndRecreation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "finance_warehouse",
		Type:        "Warehouse",
	})

	cfg := testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
}
`, wsID, whID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Initial creation, then simulate item deletion in Fabric
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "warehouse_name", "finance_warehouse"),
					func(_ *terraform.State) error {
						fabricSrv.RemoveItem(wsID, whID)
						return nil
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccWarehousePermissionResource_TypeMismatchError(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "44444444-4444-4444-4444-444444444444"
	principalID := "33333333-3333-3333-3333-333333333333"

	// Upsert as Lakehouse, not Warehouse
	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "bronze_lakehouse",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
}
`, wsID, lhID, principalID),
				ExpectError: regexp.MustCompile(`Item Type Mismatch`),
			},
		},
	})
}

func TestAccWarehousePermissionResource_NotificationAndValidation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "notify_warehouse",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Validation error when message is set without notify=true
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read"]
  message        = "Invalid without notify"
}
`, wsID, whID, principalID),
				ExpectError: regexp.MustCompile(`The 'message' attribute requires 'notify = true'`),
			},
			// 2. Validation error when permissions set does not contain Read
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["ReadData"]
}
`, wsID, whID, principalID),
				ExpectError: regexp.MustCompile(`The 'permissions' set must contain 'Read'`),
			},
			// 3. Successful create with notify=true and message
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
  notify         = true
  message        = "Welcome to warehouse access!"
}
`, wsID, whID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "message", "Welcome to warehouse access!"),
				),
			},
			// 4. Update message and permissions in-place
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData", "ReadAll"]
  notify         = true
  message        = "Updated warehouse access permissions."
}
`, wsID, whID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "message", "Updated warehouse access permissions."),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "permissions.#", "3"),
				),
			},
		},
	})
}

func TestAccWarehousePermissionResource_SPNClusterDiscoveryCleanCRUD(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	// Simulate portal being forbidden (returns HTTP 403 on GET /)
	pbiSrv.SetPortalForbidden(true)

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "finance_warehouse",
		Type:        "Warehouse",
	})

	spnToken := createSyntheticAccJWT(`{"idtyp":"app","appid":"app-spn-acc","oid":"spn-oid-acc","tid":"tenant-acc","roles":["Tenant.ReadWrite.All"]}`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithTokens(fabricSrv, pbiSrv, spnToken, spnToken) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
}
`, wsID, whID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "id", fmt.Sprintf("%s/%s/User/%s", wsID, whID, principalID)),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "warehouse_id", whID),
					resource.TestCheckResourceAttr("fabricext_warehouse_permission.test", "permissions.#", "2"),
					func(_ *terraform.State) error {
						if pbiSrv.PortalRequestCount() != 0 {
							return fmt.Errorf("expected 0 portal requests for app-only SPN token, got %d", pbiSrv.PortalRequestCount())
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccWarehousePermissionResource_SPN401ErrorDiagnostics(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "finance_warehouse",
		Type:        "Warehouse",
	})

	pbiSrv.SetAccessAuthError(401, "PowerBINotAuthorizedException")
	spnToken := createSyntheticAccJWT(`{"idtyp":"app","appid":"app-spn-acc","oid":"spn-oid-acc","tid":"tenant-acc","roles":["Tenant.ReadWrite.All"]}`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithTokens(fabricSrv, pbiSrv, spnToken, spnToken) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = %q
  warehouse_id   = %q
  principal_id   = %q
  principal_type = "User"
  permissions    = ["Read", "ReadData"]
}
`, wsID, whID, principalID),
				ExpectError: regexp.MustCompile(`(?s)fault_class=ServicePrincipalTenantRoleConflict.*x_powerbi_error=PowerBINotAuthorizedException.*caller_posture=\[.*appid=app-spn-acc.*roles=\[Tenant\.ReadWrite\.All\].*\]`),
			},
		},
	})
}
