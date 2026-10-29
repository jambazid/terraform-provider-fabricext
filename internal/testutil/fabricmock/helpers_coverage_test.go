// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package fabricmock

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestServer_InspectorsAndHelpers(t *testing.T) {
	t.Parallel()

	srv := NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"

	srv.UpsertItem(Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "analytics_wh",
		Type:        "Warehouse",
	})

	srv.InjectThrottle(http.StatusTooManyRequests, 2, 1)

	// Trigger throttled request
	resp, err := http.Get(srv.URL() + "/v1/workspaces/" + wsID + "/items?type=Warehouse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 throttle, got %d", resp.StatusCode)
	}

	// Trigger next request after throttle exhausted
	resp2, err := http.Get(srv.URL() + "/v1/workspaces/" + wsID + "/items?type=Warehouse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK after throttle, got %d", resp2.StatusCode)
	}

	_ = srv.ListItemsCallCount(wsID, "Warehouse")

	// Test malformed JSON dispatch error on Lakehouse roles
	badReq, err := http.Post(
		srv.URL()+"/v1/workspaces/"+wsID+"/items/invalid-endpoint",
		"application/json",
		bytes.NewReader([]byte("{malformed-json")),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = badReq.Body.Close()
	if badReq.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON / route not found, got %d", badReq.StatusCode)
	}
}

func TestServer_AdditionalCoverage(t *testing.T) {
	t.Parallel()

	srv := NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "33333333-3333-3333-3333-333333333333"

	// 1. GetDataAccessRoles on non-existent lakehouse returns nil
	if roles := srv.GetDataAccessRoles(wsID, "missing-lh"); roles != nil {
		t.Errorf("expected nil roles for missing lakehouse, got %v", roles)
	}

	// 2. RemoveDataAccessRole on non-existent role
	srv.RemoveDataAccessRole(wsID, lhID, "missing-role")

	// 3. InjectETagConflicts on non-initialized lakehouse
	srv.InjectETagConflicts("ws-new", "lh-new", 2)
	srv.SetItemsPageSize(5)
	srv.SetDataAccessRolesPageSize(5)
	srv.EnableLakehouseSecurity(wsID, lhID)
	srv.SetLakehouseSecurityEnabled(wsID, lhID, true)
	srv.RemoveItem(wsID, "missing-item")

	// 4. PUT /dataAccessRoles on non-existent lakehouse (404)
	req404, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/v1/workspaces/%s/items/%s/dataAccessRoles", srv.URL(), wsID, lhID),
		bytes.NewReader([]byte(`{"value":[]}`)),
	)
	if err != nil {
		t.Fatalf("request creation error: %v", err)
	}
	req404.Header.Set("If-Match", `"1"`)
	req404.Header.Set("Content-Type", "application/json")
	resp404, err := http.DefaultClient.Do(req404)
	if err != nil {
		t.Fatalf("request execution error: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp404.Body)
	_ = resp404.Body.Close()
	if resp404.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing lakehouse PUT, got %d; body: %s", resp404.StatusCode, string(bodyBytes))
	}

	// 5. Create lakehouse item
	srv.UpsertItem(Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "gold_lakehouse",
		Type:        "Lakehouse",
	})

	// 6. PUT /dataAccessRoles with missing If-Match quotes (400)
	reqNoQuotes, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/v1/workspaces/%s/items/%s/dataAccessRoles", srv.URL(), wsID, lhID),
		bytes.NewReader([]byte(`{"value":[]}`)),
	)
	if err != nil {
		t.Fatalf("request creation error: %v", err)
	}
	reqNoQuotes.Header.Set("If-Match", "unquoted-etag")
	reqNoQuotes.Header.Set("Content-Type", "application/json")
	respNoQuotes, err := http.DefaultClient.Do(reqNoQuotes)
	if err != nil {
		t.Fatalf("request execution error: %v", err)
	}
	_ = respNoQuotes.Body.Close()
	if respNoQuotes.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for unquoted If-Match, got %d", respNoQuotes.StatusCode)
	}

	// 7. PUT /dataAccessRoles with role name > 128 chars (400)
	longName := strings.Repeat("a", 130)
	longRoleBody := fmt.Sprintf(`{"value":[{"name":%q,"decisionRules":[]}]}`, longName)
	reqLong, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/v1/workspaces/%s/items/%s/dataAccessRoles", srv.URL(), wsID, lhID),
		bytes.NewReader([]byte(longRoleBody)),
	)
	if err != nil {
		t.Fatalf("request creation error: %v", err)
	}
	reqLong.Header.Set("If-Match", `"1"`)
	reqLong.Header.Set("Content-Type", "application/json")
	respLong, err := http.DefaultClient.Do(reqLong)
	if err != nil {
		t.Fatalf("request execution error: %v", err)
	}
	_ = respLong.Body.Close()
	if respLong.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for role name > 128, got %d", respLong.StatusCode)
	}

	// 8. Unsupported route / method in spec (400 Bad Request via kin-openapi)
	reqPatch, err := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/v1/workspaces/%s/items/%s/dataAccessRoles", srv.URL(), wsID, lhID),
		bytes.NewReader([]byte(`{}`)),
	)
	if err != nil {
		t.Fatalf("request creation error: %v", err)
	}
	reqPatch.Header.Set("Content-Type", "application/json")
	respPatch, err := http.DefaultClient.Do(reqPatch)
	if err != nil {
		t.Fatalf("request execution error: %v", err)
	}
	_ = respPatch.Body.Close()
	if respPatch.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unmapped route, got %d", respPatch.StatusCode)
	}
}
