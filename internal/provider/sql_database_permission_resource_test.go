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

func TestAccSQLDatabasePermissionResource_CRUDDowngradeAndImport(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "44444444-4444-4444-4444-444444444444"
	principalID := "55555555-5555-5555-5555-555555555555"

	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "operational_orders_db",
		Type:        "SQLDatabase",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create with role_type = "read_data"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  principal_type    = "Group"
  role_type         = "read_data"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "id", wsID+"/"+dbID+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "sql_database_id", dbID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "role_type", "read_data"),
				),
			},
			// 2. Change to role_type = "read_spark"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  principal_type    = "Group"
  role_type         = "read_spark"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "role_type", "read_spark"),
					func(_ *terraform.State) error {
						perms := srv.GetPrincipalPermissions(wsID, "SQLDatabase", dbID, principalID)
						if !slices.Contains(perms, "ReadAll") || !slices.Contains(perms, "SubscribeOneLakeEvents") {
							return fmt.Errorf("expected ReadAll and SubscribeOneLakeEvents on server, got %v", perms)
						}
						if slices.Contains(perms, "ReadData") {
							return fmt.Errorf("expected ReadData to be revoked when switching to read_spark, got %v", perms)
						}
						return nil
					},
				),
			},
			// 3. Downgrade to role_type = "read"
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  principal_type    = "Group"
  role_type         = "read"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "role_type", "read"),
					func(_ *terraform.State) error {
						perms := srv.GetPrincipalPermissions(wsID, "SQLDatabase", dbID, principalID)
						if !slices.Equal(perms, []string{"Read"}) {
							return fmt.Errorf("expected only [Read] after downgrade, got %v", perms)
						}
						return nil
					},
				),
			},
			// 4. Test write and reshare presets
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  principal_type    = "Group"
  role_type         = "write"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "role_type", "write"),
				),
			},
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  principal_type    = "Group"
  role_type         = "reshare"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.test", "role_type", "reshare"),
				),
			},
			// 5. ImportState verification
			{
				ResourceName:      "fabricext_sql_database_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_Disappears(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "44444444-4444-4444-4444-444444444444"
	principalID := "55555555-5555-5555-5555-555555555555"

	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "operational_orders_db",
		Type:        "SQLDatabase",
	})

	cfg := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = %q
  sql_database_name = "operational_orders_db"
  principal_id      = %q
  role_type         = "read_data"
}
`, wsID, principalID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.ClearPrincipalPermissions(wsID, "SQLDatabase", dbID, principalID)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.RemoveItem(wsID, dbID)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "invalid_role" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "operational_orders_db"
  principal_id      = "55555555-5555-5555-5555-555555555555"
  role_type         = "super_admin"
}
`,
				ExpectError: regexp.MustCompile(`Attribute role_type value must be one of`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "invalid_uuid" {
  workspace_id      = "invalid-uuid"
  sql_database_name = "operational_orders_db"
  principal_id      = "55555555-5555-5555-5555-555555555555"
  role_type         = "read_data"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_DirectIDReference(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "44444444-4444-4444-4444-444444444444"
	dbID2 := "55555555-5555-5555-5555-555555555555"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "direct_db",
		Type:        "SQLDatabase",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          dbID2,
		WorkspaceID: wsID,
		DisplayName: "direct_db_2",
		Type:        "SQLDatabase",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "direct" {
  workspace_id      = %q
  sql_database_id   = %q
  principal_id      = %q
  role_type         = "read_data"
}
`, wsID, dbID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "id", wsID+"/"+dbID+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_id", dbID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_name", "direct_db"),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "principal_type", "Group"),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "role_type", "read_data"),
				),
			},
			{
				ResourceName:      "fabricext_sql_database_permission.direct",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Replace resource by switching to sql_database_name only (clears sql_database_id in plan)
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "direct" {
  workspace_id      = %q
  sql_database_name = "direct_db_2"
  principal_id      = %q
  role_type         = "read_data"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "id", wsID+"/"+dbID2+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_id", dbID2),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_name", "direct_db_2"),
				),
			},
			// Step 4: Replace resource by changing sql_database_name (ensures counterpart ID is re-resolved)
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "direct" {
  workspace_id      = %q
  sql_database_name = "direct_db"
  principal_id      = %q
  role_type         = "read_data"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "id", wsID+"/"+dbID+"/Group/"+principalID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_id", dbID),
					resource.TestCheckResourceAttr("fabricext_sql_database_permission.direct", "sql_database_name", "direct_db"),
				),
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_MismatchedIdentifiers(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "correct_db_name",
		Type:        "SQLDatabase",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_sql_database_permission" "mismatch" {
  workspace_id      = %q
  sql_database_id   = %q
  sql_database_name = "wrong_db_name"
  principal_id      = %q
  role_type         = "read_data"
}
`, wsID, dbID, principalID),
				ExpectError: regexp.MustCompile(`Conflicting SQL Database Identifiers`),
			},
		},
	})
}
