// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package pbimock

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestPBIMockServer_EdgeCasesAndMutations(t *testing.T) {
	s, err := NewServer()
	if err != nil {
		t.Fatalf("failed to create pbimock server: %v", err)
	}
	defer s.Close()

	var mutations []string
	s.OnMutation = func(action, itemID, principalID string, _ []string) {
		mutations = append(mutations, fmt.Sprintf("%s:%s:%s", action, itemID, principalID))
	}

	itemID := "00000000-0000-0000-0000-000000000001"
	groupPrincipalID := "22222222-2222-2222-2222-222222222222"

	// 1. Grant to Group via POST /metadata/access/notify
	notifyBody := map[string]any{
		"dashboards": []any{},
		"reports":    []any{},
		"workbooks":  []any{},
		"models":     []any{},
		"datamarts":  []any{},
		"artifacts": []any{
			map[string]any{
				"id":                  1,
				"artifactObjectId":    itemID,
				"permissions":         1,
				"artifactPermissions": 7, // Read, ReadData, ReadAll
				"groupObjectId":       groupPrincipalID,
				"isServicePrincipal":  false,
			},
		},
		"emailNotificationMessage": "Welcome to gold dataset",
	}
	raw, _ := json.Marshal(notifyBody)
	resp, err := http.Post(s.URL+"/metadata/access/notify", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("failed POST /metadata/access/notify: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// 2. Update via PUT /metadata/access
	putBody := map[string]any{
		"dashboards": []any{},
		"reports":    []any{},
		"workbooks":  []any{},
		"models":     []any{},
		"datamarts":  []any{},
		"artifacts": []any{
			map[string]any{
				"id":                  1,
				"artifactObjectId":    itemID,
				"permissions":         1,
				"artifactPermissions": 15, // Reshare added
				"groupObjectId":       groupPrincipalID,
				"isServicePrincipal":  false,
			},
		},
	}
	rawPut, _ := json.Marshal(putBody)
	reqPut, _ := http.NewRequest(http.MethodPut, s.URL+"/metadata/access", bytes.NewReader(rawPut))
	reqPut.Header.Set("Content-Type", "application/json")
	respPut, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("failed PUT /metadata/access: %v", err)
	}
	_ = respPut.Body.Close()
	if respPut.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on PUT, got %d", respPut.StatusCode)
	}

	// 3. Revoke via POST /metadata/access (perms=0)
	revokeBody := map[string]any{
		"dashboards": []any{},
		"reports":    []any{},
		"workbooks":  []any{},
		"models":     []any{},
		"datamarts":  []any{},
		"artifacts": []any{
			map[string]any{
				"id":                  1,
				"artifactObjectId":    itemID,
				"permissions":         0,
				"artifactPermissions": 0,
				"groupObjectId":       groupPrincipalID,
				"isServicePrincipal":  false,
			},
			map[string]any{
				"id":                  1,
				"artifactObjectId":    itemID,
				"permissions":         0,
				"artifactPermissions": 0,
				"userId":              float64(1001),
			},
		},
	}
	rawRevoke, _ := json.Marshal(revokeBody)
	respRevoke, err := http.Post(s.URL+"/metadata/access", "application/json", bytes.NewReader(rawRevoke))
	if err != nil {
		t.Fatalf("failed revoke: %v", err)
	}
	_ = respRevoke.Body.Close()
	if respRevoke.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on revoke, got %d", respRevoke.StatusCode)
	}

	// 4. Verify mutations were recorded
	if len(mutations) < 3 {
		t.Errorf("expected at least 3 mutation events, got %d", len(mutations))
	}

	// 5. Test invalid route (404)
	respInvalid, err := http.Get(s.URL + "/invalid-route-not-in-spec")
	if err != nil {
		t.Fatalf("failed GET invalid route: %v", err)
	}
	_ = respInvalid.Body.Close()
	if respInvalid.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for route not in spec, got %d", respInvalid.StatusCode)
	}

	// 6. Test invalid JSON body on access (400)
	reqBadJSON, _ := http.NewRequest(http.MethodPost, s.URL+"/metadata/access", bytes.NewReader([]byte("{invalid-json")))
	reqBadJSON.Header.Set("Content-Type", "application/json")
	respBadJSON, err := http.DefaultClient.Do(reqBadJSON)
	if err != nil {
		t.Fatalf("failed POST bad json: %v", err)
	}
	_ = respBadJSON.Body.Close()
	if respBadJSON.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 on bad JSON, got %d", respBadJSON.StatusCode)
	}
}
