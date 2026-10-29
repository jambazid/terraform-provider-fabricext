// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func TestRequiresReplaceIfConfiguredModifier(t *testing.T) {
	t.Parallel()

	mod := RequiresReplaceIfConfigured()
	if mod.Description(context.Background()) == "" {
		t.Fatal("expected non-empty description")
	}
	if mod.MarkdownDescription(context.Background()) == "" {
		t.Fatal("expected non-empty markdown description")
	}

	// 1. ConfigValue is null -> no replacement required
	var resp1 planmodifier.StringResponse
	mod.PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		StateValue:  types.StringValue("old"),
		PlanValue:   types.StringValue("new"),
	}, &resp1)
	if resp1.RequiresReplace {
		t.Fatal("expected RequiresReplace = false when ConfigValue is null")
	}

	// 2. StateValue is null -> no replacement required
	var resp2 planmodifier.StringResponse
	mod.PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: types.StringValue("new"),
		StateValue:  types.StringNull(),
		PlanValue:   types.StringValue("new"),
	}, &resp2)
	if resp2.RequiresReplace {
		t.Fatal("expected RequiresReplace = false when StateValue is null")
	}

	// 3. PlanValue equals StateValue -> no replacement required
	var resp3 planmodifier.StringResponse
	mod.PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: types.StringValue("same"),
		StateValue:  types.StringValue("same"),
		PlanValue:   types.StringValue("same"),
	}, &resp3)
	if resp3.RequiresReplace {
		t.Fatal("expected RequiresReplace = false when Plan equals State")
	}

	// 4. Configured and changed -> requires replacement
	var resp4 planmodifier.StringResponse
	mod.PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: types.StringValue("new"),
		StateValue:  types.StringValue("old"),
		PlanValue:   types.StringValue("new"),
	}, &resp4)
	if !resp4.RequiresReplace {
		t.Fatal("expected RequiresReplace = true when Configured and changed")
	}
}

func TestParseItemPermissionImportID(t *testing.T) {
	t.Parallel()

	validWS := "11111111-1111-1111-1111-111111111111"
	validItem := "22222222-2222-2222-2222-222222222222"
	validPrincipal := "33333333-3333-3333-3333-333333333333"

	// Success
	ws, item, pType, pID, err := parseItemPermissionImportID(
		fmt.Sprintf("%s/%s/User/%s", validWS, validItem, validPrincipal),
		"Warehouse",
	)
	if err != nil {
		t.Fatalf("unexpected error on valid import ID: %v", err)
	}
	if ws != validWS || item != validItem || pType != "User" || pID != validPrincipal {
		t.Fatalf("unexpected parsed values: %s, %s, %s, %s", ws, item, pType, pID)
	}

	// Invalid segment count
	_, _, _, _, err = parseItemPermissionImportID("too/few/parts", "Warehouse")
	if err == nil || !strings.Contains(err.Error(), "expected format") {
		t.Fatalf("expected format error, got %v", err)
	}

	// Invalid workspace UUID
	_, _, _, _, err = parseItemPermissionImportID(
		fmt.Sprintf("not-a-uuid/%s/User/%s", validItem, validPrincipal),
		"Warehouse",
	)
	if err == nil || !strings.Contains(err.Error(), "workspace_id") {
		t.Fatalf("expected workspace_id error, got %v", err)
	}

	// Empty item identifier
	_, _, _, _, err = parseItemPermissionImportID(
		fmt.Sprintf("%s/   /User/%s", validWS, validPrincipal),
		"Warehouse",
	)
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty identifier error, got %v", err)
	}

	// Invalid principal UUID
	_, _, _, _, err = parseItemPermissionImportID(
		fmt.Sprintf("%s/%s/User/bad-uuid", validWS, validItem),
		"Warehouse",
	)
	if err == nil || !strings.Contains(err.Error(), "principal_id") {
		t.Fatalf("expected principal_id error, got %v", err)
	}

	// Invalid principal type
	_, _, _, _, err = parseItemPermissionImportID(
		fmt.Sprintf("%s/%s/UnsupportedType/%s", validWS, validItem, validPrincipal),
		"Warehouse",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid principal_type") {
		t.Fatalf("expected invalid principal_type error, got %v", err)
	}
}

func TestFilterManagedPermissions(t *testing.T) {
	t.Parallel()

	// Lakehouse permissions
	lhPerms := []string{"Read", "readdata", "read_all", "Reshare", "Execute", "Write"}
	filteredLH := filterManagedPermissions(lhPerms, "Lakehouse")
	expectedLH := []string{"Read", "ReadData", "ReadAll", "Reshare", "Execute"}
	if len(filteredLH) != len(expectedLH) {
		t.Fatalf("expected %d perms, got %d (%v)", len(expectedLH), len(filteredLH), filteredLH)
	}

	// SQLDatabase permissions (includes Write)
	sqlPerms := []string{"Read", "ReadData", "ReadAll", "Reshare", "Execute", "Write"}
	filteredSQL := filterManagedPermissions(sqlPerms, "SQLDatabase")
	if len(filteredSQL) != 6 {
		t.Fatalf("expected 6 perms for SQLDatabase, got %d (%v)", len(filteredSQL), filteredSQL)
	}

	// Warehouse permissions (Write and Execute not supported on Warehouse)
	whPerms := []string{"Read", "ReadData", "ReadAll", "Reshare", "Execute", "Write"}
	filteredWH := filterManagedPermissions(whPerms, "Warehouse")
	if len(filteredWH) != 4 {
		t.Fatalf("expected 4 perms for Warehouse, got %d (%v)", len(filteredWH), filteredWH)
	}
}

func TestReconcileItemIdentifiersCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. Neither ID nor Name configured -> error
	_, _, diags := reconcileItemIdentifiersCreate(ctx, nil, "Warehouse", "11111111-1111-1111-1111-111111111111", "", "")
	if !diags.HasError() {
		t.Fatal("expected error when neither ID nor Name configured")
	}

	// Create test client with failing transport
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

	// 2. ID configured, client returns network error
	_, _, diags = reconcileItemIdentifiersCreate(ctx, badClient, "Warehouse", "11111111-1111-1111-1111-111111111111", "", "22222222-2222-2222-2222-222222222222")
	if !diags.HasError() {
		t.Fatal("expected error on failed client call with ID")
	}

	// 3. Name configured, client returns network error
	_, _, diags = reconcileItemIdentifiersCreate(ctx, badClient, "Warehouse", "11111111-1111-1111-1111-111111111111", "my-warehouse", "")
	if !diags.HasError() {
		t.Fatal("expected error on failed client call with Name")
	}

	srv := fabricmock.NewServer(t)
	mockClient, err := client.NewFabricClient(client.Config{
		Endpoint: srv.URL(),
		TokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "my-warehouse",
		Type:        "Warehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          "33333333-3333-3333-3333-333333333333",
		WorkspaceID: wsID,
		DisplayName: "my-lakehouse",
		Type:        "Lakehouse",
	})

	// 4. ID not found
	_, _, diags = reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "", "99999999-9999-9999-9999-999999999999")
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Not Found") {
		t.Fatalf("expected Not Found error for missing ID, got %v", diags)
	}

	// 5. Type mismatch (lookup Warehouse with Lakehouse ID)
	_, _, diags = reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "", "33333333-3333-3333-3333-333333333333")
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Item Type Mismatch") {
		t.Fatalf("expected Item Type Mismatch error, got %v", diags)
	}

	// 6. Name mismatch (ID found with name "my-warehouse", but configured "other-name")
	_, _, diags = reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "other-name", whID)
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Identifier Mismatch") {
		t.Fatalf("expected Identifier Mismatch error, got %v", diags)
	}

	// 7. Success with ID and matching Name
	resID, resName, diags := reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "my-warehouse", whID)
	if diags.HasError() {
		t.Fatalf("unexpected error on valid ID and Name: %v", diags)
	}
	if resID != whID || resName != "my-warehouse" {
		t.Fatalf("unexpected resolved values: %s, %s", resID, resName)
	}

	// 8. Name not found
	_, _, diags = reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "absent-warehouse", "")
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Not Found") {
		t.Fatalf("expected Not Found error for missing Name, got %v", diags)
	}

	// 9. Success with Name only
	resID, resName, diags = reconcileItemIdentifiersCreate(ctx, mockClient, "Warehouse", wsID, "my-warehouse", "")
	if diags.HasError() {
		t.Fatalf("unexpected error on valid Name: %v", diags)
	}
	if resID != whID || resName != "my-warehouse" {
		t.Fatalf("unexpected resolved values: %s, %s", resID, resName)
	}
}
