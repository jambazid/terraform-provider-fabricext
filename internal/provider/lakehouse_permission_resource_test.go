// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

type stateCheckFunc func(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse)

func (f stateCheckFunc) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	f(ctx, req, resp)
}

func TestAccLakehousePermissionResource_CRUDAndImport(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"
	principalID := "77777777-7777-7777-7777-777777777771"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "raw_bronze_lh",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Create OneLake Data Access Role
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = [%q]
  principal_type = "Group"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "id", wsID+"/"+lhID+"/BronzeReaders"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "lakehouse_id", lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "paths.#", "2"),
				),
			},
			// 2. Update paths in-place
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers", "/Tables/orders", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = [%q]
  principal_type = "Group"
}
`, wsID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.test", "paths.#", "3"),
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

func TestAccLakehousePermissionResource_ParallelForEachAndETagConflict(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "raw_bronze_lh",
		Type:        "Lakehouse",
	})

	// Inject 2 ETag 412 Precondition Failed conflicts during the parallel for_each apply.
	srv.InjectETagConflicts(wsID, lhID, 2)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
locals {
  roles = {
    FinanceReaders = {
      paths        = ["/Tables/finance_ledger"]
      principal_id = "88888888-8888-8888-8888-000000000001"
    }
    MarketingReaders = {
      paths        = ["/Tables/campaigns"]
      principal_id = "88888888-8888-8888-8888-000000000002"
    }
    InventoryReaders = {
      paths        = ["/Tables/stock_levels"]
      principal_id = "88888888-8888-8888-8888-000000000003"
    }
  }
}

resource "fabricext_lakehouse_permission" "batch" {
  for_each = local.roles

  workspace_id   = %q
  lakehouse_name = "raw_bronze_lh"
  role_name      = each.key
  paths          = each.value.paths
  principal_ids  = [each.value.principal_id]
  principal_type = "Group"
}
`, wsID),
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckFunc(func(_ context.Context, _ statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
						roles := srv.GetDataAccessRoles(wsID, lhID)
						names := make([]string, 0, len(roles))
						for _, r := range roles {
							names = append(names, r.Name)
						}
						for _, expected := range []string{"DefaultReader", "ConcurrentSiblingRole", "FinanceReaders", "MarketingReaders", "InventoryReaders"} {
							if !slices.Contains(names, expected) {
								resp.Error = fmt.Errorf("expected Lakehouse role %q to be preserved, got %v", expected, names)
								return
							}
						}
					}),
				},
			},
			// Destroy the for_each batch while verifying DefaultReader and ConcurrentSiblingRole remain intact
			{
				Config: testAccProviderConfig(srv),
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckFunc(func(_ context.Context, _ statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
						roles := srv.GetDataAccessRoles(wsID, lhID)
						names := make([]string, 0, len(roles))
						for _, r := range roles {
							names = append(names, r.Name)
						}
						for _, expected := range []string{"DefaultReader", "ConcurrentSiblingRole"} {
							if !slices.Contains(names, expected) {
								resp.Error = fmt.Errorf("expected unmanaged role %q to remain after destroy, got %v", expected, names)
								return
							}
						}
						for _, deleted := range []string{"FinanceReaders", "MarketingReaders", "InventoryReaders"} {
							if slices.Contains(names, deleted) {
								resp.Error = fmt.Errorf("expected managed role %q to be deleted, got %v", deleted, names)
								return
							}
						}
					}),
				},
			},
		},
	})
}

func TestAccLakehousePermissionResource_Disappears(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "raw_bronze_lh",
		Type:        "Lakehouse",
	})

	cfg := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "test" {
  workspace_id   = %q
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers"]
  principal_ids  = ["77777777-7777-7777-7777-777777777771"]
  principal_type = "ManagedIdentity"
}
`, wsID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.RemoveDataAccessRole(wsID, lhID, "BronzeReaders")
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.RemoveItem(wsID, lhID)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccLakehousePermissionResource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "invalid_role_name" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "123InvalidStart"
  paths          = ["/Tables/customers"]
  principal_ids  = ["77777777-7777-7777-7777-777777777771"]
}
`,
				ExpectError: regexp.MustCompile(`must start with a letter`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "empty_paths" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = []
  principal_ids  = ["77777777-7777-7777-7777-777777777771"]
}
`,
				ExpectError: regexp.MustCompile(`Attribute paths set must contain at least 1 elements`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "missing_definition" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
}
`,
				ExpectError: regexp.MustCompile(`Missing Role Definition`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "conflicting_definition" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "raw_bronze_lh"
  role_name      = "BronzeReaders"
  paths          = ["/Tables/customers"]
  principal_ids  = ["77777777-7777-7777-7777-777777777771"]

  decision_rule {
    paths = ["/Tables/sales"]
  }
}
`,
				ExpectError: regexp.MustCompile(`Conflicting Role Definition`),
			},
		},
	})
}

func TestAccLakehousePermissionResource_SimpleAndAdvancedParity(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"
	principalID := "77777777-7777-7777-7777-777777777771"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "raw_bronze_lh",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// 1. Simple flat mode using direct lakehouse_id
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "parity" {
  workspace_id   = %q
  lakehouse_id   = %q
  role_name      = "ParityRole"
  paths          = ["/Tables/customers", "/Files/landing"]
  actions        = ["Read"]
  principal_ids  = [%q]
  principal_type = "Group"
}
`, wsID, lhID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "id", wsID+"/"+lhID+"/ParityRole"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "lakehouse_id", lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "lakehouse_name", "raw_bronze_lh"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "paths.#", "2"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "actions.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "principal_ids.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "principal_type", "Group"),
				),
			},
			// 2. In-place update to advanced structured mode with RLS and CLS
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "parity" {
  workspace_id = %q
  lakehouse_id = %q
  role_name    = "ParityRole"
  kind         = "Policy"

  decision_rule {
    paths   = ["/Tables/customers"]
    actions = ["Read"]
    effect  = "Permit"

    row_constraint {
      table_path = "/Tables/customers"
      predicate  = "Region = 'EMEA'"
    }

    column_constraint {
      table_path = "/Tables/customers"
      columns    = ["customer_id", "email"]
      action     = "Read"
      effect     = "Permit"
    }
  }

  entra_member {
    object_id   = %q
    object_type = "Group"
  }
}
`, wsID, lhID, principalID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "id", wsID+"/"+lhID+"/ParityRole"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "kind", "Policy"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.paths.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.row_constraint.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.row_constraint.0.table_path", "/Tables/customers"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.row_constraint.0.predicate", "Region = 'EMEA'"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.column_constraint.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.column_constraint.0.table_path", "/Tables/customers"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "decision_rule.0.column_constraint.0.columns.#", "2"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "entra_member.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "entra_member.0.object_id", principalID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.parity", "entra_member.0.object_type", "Group"),
				),
			},
			// 3. ImportState verification of advanced mode
			{
				ResourceName:      "fabricext_lakehouse_permission.parity",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakehousePermissionResource_MixedMembersAndShortcuts(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"
	memberUser := "77777777-7777-7777-7777-777777777771"
	memberSP := "77777777-7777-7777-7777-777777777772"
	shortcutSource := "11111111-1111-1111-1111-111111111111/88888888-8888-8888-8888-888888888888"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "gold_analytics_lh",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_permission" "mixed" {
  workspace_id   = %q
  lakehouse_name = "gold_analytics_lh"
  role_name      = "MixedRole"

  decision_rule {
    paths   = ["/Tables/sales"]
    actions = ["Read"]
  }

  decision_rule {
    paths   = ["/Tables/dim_date", "/Tables/dim_geo"]
    actions = ["Read"]
  }

  entra_member {
    object_id   = %q
    object_type = "User"
  }

  entra_member {
    object_id   = %q
    object_type = "ServicePrincipal"
    tenant_id   = "77777777-7777-7777-7777-777777777777"
  }

  fabric_item_member {
    source_path = %q
    item_access = ["ReadAll"]
  }
}
`, wsID, memberUser, memberSP, shortcutSource),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "id", wsID+"/"+lhID+"/MixedRole"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "lakehouse_id", lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "decision_rule.#", "2"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "entra_member.#", "2"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "fabric_item_member.#", "1"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_permission.mixed", "fabric_item_member.0.source_path", shortcutSource),
				),
			},
			{
				ResourceName:      "fabricext_lakehouse_permission.mixed",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
