// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package fabricmock

import (
	"bytes"
	"net/http"
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

	_ = srv.OperationSequence()
	_ = srv.LastRevokedPermissions()
	_ = srv.LastGrantedPermissions()

	// Test malformed JSON dispatch error
	badReq, err := http.Post(
		srv.URL()+"/v1/workspaces/"+wsID+"/warehouses/"+whID+"/grantPermissions",
		"application/json",
		bytes.NewReader([]byte("{malformed-json")),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = badReq.Body.Close()
	if badReq.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON, got %d", badReq.StatusCode)
	}
}
