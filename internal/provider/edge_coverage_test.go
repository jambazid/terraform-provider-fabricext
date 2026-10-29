// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	rtest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
	fabadmin "github.com/microsoft/fabric-sdk-go/fabric/admin"
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

	t.Run("LakehouseDataAccessRoleResource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		r, ok := NewLakehouseDataAccessRoleResource().(tfsdkresource.ResourceWithConfigure)
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

	t.Run("LakehouseOnelakeSecurityResource with invalid provider data type", func(t *testing.T) {
		t.Parallel()
		r, ok := NewLakehouseOnelakeSecurityResource().(tfsdkresource.ResourceWithConfigure)
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

	t.Run("Resources and DataSource with nil provider data", func(t *testing.T) {
		t.Parallel()

		lh, ok := NewLakehouseDataAccessRoleResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected LakehouseDataAccessRoleResource to implement ResourceWithConfigure")
		}
		var lhResp tfsdkresource.ConfigureResponse
		lh.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &lhResp)
		if lhResp.Diagnostics.HasError() {
			t.Fatal("unexpected diagnostic error when provider data is nil")
		}

		lhSec, ok := NewLakehouseOnelakeSecurityResource().(tfsdkresource.ResourceWithConfigure)
		if !ok {
			t.Fatal("expected LakehouseOnelakeSecurityResource to implement ResourceWithConfigure")
		}
		var lhSecResp tfsdkresource.ConfigureResponse
		lhSec.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &lhSecResp)
		if lhSecResp.Diagnostics.HasError() {
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

	lhCfg := testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_data_access_role" "lh" {
  workspace_id   = "11111111-1111-1111-1111-111111111111"
  lakehouse_name = "lh"
  role_name      = "custom"
  principal_id   = "22222222-2222-2222-2222-222222222222"
  principal_type = "User"
}
`

	lhSecCfg := testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_onelake_security" "sec" {
  workspace_id = "11111111-1111-1111-1111-111111111111"
  lakehouse_id = "22222222-2222-2222-2222-222222222222"
}
`

	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			// Lakehouse import errors
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_data_access_role.lh",
				ImportState:   true,
				ImportStateId: "too/few",
				ExpectError:   regexp.MustCompile(`Expected import identifier in format`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_data_access_role.lh",
				ImportState:   true,
				ImportStateId: "bad-ws/22222222-2222-2222-2222-222222222222/role_name",
				ExpectError:   regexp.MustCompile(`workspace_id .* must be a valid UUID`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_data_access_role.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/bad-lh/role_name",
				ExpectError:   regexp.MustCompile(`lakehouse_id .* must be a valid UUID`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_data_access_role.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/123invalid",
				ExpectError:   regexp.MustCompile(`role_name .* must start with a letter`),
			},
			{
				Config:        lhCfg,
				ResourceName:  "fabricext_lakehouse_data_access_role.lh",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/44444444-4444-4444-4444-444444444444/custom_role",
				ExpectError:   regexp.MustCompile(`Unable to Resolve Imported Lakehouse`),
			},
			// Lakehouse OneLake Security import errors
			{
				Config:        lhSecCfg,
				ResourceName:  "fabricext_lakehouse_onelake_security.sec",
				ImportState:   true,
				ImportStateId: "too/many/parts/here",
				ExpectError:   regexp.MustCompile(`Expected composite identifier in format`),
			},
			{
				Config:        lhSecCfg,
				ResourceName:  "fabricext_lakehouse_onelake_security.sec",
				ImportState:   true,
				ImportStateId: "bad-ws/22222222-2222-2222-2222-222222222222",
				ExpectError:   regexp.MustCompile(`workspace_id .* must be a valid UUID`),
			},
			{
				Config:        lhSecCfg,
				ResourceName:  "fabricext_lakehouse_onelake_security.sec",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/bad-lh",
				ExpectError:   regexp.MustCompile(`lakehouse_id .* must be a valid UUID`),
			},
			{
				Config:        lhSecCfg,
				ResourceName:  "fabricext_lakehouse_onelake_security.sec",
				ImportState:   true,
				ImportStateId: "11111111-1111-1111-1111-111111111111/99999999-9999-9999-9999-999999999999",
				ExpectError:   regexp.MustCompile(`Unable to Resolve Imported Lakehouse`),
			},
		},
	})
}

func TestAccLakehouseDataAccessRoleResource_APIErrors(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)

	// Lakehouse does not exist in mock -> Create fails resolving ID
	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			{
				Config: testAccProviderConfig(srv) + `
resource "fabricext_lakehouse_data_access_role" "missing" {
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
	lhID := "44444444-4444-4444-4444-444444444444"
	pid := "55555555-5555-5555-5555-555555555555"
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "lh_edge",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	rtest.UnitTest(t, rtest.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []rtest.TestStep{
			// 1. Create with initial values
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_data_access_role" "lh" {
  workspace_id   = %q
  lakehouse_name = "lh_edge"
  role_name      = "custom_role"
  principal_ids  = [%q]
  principal_type = "User"
  paths          = ["/Tables/a"]
}
`, wsID, pid),
			},
			// 2. Update lakehouse in place
			{
				Config: testAccProviderConfig(srv) + fmt.Sprintf(`
resource "fabricext_lakehouse_data_access_role" "lh" {
  workspace_id   = %q
  lakehouse_name = "lh_edge"
  role_name      = "custom_role"
  principal_ids  = [%q]
  principal_type = "User"
  paths          = ["/Tables/a", "/Tables/b"]
}
`, wsID, pid),
			},
		},
	})
}

func TestLakehouse_BuildRolePayloadEdgeCases(t *testing.T) {
	t.Parallel()

	r := &LakehouseDataAccessRoleResource{}
	ctx := context.Background()

	// 1. Null sets
	mBadPrincipals := &LakehouseDataAccessRoleResourceModel{
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
	mUnknown := &LakehouseDataAccessRoleResourceModel{
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
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

	t.Run("Lakehouse Update and Delete errors", func(t *testing.T) {
		t.Parallel()
		r := &LakehouseDataAccessRoleResource{client: badClient}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"22222222-2222-2222-2222-222222222222"})
		paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/a"})
		actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

		planModel := LakehouseDataAccessRoleResourceModel{
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
		stateModel := LakehouseDataAccessRoleResourceModel{
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

	t.Run("LakehouseOnelakeSecurity Update and nil client errors", func(t *testing.T) {
		t.Parallel()
		rNil := &LakehouseOnelakeSecurityResource{client: nil}
		var schemaResp tfsdkresource.SchemaResponse
		rNil.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		planModel := LakehouseOnelakeSecurityResourceModel{
			WorkspaceID:             types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:           types.StringValue("lh"),
			LakehouseID:             types.StringValue("33333333-3333-3333-3333-333333333333"),
			RemoveDefaultReaderRole: types.BoolValue(true),
		}
		stateModel := LakehouseOnelakeSecurityResourceModel{
			ID:                      types.StringValue("11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333"),
			WorkspaceID:             types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:           types.StringValue("lh"),
			LakehouseID:             types.StringValue("33333333-3333-3333-3333-333333333333"),
			RemoveDefaultReaderRole: types.BoolValue(false),
			DefaultReaderRoleExists: types.BoolValue(true),
			Enabled:                 types.BoolValue(true),
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
		rNil.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error updating lakehouse security with nil client")
		}

		var createResp tfsdkresource.CreateResponse
		rNil.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createResp)
		if !createResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse security with nil client")
		}

		var readResp tfsdkresource.ReadResponse
		rNil.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error reading lakehouse security with nil client")
		}

		// Test with failing client
		rBad := &LakehouseOnelakeSecurityResource{client: badClient}
		var updateBadResp tfsdkresource.UpdateResponse
		rBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateBadResp)
		if !updateBadResp.Diagnostics.HasError() {
			t.Fatal("expected error updating lakehouse security with failing client")
		}

		var createBadResp tfsdkresource.CreateResponse
		rBad.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createBadResp)
		if !createBadResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse security with failing client")
		}

		var readBadResp tfsdkresource.ReadResponse
		rBad.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readBadResp)
		if !readBadResp.Diagnostics.HasError() {
			t.Fatal("expected error reading lakehouse security with failing client")
		}

		var delBadResp tfsdkresource.DeleteResponse
		rBad.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delBadResp)
		if delBadResp.Diagnostics.WarningsCount() == 0 {
			t.Fatal("expected warning deleting lakehouse security")
		}

		var impNilResp tfsdkresource.ImportStateResponse
		rNil.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: "11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333"}, &impNilResp)
		if !impNilResp.Diagnostics.HasError() {
			t.Fatal("expected error importing lakehouse security with nil client")
		}

		var impBadResp tfsdkresource.ImportStateResponse
		rBad.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: "11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333333"}, &impBadResp)
		if !impBadResp.Diagnostics.HasError() {
			t.Fatal("expected error importing lakehouse security with failing client")
		}

		planNameOnly := LakehouseOnelakeSecurityResourceModel{
			WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName: types.StringValue("lh-name-only"),
		}
		pName := tfsdk.Plan{Schema: schemaResp.Schema}
		_ = pName.Set(ctx, &planNameOnly)
		var createBadNameResp tfsdkresource.CreateResponse
		rBad.Create(ctx, tfsdkresource.CreateRequest{Plan: pName}, &createBadNameResp)
		if !createBadNameResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse security with failing client on name only")
		}

		planIDOnly := LakehouseOnelakeSecurityResourceModel{
			WorkspaceID: types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseID: types.StringValue("33333333-3333-3333-3333-333333333333"),
		}
		pID := tfsdk.Plan{Schema: schemaResp.Schema}
		_ = pID.Set(ctx, &planIDOnly)
		var createBadIDResp tfsdkresource.CreateResponse
		rBad.Create(ctx, tfsdkresource.CreateRequest{Plan: pID}, &createBadIDResp)
		if !createBadIDResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse security with failing client on id only")
		}

		planEmpty := LakehouseOnelakeSecurityResourceModel{
			WorkspaceID: types.StringValue("11111111-1111-1111-1111-111111111111"),
		}
		pEmpty := tfsdk.Plan{Schema: schemaResp.Schema}
		_ = pEmpty.Set(ctx, &planEmpty)
		var createBadEmptyResp tfsdkresource.CreateResponse
		rBad.Create(ctx, tfsdkresource.CreateRequest{Plan: pEmpty}, &createBadEmptyResp)
		if !createBadEmptyResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse security with missing identifier")
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

	t.Run("Lakehouse Create error paths", func(t *testing.T) {
		t.Parallel()

		// 3. Lakehouse Create role error
		lh := &LakehouseDataAccessRoleResource{client: badClient}
		var lhSchema tfsdkresource.SchemaResponse
		lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)

		principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"22222222-2222-2222-2222-222222222222"})
		paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/a"})
		actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

		lhPlanModel := LakehouseDataAccessRoleResourceModel{
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
	r := &LakehouseDataAccessRoleResource{tenantID: "default-tenant-id"}

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

	var state LakehouseDataAccessRoleResourceModel
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

	priorState := LakehouseDataAccessRoleResourceModel{
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
	r := &LakehouseDataAccessRoleResource{}
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

func TestResource_NilClientErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 3. LakehouseDataAccessRoleResource
	{
		r := &LakehouseDataAccessRoleResource{client: nil}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		plan := tfsdk.Plan{Schema: schemaResp.Schema}
		state := tfsdk.State{Schema: schemaResp.Schema}

		var createResp tfsdkresource.CreateResponse
		r.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createResp)
		if !createResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client Create")
		}

		var readResp tfsdkresource.ReadResponse
		r.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client Read")
		}

		var updateResp tfsdkresource.UpdateResponse
		r.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client Update")
		}

		var delResp tfsdkresource.DeleteResponse
		r.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delResp)
		if !delResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client Delete")
		}

		var importResp tfsdkresource.ImportStateResponse
		r.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: "ws/lh/role"}, &importResp)
		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client ImportState")
		}
	}

	// 4. ItemDataSource
	{
		d := &ItemDataSource{client: nil}
		var schemaResp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

		cfg := tfsdk.Config{Schema: schemaResp.Schema}
		var readResp datasource.ReadResponse
		d.Read(ctx, datasource.ReadRequest{Config: cfg}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on nil client Read")
		}
	}
}

func TestReconcileItemIdentifiersPlan_Exhaustive(t *testing.T) {
	t.Parallel()

	// 1. nameConfig is unknown, idConfig is null -> idPlan becomes unknown
	{
		var namePlan types.String
		idPlan := types.StringValue("state-id")
		reconcileItemIdentifiersPlan(
			types.StringUnknown(), types.StringNull(),
			types.StringNull(), types.StringValue("state-id"),
			&namePlan, &idPlan,
		)
		if !idPlan.IsUnknown() {
			t.Fatalf("expected idPlan unknown, got %v", idPlan)
		}
	}

	// 2. nameConfig is value, idConfig is null, nameState is null -> idPlan becomes unknown
	{
		var namePlan types.String
		idPlan := types.StringValue("state-id")
		reconcileItemIdentifiersPlan(
			types.StringValue("new-name"), types.StringNull(),
			types.StringNull(), types.StringValue("state-id"),
			&namePlan, &idPlan,
		)
		if !idPlan.IsUnknown() {
			t.Fatalf("expected idPlan unknown, got %v", idPlan)
		}
	}

	// 3. nameConfig is value, idConfig is null, nameState is different -> idPlan becomes unknown
	{
		var namePlan types.String
		idPlan := types.StringValue("state-id")
		reconcileItemIdentifiersPlan(
			types.StringValue("new-name"), types.StringNull(),
			types.StringValue("old-name"), types.StringValue("state-id"),
			&namePlan, &idPlan,
		)
		if !idPlan.IsUnknown() {
			t.Fatalf("expected idPlan unknown, got %v", idPlan)
		}
	}

	// 4. nameConfig is value, idConfig is null, nameState is identical -> idPlan unchanged
	{
		var namePlan types.String
		idPlan := types.StringValue("state-id")
		reconcileItemIdentifiersPlan(
			types.StringValue("same-name"), types.StringNull(),
			types.StringValue("same-name"), types.StringValue("state-id"),
			&namePlan, &idPlan,
		)
		if idPlan.ValueString() != "state-id" {
			t.Fatalf("expected idPlan unchanged, got %v", idPlan)
		}
	}

	// 5. idConfig is unknown, nameConfig is null -> namePlan becomes unknown
	{
		namePlan := types.StringValue("state-name")
		var idPlan types.String
		reconcileItemIdentifiersPlan(
			types.StringNull(), types.StringUnknown(),
			types.StringValue("state-name"), types.StringNull(),
			&namePlan, &idPlan,
		)
		if !namePlan.IsUnknown() {
			t.Fatalf("expected namePlan unknown, got %v", namePlan)
		}
	}

	// 6. idConfig is value, nameConfig is null, idState is null -> namePlan becomes unknown
	{
		namePlan := types.StringValue("state-name")
		var idPlan types.String
		reconcileItemIdentifiersPlan(
			types.StringNull(), types.StringValue("new-id"),
			types.StringValue("state-name"), types.StringNull(),
			&namePlan, &idPlan,
		)
		if !namePlan.IsUnknown() {
			t.Fatalf("expected namePlan unknown, got %v", namePlan)
		}
	}

	// 7. idConfig is value, nameConfig is null, idState is different -> namePlan becomes unknown
	{
		namePlan := types.StringValue("state-name")
		var idPlan types.String
		reconcileItemIdentifiersPlan(
			types.StringNull(), types.StringValue("new-id"),
			types.StringValue("state-name"), types.StringValue("old-id"),
			&namePlan, &idPlan,
		)
		if !namePlan.IsUnknown() {
			t.Fatalf("expected namePlan unknown, got %v", namePlan)
		}
	}

	// 8. idConfig is value, nameConfig is null, idState is identical -> namePlan unchanged
	{
		namePlan := types.StringValue("state-name")
		var idPlan types.String
		reconcileItemIdentifiersPlan(
			types.StringNull(), types.StringValue("same-id"),
			types.StringValue("state-name"), types.StringValue("same-id"),
			&namePlan, &idPlan,
		)
		if namePlan.ValueString() != "state-name" {
			t.Fatalf("expected namePlan unchanged, got %v", namePlan)
		}
	}
}

func TestResource_APIErrors_Exhaustive(t *testing.T) {
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

	// Lakehouse Read/Update/Delete/Import with failing client
	{
		r := &LakehouseDataAccessRoleResource{client: badClient}
		var schemaResp tfsdkresource.SchemaResponse
		r.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

		principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"33333333-3333-3333-3333-333333333333"})
		paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/a"})
		actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

		stateModel := LakehouseDataAccessRoleResourceModel{
			ID:               types.StringValue("11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/CustomRole"),
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseName:    types.StringValue("lh"),
			LakehouseID:      types.StringValue("22222222-2222-2222-2222-222222222222"),
			RoleName:         types.StringValue("CustomRole"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}
		state := tfsdk.State{Schema: schemaResp.Schema}
		_ = state.Set(ctx, &stateModel)

		var readResp tfsdkresource.ReadResponse
		r.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on Lakehouse Read with failing client")
		}

		var importResp tfsdkresource.ImportStateResponse
		r.ImportState(ctx, tfsdkresource.ImportStateRequest{
			ID: "11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/CustomRole",
		}, &importResp)
		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected error on Lakehouse ImportState with failing client")
		}

		// Create with ID only where GetItemByID fails
		lhPlanIDOnly := LakehouseDataAccessRoleResourceModel{
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseID:      types.StringValue("22222222-2222-2222-2222-222222222222"),
			RoleName:         types.StringValue("CustomRole"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}
		planIDOnly := tfsdk.Plan{Schema: schemaResp.Schema}
		_ = planIDOnly.Set(ctx, &lhPlanIDOnly)
		var createRespIDOnly tfsdkresource.CreateResponse
		r.Create(ctx, tfsdkresource.CreateRequest{Plan: planIDOnly}, &createRespIDOnly)
		if !createRespIDOnly.Diagnostics.HasError() {
			t.Fatal("expected error on Lakehouse Create with failing ID lookup")
		}

		// Create with both ID and Name where GetItemByID fails
		lhPlanBoth := LakehouseDataAccessRoleResourceModel{
			WorkspaceID:      types.StringValue("11111111-1111-1111-1111-111111111111"),
			LakehouseID:      types.StringValue("22222222-2222-2222-2222-222222222222"),
			LakehouseName:    types.StringValue("lh"),
			RoleName:         types.StringValue("CustomRole"),
			PrincipalIDs:     principalIDs,
			PrincipalType:    types.StringValue("User"),
			Paths:            paths,
			Actions:          actions,
			DecisionRule:     types.ListNull(decisionRuleElemType),
			EntraMember:      types.SetNull(entraMemberElemType),
			FabricItemMember: types.SetNull(fabricItemMemberElemType),
		}
		planBoth := tfsdk.Plan{Schema: schemaResp.Schema}
		_ = planBoth.Set(ctx, &lhPlanBoth)
		var createRespBoth tfsdkresource.CreateResponse
		r.Create(ctx, tfsdkresource.CreateRequest{Plan: planBoth}, &createRespBoth)
		if !createRespBoth.Diagnostics.HasError() {
			t.Fatal("expected error on Lakehouse Create with both ID and Name lookup failure")
		}
	}
}

func TestItemShareGateAndAccessDetails_EdgeCoverage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. Unconfigured client diagnostics
	gateRes := &ItemShareGateResource{}
	var schemaResp tfsdkresource.SchemaResponse
	gateRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

	expectedPerms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
	planModel := ItemShareGateResourceModel{
		WorkspaceID:         types.StringValue("11111111-1111-1111-1111-111111111111"),
		ItemID:              types.StringValue("22222222-2222-2222-2222-222222222222"),
		PrincipalID:         types.StringValue("33333333-3333-3333-3333-333333333333"),
		ExpectedPermissions: expectedPerms,
		MatchType:           types.StringValue("subset"),
	}
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan.Set(ctx, &planModel)
	state := tfsdk.State{Schema: schemaResp.Schema}
	_ = state.Set(ctx, &planModel)

	var gCreateResp tfsdkresource.CreateResponse
	gateRes.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &gCreateResp)
	if !gCreateResp.Diagnostics.HasError() {
		t.Fatal("expected error on unconfigured Create")
	}

	var gReadResp tfsdkresource.ReadResponse
	gateRes.Read(ctx, tfsdkresource.ReadRequest{State: state}, &gReadResp)
	if !gReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on unconfigured Read")
	}

	var gUpdateResp tfsdkresource.UpdateResponse
	gateRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &gUpdateResp)
	if !gUpdateResp.Diagnostics.HasError() {
		t.Fatal("expected error on unconfigured Update")
	}

	var gImportResp tfsdkresource.ImportStateResponse
	gateRes.ImportState(ctx, tfsdkresource.ImportStateRequest{}, &gImportResp)
	if !gImportResp.Diagnostics.HasError() {
		t.Fatal("expected error on unconfigured ImportState")
	}

	detailsDS := &ItemAccessDetailsDataSource{}
	var dsSchemaResp datasource.SchemaResponse
	detailsDS.Schema(ctx, datasource.SchemaRequest{}, &dsSchemaResp)
	dsConfigModel := ItemAccessDetailsDataSourceModel{
		WorkspaceID:          types.StringValue("11111111-1111-1111-1111-111111111111"),
		ItemID:               types.StringValue("22222222-2222-2222-2222-222222222222"),
		Type:                 types.StringNull(),
		AccessDetails:        types.ListNull(types.ObjectType{AttrTypes: accessDetailElemAttrTypes}),
		PrincipalPermissions: types.MapNull(types.SetType{ElemType: types.StringType}),
	}
	dsState := tfsdk.State{Schema: dsSchemaResp.Schema}
	if diags := dsState.Set(ctx, &dsConfigModel); diags.HasError() {
		t.Fatalf("failed to set dsState: %v", diags)
	}
	dsConfig := tfsdk.Config{
		Schema: dsSchemaResp.Schema,
		Raw:    dsState.Raw,
	}
	var dsReadResp datasource.ReadResponse
	detailsDS.Read(ctx, datasource.ReadRequest{Config: dsConfig}, &dsReadResp)
	if !dsReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on unconfigured DataSource Read")
	}

	// 2. Configure with invalid ProviderData
	var gConfResp tfsdkresource.ConfigureResponse
	gateRes.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: "invalid"}, &gConfResp)
	if !gConfResp.Diagnostics.HasError() {
		t.Fatal("expected error on invalid gate ProviderData")
	}

	var dsConfResp datasource.ConfigureResponse
	detailsDS.Configure(ctx, datasource.ConfigureRequest{ProviderData: "invalid"}, &dsConfResp)
	if !dsConfResp.Diagnostics.HasError() {
		t.Fatal("expected error on invalid data source ProviderData")
	}

	// 3. backoffWithJitter coverage in production mode
	bProd := backoffWithJitter(100*time.Second, false)
	if bProd < 70*time.Second || bProd > 130*time.Second {
		t.Errorf("unexpected production backoff with jitter: %v", bProd)
	}
	bFloor := backoffWithJitter(5*time.Second, false)
	if bFloor < 10*time.Second {
		t.Errorf("expected clamp to 10s floor: %v", bFloor)
	}
	bTest := backoffWithJitter(5*time.Millisecond, true)
	if bTest != 5*time.Millisecond {
		t.Errorf("expected test backoff to remain unchanged: %v", bTest)
	}

	// 4. normalizePermission coverage
	if normalizePermission(" Read_Data ") != "readdata" {
		t.Errorf("unexpected normalizePermission output: %s", normalizePermission(" Read_Data "))
	}
	if normalizePermission("ReadAll") != "readall" {
		t.Errorf("unexpected normalizePermission output: %s", normalizePermission("ReadAll"))
	}

	// 5. stringOrNull helper coverage
	if !stringOrNull("").IsNull() {
		t.Fatal("expected empty string to yield StringNull")
	}
	if stringOrNull("valid").ValueString() != "valid" {
		t.Fatal("expected StringValue for non-empty string")
	}

	// 6. checkPrincipalMatch unit branches
	emptyMatch, _ := checkPrincipalMatch(nil, "pid", "", []string{"Read"}, "subset")
	if emptyMatch {
		t.Fatal("expected false for empty access details")
	}

	sampleDetails := []client.ItemAccessDetailsItem{
		{
			Principal: client.PrincipalDetail{
				ID:   "33333333-3333-3333-3333-333333333333",
				Type: "User",
			},
			ItemAccessDetails: client.ItemAccessDetailModel{
				Type:        "Warehouse",
				Permissions: []string{"Read"},
			},
		},
	}

	// Wrong principal ID
	idMismatch, _ := checkPrincipalMatch(sampleDetails, "44444444-4444-4444-4444-444444444444", "", []string{"Read"}, "subset")
	if idMismatch {
		t.Fatal("expected false for ID mismatch")
	}

	// Principal type filter mismatch
	typeMismatch, _ := checkPrincipalMatch(sampleDetails, "33333333-3333-3333-3333-333333333333", "Group", []string{"Read"}, "subset")
	if typeMismatch {
		t.Fatal("expected false for principal type mismatch")
	}

	// Exact match with count mismatch
	exactFail, _ := checkPrincipalMatch(sampleDetails, "33333333-3333-3333-3333-333333333333", "User", []string{"Read", "Write"}, "exact")
	if exactFail {
		t.Fatal("expected false for exact match count mismatch")
	}

	// Exact match with wrong permission
	exactWrong, _ := checkPrincipalMatch(sampleDetails, "33333333-3333-3333-3333-333333333333", "User", []string{"Write"}, "exact")
	if exactWrong {
		t.Fatal("expected false for exact match wrong permission")
	}

	// Subset match with missing permission
	subsetMissing, _ := checkPrincipalMatch(sampleDetails, "33333333-3333-3333-3333-333333333333", "User", []string{"Read", "Write"}, "subset")
	if subsetMissing {
		t.Fatal("expected false for subset match with missing permission")
	}

	// 7. ImportState validation error paths with configured client
	srv := fabricmock.NewServer(t)
	c, err := client.NewFabricClient(client.Config{
		Endpoint: srv.URL(),
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	configuredGate := &ItemShareGateResource{client: c}

	invalidIDs := []string{
		"only-two/parts",
		"not-uuid/22222222-2222-2222-2222-222222222222/33333333-3333-3333-3333-333333333333",
		"11111111-1111-1111-1111-111111111111/not-uuid/33333333-3333-3333-3333-333333333333",
		"11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/not-uuid",
	}
	for _, invID := range invalidIDs {
		var impResp tfsdkresource.ImportStateResponse
		configuredGate.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: invID}, &impResp)
		if !impResp.Diagnostics.HasError() {
			t.Fatalf("expected ImportState error for invalid ID %q", invID)
		}
	}

	// Principal not found on item during import
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "empty_item",
		Type:        "Warehouse",
	})
	var impMissingResp tfsdkresource.ImportStateResponse
	configuredGate.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/%s/99999999-9999-9999-9999-999999999999", wsID, itemID),
	}, &impMissingResp)
	if !impMissingResp.Diagnostics.HasError() {
		t.Fatal("expected ImportState error for missing principal")
	}

	// 8. pollForItemShare context canceled branch
	cancCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, _, pollErr := configuredGate.pollForItemShare(
		cancCtx, wsID, itemID, nil, "33333333-3333-3333-3333-333333333333", "", []string{"Read"}, "subset", 1*time.Minute,
	)
	if pollErr == nil || !errors.Is(pollErr, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got: %v", pollErr)
	}

	// 9. Read when item returns 404 -> RemoveResource
	var gSchemaResp tfsdkresource.SchemaResponse
	configuredGate.Schema(ctx, tfsdkresource.SchemaRequest{}, &gSchemaResp)

	missingItemID := "88888888-8888-8888-8888-888888888888"
	expectedPerms, _ = types.SetValueFrom(ctx, types.StringType, []string{"Read"})
	stateModel404 := ItemShareGateResourceModel{
		ID:                  types.StringValue(fmt.Sprintf("%s/%s/33333333-3333-3333-3333-333333333333", wsID, missingItemID)),
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(missingItemID),
		PrincipalID:         types.StringValue("33333333-3333-3333-3333-333333333333"),
		ExpectedPermissions: expectedPerms,
		MatchType:           types.StringValue("subset"),
	}
	state404 := tfsdk.State{Schema: gSchemaResp.Schema}
	_ = state404.Set(ctx, &stateModel404)
	var read404Resp tfsdkresource.ReadResponse
	read404Resp.State = state404
	configuredGate.Read(ctx, tfsdkresource.ReadRequest{State: state404}, &read404Resp)
	if read404Resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on 404 Read: %v", read404Resp.Diagnostics)
	}
	if !read404Resp.State.Raw.IsNull() {
		t.Fatal("expected state removed on 404 Read")
	}

	// 10. Read when permissions drift (!matched) -> RemoveResource
	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{},
	})
	stateModelDrift := ItemShareGateResourceModel{
		ID:                  types.StringValue(fmt.Sprintf("%s/%s/33333333-3333-3333-3333-333333333333", wsID, itemID)),
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(itemID),
		PrincipalID:         types.StringValue("33333333-3333-3333-3333-333333333333"),
		ExpectedPermissions: expectedPerms,
		MatchType:           types.StringValue("subset"),
	}
	stateDrift := tfsdk.State{Schema: gSchemaResp.Schema}
	_ = stateDrift.Set(ctx, &stateModelDrift)
	var readDriftResp tfsdkresource.ReadResponse
	readDriftResp.State = stateDrift
	configuredGate.Read(ctx, tfsdkresource.ReadRequest{State: stateDrift}, &readDriftResp)
	if readDriftResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on drift Read: %v", readDriftResp.Diagnostics)
	}
	if !readDriftResp.State.Raw.IsNull() {
		t.Fatal("expected state removed on drift Read")
	}

	// 11. Update and Create with invalid timeouts
	badTimeoutModel := ItemShareGateResourceModel{
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(itemID),
		PrincipalID:         types.StringValue("33333333-3333-3333-3333-333333333333"),
		ExpectedPermissions: expectedPerms,
		MatchType:           types.StringValue("subset"),
		Timeouts: &ItemShareGateTimeoutsModel{
			Create: types.StringValue("invalid-duration"),
			Update: types.StringValue("invalid-duration"),
		},
	}
	badPlan := tfsdk.Plan{Schema: gSchemaResp.Schema}
	_ = badPlan.Set(ctx, &badTimeoutModel)
	var createBadTimeoutResp tfsdkresource.CreateResponse
	configuredGate.Create(ctx, tfsdkresource.CreateRequest{Plan: badPlan}, &createBadTimeoutResp)
	if !createBadTimeoutResp.Diagnostics.HasError() {
		t.Fatal("expected error on Create with invalid timeout")
	}

	var updateBadTimeoutResp tfsdkresource.UpdateResponse
	configuredGate.Update(ctx, tfsdkresource.UpdateRequest{Plan: badPlan, State: stateDrift}, &updateBadTimeoutResp)
	if !updateBadTimeoutResp.Diagnostics.HasError() {
		t.Fatal("expected error on Update with invalid timeout")
	}
}

func baseTestProviderModel() FabricProviderModel {
	return FabricProviderModel{
		Endpoint:                       types.StringNull(),
		AccessToken:                    types.StringNull(),
		TenantID:                       types.StringNull(),
		TenantIDFilePath:               types.StringNull(),
		ClientID:                       types.StringNull(),
		ClientIDFilePath:               types.StringNull(),
		ClientSecret:                   types.StringNull(),
		ClientSecretFilePath:           types.StringNull(),
		ClientCertificate:              types.StringNull(),
		ClientCertificateFilePath:      types.StringNull(),
		ClientCertificatePassword:      types.StringNull(),
		UseMSI:                         types.BoolNull(),
		UseOIDC:                        types.BoolNull(),
		OIDCToken:                      types.StringNull(),
		OIDCTokenFilePath:              types.StringNull(),
		OIDCRequestToken:               types.StringNull(),
		OIDCRequestURL:                 types.StringNull(),
		AzureDevOpsServiceConnectionID: types.StringNull(),
		UseCLI:                         types.BoolNull(),
		UseDevCLI:                      types.BoolNull(),
		Environment:                    types.StringNull(),
		AuxiliaryTenantIDs:             types.ListNull(types.StringType),
		RequestTimeout:                 types.StringNull(),
		SkipCredentialsValidation:      types.BoolNull(),
		PowerBIEndpoint:                types.StringNull(),
		PowerBIAccessToken:             types.StringNull(),
	}
}

func TestProvider_ConfigureEnvironmentsAndEdgeCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := New("test")()
	var schemaResp tfsdkprovider.SchemaResponse
	p.Schema(ctx, tfsdkprovider.SchemaRequest{}, &schemaResp)

	// 1. usgovernment environment with use_oidc and auxiliary_tenant_ids
	auxList, _ := types.ListValueFrom(ctx, types.StringType, []string{"00000000-0000-0000-0000-000000000099"})
	modelGov := baseTestProviderModel()
	modelGov.Environment = types.StringValue("usgovernment")
	modelGov.UseOIDC = types.BoolValue(true)
	modelGov.AuxiliaryTenantIDs = auxList
	modelGov.AccessToken = types.StringValue("mock-token")
	modelGov.SkipCredentialsValidation = types.BoolValue(true)

	pGov := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = pGov.Set(ctx, &modelGov)
	cfgGov := tfsdk.Config{Schema: schemaResp.Schema, Raw: pGov.Raw}
	var respGov tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{Config: cfgGov}, &respGov)
	if respGov.Diagnostics.HasError() {
		t.Fatalf("unexpected error configuring usgovernment: %v", respGov.Diagnostics)
	}

	// 2. china environment with fallback tenant ID
	modelChina := baseTestProviderModel()
	modelChina.Environment = types.StringValue("china")
	modelChina.AccessToken = types.StringValue("mock-token")
	modelChina.SkipCredentialsValidation = types.BoolValue(true)

	pChina := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = pChina.Set(ctx, &modelChina)
	cfgChina := tfsdk.Config{Schema: schemaResp.Schema, Raw: pChina.Raw}
	var respChina tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{Config: cfgChina}, &respChina)
	if respChina.Diagnostics.HasError() {
		t.Fatalf("unexpected error configuring china: %v", respChina.Diagnostics)
	}
	dChina, ok := respChina.ResourceData.(*Data)
	if !ok || dChina.TenantID != defaultFallbackTenantID {
		t.Fatalf("expected defaultFallbackTenantID, got %+v", dChina)
	}

	// 3. Invalid request timeout causes Configure to fail
	modelBadTimeout := baseTestProviderModel()
	modelBadTimeout.RequestTimeout = types.StringValue("invalid-duration-xyz")
	modelBadTimeout.AccessToken = types.StringValue("mock-token")
	modelBadTimeout.SkipCredentialsValidation = types.BoolValue(true)

	pBadTimeout := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = pBadTimeout.Set(ctx, &modelBadTimeout)
	cfgBadTimeout := tfsdk.Config{Schema: schemaResp.Schema, Raw: pBadTimeout.Raw}
	var respBadTimeout tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{Config: cfgBadTimeout}, &respBadTimeout)
	if !respBadTimeout.Diagnostics.HasError() {
		t.Fatal("expected error on invalid request timeout")
	}

	// 4. Missing tenant/client ID with client secret fails ValidateConfig
	modelBadSecret := baseTestProviderModel()
	modelBadSecret.ClientSecret = types.StringValue("some-secret")
	modelBadSecret.TenantID = types.StringValue("")
	modelBadSecret.ClientID = types.StringValue("")
	modelBadSecret.AccessToken = types.StringNull()
	modelBadSecret.SkipCredentialsValidation = types.BoolValue(true)

	pBadSecret := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = pBadSecret.Set(ctx, &modelBadSecret)
	cfgBadSecret := tfsdk.Config{Schema: schemaResp.Schema, Raw: pBadSecret.Raw}
	var respBadSecret tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{Config: cfgBadSecret}, &respBadSecret)
	if !respBadSecret.Diagnostics.HasError() {
		t.Fatal("expected error on client secret without tenant/client id")
	}

	// 5. Invalid tenant_id_file_path fails ResolveTenantID
	modelBadTenantFile := baseTestProviderModel()
	modelBadTenantFile.TenantIDFilePath = types.StringValue("/nonexistent/tenant/file/path")
	modelBadTenantFile.AccessToken = types.StringValue("mock-token")
	modelBadTenantFile.SkipCredentialsValidation = types.BoolValue(true)

	pBadTenantFile := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = pBadTenantFile.Set(ctx, &modelBadTenantFile)
	cfgBadTenantFile := tfsdk.Config{Schema: schemaResp.Schema, Raw: pBadTenantFile.Raw}
	var respBadTenantFile tfsdkprovider.ConfigureResponse
	p.Configure(ctx, tfsdkprovider.ConfigureRequest{Config: cfgBadTenantFile}, &respBadTenantFile)
	if !respBadTenantFile.Diagnostics.HasError() {
		t.Fatal("expected error on non-existent tenant_id_file_path")
	}
}

func TestLakehouseOnelakeSecurity_NameResolutionSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv := fabricmock.NewServer(t)
	wsID := "00000000-0000-0000-0000-000000000001"
	lhID := "00000000-0000-0000-0000-000000000002"
	lhName := "gold_lakehouse"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: lhName,
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	c, err := client.NewFabricClient(client.Config{
		Endpoint: srv.URL(),
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	res := &LakehouseOnelakeSecurityResource{client: c}
	var schemaResp tfsdkresource.SchemaResponse
	res.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

	// 1. Create with LakehouseName only (resolving name to ID)
	planModel := LakehouseOnelakeSecurityResourceModel{
		WorkspaceID:             types.StringValue(wsID),
		LakehouseName:           types.StringValue(lhName),
		LakehouseID:             types.StringNull(),
		RemoveDefaultReaderRole: types.BoolValue(false),
	}
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan.Set(ctx, &planModel)
	var createResp tfsdkresource.CreateResponse
	createResp.State = tfsdk.State{Schema: schemaResp.Schema}
	res.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error creating with name only: %v", createResp.Diagnostics)
	}
	var createdModel LakehouseOnelakeSecurityResourceModel
	_ = createResp.State.Get(ctx, &createdModel)
	if createdModel.LakehouseID.ValueString() != lhID {
		t.Errorf("expected resolved lakehouse id %s, got %s", lhID, createdModel.LakehouseID.ValueString())
	}

	// 2. ImportState with valid composite ID (wsID/lhID)
	var impResp tfsdkresource.ImportStateResponse
	impModel := LakehouseOnelakeSecurityResourceModel{
		ID:                      types.StringNull(),
		WorkspaceID:             types.StringNull(),
		LakehouseID:             types.StringNull(),
		LakehouseName:           types.StringNull(),
		RemoveDefaultReaderRole: types.BoolNull(),
		DefaultReaderRoleExists: types.BoolNull(),
		Enabled:                 types.BoolNull(),
	}
	impState := tfsdk.State{Schema: schemaResp.Schema}
	_ = impState.Set(ctx, &impModel)
	impResp.State = impState
	res.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/%s", wsID, lhID),
	}, &impResp)
	if impResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error importing: %v", impResp.Diagnostics)
	}
	var importedModel LakehouseOnelakeSecurityResourceModel
	_ = impResp.State.Get(ctx, &importedModel)
	if importedModel.LakehouseID.ValueString() != lhID {
		t.Errorf("expected imported lakehouse id %s, got %s", lhID, importedModel.LakehouseID.ValueString())
	}

	// ImportState with invalid formats
	invalidIDs := []string{
		"only-one-part",
		fmt.Sprintf("not-a-uuid/%s", lhID),
		fmt.Sprintf("%s/not-a-uuid", wsID),
	}
	for _, badID := range invalidIDs {
		var badImpResp tfsdkresource.ImportStateResponse
		badState := tfsdk.State{Schema: schemaResp.Schema}
		_ = badState.Set(ctx, &impModel)
		badImpResp.State = badState
		res.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: badID}, &badImpResp)
		if !badImpResp.Diagnostics.HasError() {
			t.Errorf("expected error importing invalid id %q", badID)
		}
	}

	// 3. Update on non-existent lakehouse returns error
	stateModel404 := LakehouseOnelakeSecurityResourceModel{
		ID:                      types.StringValue(wsID + "/00000000-0000-0000-0000-000000000099"),
		WorkspaceID:             types.StringValue(wsID),
		LakehouseID:             types.StringValue("00000000-0000-0000-0000-000000000099"),
		LakehouseName:           types.StringValue("missing-lh"),
		RemoveDefaultReaderRole: types.BoolValue(true),
	}
	state404 := tfsdk.State{Schema: schemaResp.Schema}
	_ = state404.Set(ctx, &stateModel404)
	plan404 := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan404.Set(ctx, &stateModel404)
	var up404Resp tfsdkresource.UpdateResponse
	res.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan404, State: state404}, &up404Resp)
	if !up404Resp.Diagnostics.HasError() {
		t.Fatal("expected error on updating non-existent lakehouse")
	}

	// 4. Create with conflicting identifiers
	planConflictModel := LakehouseOnelakeSecurityResourceModel{
		WorkspaceID:             types.StringValue(wsID),
		LakehouseID:             types.StringValue(lhID),
		LakehouseName:           types.StringValue("mismatched-name"),
		RemoveDefaultReaderRole: types.BoolValue(false),
	}
	planConflict := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = planConflict.Set(ctx, &planConflictModel)
	var createConflictResp tfsdkresource.CreateResponse
	createConflictResp.State = tfsdk.State{Schema: schemaResp.Schema}
	res.Create(ctx, tfsdkresource.CreateRequest{Plan: planConflict}, &createConflictResp)
	if !createConflictResp.Diagnostics.HasError() {
		t.Fatal("expected error on conflicting lakehouse identifiers")
	}

	// 5. Update to remove DefaultReader role
	stateModelValid := LakehouseOnelakeSecurityResourceModel{
		ID:                      types.StringValue(fmt.Sprintf("%s/%s", wsID, lhID)),
		WorkspaceID:             types.StringValue(wsID),
		LakehouseID:             types.StringValue(lhID),
		LakehouseName:           types.StringValue(lhName),
		RemoveDefaultReaderRole: types.BoolValue(false),
		DefaultReaderRoleExists: types.BoolValue(true),
		Enabled:                 types.BoolValue(true),
	}
	stateValid := tfsdk.State{Schema: schemaResp.Schema}
	_ = stateValid.Set(ctx, &stateModelValid)
	planRemoveModel := stateModelValid
	planRemoveModel.RemoveDefaultReaderRole = types.BoolValue(true)
	planRemove := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = planRemove.Set(ctx, &planRemoveModel)
	var upRemoveResp tfsdkresource.UpdateResponse
	upRemoveResp.State = tfsdk.State{Schema: schemaResp.Schema}
	res.Update(ctx, tfsdkresource.UpdateRequest{Plan: planRemove, State: stateValid}, &upRemoveResp)
	if upRemoveResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error removing default reader: %v", upRemoveResp.Diagnostics)
	}
	var updatedModel LakehouseOnelakeSecurityResourceModel
	_ = upRemoveResp.State.Get(ctx, &updatedModel)
	if updatedModel.DefaultReaderRoleExists.ValueBool() {
		t.Errorf("expected default reader role to be removed")
	}

	// 6. Nil client error handling across CRUD
	resNil := &LakehouseOnelakeSecurityResource{client: nil}
	var nilCreateResp tfsdkresource.CreateResponse
	nilCreateResp.State = tfsdk.State{Schema: schemaResp.Schema}
	resNil.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &nilCreateResp)
	if !nilCreateResp.Diagnostics.HasError() {
		t.Error("expected error on Create with nil client")
	}

	var nilReadResp tfsdkresource.ReadResponse
	nilReadResp.State = stateValid
	resNil.Read(ctx, tfsdkresource.ReadRequest{State: stateValid}, &nilReadResp)
	if !nilReadResp.Diagnostics.HasError() {
		t.Error("expected error on Read with nil client")
	}

	var nilUpdateResp tfsdkresource.UpdateResponse
	nilUpdateResp.State = stateValid
	resNil.Update(ctx, tfsdkresource.UpdateRequest{Plan: planRemove, State: stateValid}, &nilUpdateResp)
	if !nilUpdateResp.Diagnostics.HasError() {
		t.Error("expected error on Update with nil client")
	}

	var nilDeleteResp tfsdkresource.DeleteResponse
	resNil.Delete(ctx, tfsdkresource.DeleteRequest{State: stateValid}, &nilDeleteResp)
	if len(nilDeleteResp.Diagnostics.Warnings()) == 0 {
		t.Error("expected warning on Delete")
	}

	var nilImportResp tfsdkresource.ImportStateResponse
	resNil.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: fmt.Sprintf("%s/%s", wsID, lhID)}, &nilImportResp)
	if !nilImportResp.Diagnostics.HasError() {
		t.Error("expected error on ImportState with nil client")
	}

	// 7. ValidateConfig with missing identifiers
	rawMissingVal := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":                         tftypes.NewValue(tftypes.String, nil),
		"workspace_id":               tftypes.NewValue(tftypes.String, wsID),
		"lakehouse_id":               tftypes.NewValue(tftypes.String, nil),
		"lakehouse_name":             tftypes.NewValue(tftypes.String, nil),
		"remove_default_reader_role": tftypes.NewValue(tftypes.Bool, false),
		"default_reader_role_exists": tftypes.NewValue(tftypes.Bool, nil),
		"enabled":                    tftypes.NewValue(tftypes.Bool, nil),
	})
	badValConfig := tfsdk.Config{Schema: schemaResp.Schema, Raw: rawMissingVal}
	var valResp tfsdkresource.ValidateConfigResponse
	res.ValidateConfig(ctx, tfsdkresource.ValidateConfigRequest{Config: badValConfig}, &valResp)
	if !valResp.Diagnostics.HasError() {
		t.Error("expected error on ValidateConfig with missing identifiers")
	}

	// 8. ModifyPlan reversion guard and null plan
	stateRevert := stateModelValid
	stateRevert.RemoveDefaultReaderRole = types.BoolValue(true)
	planRevert := stateModelValid
	planRevert.RemoveDefaultReaderRole = types.BoolValue(false)
	stateRevertObj := tfsdk.State{Schema: schemaResp.Schema}
	_ = stateRevertObj.Set(ctx, &stateRevert)
	planRevertObj := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = planRevertObj.Set(ctx, &planRevert)
	configRevertRaw := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":                         tftypes.NewValue(tftypes.String, fmt.Sprintf("%s/%s", wsID, lhID)),
		"workspace_id":               tftypes.NewValue(tftypes.String, wsID),
		"lakehouse_id":               tftypes.NewValue(tftypes.String, lhID),
		"lakehouse_name":             tftypes.NewValue(tftypes.String, lhName),
		"remove_default_reader_role": tftypes.NewValue(tftypes.Bool, false),
		"default_reader_role_exists": tftypes.NewValue(tftypes.Bool, true),
		"enabled":                    tftypes.NewValue(tftypes.Bool, true),
	})
	configRevertObj := tfsdk.Config{Schema: schemaResp.Schema, Raw: configRevertRaw}
	var modRevertResp tfsdkresource.ModifyPlanResponse
	modRevertResp.Plan = planRevertObj
	res.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{
		State:  stateRevertObj,
		Plan:   planRevertObj,
		Config: configRevertObj,
	}, &modRevertResp)
	if !modRevertResp.Diagnostics.HasError() {
		t.Error("expected error on reverting remove_default_reader_role")
	}

	// Null plan (destroy)
	nullPlan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}
	var modNullResp tfsdkresource.ModifyPlanResponse
	res.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{Plan: nullPlan}, &modNullResp)
	if modNullResp.Diagnostics.HasError() {
		t.Error("expected no error on null ModifyPlan")
	}

	// 9. ImportState on non-existent lakehouse and unenabled lakehouse
	var impNotFoundResp tfsdkresource.ImportStateResponse
	impNotFoundResp.State = impState
	res.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/00000000-0000-0000-0000-000000000099", wsID),
	}, &impNotFoundResp)
	if !impNotFoundResp.Diagnostics.HasError() {
		t.Error("expected error importing non-existent lakehouse")
	}

	unenabledLH := "00000000-0000-0000-0000-000000000088"
	srv.UpsertItem(fabricmock.Item{
		ID:          unenabledLH,
		WorkspaceID: wsID,
		DisplayName: "unsecured_lakehouse",
		Type:        "Lakehouse",
	})
	var impUnenabledResp tfsdkresource.ImportStateResponse
	impUnenabledResp.State = impState
	res.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/%s", wsID, unenabledLH),
	}, &impUnenabledResp)
	if !impUnenabledResp.Diagnostics.HasError() {
		t.Error("expected error importing lakehouse with security not enabled")
	}

	// 10. Read when security is disabled/unenabled removes resource
	stateUnenabledModel := LakehouseOnelakeSecurityResourceModel{
		ID:          types.StringValue(fmt.Sprintf("%s/%s", wsID, unenabledLH)),
		WorkspaceID: types.StringValue(wsID),
		LakehouseID: types.StringValue(unenabledLH),
	}
	stateUnenabled := tfsdk.State{Schema: schemaResp.Schema}
	_ = stateUnenabled.Set(ctx, &stateUnenabledModel)
	var readUnenabledResp tfsdkresource.ReadResponse
	readUnenabledResp.State = stateUnenabled
	res.Read(ctx, tfsdkresource.ReadRequest{State: stateUnenabled}, &readUnenabledResp)
	if !readUnenabledResp.State.Raw.IsNull() {
		t.Error("expected state removed when lakehouse security is not enabled")
	}
}

func TestItemDataSource_ReadEdges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := &ItemDataSource{client: nil}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	rawVal := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"workspace_id": tftypes.NewValue(tftypes.String, "11111111-1111-1111-1111-111111111111"),
		"display_name": tftypes.NewValue(tftypes.String, "analytics_wh"),
		"type":         tftypes.NewValue(tftypes.String, "Warehouse"),
	})
	cfg := tfsdk.Config{Schema: schemaResp.Schema, Raw: rawVal}

	var readResp datasource.ReadResponse
	d.Read(ctx, datasource.ReadRequest{Config: cfg}, &readResp)
	if !readResp.Diagnostics.HasError() {
		t.Fatal("expected error reading item data source with nil client")
	}
}

func TestItemShareGateResource_DetailedErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv := fabricmock.NewServer(t)
	wsID := "00000000-0000-0000-0000-000000000001"
	itemID := "00000000-0000-0000-0000-000000000002"
	principalID := "00000000-0000-0000-0000-000000000003"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "sample_wh",
		Type:        "Warehouse",
	})
	pType := fabadmin.PrincipalTypeUser
	iType := fabadmin.ItemType("Warehouse")
	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.UserPrincipal{
					ID:   &principalID,
					Type: &pType,
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:        &iType,
					Permissions: []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
				},
			},
		},
	})

	c, err := client.NewFabricClient(client.Config{
		Endpoint: srv.URL(),
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	gate := &ItemShareGateResource{client: c}
	var schemaResp tfsdkresource.SchemaResponse
	gate.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

	expectedPerms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
	stateModel := ItemShareGateResourceModel{
		ID:                  types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID)),
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(itemID),
		PrincipalID:         types.StringValue(principalID),
		PrincipalType:       types.StringNull(),
		Type:                types.StringNull(),
		ExpectedPermissions: expectedPerms,
		MatchType:           types.StringValue("exact"),
		Timeouts: &ItemShareGateTimeoutsModel{
			Read:   types.StringValue("5s"),
			Create: types.StringValue("5s"),
			Update: types.StringValue("5s"),
		},
	}
	state := tfsdk.State{Schema: schemaResp.Schema}
	_ = state.Set(ctx, &stateModel)

	// 1. Read populates PrincipalType and Type when null
	var readResp tfsdkresource.ReadResponse
	readResp.State = state
	gate.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on Read: %v", readResp.Diagnostics)
	}
	var populatedModel ItemShareGateResourceModel
	_ = readResp.State.Get(ctx, &populatedModel)
	if populatedModel.PrincipalType.ValueString() != "User" {
		t.Errorf("expected PrincipalType User, got %s", populatedModel.PrincipalType.ValueString())
	}
	if populatedModel.Type.ValueString() != "Warehouse" {
		t.Errorf("expected Type Warehouse, got %s", populatedModel.Type.ValueString())
	}

	// 2. Read with AdminAPIAccessDeniedError
	srv.SetAdminForbidden(wsID, itemID, true)
	var readForbiddenResp tfsdkresource.ReadResponse
	readForbiddenResp.State = state
	gate.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readForbiddenResp)
	if !readForbiddenResp.Diagnostics.HasError() {
		t.Fatal("expected error on admin forbidden read")
	}

	// 3. Read with failing client (generic error)
	badClient, _ := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		HTTPClient: &http.Client{
			Transport: &errRoundTripper{},
		},
	})
	gateBad := &ItemShareGateResource{client: badClient}
	var readBadResp tfsdkresource.ReadResponse
	readBadResp.State = state
	gateBad.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readBadResp)
	if !readBadResp.Diagnostics.HasError() {
		t.Fatal("expected error on generic client failure")
	}

	// 4. Update with invalid update timeout
	badTimeoutPlan := ItemShareGateResourceModel{
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(itemID),
		PrincipalID:         types.StringValue(principalID),
		ExpectedPermissions: expectedPerms,
		Timeouts: &ItemShareGateTimeoutsModel{
			Update: types.StringValue("invalid-duration-xyz"),
		},
	}
	badTimeoutPlanObj := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = badTimeoutPlanObj.Set(ctx, &badTimeoutPlan)
	var updateBadTimeoutResp tfsdkresource.UpdateResponse
	gate.Update(ctx, tfsdkresource.UpdateRequest{Plan: badTimeoutPlanObj, State: state}, &updateBadTimeoutResp)
	if !updateBadTimeoutResp.Diagnostics.HasError() {
		t.Fatal("expected error on invalid update timeout duration")
	}

	// 5. Update with nil client
	gateNil := &ItemShareGateResource{client: nil}
	var updateNilResp tfsdkresource.UpdateResponse
	gateNil.Update(ctx, tfsdkresource.UpdateRequest{Plan: badTimeoutPlanObj, State: state}, &updateNilResp)
	if !updateNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on update with nil client")
	}

	// 6. Successful Update
	srv.SetAdminForbidden(wsID, itemID, false)
	updatePlanModel := stateModel
	updatePlanModel.PrincipalType = types.StringNull()
	updatePlanModel.Type = types.StringNull()
	updatePlanObj := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = updatePlanObj.Set(ctx, &updatePlanModel)
	var updateResp tfsdkresource.UpdateResponse
	updateResp.State = tfsdk.State{Schema: schemaResp.Schema}
	gate.Update(ctx, tfsdkresource.UpdateRequest{Plan: updatePlanObj, State: state}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on update: %v", updateResp.Diagnostics)
	}

	// 7. Create with already cancelled context
	canceledCtx, cancelFn := context.WithCancel(ctx)
	cancelFn()
	var createCanceledResp tfsdkresource.CreateResponse
	createCanceledResp.State = state
	gate.Create(canceledCtx, tfsdkresource.CreateRequest{Plan: updatePlanObj}, &createCanceledResp)
	if !createCanceledResp.Diagnostics.HasError() {
		t.Fatal("expected error on cancelled context Create")
	}

	// 8. Create with 401 Unauthorized API error from ListItemAccessDetails
	p401Client, _ := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errorCode":"Unauthorized","message":"token expired"}`)),
				}, nil
			}),
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	gate401 := &ItemShareGateResource{client: p401Client}
	var create401Resp tfsdkresource.CreateResponse
	create401Resp.State = state
	gate401.Create(ctx, tfsdkresource.CreateRequest{Plan: updatePlanObj}, &create401Resp)
	if !create401Resp.Diagnostics.HasError() {
		t.Fatal("expected error on 401 unauthorized")
	}
}

func TestLakehouseDataAccessRole_CoverageExpansion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv := fabricmock.NewServer(t)
	wsID := "00000000-0000-0000-0000-000000000001"
	lhID := "00000000-0000-0000-0000-000000000002"
	lhName := "lakehouse_gold"
	roleName := "TestRole"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: lhName,
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	c, err := client.NewFabricClient(client.Config{
		Endpoint: srv.URL(),
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	badClient, _ := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		HTTPClient: &http.Client{
			Transport: &errRoundTripper{},
		},
	})

	roleRes := &LakehouseDataAccessRoleResource{client: c}
	var schemaResp tfsdkresource.SchemaResponse
	roleRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &schemaResp)

	principalIDs, _ := types.SetValueFrom(ctx, types.StringType, []string{"00000000-0000-0000-0000-000000000099"})
	paths, _ := types.SetValueFrom(ctx, types.StringType, []string{"/Tables/sales"})
	actions, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	planModel := LakehouseDataAccessRoleResourceModel{
		WorkspaceID:      types.StringValue(wsID),
		LakehouseID:      types.StringValue(lhID),
		LakehouseName:    types.StringValue(lhName),
		RoleName:         types.StringValue(roleName),
		PrincipalIDs:     principalIDs,
		PrincipalType:    types.StringValue("User"),
		Paths:            paths,
		Actions:          actions,
		DecisionRule:     types.ListNull(decisionRuleElemType),
		EntraMember:      types.SetNull(entraMemberElemType),
		FabricItemMember: types.SetNull(fabricItemMemberElemType),
	}
	stateModel := planModel
	stateModel.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, lhID, roleName))

	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = plan.Set(ctx, &planModel)
	state := tfsdk.State{Schema: schemaResp.Schema}
	_ = state.Set(ctx, &stateModel)

	// 1. Nil client tests
	nilRes := &LakehouseDataAccessRoleResource{client: nil}
	var createNilResp tfsdkresource.CreateResponse
	nilRes.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createNilResp)
	if !createNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on Create with nil client")
	}

	var readNilResp tfsdkresource.ReadResponse
	nilRes.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readNilResp)
	if !readNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on Read with nil client")
	}

	var updateNilResp tfsdkresource.UpdateResponse
	nilRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateNilResp)
	if !updateNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on Update with nil client")
	}

	var delNilResp tfsdkresource.DeleteResponse
	nilRes.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delNilResp)
	if !delNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on Delete with nil client")
	}

	var impNilResp tfsdkresource.ImportStateResponse
	nilRes.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: fmt.Sprintf("%s/%s/%s", wsID, lhID, roleName)}, &impNilResp)
	if !impNilResp.Diagnostics.HasError() {
		t.Fatal("expected error on ImportState with nil client")
	}

	// 2. Create conflicting lakehouse identifiers (name != displayName)
	badNamePlanModel := planModel
	badNamePlanModel.LakehouseName = types.StringValue("wrong_name")
	badNamePlan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = badNamePlan.Set(ctx, &badNamePlanModel)
	var createConflictResp tfsdkresource.CreateResponse
	roleRes.Create(ctx, tfsdkresource.CreateRequest{Plan: badNamePlan}, &createConflictResp)
	if !createConflictResp.Diagnostics.HasError() {
		t.Fatal("expected error on Create with conflicting lakehouse name")
	}

	// 3. Create with lakehouse_name not found
	missingNamePlanModel := planModel
	missingNamePlanModel.LakehouseID = types.StringNull()
	missingNamePlanModel.LakehouseName = types.StringValue("nonexistent_lakehouse")
	missingNamePlan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = missingNamePlan.Set(ctx, &missingNamePlanModel)
	var createMissingNameResp tfsdkresource.CreateResponse
	roleRes.Create(ctx, tfsdkresource.CreateRequest{Plan: missingNamePlan}, &createMissingNameResp)
	if !createMissingNameResp.Diagnostics.HasError() {
		t.Fatal("expected error on Create with missing lakehouse name")
	}

	// 3b. Create with lakehouse_id not found
	missingIDPlanModel := planModel
	missingIDPlanModel.LakehouseID = types.StringValue("00000000-0000-0000-0000-000000000099")
	missingIDPlanModel.LakehouseName = types.StringNull()
	missingIDPlan := tfsdk.Plan{Schema: schemaResp.Schema}
	_ = missingIDPlan.Set(ctx, &missingIDPlanModel)
	var createMissingIDResp tfsdkresource.CreateResponse
	roleRes.Create(ctx, tfsdkresource.CreateRequest{Plan: missingIDPlan}, &createMissingIDResp)
	if !createMissingIDResp.Diagnostics.HasError() {
		t.Fatal("expected error on Create with missing lakehouse ID")
	}

	// 4. Update with failing client
	badRes := &LakehouseDataAccessRoleResource{client: badClient}
	var updateBadResp tfsdkresource.UpdateResponse
	badRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateBadResp)
	if !updateBadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Update with failing client")
	}

	// 5. Delete with failing client
	var delBadResp tfsdkresource.DeleteResponse
	badRes.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delBadResp)
	if !delBadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Delete with failing client")
	}

	// 6. Read with lakehouse 404 (removed from state)
	state404Model := stateModel
	state404Model.LakehouseID = types.StringValue("00000000-0000-0000-0000-000000000099")
	state404 := tfsdk.State{Schema: schemaResp.Schema}
	_ = state404.Set(ctx, &state404Model)
	var read404Resp tfsdkresource.ReadResponse
	read404Resp.State = state404
	roleRes.Read(ctx, tfsdkresource.ReadRequest{State: state404}, &read404Resp)
	if read404Resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on 404 read: %v", read404Resp.Diagnostics)
	}
	if !read404Resp.State.Raw.IsNull() {
		t.Fatal("expected state removed on 404 lakehouse read")
	}

	// 7. Read with role 404 (removed from state)
	stateMissingRoleModel := stateModel
	stateMissingRoleModel.RoleName = types.StringValue("MissingRole")
	stateMissingRole := tfsdk.State{Schema: schemaResp.Schema}
	_ = stateMissingRole.Set(ctx, &stateMissingRoleModel)
	var readMissingRoleResp tfsdkresource.ReadResponse
	readMissingRoleResp.State = stateMissingRole
	roleRes.Read(ctx, tfsdkresource.ReadRequest{State: stateMissingRole}, &readMissingRoleResp)
	if readMissingRoleResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on missing role read: %v", readMissingRoleResp.Diagnostics)
	}
	if !readMissingRoleResp.State.Raw.IsNull() {
		t.Fatal("expected state removed on 404 role read")
	}

	// 8. ImportState invalid IDs
	invalidImportIDs := []string{
		"only-one-part",
		fmt.Sprintf("%s/%s", wsID, lhID),
		fmt.Sprintf("not-a-uuid/%s/%s", lhID, roleName),
		fmt.Sprintf("%s/not-a-uuid/%s", wsID, roleName),
		fmt.Sprintf("%s/%s/123-invalid-regex", wsID, lhID),
		fmt.Sprintf("%s/00000000-0000-0000-0000-000000000099/%s", wsID, roleName),
		fmt.Sprintf("%s/%s/NonExistentRole", wsID, lhID),
	}
	for _, badID := range invalidImportIDs {
		var badImpResp tfsdkresource.ImportStateResponse
		roleRes.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: badID}, &badImpResp)
		if !badImpResp.Diagnostics.HasError() {
			t.Errorf("expected error importing invalid id %q", badID)
		}
	}
}
