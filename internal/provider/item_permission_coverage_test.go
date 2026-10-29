// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func TestItemPermissionResources_UpstreamRenameAndDisappear(t *testing.T) {
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
		BaseBackoff:          1 * time.Millisecond,
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

	wsID := "00000000-0000-0000-0000-000000000001"
	whID := "00000000-0000-0000-0000-000000000010"
	sqlID := "00000000-0000-0000-0000-000000000020"
	lhID := "00000000-0000-0000-0000-000000000030"
	nbID := "00000000-0000-0000-0000-000000000040" // Notebook item (for type mismatch)
	principalID := "00000000-0000-0000-0000-000000000099"
	perms, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	// Seed items in fabricmock
	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		Type:        "Warehouse",
		DisplayName: "wh-renamed",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          sqlID,
		WorkspaceID: wsID,
		Type:        "SQLDatabase",
		DisplayName: "sql-renamed",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		Type:        "Lakehouse",
		DisplayName: "lh-renamed",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          nbID,
		WorkspaceID: wsID,
		Type:        "Notebook",
		DisplayName: "notebook-item",
	})

	// Seed permissions in pbimock for renamed items
	_ = mockClient.UpsertItemDirectPermissions(ctx, wsID, whID, "Warehouse", client.ItemDirectShare{
		PrincipalID:   principalID,
		PrincipalType: "User",
		Permissions:   []string{"Read"},
	})
	_ = mockClient.UpsertItemDirectPermissions(ctx, wsID, sqlID, "SQLDatabase", client.ItemDirectShare{
		PrincipalID:   principalID,
		PrincipalType: "User",
		Permissions:   []string{"Read"},
	})
	_ = mockClient.UpsertItemDirectPermissions(ctx, wsID, lhID, "Lakehouse", client.ItemDirectShare{
		PrincipalID:   principalID,
		PrincipalType: "User",
		Permissions:   []string{"Read"},
	})

	// 1. Warehouse Read detects upstream rename and updates state
	{
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, whID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(whID),
			WarehouseName: types.StringValue("wh-old-name"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: whSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error reading renamed warehouse: %v", readResp.Diagnostics)
		}
		var updatedModel WarehousePermissionResourceModel
		_ = readResp.State.Get(ctx, &updatedModel)
		if updatedModel.WarehouseName.ValueString() != "wh-renamed" {
			t.Errorf("expected warehouse name updated to wh-renamed, got %q", updatedModel.WarehouseName.ValueString())
		}
	}

	// 2. SQLDatabase Read detects upstream rename and updates state
	{
		stateModel := SQLDatabasePermissionResourceModel{
			ID:              types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, sqlID, principalID)),
			WorkspaceID:     types.StringValue(wsID),
			SQLDatabaseID:   types.StringValue(sqlID),
			SQLDatabaseName: types.StringValue("sql-old-name"),
			PrincipalID:     types.StringValue(principalID),
			PrincipalType:   types.StringValue("User"),
			Permissions:     perms,
		}
		state := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		sql.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error reading renamed sqldatabase: %v", readResp.Diagnostics)
		}
		var updatedModel SQLDatabasePermissionResourceModel
		_ = readResp.State.Get(ctx, &updatedModel)
		if updatedModel.SQLDatabaseName.ValueString() != "sql-renamed" {
			t.Errorf("expected sqldatabase name updated to sql-renamed, got %q", updatedModel.SQLDatabaseName.ValueString())
		}
	}

	// 3. Lakehouse Read detects upstream rename and updates state
	{
		stateModel := LakehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, lhID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			LakehouseID:   types.StringValue(lhID),
			LakehouseName: types.StringValue("lh-old-name"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		lh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error reading renamed lakehouse: %v", readResp.Diagnostics)
		}
		var updatedModel LakehousePermissionResourceModel
		_ = readResp.State.Get(ctx, &updatedModel)
		if updatedModel.LakehouseName.ValueString() != "lh-renamed" {
			t.Errorf("expected lakehouse name updated to lh-renamed, got %q", updatedModel.LakehouseName.ValueString())
		}
	}

	// 4. Type mismatch: Warehouse resource pointing to Notebook item
	{
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, nbID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(nbID),
			WarehouseName: types.StringValue("notebook-item"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: whSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on warehouse reading a notebook item")
		}
	}

	// 5. Type mismatch: SQLDatabase resource pointing to Notebook item
	{
		stateModel := SQLDatabasePermissionResourceModel{
			ID:              types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, nbID, principalID)),
			WorkspaceID:     types.StringValue(wsID),
			SQLDatabaseID:   types.StringValue(nbID),
			SQLDatabaseName: types.StringValue("notebook-item"),
			PrincipalID:     types.StringValue(principalID),
			PrincipalType:   types.StringValue("User"),
			Permissions:     perms,
		}
		state := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		sql.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on sqldatabase reading a notebook item")
		}
	}

	// 6. Type mismatch: Lakehouse resource pointing to Notebook item
	{
		stateModel := LakehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, nbID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			LakehouseID:   types.StringValue(nbID),
			LakehouseName: types.StringValue("notebook-item"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		lh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error on lakehouse reading a notebook item")
		}
	}

	// 7. Out-of-band revocation: permission was deleted directly
	otherPrincipal := "00000000-0000-0000-0000-000000000088"
	{
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, whID, otherPrincipal)),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(whID),
			WarehouseName: types.StringValue("wh-renamed"),
			PrincipalID:   types.StringValue(otherPrincipal),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: whSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on out-of-band revocation read: %v", readResp.Diagnostics)
		}
		if !readResp.State.Raw.IsNull() {
			t.Fatal("expected state removed when share is revoked out of band")
		}
	}

	// 8. Delete when item is already 404 (absent) returns cleanly
	missingID := "00000000-0000-0000-0000-000000000077"
	{
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, missingID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(missingID),
			WarehouseName: types.StringValue("missing-wh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: whSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var delResp tfsdkresource.DeleteResponse
		wh.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delResp)
		if delResp.Diagnostics.HasError() {
			t.Fatalf("expected clean delete on 404 item: %v", delResp.Diagnostics)
		}
	}

	// 9. ImportState by name success
	{
		initModel := WarehousePermissionResourceModel{
			ID:            types.StringNull(),
			WorkspaceID:   types.StringNull(),
			WarehouseID:   types.StringNull(),
			WarehouseName: types.StringNull(),
			PrincipalID:   types.StringNull(),
			PrincipalType: types.StringNull(),
			Permissions:   types.SetNull(types.StringType),
			Notify:        types.BoolNull(),
			Message:       types.StringNull(),
		}
		impState := tfsdk.State{Schema: whSchemaResp.Schema}
		if diags := impState.Set(ctx, &initModel); diags.HasError() {
			t.Fatalf("failed to init impState: %v", diags)
		}
		var impResp tfsdkresource.ImportStateResponse
		impResp.State = impState
		wh.ImportState(ctx, tfsdkresource.ImportStateRequest{
			ID: fmt.Sprintf("%s/wh-renamed/User/%s", wsID, principalID),
		}, &impResp)
		if impResp.Diagnostics.HasError() {
			t.Fatalf("expected successful import by name: %v", impResp.Diagnostics)
		}
		var impModel WarehousePermissionResourceModel
		_ = impResp.State.Get(ctx, &impModel)
		if impModel.WarehouseID.ValueString() != whID {
			t.Errorf("expected warehouse_id %s, got %s", whID, impModel.WarehouseID.ValueString())
		}
		if impModel.WarehouseName.ValueString() != "wh-renamed" {
			t.Errorf("expected warehouse_name wh-renamed, got %s", impModel.WarehouseName.ValueString())
		}
	}

	// 10. ImportState invalid identifier format
	{
		var impResp tfsdkresource.ImportStateResponse
		wh.ImportState(ctx, tfsdkresource.ImportStateRequest{
			ID: "invalid/import/id",
		}, &impResp)
		if !impResp.Diagnostics.HasError() {
			t.Fatal("expected error on invalid import id")
		}
	}

	// 11. SQLDatabase out-of-band revocation, delete 404, import by name, import invalid ID
	{
		stateModel := SQLDatabasePermissionResourceModel{
			ID:              types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, sqlID, otherPrincipal)),
			WorkspaceID:     types.StringValue(wsID),
			SQLDatabaseID:   types.StringValue(sqlID),
			SQLDatabaseName: types.StringValue("sql-renamed"),
			PrincipalID:     types.StringValue(otherPrincipal),
			PrincipalType:   types.StringValue("User"),
			Permissions:     perms,
		}
		state := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		sql.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on SQLDatabase out-of-band revocation read: %v", readResp.Diagnostics)
		}
		if !readResp.State.Raw.IsNull() {
			t.Fatal("expected state removed when sqldatabase share is revoked out of band")
		}

		// Delete 404
		var delResp tfsdkresource.DeleteResponse
		stateModel.SQLDatabaseID = types.StringValue(missingID)
		state404 := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = state404.Set(ctx, &stateModel)
		sql.Delete(ctx, tfsdkresource.DeleteRequest{State: state404}, &delResp)
		if delResp.Diagnostics.HasError() {
			t.Fatalf("expected clean delete on 404 sqldatabase: %v", delResp.Diagnostics)
		}

		// Import by name
		initModel := SQLDatabasePermissionResourceModel{
			ID:              types.StringNull(),
			WorkspaceID:     types.StringNull(),
			SQLDatabaseID:   types.StringNull(),
			SQLDatabaseName: types.StringNull(),
			PrincipalID:     types.StringNull(),
			PrincipalType:   types.StringNull(),
			Permissions:     types.SetNull(types.StringType),
			Notify:          types.BoolNull(),
			Message:         types.StringNull(),
		}
		impState := tfsdk.State{Schema: sqlSchemaResp.Schema}
		_ = impState.Set(ctx, &initModel)
		var impResp tfsdkresource.ImportStateResponse
		impResp.State = impState
		sql.ImportState(ctx, tfsdkresource.ImportStateRequest{
			ID: fmt.Sprintf("%s/sql-renamed/User/%s", wsID, principalID),
		}, &impResp)
		if impResp.Diagnostics.HasError() {
			t.Fatalf("expected successful sqldatabase import by name: %v", impResp.Diagnostics)
		}

		// Import invalid ID
		var impBadResp tfsdkresource.ImportStateResponse
		sql.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: "invalid/id"}, &impBadResp)
		if !impBadResp.Diagnostics.HasError() {
			t.Fatal("expected error on invalid sqldatabase import id")
		}
	}

	// 12. Lakehouse out-of-band revocation, delete 404, import by name, import invalid ID
	{
		stateModel := LakehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, lhID, otherPrincipal)),
			WorkspaceID:   types.StringValue(wsID),
			LakehouseID:   types.StringValue(lhID),
			LakehouseName: types.StringValue("lh-renamed"),
			PrincipalID:   types.StringValue(otherPrincipal),
			PrincipalType: types.StringValue("User"),
			Permissions:   perms,
		}
		state := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = state.Set(ctx, &stateModel)
		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		lh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on Lakehouse out-of-band revocation read: %v", readResp.Diagnostics)
		}
		if !readResp.State.Raw.IsNull() {
			t.Fatal("expected state removed when lakehouse share is revoked out of band")
		}

		// Delete 404
		var delResp tfsdkresource.DeleteResponse
		stateModel.LakehouseID = types.StringValue(missingID)
		state404 := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = state404.Set(ctx, &stateModel)
		lh.Delete(ctx, tfsdkresource.DeleteRequest{State: state404}, &delResp)
		if delResp.Diagnostics.HasError() {
			t.Fatalf("expected clean delete on 404 lakehouse: %v", delResp.Diagnostics)
		}

		// Import by name
		initModel := LakehousePermissionResourceModel{
			ID:            types.StringNull(),
			WorkspaceID:   types.StringNull(),
			LakehouseID:   types.StringNull(),
			LakehouseName: types.StringNull(),
			PrincipalID:   types.StringNull(),
			PrincipalType: types.StringNull(),
			Permissions:   types.SetNull(types.StringType),
			Notify:        types.BoolNull(),
			Message:       types.StringNull(),
		}
		impState := tfsdk.State{Schema: lhSchemaResp.Schema}
		_ = impState.Set(ctx, &initModel)
		var impResp tfsdkresource.ImportStateResponse
		impResp.State = impState
		lh.ImportState(ctx, tfsdkresource.ImportStateRequest{
			ID: fmt.Sprintf("%s/lh-renamed/User/%s", wsID, principalID),
		}, &impResp)
		if impResp.Diagnostics.HasError() {
			t.Fatalf("expected successful lakehouse import by name: %v", impResp.Diagnostics)
		}

		// Import invalid ID
		var impBadResp tfsdkresource.ImportStateResponse
		lh.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: "invalid/id"}, &impBadResp)
		if !impBadResp.Diagnostics.HasError() {
			t.Fatal("expected error on invalid lakehouse import id")
		}
	}
}

type errRoundTripperTest struct{}

func (e *errRoundTripperTest) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("injected network error for test")
}

func TestItemPermissionResources_ErrorsAndDriftPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	wsID := "00000000-0000-0000-0000-000000000001"
	itemID := "00000000-0000-0000-0000-000000000002"
	principalID := "00000000-0000-0000-0000-000000000003"

	badClient, err := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		PowerBITokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		HTTPClient: &http.Client{
			Transport: &errRoundTripperTest{},
		},
	})
	if err != nil {
		t.Fatalf("failed to create badClient: %v", err)
	}

	// 1. Warehouse nil client and error paths
	{
		whRes := NewWarehousePermissionResource()
		wh, ok := whRes.(*WarehousePermissionResource)
		if !ok {
			t.Fatalf("expected *WarehousePermissionResource, got %T", whRes)
		}
		var whSchema tfsdkresource.SchemaResponse
		wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

		permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
		planModel := WarehousePermissionResourceModel{
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(itemID),
			WarehouseName: types.StringValue("wh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   permsVal,
			Notify:        types.BoolValue(false),
			Message:       types.StringNull(),
		}
		stateModel := planModel
		stateModel.ID = types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID))

		plan := tfsdk.Plan{Schema: whSchema.Schema}
		_ = plan.Set(ctx, &planModel)
		state := tfsdk.State{Schema: whSchema.Schema}
		_ = state.Set(ctx, &stateModel)

		// Delete with nil client
		whNilRes := NewWarehousePermissionResource()
		whNil, ok := whNilRes.(*WarehousePermissionResource)
		if !ok {
			t.Fatalf("expected *WarehousePermissionResource, got %T", whNilRes)
		}
		var delNilResp tfsdkresource.DeleteResponse
		whNil.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delNilResp)
		if !delNilResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting warehouse permission with nil client")
		}

		// Create with invalid principal_type
		whBad := &WarehousePermissionResource{client: badClient}
		badPlanModel := planModel
		badPlanModel.PrincipalType = types.StringValue("BogusPrincipalType")
		badPlan := tfsdk.Plan{Schema: whSchema.Schema}
		_ = badPlan.Set(ctx, &badPlanModel)
		var createBadTypeResp tfsdkresource.CreateResponse
		whBad.Create(ctx, tfsdkresource.CreateRequest{Plan: badPlan}, &createBadTypeResp)
		if !createBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error creating warehouse permission with invalid principal_type")
		}

		// Update with invalid principal_type
		var updateBadTypeResp tfsdkresource.UpdateResponse
		whBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: badPlan, State: state}, &updateBadTypeResp)
		if !updateBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error updating warehouse permission with invalid principal_type")
		}

		// Create with failing client
		var createFailResp tfsdkresource.CreateResponse
		whBad.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createFailResp)
		if !createFailResp.Diagnostics.HasError() {
			t.Fatal("expected error creating warehouse permission with failing client")
		}

		// Update with failing client
		var updateFailResp tfsdkresource.UpdateResponse
		whBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateFailResp)
		if !updateFailResp.Diagnostics.HasError() {
			t.Fatal("expected error updating warehouse permission with failing client")
		}

		// Read with failing client (non-404 error)
		var readFailResp tfsdkresource.ReadResponse
		whBad.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readFailResp)
		if !readFailResp.Diagnostics.HasError() {
			t.Fatal("expected error reading warehouse permission with failing client")
		}

		// Delete with failing client
		var delFailResp tfsdkresource.DeleteResponse
		whBad.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delFailResp)
		if !delFailResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting warehouse permission with failing client")
		}
	}

	// 2. SQLDatabase nil client and error paths
	{
		sqlRes := NewSQLDatabasePermissionResource()
		sql, ok := sqlRes.(*SQLDatabasePermissionResource)
		if !ok {
			t.Fatalf("expected *SQLDatabasePermissionResource, got %T", sqlRes)
		}
		var sqlSchema tfsdkresource.SchemaResponse
		sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchema)

		permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
		planModel := SQLDatabasePermissionResourceModel{
			WorkspaceID:     types.StringValue(wsID),
			SQLDatabaseID:   types.StringValue(itemID),
			SQLDatabaseName: types.StringValue("sqldb"),
			PrincipalID:     types.StringValue(principalID),
			PrincipalType:   types.StringValue("User"),
			Permissions:     permsVal,
			Notify:          types.BoolValue(false),
			Message:         types.StringNull(),
		}
		stateModel := planModel
		stateModel.ID = types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID))

		plan := tfsdk.Plan{Schema: sqlSchema.Schema}
		_ = plan.Set(ctx, &planModel)
		state := tfsdk.State{Schema: sqlSchema.Schema}
		_ = state.Set(ctx, &stateModel)

		// Delete with nil client
		sqlNilRes := NewSQLDatabasePermissionResource()
		sqlNil, ok := sqlNilRes.(*SQLDatabasePermissionResource)
		if !ok {
			t.Fatalf("expected *SQLDatabasePermissionResource, got %T", sqlNilRes)
		}
		var delNilResp tfsdkresource.DeleteResponse
		sqlNil.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delNilResp)
		if !delNilResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting sql database permission with nil client")
		}

		// Create with invalid principal_type
		sqlBad := &SQLDatabasePermissionResource{client: badClient}
		badPlanModel := planModel
		badPlanModel.PrincipalType = types.StringValue("BogusPrincipalType")
		badPlan := tfsdk.Plan{Schema: sqlSchema.Schema}
		_ = badPlan.Set(ctx, &badPlanModel)
		var createBadTypeResp tfsdkresource.CreateResponse
		sqlBad.Create(ctx, tfsdkresource.CreateRequest{Plan: badPlan}, &createBadTypeResp)
		if !createBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error creating sql database permission with invalid principal_type")
		}

		// Update with invalid principal_type
		var updateBadTypeResp tfsdkresource.UpdateResponse
		sqlBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: badPlan, State: state}, &updateBadTypeResp)
		if !updateBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error updating sql database permission with invalid principal_type")
		}

		// Create with failing client
		var createFailResp tfsdkresource.CreateResponse
		sqlBad.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createFailResp)
		if !createFailResp.Diagnostics.HasError() {
			t.Fatal("expected error creating sql database permission with failing client")
		}

		// Update with failing client
		var updateFailResp tfsdkresource.UpdateResponse
		sqlBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateFailResp)
		if !updateFailResp.Diagnostics.HasError() {
			t.Fatal("expected error updating sql database permission with failing client")
		}

		// Read with failing client
		var readFailResp tfsdkresource.ReadResponse
		sqlBad.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readFailResp)
		if !readFailResp.Diagnostics.HasError() {
			t.Fatal("expected error reading sql database permission with failing client")
		}

		// Delete with failing client
		var delFailResp tfsdkresource.DeleteResponse
		sqlBad.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delFailResp)
		if !delFailResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting sql database permission with failing client")
		}
	}

	// 3. Lakehouse nil client and error paths
	{
		lhRes := NewLakehousePermissionResource()
		lh, ok := lhRes.(*LakehousePermissionResource)
		if !ok {
			t.Fatalf("expected *LakehousePermissionResource, got %T", lhRes)
		}
		var lhSchema tfsdkresource.SchemaResponse
		lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)

		permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
		planModel := LakehousePermissionResourceModel{
			WorkspaceID:   types.StringValue(wsID),
			LakehouseID:   types.StringValue(itemID),
			LakehouseName: types.StringValue("lh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   permsVal,
			Notify:        types.BoolValue(false),
			Message:       types.StringNull(),
		}
		stateModel := planModel
		stateModel.ID = types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID))

		plan := tfsdk.Plan{Schema: lhSchema.Schema}
		_ = plan.Set(ctx, &planModel)
		state := tfsdk.State{Schema: lhSchema.Schema}
		_ = state.Set(ctx, &stateModel)

		// Delete with nil client
		lhNilRes := NewLakehousePermissionResource()
		lhNil, ok := lhNilRes.(*LakehousePermissionResource)
		if !ok {
			t.Fatalf("expected *LakehousePermissionResource, got %T", lhNilRes)
		}
		var delNilResp tfsdkresource.DeleteResponse
		lhNil.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delNilResp)
		if !delNilResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting lakehouse permission with nil client")
		}

		// Create with invalid principal_type
		lhBad := &LakehousePermissionResource{client: badClient}
		badPlanModel := planModel
		badPlanModel.PrincipalType = types.StringValue("BogusPrincipalType")
		badPlan := tfsdk.Plan{Schema: lhSchema.Schema}
		_ = badPlan.Set(ctx, &badPlanModel)
		var createBadTypeResp tfsdkresource.CreateResponse
		lhBad.Create(ctx, tfsdkresource.CreateRequest{Plan: badPlan}, &createBadTypeResp)
		if !createBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse permission with invalid principal_type")
		}

		// Update with invalid principal_type
		var updateBadTypeResp tfsdkresource.UpdateResponse
		lhBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: badPlan, State: state}, &updateBadTypeResp)
		if !updateBadTypeResp.Diagnostics.HasError() {
			t.Fatal("expected error updating lakehouse permission with invalid principal_type")
		}

		// Create with failing client
		var createFailResp tfsdkresource.CreateResponse
		lhBad.Create(ctx, tfsdkresource.CreateRequest{Plan: plan}, &createFailResp)
		if !createFailResp.Diagnostics.HasError() {
			t.Fatal("expected error creating lakehouse permission with failing client")
		}

		// Update with failing client
		var updateFailResp tfsdkresource.UpdateResponse
		lhBad.Update(ctx, tfsdkresource.UpdateRequest{Plan: plan, State: state}, &updateFailResp)
		if !updateFailResp.Diagnostics.HasError() {
			t.Fatal("expected error updating lakehouse permission with failing client")
		}

		// Read with failing client
		var readFailResp tfsdkresource.ReadResponse
		lhBad.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readFailResp)
		if !readFailResp.Diagnostics.HasError() {
			t.Fatal("expected error reading lakehouse permission with failing client")
		}

		// Delete with failing client
		var delFailResp tfsdkresource.DeleteResponse
		lhBad.Delete(ctx, tfsdkresource.DeleteRequest{State: state}, &delFailResp)
		if !delFailResp.Diagnostics.HasError() {
			t.Fatal("expected error deleting lakehouse permission with failing client")
		}
	}

	// 4. Drift removal when direct share has no managed permissions
	{
		fabSrv := fabricmock.NewServer(t)
		pbiSrv, err := pbimock.NewServer()
		if err != nil {
			t.Fatalf("failed to create pbimock: %v", err)
		}
		defer pbiSrv.Close()

		fabSrv.UpsertItem(fabricmock.Item{
			ID:          itemID,
			WorkspaceID: wsID,
			DisplayName: "sample_wh",
			Type:        "Warehouse",
		})
		// Register share with 0 permissions (core=0, artifact=0)
		pbiSrv.SeedDirectAccess(itemID, map[string]any{
			"userObjectId":        principalID,
			"permissions":         0,
			"artifactPermissions": 0,
		})

		c, err := client.NewFabricClient(client.Config{
			Endpoint:        fabSrv.URL(),
			PowerBIEndpoint: pbiSrv.URL,
			TokenProvider: func(context.Context) (string, error) {
				return "mock-token", nil
			},
			PowerBITokenProvider: func(context.Context) (string, error) {
				return "mock-token", nil
			},
			BaseBackoff: 5 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		wh := &WarehousePermissionResource{client: c}
		var whSchema tfsdkresource.SchemaResponse
		wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

		permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
		stateModel := WarehousePermissionResourceModel{
			ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)),
			WorkspaceID:   types.StringValue(wsID),
			WarehouseID:   types.StringValue(itemID),
			WarehouseName: types.StringValue("sample_wh"),
			PrincipalID:   types.StringValue(principalID),
			PrincipalType: types.StringValue("User"),
			Permissions:   permsVal,
			Notify:        types.BoolValue(false),
			Message:       types.StringNull(),
		}
		state := tfsdk.State{Schema: whSchema.Schema}
		_ = state.Set(ctx, &stateModel)

		var readResp tfsdkresource.ReadResponse
		readResp.State = state
		wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error on unmanaged permissions drift read: %v", readResp.Diagnostics)
		}
		if !readResp.State.Raw.IsNull() {
			t.Fatal("expected state removed when all managed permissions were drifted/revoked")
		}
	}
}

func TestItemPermissionResources_TypeMismatchAndUpstreamRename(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	fabSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("failed to create pbimock: %v", err)
	}
	defer pbiSrv.Close()

	// 1. Mismatched item type: Lakehouse item queried via WarehousePermissionResource
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "my_lakehouse",
		Type:        "Lakehouse",
	})

	c, err := client.NewFabricClient(client.Config{
		Endpoint:        fabSrv.URL(),
		PowerBIEndpoint: pbiSrv.URL,
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		PowerBITokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	wh := &WarehousePermissionResource{client: c}
	var whSchema tfsdkresource.SchemaResponse
	wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

	permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})
	stateModel := WarehousePermissionResourceModel{
		ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)),
		WorkspaceID:   types.StringValue(wsID),
		WarehouseID:   types.StringValue(itemID),
		WarehouseName: types.StringValue("my_lakehouse"),
		PrincipalID:   types.StringValue(principalID),
		PrincipalType: types.StringValue("User"),
		Permissions:   permsVal,
		Notify:        types.BoolValue(false),
		Message:       types.StringNull(),
	}
	state := tfsdk.State{Schema: whSchema.Schema}
	_ = state.Set(ctx, &stateModel)

	var readMismatchResp tfsdkresource.ReadResponse
	readMismatchResp.State = tfsdk.State{Schema: whSchema.Schema}
	wh.Read(ctx, tfsdkresource.ReadRequest{State: state}, &readMismatchResp)
	if !readMismatchResp.Diagnostics.HasError() {
		t.Fatal("expected error on item type mismatch")
	}

	// Type mismatch on SQLDatabase
	sql := &SQLDatabasePermissionResource{client: c}
	var sqlSchema tfsdkresource.SchemaResponse
	sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchema)
	sqlStateModel := SQLDatabasePermissionResourceModel{
		ID:              types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)),
		WorkspaceID:     types.StringValue(wsID),
		SQLDatabaseID:   types.StringValue(itemID),
		SQLDatabaseName: types.StringValue("my_lakehouse"),
		PrincipalID:     types.StringValue(principalID),
		PrincipalType:   types.StringValue("User"),
		Permissions:     permsVal,
		Notify:          types.BoolValue(false),
		Message:         types.StringNull(),
	}
	sqlState := tfsdk.State{Schema: sqlSchema.Schema}
	_ = sqlState.Set(ctx, &sqlStateModel)
	var readSQLMismatchResp tfsdkresource.ReadResponse
	readSQLMismatchResp.State = tfsdk.State{Schema: sqlSchema.Schema}
	sql.Read(ctx, tfsdkresource.ReadRequest{State: sqlState}, &readSQLMismatchResp)
	if !readSQLMismatchResp.Diagnostics.HasError() {
		t.Fatal("expected error on sql database item type mismatch")
	}

	// Type mismatch on Lakehouse (querying a Warehouse item)
	whItemForLh := "55555555-5555-5555-5555-555555555555"
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          whItemForLh,
		WorkspaceID: wsID,
		DisplayName: "real_wh",
		Type:        "Warehouse",
	})
	lh := &LakehousePermissionResource{client: c}
	var lhSchema tfsdkresource.SchemaResponse
	lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)
	lhStateModel := LakehousePermissionResourceModel{
		ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, whItemForLh, principalID)),
		WorkspaceID:   types.StringValue(wsID),
		LakehouseID:   types.StringValue(whItemForLh),
		LakehouseName: types.StringValue("real_wh"),
		PrincipalID:   types.StringValue(principalID),
		PrincipalType: types.StringValue("User"),
		Permissions:   permsVal,
		Notify:        types.BoolValue(false),
		Message:       types.StringNull(),
	}
	lhState := tfsdk.State{Schema: lhSchema.Schema}
	_ = lhState.Set(ctx, &lhStateModel)
	var readLhMismatchResp tfsdkresource.ReadResponse
	readLhMismatchResp.State = tfsdk.State{Schema: lhSchema.Schema}
	lh.Read(ctx, tfsdkresource.ReadRequest{State: lhState}, &readLhMismatchResp)
	if !readLhMismatchResp.Diagnostics.HasError() {
		t.Fatal("expected error on lakehouse item type mismatch")
	}

	// 2. Upstream rename detection
	whID2 := "44444444-4444-4444-4444-444444444444"
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          whID2,
		WorkspaceID: wsID,
		DisplayName: "original_wh",
		Type:        "Warehouse",
	})
	pbiSrv.SeedDirectAccess(whID2, map[string]any{
		"userObjectId":        principalID,
		"permissions":         1,
		"artifactPermissions": 0,
	})

	stateModel2 := stateModel
	stateModel2.WarehouseID = types.StringValue(whID2)
	stateModel2.WarehouseName = types.StringValue("old_stale_name")
	state2 := tfsdk.State{Schema: whSchema.Schema}
	_ = state2.Set(ctx, &stateModel2)

	var readRenameResp tfsdkresource.ReadResponse
	readRenameResp.State = tfsdk.State{Schema: whSchema.Schema}
	wh.Read(ctx, tfsdkresource.ReadRequest{State: state2}, &readRenameResp)
	if readRenameResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on rename read: %v", readRenameResp.Diagnostics)
	}
	var updatedState WarehousePermissionResourceModel
	_ = readRenameResp.State.Get(ctx, &updatedState)
	if updatedState.WarehouseName.ValueString() != "original_wh" {
		t.Errorf("expected updated name original_wh, got: %s", updatedState.WarehouseName.ValueString())
	}
}

func TestItemPermissionResources_ImportByNameAndNilClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	// 1. Nil client on ImportState across all three
	whNil := &WarehousePermissionResource{client: nil}
	var impRespWh tfsdkresource.ImportStateResponse
	whNil.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)}, &impRespWh)
	if !impRespWh.Diagnostics.HasError() {
		t.Error("expected error importing warehouse permission with nil client")
	}

	sqlNil := &SQLDatabasePermissionResource{client: nil}
	var impRespSQL tfsdkresource.ImportStateResponse
	sqlNil.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)}, &impRespSQL)
	if !impRespSQL.Diagnostics.HasError() {
		t.Error("expected error importing sql database permission with nil client")
	}

	lhNil := &LakehousePermissionResource{client: nil}
	var impRespLh tfsdkresource.ImportStateResponse
	lhNil.ImportState(ctx, tfsdkresource.ImportStateRequest{ID: fmt.Sprintf("%s/%s/User/%s", wsID, itemID, principalID)}, &impRespLh)
	if !impRespLh.Diagnostics.HasError() {
		t.Error("expected error importing lakehouse permission with nil client")
	}

	// 2. Import by name resolution (success and failure)
	fabSrv := fabricmock.NewServer(t)
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "sample_wh",
		Type:        "Warehouse",
	})
	c, _ := client.NewFabricClient(client.Config{
		Endpoint:    fabSrv.URL(),
		BaseBackoff: 5 * time.Millisecond,
		TokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
	})
	wh := &WarehousePermissionResource{client: c}
	var whSchema tfsdkresource.SchemaResponse
	wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

	// Failure: non-existent name
	var impFail tfsdkresource.ImportStateResponse
	wh.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/missing_warehouse/User/%s", wsID, principalID),
	}, &impFail)
	if !impFail.Diagnostics.HasError() {
		t.Error("expected error importing by non-existent name")
	}

	// Success: valid name
	var impSuccess tfsdkresource.ImportStateResponse
	impModel := WarehousePermissionResourceModel{
		ID:            types.StringNull(),
		WorkspaceID:   types.StringNull(),
		WarehouseID:   types.StringNull(),
		WarehouseName: types.StringNull(),
		PrincipalType: types.StringNull(),
		PrincipalID:   types.StringNull(),
		Permissions:   types.SetNull(types.StringType),
		Notify:        types.BoolNull(),
		Message:       types.StringNull(),
	}
	impState := tfsdk.State{Schema: whSchema.Schema}
	_ = impState.Set(ctx, &impModel)
	impSuccess.State = impState
	wh.ImportState(ctx, tfsdkresource.ImportStateRequest{
		ID: fmt.Sprintf("%s/sample_wh/User/%s", wsID, principalID),
	}, &impSuccess)
	if impSuccess.Diagnostics.HasError() {
		t.Fatalf("unexpected error importing by name: %v", impSuccess.Diagnostics)
	}
}

func TestItemPermissionResources_ModifyPlanEdges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	wh := &WarehousePermissionResource{}
	var whSchema tfsdkresource.SchemaResponse
	wh.Schema(ctx, tfsdkresource.SchemaRequest{}, &whSchema)

	// Destroy case: req.Plan.Raw.IsNull()
	nullPlan := tfsdk.Plan{Schema: whSchema.Schema, Raw: tftypes.NewValue(whSchema.Schema.Type().TerraformType(ctx), nil)}
	var modResp tfsdkresource.ModifyPlanResponse
	modResp.Plan = nullPlan
	wh.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{Plan: nullPlan}, &modResp)
	if modResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on null modify plan: %v", modResp.Diagnostics)
	}

	sql := &SQLDatabasePermissionResource{}
	var sqlSchema tfsdkresource.SchemaResponse
	sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchema)
	var modSQLResp tfsdkresource.ModifyPlanResponse
	modSQLResp.Plan = nullPlan
	sql.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{Plan: nullPlan}, &modSQLResp)
	if modSQLResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on null modify plan sql: %v", modSQLResp.Diagnostics)
	}

	lh := &LakehousePermissionResource{}
	var lhSchema tfsdkresource.SchemaResponse
	lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)
	var modLhResp tfsdkresource.ModifyPlanResponse
	modLhResp.Plan = nullPlan
	lh.ModifyPlan(ctx, tfsdkresource.ModifyPlanRequest{Plan: nullPlan}, &modLhResp)
	if modLhResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on null modify plan lh: %v", modLhResp.Diagnostics)
	}
}

func TestItemPermissionResources_UpstreamRenameAndDisappear_EdgeCoverage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wsID := "11111111-1111-1111-1111-111111111111"
	sqlID := "22222222-2222-2222-2222-222222222222"
	lhID := "33333333-3333-3333-3333-333333333333"
	principalID := "44444444-4444-4444-4444-444444444444"

	fabSrv := fabricmock.NewServer(t)
	pbiSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("failed to create pbimock: %v", err)
	}
	defer pbiSrv.Close()

	fabSrv.UpsertItem(fabricmock.Item{
		ID:          sqlID,
		WorkspaceID: wsID,
		DisplayName: "original_sqldb",
		Type:        "SQLDatabase",
	})
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "original_lakehouse",
		Type:        "Lakehouse",
	})

	c, _ := client.NewFabricClient(client.Config{
		Endpoint:        fabSrv.URL(),
		PowerBIEndpoint: pbiSrv.URL,
		TokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
		PowerBITokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})

	// 1. SQLDatabase: upstream rename and disappear
	pbiSrv.SeedDirectAccess(sqlID, map[string]any{
		"userObjectId":        principalID,
		"permissions":         1,
		"artifactPermissions": 0,
	})
	sql := &SQLDatabasePermissionResource{client: c}
	var sqlSchema tfsdkresource.SchemaResponse
	sql.Schema(ctx, tfsdkresource.SchemaRequest{}, &sqlSchema)
	permsVal, _ := types.SetValueFrom(ctx, types.StringType, []string{"Read"})

	sqlStateModel := SQLDatabasePermissionResourceModel{
		ID:              types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, sqlID, principalID)),
		WorkspaceID:     types.StringValue(wsID),
		SQLDatabaseID:   types.StringValue(sqlID),
		SQLDatabaseName: types.StringValue("stale_sqldb_name"),
		PrincipalID:     types.StringValue(principalID),
		PrincipalType:   types.StringValue("User"),
		Permissions:     permsVal,
		Notify:          types.BoolValue(false),
		Message:         types.StringNull(),
	}
	sqlState := tfsdk.State{Schema: sqlSchema.Schema}
	_ = sqlState.Set(ctx, &sqlStateModel)

	// Rename check
	var sqlReadResp tfsdkresource.ReadResponse
	sqlReadResp.State = tfsdk.State{Schema: sqlSchema.Schema}
	sql.Read(ctx, tfsdkresource.ReadRequest{State: sqlState}, &sqlReadResp)
	if sqlReadResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on sql read: %v", sqlReadResp.Diagnostics)
	}
	var sqlUpdatedModel SQLDatabasePermissionResourceModel
	_ = sqlReadResp.State.Get(ctx, &sqlUpdatedModel)
	if sqlUpdatedModel.SQLDatabaseName.ValueString() != "original_sqldb" {
		t.Errorf("expected original_sqldb, got %s", sqlUpdatedModel.SQLDatabaseName.ValueString())
	}

	// Disappear check (zero perms)
	sqlIDDisappear := "66666666-6666-6666-6666-666666666666"
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          sqlIDDisappear,
		WorkspaceID: wsID,
		DisplayName: "disappearing_sqldb",
		Type:        "SQLDatabase",
	})
	pbiSrv.SeedDirectAccess(sqlIDDisappear, map[string]any{
		"userObjectId":        principalID,
		"permissions":         0,
		"artifactPermissions": 0,
	})
	sqlDisappearStateModel := sqlStateModel
	sqlDisappearStateModel.SQLDatabaseID = types.StringValue(sqlIDDisappear)
	sqlDisappearState := tfsdk.State{Schema: sqlSchema.Schema}
	_ = sqlDisappearState.Set(ctx, &sqlDisappearStateModel)

	var sqlDisappearResp tfsdkresource.ReadResponse
	sqlDisappearResp.State = sqlDisappearState
	sql.Read(ctx, tfsdkresource.ReadRequest{State: sqlDisappearState}, &sqlDisappearResp)
	if !sqlDisappearResp.State.Raw.IsNull() {
		t.Error("expected sql database permission removed from state on zero perms")
	}

	// 2. Lakehouse: upstream rename and disappear
	pbiSrv.SeedDirectAccess(lhID, map[string]any{
		"userObjectId":        principalID,
		"permissions":         1,
		"artifactPermissions": 0,
	})
	lh := &LakehousePermissionResource{client: c}
	var lhSchema tfsdkresource.SchemaResponse
	lh.Schema(ctx, tfsdkresource.SchemaRequest{}, &lhSchema)

	lhStateModel := LakehousePermissionResourceModel{
		ID:            types.StringValue(fmt.Sprintf("%s/%s/User/%s", wsID, lhID, principalID)),
		WorkspaceID:   types.StringValue(wsID),
		LakehouseID:   types.StringValue(lhID),
		LakehouseName: types.StringValue("stale_lh_name"),
		PrincipalID:   types.StringValue(principalID),
		PrincipalType: types.StringValue("User"),
		Permissions:   permsVal,
		Notify:        types.BoolValue(false),
		Message:       types.StringNull(),
	}
	lhState := tfsdk.State{Schema: lhSchema.Schema}
	_ = lhState.Set(ctx, &lhStateModel)

	// Rename check
	var lhReadResp tfsdkresource.ReadResponse
	lhReadResp.State = tfsdk.State{Schema: lhSchema.Schema}
	lh.Read(ctx, tfsdkresource.ReadRequest{State: lhState}, &lhReadResp)
	if lhReadResp.Diagnostics.HasError() {
		t.Fatalf("unexpected error on lh read: %v", lhReadResp.Diagnostics)
	}
	var lhUpdatedModel LakehousePermissionResourceModel
	_ = lhReadResp.State.Get(ctx, &lhUpdatedModel)
	if lhUpdatedModel.LakehouseName.ValueString() != "original_lakehouse" {
		t.Errorf("expected original_lakehouse, got %s", lhUpdatedModel.LakehouseName.ValueString())
	}

	// Disappear check (zero perms)
	lhIDDisappear := "77777777-7777-7777-7777-777777777777"
	fabSrv.UpsertItem(fabricmock.Item{
		ID:          lhIDDisappear,
		WorkspaceID: wsID,
		DisplayName: "disappearing_lakehouse",
		Type:        "Lakehouse",
	})
	pbiSrv.SeedDirectAccess(lhIDDisappear, map[string]any{
		"userObjectId":        principalID,
		"permissions":         0,
		"artifactPermissions": 0,
	})
	lhDisappearStateModel := lhStateModel
	lhDisappearStateModel.LakehouseID = types.StringValue(lhIDDisappear)
	lhDisappearState := tfsdk.State{Schema: lhSchema.Schema}
	_ = lhDisappearState.Set(ctx, &lhDisappearStateModel)

	var lhDisappearResp tfsdkresource.ReadResponse
	lhDisappearResp.State = lhDisappearState
	lh.Read(ctx, tfsdkresource.ReadRequest{State: lhDisappearState}, &lhDisappearResp)
	if !lhDisappearResp.State.Raw.IsNull() {
		t.Error("expected lakehouse permission removed from state on zero perms")
	}
}
