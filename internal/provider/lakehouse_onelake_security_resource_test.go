// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestAccLakehouseOnelakeSecurity_basic(t *testing.T) {
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
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id   = %q
  lakehouse_name = "raw_bronze_lh"
}
`, wsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "id", wsID+"/"+lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "workspace_id", wsID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "lakehouse_id", lhID),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "lakehouse_name", "raw_bronze_lh"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "remove_default_reader_role", "true"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "false"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "enabled", "true"),
				),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_purgeDefault(t *testing.T) {
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
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = true
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "false"),
					func(_ *terraform.State) error {
						roles := srv.GetDataAccessRoles(wsID, lhID)
						for _, r := range roles {
							if r.Name == "DefaultReader" {
								return fmt.Errorf("expected DefaultReader to be purged from mock, but it was found")
							}
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_retainDefault(t *testing.T) {
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
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = false
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "remove_default_reader_role", "false"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "true"),
					func(_ *terraform.State) error {
						roles := srv.GetDataAccessRoles(wsID, lhID)
						for _, r := range roles {
							if r.Name == "DefaultReader" {
								return nil
							}
						}
						return fmt.Errorf("expected DefaultReader to be retained in mock, but it was missing")
					},
				),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_unenabledFailsLoud(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	// Item registered without EnableLakehouseSecurity
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "unenabled_lh",
		Type:        "Lakehouse",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id = %q
  lakehouse_id = %q
}
`, wsID, lhID),
				ExpectError: regexp.MustCompile(`OneLake Data Access Security Not Enabled`),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_driftRemoval(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "drift_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	cfg := testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id = %q
  lakehouse_id = %q
}
`, wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(_ *terraform.State) error {
					srv.SetLakehouseSecurityEnabled(wsID, lhID, false)
					return nil
				},
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig: func() {
					srv.EnableLakehouseSecurity(wsID, lhID)
				},
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

func TestAccLakehouseOnelakeSecurity_blockReversion(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "reversion_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = true
}
`, wsID, lhID),
			},
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = false
}
`, wsID, lhID),
				ExpectError: regexp.MustCompile(`Cannot revert 'remove_default_reader_role' from true to false`),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_deleteWarning(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "delete_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id = %q
  lakehouse_id = %q
}
`, wsID, lhID),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_import(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "import_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id   = %q
  lakehouse_name = "import_lh"
}
`, wsID),
			},
			{
				ResourceName:      "fabricext_lakehouse_onelake_security.test",
				ImportState:       true,
				ImportStateId:     wsID + "/" + lhID,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_updatePurge(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "update_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = false
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "true"),
				),
			},
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = true
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "false"),
				),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_purgeWhenAlreadyAbsent(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "absent_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)
	// Remove DefaultReader out-of-band before resource Create runs
	srv.RemoveDataAccessRole(wsID, lhID, "DefaultReader")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = true
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "false"),
				),
			},
		},
	})
}

func TestLakehouseOnelakeSecurityResource_DeleteWarningDiagnostic(t *testing.T) {
	t.Parallel()

	r := &LakehouseOnelakeSecurityResource{}
	var resp tfsdkresource.DeleteResponse
	r.Delete(context.Background(), tfsdkresource.DeleteRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors on delete, got: %v", resp.Diagnostics.Errors())
	}
	warnings := resp.Diagnostics.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning diagnostic on Delete, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0].Summary(), "OneLake Security Cannot Be Disabled via REST API") {
		t.Fatalf("unexpected warning summary: %s", warnings[0].Summary())
	}
}

func TestAccLakehouseOnelakeSecurity_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "missing_identifiers" {
  workspace_id = %q
}
`, wsID),
				ExpectError: regexp.MustCompile(`At least one of 'lakehouse_id' or 'lakehouse_name' must be configured`),
			},
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "empty_name" {
  workspace_id   = %q
  lakehouse_name = ""
}
`, wsID),
				ExpectError: regexp.MustCompile(`Attribute lakehouse_name string length must be at least 1`),
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_importWithDefaultReader(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "import_retain_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id               = %q
  lakehouse_id               = %q
  remove_default_reader_role = false
}
`, wsID, lhID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "remove_default_reader_role", "false"),
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "default_reader_role_exists", "true"),
				),
			},
			{
				ResourceName:            "fabricext_lakehouse_onelake_security.test",
				ImportState:             true,
				ImportStateId:           wsID + "/" + lhID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"lakehouse_name"},
			},
		},
	})
}

func TestAccLakehouseOnelakeSecurity_renameIdentifier(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID1 := "66666666-6666-6666-6666-666666666666"
	lhID2 := "77777777-7777-7777-7777-777777777777"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID1,
		WorkspaceID: wsID,
		DisplayName: "initial_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID1)

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID2,
		WorkspaceID: wsID,
		DisplayName: "renamed_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID2)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id   = %q
  lakehouse_name = "initial_lh"
}
`, wsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "lakehouse_id", lhID1),
				),
			},
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_onelake_security" "test" {
  workspace_id   = %q
  lakehouse_name = "renamed_lh"
}
`, wsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fabricext_lakehouse_onelake_security.test", "lakehouse_id", lhID2),
				),
			},
		},
	})
}
