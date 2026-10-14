// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestAccSQLDatabasePermissionResource_DeprecatedCreateFails(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	tfresource.Test(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "test" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "orders_db"
  principal_id      = "33333333-3333-3333-3333-333333333333"
  principal_type    = "Group"
  role_type         = "read"
}
`,
				ExpectError: regexp.MustCompile(`Resource Not Supported by Microsoft Fabric`),
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_ValidationErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "invalid_role" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "orders_db"
  principal_id      = "33333333-3333-3333-3333-333333333333"
  role_type         = "admin"
}
`,
				ExpectError: regexp.MustCompile(`Attribute role_type value must be one of`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "invalid_uuid" {
  workspace_id      = "not-a-uuid"
  sql_database_name = "orders_db"
  principal_id      = "33333333-3333-3333-3333-333333333333"
  role_type         = "read"
}
`,
				ExpectError: regexp.MustCompile(`must be a valid UUID`),
			},
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "invalid_principal_type" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "orders_db"
  principal_id      = "33333333-3333-3333-3333-333333333333"
  principal_type    = "InvalidType"
  role_type         = "read"
}
`,
				ExpectError: regexp.MustCompile(`Attribute principal_type value must be one of`),
			},
		},
	})
}

func TestSQLDatabasePermissionResource_TombstoneLifecycle(t *testing.T) {
	t.Parallel()

	res := NewSQLDatabasePermissionResource()
	ctx := context.Background()

	// 1. Schema check
	var schemaResp resource.SchemaResponse
	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Schema.DeprecationMessage == "" {
		t.Fatalf("expected non-empty DeprecationMessage on schema")
	}

	// 2. Read preserves state
	stateModel := SQLDatabasePermissionResourceModel{
		ID:              types.StringValue("11111111-1111-1111-1111-111111111111/44444444-4444-4444-4444-444444444444/Group/33333333-3333-3333-3333-333333333333"),
		WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
		SQLDatabaseName: types.StringValue("orders_db"),
		SQLDatabaseID:   types.StringValue("44444444-4444-4444-4444-444444444444"),
		PrincipalID:     types.StringValue("33333333-3333-3333-3333-333333333333"),
		PrincipalType:   types.StringValue("Group"),
		RoleType:        types.StringValue("read"),
	}

	readReq := resource.ReadRequest{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}
	readResp := resource.ReadResponse{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}
	readReq.State.Set(ctx, &stateModel)
	res.Read(ctx, readReq, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error in Read: %v", readResp.Diagnostics.Errors())
	}
	var readBack SQLDatabasePermissionResourceModel
	readResp.State.Get(ctx, &readBack)
	if readBack.ID.ValueString() != stateModel.ID.ValueString() {
		t.Fatalf("expected state preserved in Read, got %s", readBack.ID.ValueString())
	}

	// 3. Delete removes resource and emits warning
	delReq := resource.DeleteRequest{
		State: readResp.State,
	}
	delResp := resource.DeleteResponse{
		State: tfsdk.State{
			Schema: schemaResp.Schema,
		},
	}
	res.Delete(ctx, delReq, &delResp)
	if delResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error in Delete: %v", delResp.Diagnostics.Errors())
	}
	if len(delResp.Diagnostics.Warnings()) == 0 {
		t.Fatalf("expected warning diagnostic in Delete")
	}
	if !delResp.State.Raw.IsNull() {
		t.Fatalf("expected raw state to be null after Delete.RemoveResource")
	}

	// 4. ImportState returns error
	importRes, ok := res.(resource.ResourceWithImportState)
	if !ok {
		t.Fatalf("expected ResourceWithImportState")
	}
	var importResp resource.ImportStateResponse
	importRes.ImportState(ctx, resource.ImportStateRequest{ID: "foo/bar/baz"}, &importResp)
	if !importResp.Diagnostics.HasError() {
		t.Fatalf("expected ImportState to return error")
	}
}
