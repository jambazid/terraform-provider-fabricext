// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package pbimock provides an OpenAPI contract-validated Power BI metadata mock HTTP server.
package pbimock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// Server represents a stateful in-memory mock Power BI metadata server.
type Server struct {
	URL                string
	Server             *httptest.Server
	mu                 sync.RWMutex
	router             routers.Router
	doc                *openapi3.T
	entries            map[string][]map[string]any // itemId -> entries
	deletedItems       map[string]bool             // itemId -> deleted (returns 404)
	portalForbidden    bool
	portalRequestCount int
	accessAuthStatus   int
	accessAuthError    string
	artifactID         int
	lastArtifacts      []map[string]any
	nextUserID         int
	nextGroupID        int
	OnMutation         func(action string, itemId string, principalId string, permissions []string)
}

// NewServer creates a new stateful mock Power BI metadata server.
func NewServer() (*Server, error) {
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("unable to resolve caller path")
	}
	specPath := filepath.Join(filepath.Dir(callerFile), "..", "..", "..", "specs", "powerbi", "metadata", "openapi.json")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading openapi spec %s: %w", specPath, err)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing openapi spec: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validating openapi spec: %w", err)
	}
	doc.Servers = nil

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	s := &Server{
		router:       router,
		doc:          doc,
		entries:      make(map[string][]map[string]any),
		deletedItems: make(map[string]bool),
		nextUserID:   1001,
		nextGroupID:  2001,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/v1.0/myorg/groups", s.handleGroups)
	mux.HandleFunc("/metadata/access", s.handleAccess)
	mux.HandleFunc("/metadata/access/notify", s.handleAccessNotify)
	mux.HandleFunc("/metadata/access/artifacts/", s.handleArtifactAccess)
	mux.HandleFunc("/metadata/artifacts/definitions", s.handleDefinitions)

	s.Server = httptest.NewServer(s.wrapValidation(mux))
	s.URL = s.Server.URL
	return s, nil
}

// Close shuts down the server.
func (s *Server) Close() {
	if s.Server != nil {
		s.Server.Close()
	}
}

// SeedDirectAccess adds direct access entries for an item.
func (s *Server) SeedDirectAccess(itemID string, entries ...map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[itemID] = append(s.entries[itemID], entries...)
}

// DeleteArtifact marks an artifact as deleted, causing GET /metadata/access/artifacts/{itemId} to return 404.
func (s *Server) DeleteArtifact(itemID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deletedItems == nil {
		s.deletedItems = make(map[string]bool)
	}
	s.deletedItems[itemID] = true
	delete(s.entries, itemID)
}

// SetPortalForbidden sets whether GET / returns HTTP 403.
func (s *Server) SetPortalForbidden(forbidden bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.portalForbidden = forbidden
}

// PortalRequestCount returns the number of requests received at GET /.
func (s *Server) PortalRequestCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.portalRequestCount
}

// SetAccessAuthError configures the mock server to return an error status (e.g. 401 or 403) with errorInfo.
func (s *Server) SetAccessAuthError(status int, errorInfo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessAuthStatus = status
	s.accessAuthError = errorInfo
}

// SetArtifactID configures the root integer "id" returned by GET /metadata/access/artifacts/{itemId}.
func (s *Server) SetArtifactID(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifactID = id
}

// LastArtifacts returns the artifacts list from the last POST or PUT to /metadata/access.
func (s *Server) LastArtifacts() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastArtifacts == nil {
		return nil
	}
	out := make([]map[string]any, len(s.lastArtifacts))
	copy(out, s.lastArtifacts)
	return out
}

func (s *Server) wrapValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		route, pathParams, err := s.router.FindRoute(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("route not found in spec: %v", err), http.StatusNotFound)
			return
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		reqValidationInput := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
		}
		if err := openapi3filter.ValidateRequest(r.Context(), reqValidationInput); err != nil {
			http.Error(w, fmt.Sprintf("request contract violation: %v", err), http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		respValidationInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: reqValidationInput,
			Status:                 rec.Code,
			Header:                 rec.Header(),
			Options: &openapi3filter.Options{
				IncludeResponseStatus: true,
			},
		}
		respValidationInput.SetBodyBytes(rec.Body.Bytes())

		if err := openapi3filter.ValidateResponse(r.Context(), respValidationInput); err != nil {
			http.Error(w, fmt.Sprintf("response contract violation: %v\nBody: %s", err, rec.Body.String()), http.StatusInternalServerError)
			return
		}

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.portalRequestCount++
	forbidden := s.portalForbidden
	s.mu.Unlock()

	if forbidden {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test-cluster=%s/", s.URL))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDefinitions(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	defs := map[string]any{
		"Datawarehouse": map[string]any{
			"type": "Datawarehouse",
			"permissions": map[string]int{
				"Read": 1, "ReadData": 2, "ReadAll": 4, "Reshare": 8, "Monitor": 16, "Audit": 64, "Restore": 128,
			},
		},
		"Lakehouse": map[string]any{
			"type": "Lakehouse",
			"permissions": map[string]int{
				"Read": 1, "ReadData": 2, "ReadAll": 4, "Execute": 64, "Reshare": 8,
			},
		},
		"SQLDbNative": map[string]any{
			"type": "SQLDbNative",
			"permissions": map[string]int{
				"Read": 1, "ReadData": 2, "ReadAll": 4, "Execute": 64, "Reshare": 8,
			},
		},
	}
	_ = json.NewEncoder(w).Encode(defs)
}

func (s *Server) handleAccess(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Artifacts []map[string]any `json:"artifacts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	type mutationEvent struct {
		action      string
		itemID      string
		principalID string
		perms       []string
	}
	var events []mutationEvent

	s.mu.Lock()
	s.lastArtifacts = payload.Artifacts
	for _, a := range payload.Artifacts {
		itemObjID, _ := a["artifactObjectId"].(string)
		if itemObjID == "" {
			continue
		}
		p, _ := a["permissions"].(float64)
		ap, _ := a["artifactPermissions"].(float64)

		var principalID string
		if uOID, ok := a["userObjectId"].(string); ok && uOID != "" {
			principalID = uOID
		} else if gOID, ok := a["groupObjectId"].(string); ok && gOID != "" {
			principalID = gOID
		} else if sOID, ok := a["servicePrincipalObjectId"].(string); ok && sOID != "" {
			principalID = sOID
		}

		if int(p) == 0 && int(ap) == 0 {
			// Revocation
			var kept []map[string]any
			for _, existing := range s.entries[itemObjID] {
				if matchPrincipal(existing, a) {
					continue
				}
				kept = append(kept, existing)
			}
			s.entries[itemObjID] = kept
			if s.OnMutation != nil {
				events = append(events, mutationEvent{
					action:      "revoke",
					itemID:      itemObjID,
					principalID: principalID,
				})
			}
		} else {
			// Grant or update
			found := false
			for i, existing := range s.entries[itemObjID] {
				if matchPrincipal(existing, a) {
					if r.Method == http.MethodPut {
						existing["permissions"] = int(p)
						existing["artifactPermissions"] = int(ap)
					} else {
						curP, _ := existing["permissions"].(int)
						curAP, _ := existing["artifactPermissions"].(int)
						existing["permissions"] = curP | int(p)
						existing["artifactPermissions"] = curAP | int(ap)
					}
					s.entries[itemObjID][i] = existing
					found = true
					break
				}
			}
			if !found {
				newEntry := map[string]any{
					"permissions":         int(p),
					"artifactPermissions": int(ap),
					"accessSource":        nil,
					"objectId":            principalID,
				}
				if uOID, ok := a["userObjectId"].(string); ok && uOID != "" {
					newEntry["userObjectId"] = uOID
					newEntry["userId"] = s.nextUserID
					s.nextUserID++
				}
				if gOID, ok := a["groupObjectId"].(string); ok && gOID != "" {
					newEntry["groupObjectId"] = gOID
					newEntry["groupId"] = s.nextGroupID
					s.nextGroupID++
				}
				if isSP, ok := a["isServicePrincipal"].(bool); ok {
					newEntry["isServicePrincipal"] = isSP
				}
				s.entries[itemObjID] = append(s.entries[itemObjID], newEntry)
			}
			if s.OnMutation != nil {
				var perms []string
				if int(p)&1 != 0 {
					perms = append(perms, "Read")
				}
				if int(p)&2 != 0 {
					perms = append(perms, "Write")
				}
				if int(ap)&2 != 0 {
					perms = append(perms, "ReadData")
				}
				if int(ap)&4 != 0 {
					perms = append(perms, "ReadAll")
				}
				if int(ap)&8 != 0 {
					perms = append(perms, "Reshare")
				}
				if int(p)&64 != 0 || int(ap)&64 != 0 {
					perms = append(perms, "Execute")
				}
				events = append(events, mutationEvent{
					action:      "grant",
					itemID:      itemObjID,
					principalID: principalID,
					perms:       perms,
				})
			}
		}
	}
	s.mu.Unlock()

	for _, ev := range events {
		if s.OnMutation != nil {
			s.OnMutation(ev.action, ev.itemID, ev.principalID, ev.perms)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAccessNotify(w http.ResponseWriter, r *http.Request) {
	s.handleAccess(w, r)
}

func (s *Server) handleGroups(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	authStatus := s.accessAuthStatus
	authErr := s.accessAuthError
	s.mu.RUnlock()

	if authStatus != 0 {
		w.Header().Set("home-cluster-uri", s.URL)
		if authErr != "" {
			w.Header().Set("X-PowerBI-Error-Info", authErr)
		}
		w.Header().Set("RequestId", "test-groups-request-id")
		w.WriteHeader(authStatus)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("home-cluster-uri", s.URL)
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{
		"@odata.context": "https://wabi-test-cluster.analysis.windows.net/v1.0/myorg/$metadata#groups",
		"@odata.count":   1,
		"value": []map[string]any{
			{
				"id":                    "f83c9197-4c61-4355-b319-7d0c7f948313",
				"isReadOnly":            false,
				"isOnDedicatedCapacity": true,
				"name":                  "Test Workspace",
			},
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleArtifactAccess(w http.ResponseWriter, r *http.Request) {
	itemID := filepath.Base(r.URL.Path)
	s.mu.RLock()
	authStatus := s.accessAuthStatus
	authErr := s.accessAuthError
	deleted := s.deletedItems != nil && s.deletedItems[itemID]
	entries := s.entries[itemID]
	rootID := s.artifactID
	if rootID == 0 {
		rootID = 1
	}
	s.mu.RUnlock()

	if authStatus != 0 {
		w.Header().Set("Content-Type", "application/json")
		if authErr != "" {
			w.Header().Set("X-PowerBI-Error-Info", authErr)
		}
		w.Header().Set("RequestId", "test-request-id-1234")
		w.WriteHeader(authStatus)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "PowerBINotAuthorizedException",
				"message": "User is not authorized to access Power BI metadata",
			},
		})
		return
	}

	if deleted {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "ItemNotFound",
				"message": fmt.Sprintf("Item %s not found", itemID),
			},
		})
		return
	}

	if entries == nil {
		entries = []map[string]any{}
	}

	resp := map[string]any{
		"id":                  rootID,
		"objectId":            itemID,
		"displayName":         "MockItem",
		"permissions":         1,
		"artifactPermissions": 7,
		"detail":              entries,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func matchPrincipal(a, b map[string]any) bool {
	if aObj, ok := a["objectId"].(string); ok && aObj != "" {
		if bObj, ok := b["objectId"].(string); ok && bObj == aObj {
			return true
		}
		if bU, ok := b["userObjectId"].(string); ok && bU == aObj {
			return true
		}
		if bG, ok := b["groupObjectId"].(string); ok && bG == aObj {
			return true
		}
	}
	if aU, ok := a["userObjectId"].(string); ok && aU != "" {
		if bU, ok := b["userObjectId"].(string); ok && bU == aU {
			return true
		}
	}
	if aG, ok := a["groupObjectId"].(string); ok && aG != "" {
		if bG, ok := b["groupObjectId"].(string); ok && bG == aG {
			return true
		}
	}
	if aUID, ok := a["userId"].(int); ok && aUID != 0 {
		if bUID, ok := b["userId"].(float64); ok && int(bUID) == aUID {
			return true
		}
		if bUID, ok := b["userId"].(int); ok && bUID == aUID {
			return true
		}
	}
	if aGID, ok := a["groupId"].(int); ok && aGID != 0 {
		if bGID, ok := b["groupId"].(float64); ok && int(bGID) == aGID {
			return true
		}
		if bGID, ok := b["groupId"].(int); ok && bGID == aGID {
			return true
		}
	}
	return false
}
