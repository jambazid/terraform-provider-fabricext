// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package fabricmock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestServer_OpenAPIContractValidation(t *testing.T) {
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

	t.Run("valid request and response pass kin-openapi filter", func(t *testing.T) {
		t.Parallel()
		resp, err := http.Get(srv.URL() + "/v1/workspaces/" + wsID + "/items?type=Warehouse")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid UUID path parameter is rejected by kin-openapi request validator", func(t *testing.T) {
		t.Parallel()
		resp, err := http.Get(srv.URL() + "/v1/workspaces/not-a-uuid/items")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request from OpenAPI contract validator, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid permission enum in grantPermissions body is rejected by kin-openapi", func(t *testing.T) {
		t.Parallel()
		body, _ := json.Marshal(map[string]any{
			"principal": map[string]any{
				"id":   "33333333-3333-3333-3333-333333333333",
				"type": "Group",
			},
			"permissions": []string{"InvalidPermissionName"},
		})
		resp, err := http.Post(
			srv.URL()+"/v1/workspaces/"+wsID+"/warehouses/"+whID+"/grantPermissions",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid permission enum, got %d", resp.StatusCode)
		}
	})
}
