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

func TestAccSQLDatabasePermissionResource_CRUDAndImport(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "operational_sqldb",
		Type:        "SQLDatabase",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create with sql_database_name and subset permissions
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_sqldb"
  principal_id      = %q
  principal_type    = "User"
  permissions       = ["Read", "ReadData"]
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "id", fmt.Sprintf("%s/%s/User/%s", wsID, dbID, principalID)),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "sql_database_id", dbID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "sql_database_name", "operational_sqldb"),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "permissions.#", "2"),
				),
			},
			// 2. Update permissions in-place (expand to Execute, Reshare)
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_id   = %q
  principal_id      = %q
  principal_type    = "User"
  permissions       = ["Read", "ReadData", "Execute", "Reshare"]
}
`, wsID, dbID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "permissions.#", "4"),
				),
			},
			// 3. ImportState verification
			{
				ResourceName:      "fabricext_sql_database_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_DriftDetectionAndRecreation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "finance_sqldb",
		Type:        "SQLDatabase",
	})

	cfg := testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id    = %q
  sql_database_id = %q
  principal_id    = %q
  principal_type  = "User"
  permissions     = ["Read"]
}
`, wsID, dbID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Initial creation, then simulate item deletion in Fabric
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "sql_database_name", "finance_sqldb"),
					func(_ *terraform.State) error {
						fabricSrv.RemoveItem(wsID, dbID)
						return nil
					},
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_TypeMismatchError(t *testing.T) {
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

	// Upsert as Warehouse, not SQLDatabase
	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "finance_warehouse",
		Type:        "Warehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id    = %q
  sql_database_id = %q
  principal_id    = %q
  principal_type  = "User"
  permissions     = ["Read"]
}
`, wsID, whID, principalID),
				ExpectError: regexp.MustCompile(`Item Type Mismatch`),
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_NotificationAndValidation(t *testing.T) {
	t.Parallel()

	fabricSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer pbiSrv.Close()

	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabricSrv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "notify_database",
		Type:        "SQLDatabase",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Validation error when message is set without notify=true
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id    = %q
  sql_database_id = %q
  principal_id    = %q
  principal_type  = "User"
  permissions     = ["Read"]
  message         = "Invalid without notify"
}
`, wsID, dbID, principalID),
				ExpectError: regexp.MustCompile(`The 'message' attribute requires 'notify = true'`),
			},
			// 2. Successful create with notify=true and message
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id    = %q
  sql_database_id = %q
  principal_id    = %q
  principal_type  = "User"
  permissions     = ["Read", "ReadData"]
  notify          = true
  message         = "Welcome to SQL database access!"
}
`, wsID, dbID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "message", "Welcome to SQL database access!"),
				),
			},
			// 3. Update message and permissions in-place
			{
				Config: testAccProviderConfigWithPBI(fabricSrv, pbiSrv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id    = %q
  sql_database_id = %q
  principal_id    = %q
  principal_type  = "User"
  permissions     = ["Read", "ReadData", "Execute"]
  notify          = true
  message         = "Updated SQL database permissions."
}
`, wsID, dbID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "notify", "true"),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "message", "Updated SQL database permissions."),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "permissions.#", "3"),
				),
			},
		},
	})
}
