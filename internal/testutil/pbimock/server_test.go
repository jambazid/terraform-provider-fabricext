// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package pbimock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPBIMockServer_ContractValidation(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatalf("failed to create pbimock server: %v", err)
	}
	defer s.Close()

	// 1. Test GET / (discovery cookie)
	resp, err := http.Get(s.URL + "/")
	if err != nil {
		t.Fatalf("failed GET /: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Errorf("expected Set-Cookie header")
	}

	// 2. Test GET /metadata/artifacts/definitions (contract-validated)
	resp, err = http.Get(s.URL + "/metadata/artifacts/definitions")
	if err != nil {
		t.Fatalf("failed GET /metadata/artifacts/definitions: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// 3. Test POST /metadata/access (contract-validated)
	grantBody := map[string]any{
		"dashboards": []any{},
		"reports":    []any{},
		"workbooks":  []any{},
		"models":     []any{},
		"datamarts":  []any{},
		"artifacts": []any{
			map[string]any{
				"id":                  1,
				"artifactObjectId":    "00000000-0000-0000-0000-000000000001",
				"permissions":         1,
				"artifactPermissions": 3,
				"userObjectId":        "11111111-1111-1111-1111-111111111111",
				"isServicePrincipal":  false,
			},
		},
	}
	raw, _ := json.Marshal(grantBody)
	resp, err = http.Post(s.URL+"/metadata/access", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("failed POST /metadata/access: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// 4. Test GET /metadata/access/artifacts/{itemId} (contract-validated)
	resp, err = http.Get(s.URL + "/metadata/access/artifacts/00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("failed GET /metadata/access/artifacts/itemId: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
