// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	rtest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestProvider_ConfigureErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid environment value", func(t *testing.T) {
		t.Parallel()
		rtest.UnitTest(t, rtest.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []rtest.TestStep{
				{
					Config: `
provider "fabricext" {
  environment  = "unsupported-cloud"
  access_token = "mock-token"
}

data "fabricext_item" "test" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "wh"
  type         = "Warehouse"
}
`,
					ExpectError: regexp.MustCompile(`Attribute environment value must be one of`),
				},
			},
		})
	})

	t.Run("invalid request timeout value", func(t *testing.T) {
		t.Parallel()
		rtest.UnitTest(t, rtest.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []rtest.TestStep{
				{
					Config: `
provider "fabricext" {
  request_timeout = "not-a-duration"
  access_token    = "mock-token"
}

data "fabricext_item" "test" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "wh"
  type         = "Warehouse"
}
`,
					ExpectError: regexp.MustCompile(`request_timeout .* must be a positive Go duration`),
				},
			},
		})
	})

	t.Run("eager credential validation failure", func(t *testing.T) {
		t.Parallel()
		f := false
		_ = f
		rtest.UnitTest(t, rtest.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []rtest.TestStep{
				{
					Config: `
provider "fabricext" {
  use_cli                         = false
  use_dev_cli                     = false
  use_msi                         = false
  skip_credentials_validation     = false
}

data "fabricext_item" "test" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "wh"
  type         = "Warehouse"
}
`,
					ExpectError: regexp.MustCompile(`Eager Credential Validation Failed`),
				},
			},
		})
	})

	t.Run("invalid credentials validate config error", func(t *testing.T) {
		t.Parallel()
		rtest.UnitTest(t, rtest.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []rtest.TestStep{
				{
					Config: `
provider "fabricext" {
  client_secret = "secret-without-client-id"
}

data "fabricext_item" "test" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "wh"
  type         = "Warehouse"
}
`,
					ExpectError: regexp.MustCompile(`Invalid Provider Credential Configuration`),
				},
			},
		})
	})

	t.Run("bad tenant id file path error", func(t *testing.T) {
		t.Parallel()
		rtest.UnitTest(t, rtest.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []rtest.TestStep{
				{
					Config: `
provider "fabricext" {
  tenant_id_file_path         = "/nonexistent/path/tenant.txt"
  skip_credentials_validation = true
}

data "fabricext_item" "test" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  display_name = "wh"
  type         = "Warehouse"
}
`,
					ExpectError: regexp.MustCompile(`Unable to Resolve Tenant ID`),
				},
			},
		})
	})
}

func TestResource_ConfigureUnexpectedTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("WarehousePermissionResource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		r, ok := NewWarehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected ResourceWithConfigure")
		}
		var resp tfsdkresource.ConfigureResponse
		r.Configure(ctx, tfsdkresource.ConfigureRequest{
			ProviderData: "invalid-string-data",
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostic error for invalid provider data")
		}
	})

	t.Run("SQLDatabasePermissionResource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		r, ok := NewSQLDatabasePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected ResourceWithConfigure")
		}
		var resp tfsdkresource.ConfigureResponse
		r.Configure(ctx, tfsdkresource.ConfigureRequest{
			ProviderData: "invalid-string-data",
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostic error for invalid provider data")
		}
	})

	t.Run("LakehousePermissionResource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		r, ok := NewLakehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected ResourceWithConfigure")
		}
		var resp tfsdkresource.ConfigureResponse
		r.Configure(ctx, tfsdkresource.ConfigureRequest{
			ProviderData: "invalid-string-data",
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostic error for invalid provider data")
		}
	})

	t.Run("ItemDataSource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		d, ok := NewItemDataSource().(datasource.DataSourceWithConfigure)
		if !ok {
			t.Fatal("expected DataSourceWithConfigure")
		}
		var resp datasource.ConfigureResponse
		d.Configure(ctx, datasource.ConfigureRequest{
			ProviderData: "invalid-string-data",
		}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostic error for invalid provider data")
		}
	})

	t.Run("Resources and DataSource with nil provider data", func(t *testing.T) {
		t.Parallel()
		wh, ok := NewWarehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected WarehousePermissionResource to implement ResourceWithConfigure")
		}
		var whResp tfsdkresource.ConfigureResponse
		wh.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &whResp)
		if whResp.Diagnostics.HasError() {
			t.Fatal("unexpected diagnostic error when provider data is nil")
		}

		sql, ok := NewSQLDatabasePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected SQLDatabasePermissionResource to implement ResourceWithConfigure")
		}
		var sqlResp tfsdkresource.ConfigureResponse
		sql.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &sqlResp)
		if sqlResp.Diagnostics.HasError() {
			t.Fatal("unexpected diagnostic error when provider data is nil")
		}

		lh, ok := NewLakehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected LakehousePermissionResource to implement ResourceWithConfigure")
		}
		var lhResp tfsdkresource.ConfigureResponse
		lh.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &lhResp)
		if lhResp.Diagnostics.HasError() {
			t.Fatal("unexpected diagnostic error when provider data is nil")
		}

		ds, ok := NewItemDataSource().(datasource.DataSourceWithConfigure)
		if !ok {
			t.Fatal("expected ItemDataSource to implement DataSourceWithConfigure")
		}
		var dsResp datasource.ConfigureResponse
		ds.Configure(ctx, datasource.ConfigureRequest{ProviderData: nil}, &dsResp)
		if dsResp.Diagnostics.HasError() {
			t.Fatal("unexpected diagnostic error when provider data is nil")
		}
	})
}

func TestAccImportState_InvalidIdentifiers(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	whCfg := testAccProviderConfig(srv) + `
resource "fabricext_warehouse_permission" "test" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  warehouse_name = "wh"
  principal_id   = "22222222-2222-2222-2222-222222222222"
  role_type      = "read"
}
`

	lhCfg := testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "lh" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "lh"
  role_name      = "custom"
  principal_id   = "22222222-2222-2222-2222-222222222222"
  principal_type = "User"
}
`

	sqlCfg := testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "sql" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "sqldb"
  principal_id      = "22222222-2222-2222-2222-222222222222"
  role_type         = "read"
}
`

	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "too/few/parts",
				ExpectError:   regexp.MustCompile(`Expected import identifier in format`),
			},
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "bad-ws/22222222-2222-2222-2222-222222222222/User/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`workspace_id .* must be a valid UUID`),
			},
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/bad-wh/User/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`warehouse_id .* must be a valid UUID`),
			},
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/bad-principal",
				ExpectError:   regexp.MustCompile(`principal_id .* must be a valid UUID`),
			},
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/ManagedIdentity/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`invalid principal_type "ManagedIdentity" for Warehouse`),
			},
			// Lakehouse import errors
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_permission.lh",
				ImportState:   true,
				ImportStateId: "too/few",
				ExpectError:   regexp.MustCompile(`Expected import identifier in format`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_permission.lh",
				ImportState:   true,
				ImportStateId: "bad-ws/22222222-2222-2222-2222-222222222222/role_name",
				ExpectError:   regexp.MustCompile(`workspace_id .* must be a valid UUID`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_permission.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/bad-lh/role_name",
				ExpectError:   regexp.MustCompile(`lakehouse_id .* must be a valid UUID`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_permission.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/123invalid",
				ExpectError:   regexp.MustCompile(`role_name .* must start with a letter`),
			},
			// SQLDatabase import errors
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "too/few/parts",
				ExpectError:   regexp.MustCompile(`Expected import identifier in format`),
			},
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "bad-ws/22222222-2222-2222-2222-222222222222/User/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`workspace_id .* must be a valid UUID`),
			},
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/bad-sqldb/User/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`sql_database_id .* must be a valid UUID`),
			},
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/bad-principal",
				ExpectError:   regexp.MustCompile(`principal_id .* must be a valid UUID`),
			},
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/ManagedIdentity/33333333-3333-3333-3333-333333333333",
				ExpectError:   regexp.MustCompile(`invalid principal_type "ManagedIdentity" for SQLDatabase`),
			},
			// Import non-existent items API error
			{
				Config:        whCfg,
				ResourceName:  "fabricext_warehouse_permission.test",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/55555555-5555-5555-5555-555555555555",
				ExpectError:   regexp.MustCompile(`Unable to Resolve Imported Warehouse`),
			},
			{
				Config:        sqlCfg,
				ResourceName:  "fabricext_sql_database_permission.sql",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333/User/55555555-5555-5555-5555-555555555555",
				ExpectError:   regexp.MustCompile(`Unable to Resolve Imported SQL Database`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_permission.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/44444444-4444-4444-4444-444444444444/custom_role",
				ExpectError:   regexp.MustCompile(`Unable to Resolve Imported Lakehouse`),
			},
		},
	})
}

func TestAccWarehousePermissionResource_APIErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	// Warehouse does not exist in mock -> Create fails resolving ID
	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_warehouse_permission" "missing" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  warehouse_name = "non_existent_wh"
  principal_id   = "22222222-2222-2222-2222-222222222222"
  role_type      = "read"
}
`,
				ExpectError: regexp.MustCompile(`not found in workspace`),
			},
		},
	})
}

func TestAccSQLDatabasePermissionResource_APIErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	// SQLDatabase does not exist in mock -> Create fails resolving ID
	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_sql_database_permission" "missing" {
  workspace_id      = "11111111-1111-1111-1111-111111111111"
  sql_database_name = "non_existent_sql"
  principal_id      = "22222222-2222-2222-2222-222222222222"
  role_type         = "read"
}
`,
				ExpectError: regexp.MustCompile(`not found in workspace`),
			},
		},
	})
}

func TestAccLakehousePermissionResource_APIErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	// Lakehouse does not exist in mock -> Create fails resolving ID
	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_permission" "missing" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "non_existent_lh"
  role_name      = "custom"
  principal_ids  = ["22222222-2222-2222-2222-222222222222"]
  principal_type = "User"
  paths          = ["*"]
}
`,
				ExpectError: regexp.MustCompile(`not found in workspace`),
			},
		},
	})
}

func TestAccPermissions_UpdateInPlaceEdge(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	sqlID := "33333333-3333-3333-3333-333333333333"
	lhID := "44444444-4444-4444-4444-444444444444"
	pid := "55555555-5555-5555-5555-555555555555"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "wh_edge",
		Type:        "Warehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          sqlID,
		WorkspaceID: wsID,
		DisplayName: "sql_edge",
		Type:        "SQLDatabase",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "lh_edge",
		Type:        "Lakehouse",
	})

	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			// 1. Create with initial values
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "wh" {
  workspace_id   = %q
  warehouse_name = "wh_edge"
  principal_id   = %q
  principal_type = "User"
  role_type      = "read"
}

resource "fabricext_sql_database_permission" "sql" {
  workspace_id      = %q
  sql_database_name = "sql_edge"
  principal_id      = %q
  principal_type    = "User"
  role_type         = "read"
}

resource "fabricext_lakehouse_permission" "lh" {
  workspace_id   = %q
  lakehouse_name = "lh_edge"
  role_name      = "custom_role"
  principal_ids  = [%q]
  principal_type = "User"
  paths          = ["/Tables/a"]
}
`, wsID, pid, wsID, pid, wsID, pid),
			},
			// 2. Update all three in place
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_warehouse_permission" "wh" {
  workspace_id   = %q
  warehouse_name = "wh_edge"
  principal_id   = %q
  principal_type = "User"
  role_type      = "write"
}

resource "fabricext_sql_database_permission" "sql" {
  workspace_id      = %q
  sql_database_name = "sql_edge"
  principal_id      = %q
  principal_type    = "User"
  role_type         = "write"
}

resource "fabricext_lakehouse_permission" "lh" {
  workspace_id   = %q
  lakehouse_name = "lh_edge"
  role_name      = "custom_role"
  principal_ids  = [%q]
  principal_type = "User"
  paths          = ["/Tables/a", "/Tables/b"]
}
`, wsID, pid, wsID, pid, wsID, pid),
			},
		},
	})
}

func TestLakehouse_BuildRolePayloadEdgeCases(t *testing.T) {
	t.Parallel()

	r := &LakehousePermissionResource{}
	ctx := context.Background()

	// 1. Null sets
	mBadPrincipals := &LakehousePermissionResourceModel{
		RoleName:      types.StringValue("CustomRole"),
		PrincipalType: types.StringValue("User"),
		PrincipalIDs:  types.SetNull(types.StringType),
		Paths:         types.SetNull(types.StringType),
		Actions:       types.SetNull(types.StringType),
	}
	payload, diags := r.buildRolePayload(ctx, mBadPrincipals)
	if diags.HasError() {
		t.Fatalf("unexpected error on null sets: %v", diags)
	}
	if payload.Name != "CustomRole" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	// 2. Unknown set elements
	mUnknown := &LakehousePermissionResourceModel{
		RoleName:      types.StringValue("CustomRole"),
		PrincipalType: types.StringValue("User"),
		PrincipalIDs:  types.SetUnknown(types.StringType),
		Paths:         types.SetUnknown(types.StringType),
		Actions:       types.SetUnknown(types.StringType),
	}
	_, diagsUnknown := r.buildRolePayload(ctx, mUnknown)
	if !diagsUnknown.HasError() {
		t.Fatal("expected diags error on unknown set elements")
	}
}

func TestProvider_ConfigureUnknownAttributeDiagnostics(t *testing.T) {
	t.Parallel()

	p := New("test")()
	ctx := context.Background()

	var schemaResp tfsdkprovider.SchemaResponse
	p.Schema(ctx, tfsdkprovider.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error getting provider schema: %v", schemaResp.Diagnostics)
	}

	rawType := schemaResp.Schema.Type().TerraformType(ctx)
	objType, ok := rawType.(tftypes.Object)
	if !ok {
		t.Fatalf("expected rawType to be tftypes.Object, got %T", rawType)
	}

	valMap := make(map[string]tftypes.Value)
	for attrName, attrType := range objType.AttributeTypes {
		valMap[attrName] = tftypes.NewValue(attrType, tftypes.UnknownValue)
	}
	knownObjWithUnknownAttrs := tftypes.NewValue(objType, valMap)

	cfg := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    knownObjWithUnknownAttrs,
	}

	var resp tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{
		Config: cfg,
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics errors on unknown values")
	}
	// Verify that at least 20 attribute errors were added
	if resp.Diagnostics.ErrorsCount() < 20 {
		t.Fatalf("expected at least 20 unknown errors, got %d", resp.Diagnostics.ErrorsCount())
	}
}

type errRoundTripper struct{}

func (e *errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("mock network failure")
}

func TestResource_UpdateAndDeleteErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	badClient, err := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		HTTPClient: &http.Client{
			Transport: &errRoundTripper{},
		},
	})
	if err != nil {
		t.Fatalf("failed to create badClient: %v", err)
	}

	t.Run("Warehouse Update and Delete errors", func(t *testing.T) {
		t.Parallel()
		r := &WarehousePermissionResource{client: badClient}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		planModel := WarehousePermissionResourceModel{
			WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
			WarehouseName: types.StringValue("wh"),
			PrincipalID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType: types.StringValue("User"),
			RoleType:      types.StringValue("write"),
		}
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue("11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333/User/22222222-2222-2222-2222-222222222222"),
			WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
			WarehouseName: types.StringValue("wh"),
			WarehouseID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
			PrincipalID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType: types.StringValue("User"),
			RoleType:      types.StringValue("read"),
		}

		plan := tfsdk.Plan{Schema: schemaResp.Schema}
		if diags := plan.Set(ctx, &planModel); diags.HasError() {
			t.Fatalf("failed to set plan: %v", diags)
		}
		state := tfsdk.State{Schema: schemaResp.Schema}
		if diags := state.Set(ctx, &stateModel); diags.HasError() {
			t.Fatalf("failed to set state: %v", diags)
		}

		var updateResp tfsdkresource.UpdateResponse
		r.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error updating warehouse with failing client")
		}

		var delResp tfsdkresource.DeleteResponse
		r.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delResp)
		if !delResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting warehouse with failing client")
		}
	})

	t.Run("SQLDatabase Update and Delete errors", func(t *testing.T) {
		t.Parallel()
		r := &SQLDatabasePermissionResource{client: badClient}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		planModel := SQLDatabasePermissionResourceModel{
			WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
			SQLDatabaseName: types.StringValue("sql"),
			PrincipalID:     types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType:   types.StringValue("User"),
			RoleType:        types.StringValue("write"),
		}
		stateModel := SQLDatabasePermissionResourceModel{
			ID:              types.StringValue("11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333/User/22222222-2222-2222-2222-222222222222"),
			WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
			SQLDatabaseName: types.StringValue("sql"),
			SQLDatabaseID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
			PrincipalID:     types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType:   types.StringValue("User"),
			RoleType:        types.StringValue("read"),
		}

		plan := tfsdk.Plan{Schema: schemaResp.Schema}
		if diags := plan.Set(ctx, &planModel); diags.HasError() {
			t.Fatalf("failed to set plan: %v", diags)
		}
		state := tfsdk.State{Schema: schemaResp.Schema}
		if diags := state.Set(ctx, &stateModel); diags.HasError() {
			t.Fatalf("failed to set state: %v", diags)
		}

		var updateResp tfsdkresource.UpdateResponse
		r.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error updating SQL database with failing client")
		}

		var delResp tfsdkresource.DeleteResponse
		r.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delResp)
		if !delResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting SQL database with failing client")
		}
	})

	t.Run("Lakehouse Update and Delete errors", func(t *testing.T) {
		t.Parallel()
		r := &LakehousePermissionResource{client: badClient}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"22222222-2222-2222-2222-222222222222"})
		paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/a"})
		actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

		planModel := LakehousePermissionResourceModel{
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:    types.StringValue("lh"),
			RoleName:         types.StringValue("custom"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}
		stateModel := LakehousePermissionResourceModel{
			ID:               types.StringValue("11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333/custom"),
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:    types.StringValue("lh"),
			LakehouseID:      types.StringValue("33333333-3333-3333-3333-333333333333"),
			RoleName:         types.StringValue("custom"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}

		plan := tfsdk.Plan{Schema: schemaResp.Schema}
		if diags := plan.Set(ctx, &planModel); diags.HasError() {
			t.Fatalf("failed to set plan: %v", diags)
		}
		state := tfsdk.State{Schema: schemaResp.Schema}
		if diags := state.Set(ctx, &stateModel); diags.HasError() {
			t.Fatalf("failed to set state: %v", diags)
		}

		var updateResp tfsdkresource.UpdateResponse
		r.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error updating lakehouse with failing client")
		}

		var delResp tfsdkresource.DeleteResponse
		r.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delResp)
		if !delResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting lakehouse with failing client")
		}
	})

	t.Run("ItemDataSource Read error", func(t *testing.T) {
		t.Parallel()
		d := &ItemDataSource{client: badClient}
		var schemaResp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

		rawVal := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
			"id":           tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			"workspace_id": tftypes.NewValue(tftypes.String, "11111111-1111-1111-1111-111111111111"),
			"display_name": tftypes.NewValue(tftypes.String, "analytics_wh"),
			"type":         tftypes.NewValue(tftypes.String, "Warehouse"),
		})

		cfg := tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    rawVal,
		}

		var readResp datasource.ReadResponse
		d.Read(ctx, datasource.ReadRequest{Config: cfg}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error reading item data source with failing client")
		}
	})

	t.Run("Warehouse, SQLDatabase, and Lakehouse Create error paths", func(t *testing.T) {
		t.Parallel()

		// 1. Warehouse Create grant error
		wh := &WarehousePermissionResource{client: badClient}
		var whSchema tfsdkresource.SchemaResponse
		wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

		whPlanModel := WarehousePermissionResourceModel{
			WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
			WarehouseName: types.StringValue("wh"),
			PrincipalID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType: types.StringValue("User"),
			RoleType:      types.StringValue("read"),
		}
		whPlan := tfsdk.Plan{Schema: whSchema.Schema}
		_ = whPlan.Set(ctx, &whPlanModel)
		var whCreateResp tfsdkresource.CreateResponse
		wh.Create(ctx, tfsdkresource.CreateRequest{Plan: whPlan}, &whCreateResp)
		if !whCreateResp.Diagnostics.HasError() {
			t.Fatal("expected error creating warehouse with failing client")
		}

		// 2. SQLDatabase Create grant error
		sql := &SQLDatabasePermissionResource{client: badClient}
		var sqlSchema tfsdkresource.SchemaResponse
		sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchema)

		sqlPlanModel := SQLDatabasePermissionResourceModel{
			WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
			SQLDatabaseName: types.StringValue("sql"),
			PrincipalID:     types.StringValue("22222222-2222-2222-2222-222222222222"),
			PrincipalType:   types.StringValue("User"),
			RoleType:        types.StringValue("read"),
		}
		sqlPlan := tfsdk.Plan{Schema: sqlSchema.Schema}
		_ = sqlPlan.Set(ctx, &sqlPlanModel)
		var sqlCreateResp tfsdkresource.CreateResponse
		sql.Create(ctx, tfsdkresource.CreateRequest{Plan: sqlPlan}, &sqlCreateResp)
		if !sqlCreateResp.Diagnostics.HasError() {
			t.Fatal("expected error creating SQL database with failing client")
		}

		// 3. Lakehouse Create role error
		lh := &LakehousePermissionResource{client: badClient}
		var lhSchema tfsdkresource.SchemaResponse
		lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)

		principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"22222222-2222-2222-2222-222222222222"})
		paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/a"})
		actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

		lhPlanModel := LakehousePermissionResourceModel{
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:    types.StringValue("lh"),
			RoleName:         types.StringValue("custom"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}
		lhPlan := tfsdk.Plan{Schema: lhSchema.Schema}
		_ = lhPlan.Set(ctx, &lhPlanModel)
		var lhCreateResp tfsdkresource.CreateResponse
		lh.Create(ctx, tfsdkresource.CreateRequest{Plan: lhPlan}, &lhCreateResp)
		if !lhCreateResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse with failing client")
		}
	})
}

func TestLakehouse_IsRoleAdvanced(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		role     *client.DataAccessRole
		expected bool
	}{
		{
			name: "zero decision rules",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{},
			},
			expected: true,
		},
		{
			name: "multiple decision rules",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{
					{Effect: "Permit"},
					{Effect: "Permit"},
				},
			},
			expected: true,
		},
		{
			name: "row constraints present",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{
					{
						Effect: "Permit",
						Constraints: &client.Constraints{
							Rows: []client.RowConstraint{{TablePath: "/table", Value: "id = 1"}},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "column constraints present",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{
					{
						Effect: "Permit",
						Constraints: &client.Constraints{
							Columns: []client.ColumnConstraint{{TablePath: "/table", ColumnNames: []string{"col1"}}},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "fabric item members present",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{{Effect: "Permit"}},
				Members: &client.Members{
					FabricItemMembers: []client.FabricItemMember{{SourcePath: "/path"}},
				},
			},
			expected: true,
		},
		{
			name: "mixed entra member object types",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{{Effect: "Permit"}},
				Members: &client.Members{
					MicrosoftEntraMembers: []client.MicrosoftEntraMember{
						{ObjectID: "id1", ObjectType: "User"},
						{ObjectID: "id2", ObjectType: "Group"},
					},
				},
			},
			expected: true,
		},
		{
			name: "uniform entra member object types",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{{Effect: "Permit"}},
				Members: &client.Members{
					MicrosoftEntraMembers: []client.MicrosoftEntraMember{
						{ObjectID: "id1", ObjectType: "User"},
						{ObjectID: "id2", ObjectType: "User"},
					},
				},
			},
			expected: false,
		},
		{
			name: "single entra member",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{{Effect: "Permit"}},
				Members: &client.Members{
					MicrosoftEntraMembers: []client.MicrosoftEntraMember{
						{ObjectID: "id1", ObjectType: "User"},
					},
				},
			},
			expected: false,
		},
		{
			name: "nil members",
			role: &client.DataAccessRole{
				DecisionRules: []client.DecisionRule{{Effect: "Permit"}},
				Members:       nil,
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isRoleAdvanced(tc.role)
			if got != tc.expected {
				t.Fatalf("expected isRoleAdvanced=%v, got %v", tc.expected, got)
			}
		})
	}
}

func TestLakehouse_PopulateLakehouseState_EdgeCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &LakehousePermissionResource{tenantID: "default-tenant-id"}

	role := &client.DataAccessRole{
		Kind: "", // should default to "Policy"
		DecisionRules: []client.DecisionRule{
			{
				Effect: "", // should default to "Permit"
				Permission: []client.PermissionScope{
					{AttributeName: "Path", AttributeValueIncludedIn: []string{"/Tables/t"}},
					// No Action attribute -> should default ruleActions to ["Read"]
				},
				Constraints: &client.Constraints{
					Rows: []client.RowConstraint{
						{TablePath: "/Tables/t", Value: "1 = 1"},
					},
					Columns: []client.ColumnConstraint{
						{
							TablePath:    "/Tables/t",
							ColumnNames:  []string{"c1"},
							ColumnAction: []string{}, // should default to "Read"
							ColumnEffect: "",         // should default to "Permit"
						},
					},
				},
			},
		},
		Members: &client.Members{
			MicrosoftEntraMembers: []client.MicrosoftEntraMember{
				{
					ObjectID:   "11111111-1111-1111-1111-111111111111",
					ObjectType: "", // should default to "Group"
					TenantID:   "", // should default to r.tenantID
				},
			},
			FabricItemMembers: []client.FabricItemMember{
				{
					SourcePath: "/Tables/t",
					ItemAccess: []string{}, // should default to ["ReadAll"]
				},
			},
		},
	}

	var state LakehousePermissionResourceModel
	diags := r.populateLakehouseStateFromRole(ctx, role, &state)
	if diags.HasError() {
		t.Fatalf("unexpected errors populating state: %v", diags)
	}

	if state.Kind.ValueString() != "Policy" {
		t.Fatalf("expected kind Policy, got %s", state.Kind.ValueString())
	}

	// Verify defaults were set when API returned empty strings
	var entraMembers []EntraMemberModel
	if d := state.EntraMember.ElementsAs(ctx, &entraMembers, false); d.HasError() {
		t.Fatalf("failed to decode entra_member elements: %v", d)
	}
	if len(entraMembers) != 1 || entraMembers[0].ObjectType.ValueString() != "Group" || !entraMembers[0].TenantID.IsNull() {
		t.Fatalf("unexpected defaulted entra_member: %+v", entraMembers)
	}

	// Verify prior member type preservation when Fabric API returns empty objectType
	priorSet, d := types.SetValueFrom(ctx, entraMemberElemType, []EntraMemberModel{
		{
			ObjectID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
			ObjectType: types.StringValue("User"),
			TenantID:   types.StringValue("custom-tenant-id"),
		},
	})
	if d.HasError() {
		t.Fatalf("failed to create prior entra set: %v", d)
	}

	priorState := LakehousePermissionResourceModel{
		EntraMember: priorSet,
	}
	diags = r.populateLakehouseStateFromRole(ctx, role, &priorState)
	if diags.HasError() {
		t.Fatalf("unexpected errors populating prior state: %v", diags)
	}
	var preservedMembers []EntraMemberModel
	if d := priorState.EntraMember.ElementsAs(ctx, &preservedMembers, false); d.HasError() {
		t.Fatalf("failed to decode preserved entra members: %v", d)
	}
	if len(preservedMembers) != 1 || preservedMembers[0].ObjectType.ValueString() != "User" || preservedMembers[0].TenantID.ValueString() != "custom-tenant-id" {
		t.Fatalf("expected preserved ObjectType 'User' and 'custom-tenant-id', got %+v", preservedMembers)
	}
}

func TestLakehouse_ModifyPlan_EdgeCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &LakehousePermissionResource{}
	var schemaResp tfsdkresource.SchemaResponse
	r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

	t.Run("null plan returns cleanly", func(t *testing.T) {
		t.Parallel()
		plan := tfsdk.Plan{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		}
		var modResp tfsdkresource.ModifyPlanResponse
		r.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{Plan: plan}, &modResp)
		if modResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on null plan: %v", modResp.Diagnostics)
		}
	})
}
