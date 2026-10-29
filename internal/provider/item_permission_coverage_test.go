// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"testing"

	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/pbimock"
)

func TestItemPermissionResources_ConfigureEdgeCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	whConf, ok := NewWarehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
	if !ok {
		t.Fatal("expected ResourceWithConfigure")
	}
	sqlConf, ok := NewSQLDatabasePermissionResource().(tfsdkresource.ResourceWithConfigure)
	if !ok {
		t.Fatal("expected ResourceWithConfigure")
	}
	lhConf, ok := NewLakehousePermissionResource().(tfsdkresource.ResourceWithConfigure)
	if !ok {
		t.Fatal("expected ResourceWithConfigure")
	}

	resources := []struct {
		name string
		r    tfsdkresource.ResourceWithConfigure
	}{
		{"Warehouse", whConf},
		{"SQLDatabase", sqlConf},
		{"Lakehouse", lhConf},
	}

	for _, res := range resources {
		t.Run(res.name+" nil provider data", func(t *testing.T) {
			t.Parallel()
			var resp tfsdkresource.ConfigureResponse
			res.r.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: nil}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error for %s on nil ProviderData", res.name)
			}
		})

		t.Run(res.name+" invalid provider data", func(t *testing.T) {
			t.Parallel()
			var resp tfsdkresource.ConfigureResponse
			res.r.Configure(ctx, tfsdkresource.ConfigureRequest{ProviderData: "invalid-string"}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("expected error for %s on invalid ProviderData", res.name)
			}
		})
	}
}

func TestItemPermissionResources_NilClientOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	wh := NewWarehousePermissionResource()
	sql := NewSQLDatabasePermissionResource()
	lh := NewLakehousePermissionResource()

	// 1. Warehouse nil client checks
	whRes, ok := wh.(*WarehousePermissionResource)
	if !ok {
		t.Fatal("expected *WarehousePermissionResource")
	}
	whRes.client = nil

	var whCreateResp tfsdkresource.CreateResponse
	whRes.Create(ctx, tfsdkresource.CreateRequest{}, &whCreateResp)
	if !whCreateResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Create with nil client")
	}

	var whReadResp tfsdkresource.ReadResponse
	whRes.Read(ctx, tfsdkresource.ReadRequest{}, &whReadResp)
	if !whReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Read with nil client")
	}

	var whUpdateResp tfsdkresource.UpdateResponse
	whRes.Update(ctx, tfsdkresource.UpdateRequest{}, &whUpdateResp)
	if !whUpdateResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Update with nil client")
	}

	var whDeleteResp tfsdkresource.DeleteResponse
	whRes.Delete(ctx, tfsdkresource.DeleteRequest{}, &whDeleteResp)
	if !whDeleteResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Delete with nil client")
	}

	var whImportResp tfsdkresource.ImportStateResponse
	whRes.ImportState(ctx, tfsdkresource.ImportStateRequest{}, &whImportResp)
	if !whImportResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse ImportState with nil client")
	}

	// 2. SQLDatabase nil client checks
	sqlRes, ok := sql.(*SQLDatabasePermissionResource)
	if !ok {
		t.Fatal("expected *SQLDatabasePermissionResource")
	}
	sqlRes.client = nil

	var sqlCreateResp tfsdkresource.CreateResponse
	sqlRes.Create(ctx, tfsdkresource.CreateRequest{}, &sqlCreateResp)
	if !sqlCreateResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Create with nil client")
	}

	var sqlReadResp tfsdkresource.ReadResponse
	sqlRes.Read(ctx, tfsdkresource.ReadRequest{}, &sqlReadResp)
	if !sqlReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Read with nil client")
	}

	var sqlUpdateResp tfsdkresource.UpdateResponse
	sqlRes.Update(ctx, tfsdkresource.UpdateRequest{}, &sqlUpdateResp)
	if !sqlUpdateResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Update with nil client")
	}

	var sqlDeleteResp tfsdkresource.DeleteResponse
	sqlRes.Delete(ctx, tfsdkresource.DeleteRequest{}, &sqlDeleteResp)
	if !sqlDeleteResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Delete with nil client")
	}

	var sqlImportResp tfsdkresource.ImportStateResponse
	sqlRes.ImportState(ctx, tfsdkresource.ImportStateRequest{}, &sqlImportResp)
	if !sqlImportResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase ImportState with nil client")
	}

	// 3. Lakehouse nil client checks
	lhRes, ok := lh.(*LakehousePermissionResource)
	if !ok {
		t.Fatal("expected *LakehousePermissionResource")
	}
	lhRes.client = nil

	var lhCreateResp tfsdkresource.CreateResponse
	lhRes.Create(ctx, tfsdkresource.CreateRequest{}, &lhCreateResp)
	if !lhCreateResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Create with nil client")
	}

	var lhReadResp tfsdkresource.ReadResponse
	lhRes.Read(ctx, tfsdkresource.ReadRequest{}, &lhReadResp)
	if !lhReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Read with nil client")
	}

	var lhUpdateResp tfsdkresource.UpdateResponse
	lhRes.Update(ctx, tfsdkresource.UpdateRequest{}, &lhUpdateResp)
	if !lhUpdateResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Update with nil client")
	}

	var lhDeleteResp tfsdkresource.DeleteResponse
	lhRes.Delete(ctx, tfsdkresource.DeleteRequest{}, &lhDeleteResp)
	if !lhDeleteResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Delete with nil client")
	}

	var lhImportResp tfsdkresource.ImportStateResponse
	lhRes.ImportState(ctx, tfsdkresource.ImportStateRequest{}, &lhImportResp)
	if !lhImportResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse ImportState with nil client")
	}
}

func TestItemPermissionResources_FailingClientOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badClient, err := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
		HTTPClient: &http.Client{
			Transport: &errRoundTripper{},
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	perms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	// 1. Warehouse failing client Update and Delete
	whRes := &WarehousePermissionResource{client: badClient}
	var whSchemaResp tfsdkresource.SchemaResponse
	whRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchemaResp)

	whModel := WarehousePermissionResourceModel{
		ID:            types.StringValue("11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/33333333-3333-3333-3333-333333333333"),
		WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
		WarehouseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
		WarehouseName: types.StringValue("my-wh"),
		PrincipalID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
		PrincipalType: types.StringValue("User"),
		Permissions:   perms,
	}

	whPlan := tfsdk.Plan{Schema: whSchemaResp.Schema}
	whState := tfsdk.State{Schema: whSchemaResp.Schema}
	_ = whPlan.Set(ctx, &whModel)
	_ = whState.Set(ctx, &whModel)

	var whUpResp tfsdkresource.UpdateResponse
	whRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: whPlan, State: whState}, &whUpResp)
	if !whUpResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Update with failing client")
	}

	var whDelResp tfsdkresource.DeleteResponse
	whRes.Delete(ctx, tfsdkresource.DeleteRequest{State: whState}, &whDelResp)
	if !whDelResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Delete with failing client")
	}

	var whReadResp tfsdkresource.ReadResponse
	whRes.Read(ctx, tfsdkresource.ReadRequest{State: whState}, &whReadResp)
	if !whReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Warehouse Read with failing client")
	}

	// 2. SQLDatabase failing client Update and Delete
	sqlRes := &SQLDatabasePermissionResource{client: badClient}
	var sqlSchemaResp tfsdkresource.SchemaResponse
	sqlRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchemaResp)

	sqlModel := SQLDatabasePermissionResourceModel{
		ID:              types.StringValue("11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/33333333-3333-3333-3333-333333333333"),
		WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
		SQLDatabaseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
		SQLDatabaseName: types.StringValue("my-sqldb"),
		PrincipalID:     types.StringValue("33333333-3333-3333-3333-333333333333"),
		PrincipalType:   types.StringValue("User"),
		Permissions:     perms,
	}

	sqlPlan := tfsdk.Plan{Schema: sqlSchemaResp.Schema}
	sqlState := tfsdk.State{Schema: sqlSchemaResp.Schema}
	_ = sqlPlan.Set(ctx, &sqlModel)
	_ = sqlState.Set(ctx, &sqlModel)

	var sqlUpResp tfsdkresource.UpdateResponse
	sqlRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: sqlPlan, State: sqlState}, &sqlUpResp)
	if !sqlUpResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Update with failing client")
	}

	var sqlDelResp tfsdkresource.DeleteResponse
	sqlRes.Delete(ctx, tfsdkresource.DeleteRequest{State: sqlState}, &sqlDelResp)
	if !sqlDelResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Delete with failing client")
	}

	var sqlReadResp tfsdkresource.ReadResponse
	sqlRes.Read(ctx, tfsdkresource.ReadRequest{State: sqlState}, &sqlReadResp)
	if !sqlReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on SQLDatabase Read with failing client")
	}

	// 3. Lakehouse failing client Update and Delete
	lhRes := &LakehousePermissionResource{client: badClient}
	var lhSchemaResp tfsdkresource.SchemaResponse
	lhRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchemaResp)

	lhModel := LakehousePermissionResourceModel{
		ID:            types.StringValue("11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/User/33333333-3333-3333-3333-333333333333"),
		WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
		LakehouseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
		LakehouseName: types.StringValue("my-lh"),
		PrincipalID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
		PrincipalType: types.StringValue("User"),
		Permissions:   perms,
	}

	lhPlan := tfsdk.Plan{Schema: lhSchemaResp.Schema}
	lhState := tfsdk.State{Schema: lhSchemaResp.Schema}
	_ = lhPlan.Set(ctx, &lhModel)
	_ = lhState.Set(ctx, &lhModel)

	var lhUpResp tfsdkresource.UpdateResponse
	lhRes.Update(ctx, tfsdkresource.UpdateRequest{Plan: lhPlan, State: lhState}, &lhUpResp)
	if !lhUpResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Update with failing client")
	}

	var lhDelResp tfsdkresource.DeleteResponse
	lhRes.Delete(ctx, tfsdkresource.DeleteRequest{State: lhState}, &lhDelResp)
	if !lhDelResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Delete with failing client")
	}

	var lhReadResp tfsdkresource.ReadResponse
	lhRes.Read(ctx, tfsdkresource.ReadRequest{State: lhState}, &lhReadResp)
	if !lhReadResp.Diagnostics.HasError() {
		t.Fatal("expected error on Lakehouse Read with failing client")
	}

	// 4. ImportState tests by name with failing client
	var whImpResp1 tfsdkresource.ImportStateResponse
	whRes.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: "11111111-1111-1111-1111-111111111111/named-wh/User/33333333-3333-3333-3333-333333333333",
	}, &whImpResp1)
	if !whImpResp1.Diagnostics.HasError() {
		t.Fatal("expected error importing warehouse by name with failing client")
	}

	var sqlImpResp1 tfsdkresource.ImportStateResponse
	sqlRes.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: "11111111-1111-1111-1111-111111111111/named-db/User/33333333-3333-3333-3333-333333333333",
	}, &sqlImpResp1)
	if !sqlImpResp1.Diagnostics.HasError() {
		t.Fatal("expected error importing sqldatabase by name with failing client")
	}

	var lhImpResp1 tfsdkresource.ImportStateResponse
	lhRes.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: "11111111-1111-1111-1111-111111111111/named-lh/User/33333333-3333-3333-3333-333333333333",
	}, &lhImpResp1)
	if !lhImpResp1.Diagnostics.HasError() {
		t.Fatal("expected error importing lakehouse by name with failing client")
	}
}

func TestItemPermissionResources_ValidateConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	whRes := NewWarehousePermissionResource()
	wh, ok := whRes.(tfsdkresource.ResourceWithValidateConfig)
	if !ok {
		t.Fatal("expected ResourceWithValidateConfig")
	}

	sqlRes := NewSQLDatabasePermissionResource()
	sql, ok := sqlRes.(tfsdkresource.ResourceWithValidateConfig)
	if !ok {
		t.Fatal("expected ResourceWithValidateConfig")
	}

	lhRes := NewLakehousePermissionResource()
	lh, ok := lhRes.(tfsdkresource.ResourceWithValidateConfig)
	if !ok {
		t.Fatal("expected ResourceWithValidateConfig")
	}

	var whSchemaResp tfsdkresource.SchemaResponse
	whRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchemaResp)
	var sqlSchemaResp tfsdkresource.SchemaResponse
	sqlRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchemaResp)
	var lhSchemaResp tfsdkresource.SchemaResponse
	lhRes.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchemaResp)

	testCases := []struct {
		name       string
		notify     types.Bool
		message    types.String
		shouldFail bool
	}{
		{
			name:       "message without notify fails",
			notify:     types.BoolValue(false),
			message:    types.StringValue("hello"),
			shouldFail: true,
		},
		{
			name:       "message with null notify fails",
			notify:     types.BoolNull(),
			message:    types.StringValue("hello"),
			shouldFail: true,
		},
		{
			name:       "message with notify = true passes",
			notify:     types.BoolValue(true),
			message:    types.StringValue("hello"),
			shouldFail: false,
		},
		{
			name:       "notify = false with null message passes",
			notify:     types.BoolValue(false),
			message:    types.StringNull(),
			shouldFail: false,
		},
	}

	perms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	for _, tc := range testCases {
		t.Run("Warehouse "+tc.name, func(t *testing.T) {
			t.Parallel()
			model := WarehousePermissionResourceModel{
				ID:            types.StringNull(),
				WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
				WarehouseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
				WarehouseName: types.StringNull(),
				PrincipalID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
				PrincipalType: types.StringValue("User"),
				Permissions:   perms,
				Notify:        tc.notify,
				Message:       tc.message,
			}
			plan := tfsdk.Plan{Schema: whSchemaResp.Schema}
			_ = plan.Set(ctx, &model)
			cfg := tfsdk.Config{Schema: whSchemaResp.Schema, Raw: plan.Raw}
			var resp tfsdkresource.ValidateConfigResponse
			wh.ValidateConfig(ctx, tfsdkresource.ValidateConfigRequest{Config: cfg}, &resp)
			if tc.shouldFail && !resp.Diagnostics.HasError() {
				t.Fatalf("expected error for Warehouse on %s", tc.name)
			}
			if !tc.shouldFail && resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error for Warehouse on %s: %v", tc.name, resp.Diagnostics)
			}
		})

		t.Run("SQLDatabase "+tc.name, func(t *testing.T) {
			t.Parallel()
			model := SQLDatabasePermissionResourceModel{
				ID:              types.StringNull(),
				WorkspaceID:     types.StringValue("11111111-1111-1111-1111-111111111111"),
				SQLDatabaseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
				SQLDatabaseName: types.StringNull(),
				PrincipalID:     types.StringValue("33333333-3333-3333-3333-333333333333"),
				PrincipalType:   types.StringValue("User"),
				Permissions:     perms,
				Notify:          tc.notify,
				Message:         tc.message,
			}
			plan := tfsdk.Plan{Schema: sqlSchemaResp.Schema}
			_ = plan.Set(ctx, &model)
			cfg := tfsdk.Config{Schema: sqlSchemaResp.Schema, Raw: plan.Raw}
			var resp tfsdkresource.ValidateConfigResponse
			sql.ValidateConfig(ctx, tfsdkresource.ValidateConfigRequest{Config: cfg}, &resp)
			if tc.shouldFail && !resp.Diagnostics.HasError() {
				t.Fatalf("expected error for SQLDatabase on %s", tc.name)
			}
			if !tc.shouldFail && resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error for SQLDatabase on %s: %v", tc.name, resp.Diagnostics)
			}
		})

		t.Run("Lakehouse "+tc.name, func(t *testing.T) {
			t.Parallel()
			model := LakehousePermissionResourceModel{
				ID:            types.StringNull(),
				WorkspaceID:   types.StringValue("11111111-1111-1111-1111-111111111111"),
				LakehouseID:   types.StringValue("22222222-2222-2222-2222-222222222222"),
				LakehouseName: types.StringNull(),
				PrincipalID:   types.StringValue("33333333-3333-3333-3333-333333333333"),
				PrincipalType: types.StringValue("User"),
				Permissions:   perms,
				Notify:        tc.notify,
				Message:       tc.message,
			}
			plan := tfsdk.Plan{Schema: lhSchemaResp.Schema}
			_ = plan.Set(ctx, &model)
			cfg := tfsdk.Config{Schema: lhSchemaResp.Schema, Raw: plan.Raw}
			var resp tfsdkresource.ValidateConfigResponse
			lh.ValidateConfig(ctx, tfsdkresource.ValidateConfigRequest{Config: cfg}, &resp)
			if tc.shouldFail && !resp.Diagnostics.HasError() {
				t.Fatalf("expected error for Lakehouse on %s", tc.name)
			}
			if !tc.shouldFail && resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error for Lakehouse on %s: %v", tc.name, resp.Diagnostics)
			}
		})
	}
}

func TestItemPermissionResources_ReadDriftAndNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	srv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("start pbimock: %v", err)
	}
	defer pbiSrv.Close()

	mockClient, err := client.NewFabricClient(client.Config{
		Endpoint:             srv.URL(),
		PowerBIEndpoint:      pbiSrv.URL,
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	wh := &WarehousePermissionResource{client: mockClient}
	sql := &SQLDatabasePermissionResource{client: mockClient}
	lh := &LakehousePermissionResource{client: mockClient}

	var whSchemaResp tfsdkresource.SchemaResponse
	wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchemaResp)
	var sqlSchemaResp tfsdkresource.SchemaResponse
	sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchemaResp)
	var lhSchemaResp tfsdkresource.SchemaResponse
	lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchemaResp)

	wsID := "11111111-1111-1111-1111-111111111111"
	missingItemID := "99999999-9999-9999-9999-999999999999"
	principalID := "33333333-3333-3333-3333-333333333333"
	perms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	// 1. Warehouse Read 404 (item does not exist) -> RemoveResource
	{
		model := WarehousePermissionResourceModel{
			ID:            types.StringValue(wsID + "/" + missingItemID + "/User/" + principalID),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(missingItemID),
			WarehouseName: types.StringValue("missing-wh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: whSchemaResp.Schema}
		_ = state.Set(ctx, &model)
		var resp tfsdkresource.ReadResponse
		resp.State = state
		wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on 404 read: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed on 404 read")
		}
	}

	// 2. SQLDatabase Read 404 (item does not exist) -> RemoveResource
	{
		model := SQLDatabasePermissionResourceModel{
			ID:              types.StringValue(wsID + "/" + missingItemID + "/User/" + principalID),
			WorkspaceID:     types.StringValue(wsID),
			SQLDatabaseID:   types.StringValue(missingItemID),
			SQLDatabaseName: types.StringValue("missing-db"),
			PrincipalID:     types.StringValue(principalID),
			PrincipalType:   types.StringValue("User"),
			Permissions:     perms,
		}
		state := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = state.Set(ctx, &model)
		var resp tfsdkresource.ReadResponse
		resp.State = state
		sql.Read(ctx, tfsdkresource.ReadRequest{State: state}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on 404 read: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed on 404 read")
		}
	}

	// 3. Lakehouse Read 404 (item does not exist) -> RemoveResource
	{
		model := LakehousePermissionResourceModel{
			ID:            types.StringValue(wsID + "/" + missingItemID + "/User/" + principalID),
			WorkspaceID:   types.StringValue(wsID),
			LakehouseID:   types.StringValue(missingItemID),
			LakehouseName: types.StringValue("missing-lh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = state.Set(ctx, &model)
		var resp tfsdkresource.ReadResponse
		resp.State = state
		lh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on 404 read: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed on 404 read")
		}
	}
}
