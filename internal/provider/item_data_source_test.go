// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestAccItemDataSource_LookupAndTypeIsolation(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222221"
	sqlEpID := "22222222-2222-2222-2222-222222222222"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "shared_gold_item",
		Type:        "Lakehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          sqlEpID,
		WorkspaceID: wsID,
		DisplayName: "shared_gold_item",
		Type:        "SQLEndpoint",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
data "fabricext_item" "lh" {
  workspace_id = %q
  display_name = "shared_gold_item"
  type         = "Lakehouse"
}

data "fabricext_item" "sqlep" {
  workspace_id = %q
  display_name = "shared_gold_item"
  type         = "SQLEndpoint"
}
`, wsID, wsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.fabricext_item.lh", "id", lhID),
					resource.TestCheckResourceAttr("data.fabricext_item.sqlep", "id", sqlEpID),
				),
			},
		},
	})
}

func TestAccItemDataSource_NotFoundError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
data "fabricext_item" "missing" {
  workspace_id = %q
  display_name = "non_existent_warehouse"
  type         = "Warehouse"
}
`, wsID),
				ExpectError: regexp.MustCompile(`not found in workspace`),
			},
		},
	})
}

func TestAccItemDataSource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
data "fabricext_item" "invalid_uuid" {
  workspace_id = "not-a-uuid"
  display_name = "sales_wh"
  type         = "Warehouse"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
			{
				Config: testAccProviderConfig(srv) + `
data "fabricext_item" "invalid_type" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "sales_nb"
  type         = "Notebook"
}
`,
				ExpectError: regexp.MustCompile(`Attribute type value must be one of`),
			},
		},
	})
}
