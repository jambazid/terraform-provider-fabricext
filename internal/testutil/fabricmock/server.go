// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package fabricmock provides an OpenAPI contract-validated Microsoft Fabric mock HTTP server.
package fabricmock

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
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	fabcore "github.com/microsoft/fabric-sdk-go/fabric/core"
)

// Ensure microsoft/fabric-sdk-go core models are linked and verified at compile time.
var (
	_ = fabcore.Item{}
	_ = fabcore.DecisionRule{}
	_ = fabcore.PermissionScope{}
	_ = fabcore.Members{}
	_ = fabcore.FabricItemMember{}
	_ = fabcore.MicrosoftEntraMember{}
)

// Item represents a Microsoft Fabric workspace item stored in the mock server.
type Item struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// Principal represents a Microsoft Entra principal in Fabric permission payloads.
type Principal struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ItemPermissionAssignment represents a single principal's permissions on an item.
type ItemPermissionAssignment struct {
	Principal   Principal `json:"principal"`
	Permissions []string  `json:"permissions"`
}

// PermissionScope defines a Path or Action attribute scope in a OneLake Data Access Role.
type PermissionScope struct {
	AttributeName            string   `json:"attributeName"`
	AttributeValueIncludedIn []string `json:"attributeValueIncludedIn"`
}

// ColumnConstraint defines a column-level security constraint (CLS) applied to a table.
type ColumnConstraint struct {
	TablePath    string   `json:"tablePath"`
	ColumnNames  []string `json:"columnNames"`
	ColumnAction []string `json:"columnAction"`
	ColumnEffect string   `json:"columnEffect"`
}

// RowConstraint defines a row-level security predicate (RLS) applied to a table.
type RowConstraint struct {
	TablePath string `json:"tablePath"`
	Value     string `json:"value"`
}

// Constraints defines row-level and column-level security constraints applied to tables.
type Constraints struct {
	Columns []ColumnConstraint `json:"columns,omitempty"`
	Rows    []RowConstraint    `json:"rows,omitempty"`
}

// DecisionRule defines the effect, permission scopes, and optional constraints of a OneLake Data Access Role.
type DecisionRule struct {
	Effect      string            `json:"effect,omitempty"`
	Permission  []PermissionScope `json:"permission"`
	Constraints *Constraints      `json:"constraints,omitempty"`
}

// FabricItemMember defines workspace item-access inheritance for a Data Access Role.
type FabricItemMember struct {
	ItemAccess []string `json:"itemAccess"`
	SourcePath string   `json:"sourcePath"`
}

// MicrosoftEntraMember defines an explicit Entra object member of a Data Access Role.
type MicrosoftEntraMember struct {
	TenantID   string `json:"tenantId"`
	ObjectID   string `json:"objectId"`
	ObjectType string `json:"objectType,omitempty"`
}

// Members holds both Fabric item members and Microsoft Entra members of a role.
type Members struct {
	FabricItemMembers     []FabricItemMember     `json:"fabricItemMembers,omitempty"`
	MicrosoftEntraMembers []MicrosoftEntraMember `json:"microsoftEntraMembers,omitempty"`
}

// DataAccessRole represents a Lakehouse OneLake Data Access Role.
type DataAccessRole struct {
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name"`
	Kind          string         `json:"kind,omitempty"`
	DecisionRules []DecisionRule `json:"decisionRules"`
	Members       *Members       `json:"members,omitempty"`
}

type lakehouseRolesState struct {
	etag              int
	roles             []DataAccessRole
	conflictsToInject int
}

type throttleRule struct {
	statusCode    int
	retryAfterSec int
	remaining     int
}

var (
	cachedRouterOnce sync.Once
	cachedRouter     routers.Router
	cachedRouterErr  error
)

// Server is a stateful, thread-safe Microsoft Fabric mock HTTP server that
// validates every request and response against the vendored OpenAPI specs.
type Server struct {
	httpServer *httptest.Server
	router     routers.Router

	mu                sync.Mutex
	items             map[string]map[string]Item                     // workspaceID -> itemID -> Item
	permissions       map[string]map[string]ItemPermissionAssignment // workspaceID/itemType/itemID -> principalID -> assignment
	lakehouseRoles    map[string]*lakehouseRolesState                // workspaceID/lakehouseID -> state
	listItemsCalls    map[string]int                                 // workspaceID/itemType -> count
	itemsPageSize     int                                            // optional pagination page size
	grantCalls        int                                            // total grantPermissions calls
	revokeCalls       int                                            // total revokePermissions calls
	lastRevokedPerms  []string                                       // permissions from most recent revokePermissions call
	lastGrantedPerms  []string                                       // permissions from most recent grantPermissions call
	operationSequence []string                                       // ordered log of "grant" / "revoke" operations
	throttle          *throttleRule                                  // optional fault injection for 429 / 5xx
}

// NewServer creates and starts a contract-validated Fabric mock server.
func NewServer(t testing.TB) *Server {
	t.Helper()

	router, err := loadOpenAPIRouter()
	if err != nil {
		t.Fatalf("fabricmock: failed to load vendored OpenAPI specs: %v", err)
	}

	s := &Server{
		router:         router,
		items:          make(map[string]map[string]Item),
		permissions:    make(map[string]map[string]ItemPermissionAssignment),
		lakehouseRoles: make(map[string]*lakehouseRolesState),
		listItemsCalls: make(map[string]int),
	}

	s.httpServer = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	t.Cleanup(func() {
		s.httpServer.Close()
	})
	return s
}

// URL returns the base URL of the mock server.
func (s *Server) URL() string {
	return s.httpServer.URL
}

// UpsertItem adds or updates an item in the mock workspace catalog.
func (s *Server) UpsertItem(item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ws, ok := s.items[item.WorkspaceID]
	if !ok {
		ws = make(map[string]Item)
		s.items[item.WorkspaceID] = ws
	}
	ws[item.ID] = item

	if strings.EqualFold(item.Type, "Lakehouse") {
		key := item.WorkspaceID + "/" + item.ID
		if _, exists := s.lakehouseRoles[key]; !exists {
			s.lakehouseRoles[key] = &lakehouseRolesState{
				etag:  1,
				roles: defaultLakehouseRoles(item.WorkspaceID, item.ID),
			}
		}
	}
}

// RemoveItem deletes an item from the mock workspace catalog (e.g. to simulate 404 drift).
func (s *Server) RemoveItem(workspaceID, itemID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ws, ok := s.items[workspaceID]; ok {
		delete(ws, itemID)
	}
	delete(s.lakehouseRoles, workspaceID+"/"+itemID)
	for k := range s.permissions {
		if strings.HasPrefix(k, workspaceID+"/") && strings.HasSuffix(k, "/"+itemID) {
			delete(s.permissions, k)
		}
	}
}

// SetItemsPageSize configures the page size for GET /v1/workspaces/{workspaceId}/items.
func (s *Server) SetItemsPageSize(size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.itemsPageSize = size
}

// ListItemsCallCount returns the number of GET /items calls for (workspaceID, itemType).
func (s *Server) ListItemsCallCount(workspaceID, itemType string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listItemsCalls[workspaceID+"/"+itemType]
}

// ClearPrincipalPermissions removes a principal's permissions from an item (simulating external revocation drift).
func (s *Server) ClearPrincipalPermissions(workspaceID, itemType, itemID, principalID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s/%s", workspaceID, itemType, itemID)
	if m, ok := s.permissions[key]; ok {
		delete(m, principalID)
	}
}

// GetPrincipalPermissions returns the current permissions slice for a principal on an item.
func (s *Server) GetPrincipalPermissions(workspaceID, itemType, itemID, principalID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s/%s", workspaceID, itemType, itemID)
	if m, ok := s.permissions[key]; ok {
		if a, exists := m[principalID]; exists {
			return slices.Clone(a.Permissions)
		}
	}
	return nil
}

// RemoveDataAccessRole deletes a named role from a Lakehouse (simulating external drift).
func (s *Server) RemoveDataAccessRole(workspaceID, lakehouseID, roleName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workspaceID + "/" + lakehouseID
	state, ok := s.lakehouseRoles[key]
	if !ok {
		return
	}
	filtered := make([]DataAccessRole, 0, len(state.roles))
	for _, r := range state.roles {
		if !strings.EqualFold(r.Name, roleName) {
			filtered = append(filtered, r)
		}
	}
	state.roles = filtered
	state.etag++
}

// GetDataAccessRoles returns a copy of all OneLake Data Access Roles currently on the Lakehouse.
func (s *Server) GetDataAccessRoles(workspaceID, lakehouseID string) []DataAccessRole {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workspaceID + "/" + lakehouseID
	state, ok := s.lakehouseRoles[key]
	if !ok {
		return nil
	}
	return slices.Clone(state.roles)
}

// InjectETagConflicts causes the next `count` PUT /dataAccessRoles calls on the Lakehouse
// to fail with 412 Precondition Failed while inserting a concurrent sibling role change.
func (s *Server) InjectETagConflicts(workspaceID, lakehouseID string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workspaceID + "/" + lakehouseID
	state, ok := s.lakehouseRoles[key]
	if !ok {
		state = &lakehouseRolesState{
			etag:  1,
			roles: defaultLakehouseRoles(workspaceID, lakehouseID),
		}
		s.lakehouseRoles[key] = state
	}
	state.conflictsToInject = count
}

// InjectThrottle configures the mock server to respond with statusCode (e.g. 429 or 503)
// and a Retry-After header for the next `count` requests.
func (s *Server) InjectThrottle(statusCode, retryAfterSec, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.throttle = &throttleRule{
		statusCode:    statusCode,
		retryAfterSec: retryAfterSec,
		remaining:     count,
	}
}

// OperationSequence returns the recorded sequence of "grant" and "revoke" calls.
func (s *Server) OperationSequence() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.operationSequence)
}

// LastRevokedPermissions returns the permissions slice from the most recent revokePermissions call.
func (s *Server) LastRevokedPermissions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.lastRevokedPerms)
}

// LastGrantedPermissions returns the permissions slice from the most recent grantPermissions call.
func (s *Server) LastGrantedPermissions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.lastGrantedPerms)
}

func defaultLakehouseRoles(workspaceID, lakehouseID string) []DataAccessRole {
	return []DataAccessRole{
		{
			ID:   "00000000-0000-0000-0000-000000000001",
			Name: "DefaultReader",
			Kind: "Policy",
			DecisionRules: []DecisionRule{
				{
					Effect: "Permit",
					Permission: []PermissionScope{
						{
							AttributeName:            "Path",
							AttributeValueIncludedIn: []string{"*"},
						},
						{
							AttributeName:            "Action",
							AttributeValueIncludedIn: []string{"Read"},
						},
					},
				},
			},
			Members: &Members{
				FabricItemMembers: []FabricItemMember{
					{
						ItemAccess: []string{"ReadAll"},
						SourcePath: workspaceID + "/" + lakehouseID,
					},
				},
			},
		},
	}
}

type bufferedResponseWriter struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *bufferedResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Read and restore request body for kin-openapi validation + handler consumption.
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("fabricmock: read request body: %v", err), http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	route, pathParams, err := s.router.FindRoute(r)
	if err != nil {
		writeContractError(w, http.StatusBadRequest, fmt.Sprintf("fabricmock: route not found in OpenAPI spec for %s %s: %v", r.Method, r.URL.Path, err))
		return
	}

	reqValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}
	if err := openapi3filter.ValidateRequest(r.Context(), reqValidationInput); err != nil {
		writeContractError(w, http.StatusBadRequest, fmt.Sprintf("fabricmock: OpenAPI request contract violation on %s %s: %v", r.Method, r.URL.Path, err))
		return
	}

	// Restore body for the business handler.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	rec := newBufferedResponseWriter()
	s.dispatch(rec, r, pathParams)

	// Restore body again before response validation.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	respValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqValidationInput,
		Status:                 rec.statusCode,
		Header:                 rec.header,
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}
	respValidationInput.SetBodyBytes(rec.body.Bytes())

	if err := openapi3filter.ValidateResponse(r.Context(), respValidationInput); err != nil {
		writeContractError(w, http.StatusInternalServerError, fmt.Sprintf("fabricmock: OpenAPI response contract violation on %s %s (status %d): %v\nBody: %s", r.Method, r.URL.Path, rec.statusCode, err, rec.body.String()))
		return
	}

	for k, vals := range rec.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rec.statusCode)
	_, _ = w.Write(rec.body.Bytes())
}

func writeContractError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errorCode": "OpenAPIContractViolation",
		"message":   msg,
	})
}

func writeFabricError(w http.ResponseWriter, status int, code, message string, retriable bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errorCode":   code,
		"message":     message,
		"requestId":   "00000000-0000-0000-0000-000000000000",
		"isRetriable": retriable,
	})
}

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
	// Check fault-injection throttle first.
	s.mu.Lock()
	if s.throttle != nil && s.throttle.remaining > 0 {
		rule := *s.throttle
		s.throttle.remaining--
		s.mu.Unlock()
		if rule.retryAfterSec > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(rule.retryAfterSec))
		}
		code := "TooManyRequests"
		if rule.statusCode >= 500 {
			code = "ServiceUnavailable"
		}
		writeFabricError(w, rule.statusCode, code, "Injected transient error", true)
		return
	}
	s.mu.Unlock()

	workspaceID := pathParams["workspaceId"]
	path := r.URL.Path

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/items"):
		s.handleListItems(w, r, workspaceID)
	case r.Method == http.MethodGet && pathParams["itemId"] != "" && !strings.HasSuffix(path, "/dataAccessRoles"):
		s.handleGetItem(w, workspaceID, pathParams["itemId"])
	case strings.Contains(path, "/warehouses/") && strings.HasSuffix(path, "/permissions"):
		s.handleListPermissions(w, workspaceID, "Warehouse", pathParams["warehouseId"])
	case strings.Contains(path, "/warehouses/") && strings.HasSuffix(path, "/grantPermissions"):
		s.handleGrantPermissions(w, r, workspaceID, "Warehouse", pathParams["warehouseId"])
	case strings.Contains(path, "/warehouses/") && strings.HasSuffix(path, "/revokePermissions"):
		s.handleRevokePermissions(w, r, workspaceID, "Warehouse", pathParams["warehouseId"])
	case strings.Contains(path, "/sqlDatabases/") && strings.HasSuffix(path, "/permissions"):
		s.handleListPermissions(w, workspaceID, "SQLDatabase", pathParams["sqlDatabaseId"])
	case strings.Contains(path, "/sqlDatabases/") && strings.HasSuffix(path, "/grantPermissions"):
		s.handleGrantPermissions(w, r, workspaceID, "SQLDatabase", pathParams["sqlDatabaseId"])
	case strings.Contains(path, "/sqlDatabases/") && strings.HasSuffix(path, "/revokePermissions"):
		s.handleRevokePermissions(w, r, workspaceID, "SQLDatabase", pathParams["sqlDatabaseId"])
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/dataAccessRoles"):
		s.handleListDataAccessRoles(w, workspaceID, pathParams["itemId"])
	case r.Method == http.MethodPut && strings.HasSuffix(path, "/dataAccessRoles"):
		s.handlePutDataAccessRoles(w, r, workspaceID, pathParams["itemId"])
	default:
		writeFabricError(w, http.StatusNotFound, "OperationNotSupported", "Unsupported mock operation", false)
	}
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request, workspaceID string) {
	itemType := r.URL.Query().Get("type")
	contToken := r.URL.Query().Get("continuationToken")

	s.mu.Lock()
	s.listItemsCalls[workspaceID+"/"+itemType]++
	ws := s.items[workspaceID]
	var matching []Item
	for _, it := range ws {
		if itemType == "" || strings.EqualFold(it.Type, itemType) {
			matching = append(matching, it)
		}
	}
	pageSize := s.itemsPageSize
	s.mu.Unlock()

	slices.SortFunc(matching, func(a, b Item) int {
		return strings.Compare(a.ID, b.ID)
	})

	start := 0
	if contToken != "" {
		if idx, err := strconv.Atoi(contToken); err == nil && idx >= 0 && idx < len(matching) {
			start = idx
		}
	}

	end := len(matching)
	var nextToken string
	if pageSize > 0 && start+pageSize < len(matching) {
		end = start + pageSize
		nextToken = strconv.Itoa(end)
	}

	page := matching[start:end]
	if page == nil {
		page = []Item{}
	}

	resp := map[string]any{
		"value": page,
	}
	if nextToken != "" {
		resp["continuationToken"] = nextToken
		resp["continuationUri"] = fmt.Sprintf("%s/v1/workspaces/%s/items?type=%s&continuationToken=%s", s.httpServer.URL, workspaceID, itemType, nextToken)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetItem(w http.ResponseWriter, workspaceID, itemID string) {
	s.mu.Lock()
	ws := s.items[workspaceID]
	item, exists := ws[itemID]
	s.mu.Unlock()

	if !exists {
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("Item %s not found in workspace %s", itemID, workspaceID), false)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(item)
}

func (s *Server) itemExistsLocked(workspaceID, itemType, itemID string) bool {
	ws, ok := s.items[workspaceID]
	if !ok {
		return false
	}
	it, exists := ws[itemID]
	if !exists {
		return false
	}
	return strings.EqualFold(it.Type, itemType)
}

func (s *Server) handleListPermissions(w http.ResponseWriter, workspaceID, itemType, itemID string) {
	s.mu.Lock()
	if !s.itemExistsLocked(workspaceID, itemType, itemID) {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("%s %s not found", itemType, itemID), false)
		return
	}

	key := fmt.Sprintf("%s/%s/%s", workspaceID, itemType, itemID)
	permMap := s.permissions[key]
	assignments := make([]ItemPermissionAssignment, 0, len(permMap))
	for _, a := range permMap {
		if len(a.Permissions) > 0 {
			assignments = append(assignments, ItemPermissionAssignment{
				Principal:   a.Principal,
				Permissions: slices.Clone(a.Permissions),
			})
		}
	}
	s.mu.Unlock()

	slices.SortFunc(assignments, func(a, b ItemPermissionAssignment) int {
		return strings.Compare(a.Principal.ID, b.Principal.ID)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"value": assignments,
	})
}

func (s *Server) handleGrantPermissions(w http.ResponseWriter, r *http.Request, workspaceID, itemType, itemID string) {
	var payload ItemPermissionAssignment
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeFabricError(w, http.StatusBadRequest, "InvalidRequest", err.Error(), false)
		return
	}

	s.mu.Lock()
	if !s.itemExistsLocked(workspaceID, itemType, itemID) {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("%s %s not found", itemType, itemID), false)
		return
	}

	key := fmt.Sprintf("%s/%s/%s", workspaceID, itemType, itemID)
	permMap, ok := s.permissions[key]
	if !ok {
		permMap = make(map[string]ItemPermissionAssignment)
		s.permissions[key] = permMap
	}

	existing := permMap[payload.Principal.ID]
	merged := slices.Clone(existing.Permissions)
	for _, p := range payload.Permissions {
		if !slices.Contains(merged, p) {
			merged = append(merged, p)
		}
	}
	permMap[payload.Principal.ID] = ItemPermissionAssignment{
		Principal:   payload.Principal,
		Permissions: merged,
	}
	s.grantCalls++
	s.lastGrantedPerms = slices.Clone(payload.Permissions)
	s.operationSequence = append(s.operationSequence, "grant")
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleRevokePermissions(w http.ResponseWriter, r *http.Request, workspaceID, itemType, itemID string) {
	var payload ItemPermissionAssignment
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeFabricError(w, http.StatusBadRequest, "InvalidRequest", err.Error(), false)
		return
	}

	s.mu.Lock()
	if !s.itemExistsLocked(workspaceID, itemType, itemID) {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("%s %s not found", itemType, itemID), false)
		return
	}

	key := fmt.Sprintf("%s/%s/%s", workspaceID, itemType, itemID)
	permMap := s.permissions[key]
	existing, exists := permMap[payload.Principal.ID]
	if !exists {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "PrincipalPermissionNotFound", "Principal permission assignment not found", false)
		return
	}

	var remaining []string
	for _, p := range existing.Permissions {
		if !slices.Contains(payload.Permissions, p) {
			remaining = append(remaining, p)
		}
	}
	if len(remaining) == 0 {
		delete(permMap, payload.Principal.ID)
	} else {
		existing.Permissions = remaining
		permMap[payload.Principal.ID] = existing
	}

	s.revokeCalls++
	s.lastRevokedPerms = slices.Clone(payload.Permissions)
	s.operationSequence = append(s.operationSequence, "revoke")
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleListDataAccessRoles(w http.ResponseWriter, workspaceID, itemID string) {
	s.mu.Lock()
	if !s.itemExistsLocked(workspaceID, "Lakehouse", itemID) {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("Lakehouse %s not found", itemID), false)
		return
	}

	key := workspaceID + "/" + itemID
	state, ok := s.lakehouseRoles[key]
	if !ok {
		state = &lakehouseRolesState{
			etag:  1,
			roles: defaultLakehouseRoles(workspaceID, itemID),
		}
		s.lakehouseRoles[key] = state
	}
	roles := slices.Clone(state.roles)
	etag := fmt.Sprintf(`"etag-v%d"`, state.etag)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Etag", etag)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"value": roles,
	})
}

func (s *Server) handlePutDataAccessRoles(w http.ResponseWriter, r *http.Request, workspaceID, itemID string) {
	var payload struct {
		Value []DataAccessRole `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeFabricError(w, http.StatusBadRequest, "InvalidRequest", err.Error(), false)
		return
	}

	ifMatch := r.Header.Get("If-Match")

	s.mu.Lock()
	if !s.itemExistsLocked(workspaceID, "Lakehouse", itemID) {
		s.mu.Unlock()
		writeFabricError(w, http.StatusNotFound, "ItemNotFound", fmt.Sprintf("Lakehouse %s not found", itemID), false)
		return
	}

	key := workspaceID + "/" + itemID
	state, ok := s.lakehouseRoles[key]
	if !ok {
		state = &lakehouseRolesState{
			etag:  1,
			roles: defaultLakehouseRoles(workspaceID, itemID),
		}
		s.lakehouseRoles[key] = state
	}

	// Simulate an external concurrent modification if conflict injection is active.
	if state.conflictsToInject > 0 {
		state.conflictsToInject--
		state.etag++
		hasInjectedSibling := false
		for _, existingRole := range state.roles {
			if existingRole.Name == "ConcurrentSiblingRole" {
				hasInjectedSibling = true
				break
			}
		}
		if !hasInjectedSibling {
			state.roles = append(state.roles, DataAccessRole{
				ID:   "00000000-0000-0000-0000-000000000099",
				Name: "ConcurrentSiblingRole",
				Kind: "Policy",
				DecisionRules: []DecisionRule{
					{
						Effect: "Permit",
						Permission: []PermissionScope{
							{AttributeName: "Path", AttributeValueIncludedIn: []string{"Tables/concurrent_audit"}},
							{AttributeName: "Action", AttributeValueIncludedIn: []string{"Read"}},
						},
					},
				},
			})
		}
	}

	currentETag := fmt.Sprintf(`"etag-v%d"`, state.etag)
	if ifMatch != "" && ifMatch != currentETag {
		s.mu.Unlock()
		writeFabricError(w, http.StatusPreconditionFailed, "PreconditionFailed", fmt.Sprintf("ETag mismatch: got %s, expected %s", ifMatch, currentETag), true)
		return
	}

	for i := range payload.Value {
		if payload.Value[i].ID == "" {
			payload.Value[i].ID = fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
		}
		if payload.Value[i].Kind == "" {
			payload.Value[i].Kind = "Policy"
		}
	}

	state.roles = payload.Value
	state.etag++
	newETag := fmt.Sprintf(`"etag-v%d"`, state.etag)
	s.mu.Unlock()

	w.Header().Set("Etag", newETag)
	w.WriteHeader(http.StatusOK)
}

func loadOpenAPIRouter() (routers.Router, error) {
	cachedRouterOnce.Do(func() {
		cachedRouter, cachedRouterErr = buildOpenAPIRouter()
	})
	return cachedRouter, cachedRouterErr
}

func buildOpenAPIRouter() (routers.Router, error) {
	openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`))

	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("unable to resolve runtime caller path")
	}
	openAPIDir := filepath.Join(filepath.Dir(callerFile), "..", "..", "..", "specs", "openapi")

	defFiles := []string{
		"common/definitions.json",
		"platform/definitions/platform.json",
		"warehouse/definitions.json",
		"lakehouse/definitions.json",
		"sqlDatabase/definitions.json",
		"overlays/item-permissions.json",
	}

	specFiles := []string{
		"platform/swagger.json",
		"warehouse/swagger.json",
		"lakehouse/swagger.json",
		"sqlDatabase/swagger.json",
		"overlays/item-permissions.json",
	}

	combinedDefs := make(map[string]any)
	for _, rel := range defFiles {
		raw, err := readJSONMap(filepath.Join(openAPIDir, rel))
		if err != nil {
			return nil, fmt.Errorf("read definitions %s: %w", rel, err)
		}
		if defs, ok := raw["definitions"].(map[string]any); ok {
			for k, v := range defs {
				combinedDefs[k] = normalizeSwaggerNode(v)
			}
		}
	}

	// Iteratively prune any unused definitions that reference external unvendored workload specs
	// (such as variableLibrary/definitions.json or deploymentPipeline definitions).
	for {
		removed := false
		for k, v := range combinedDefs {
			if !allRefsExist(v, combinedDefs) {
				delete(combinedDefs, k)
				removed = true
			}
		}
		if !removed {
			break
		}
	}

	combinedPaths := make(map[string]any)
	for _, rel := range specFiles {
		raw, err := readJSONMap(filepath.Join(openAPIDir, rel))
		if err != nil {
			return nil, fmt.Errorf("read swagger %s: %w", rel, err)
		}
		if paths, ok := raw["paths"].(map[string]any); ok {
			for k, v := range paths {
				if strings.Contains(k, "?") {
					continue
				}
				normalized := normalizeSwaggerNode(v)
				if !allRefsExist(normalized, combinedDefs) {
					continue
				}
				fullPath := "/v1" + k
				combinedPaths[fullPath] = normalized
			}
		}
	}

	combinedSwagger2 := map[string]any{
		"swagger": "2.0",
		"info": map[string]any{
			"title":   "Microsoft Fabric Combined REST API Contract",
			"version": "v1",
		},
		"paths":       combinedPaths,
		"definitions": combinedDefs,
	}

	rawBytes, err := json.Marshal(combinedSwagger2)
	if err != nil {
		return nil, fmt.Errorf("marshal combined swagger2: %w", err)
	}

	var doc2 openapi2.T
	if err := json.Unmarshal(rawBytes, &doc2); err != nil {
		return nil, fmt.Errorf("unmarshal openapi2.T: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return nil, fmt.Errorf("convert swagger2 to openapi3: %w", err)
	}
	doc3.Servers = nil

	if err := doc3.Validate(context.Background(), openapi3.DisableExamplesValidation()); err != nil {
		return nil, fmt.Errorf("validate converted openapi3 doc: %w", err)
	}

	return gorillamux.NewRouter(doc3)
}

func allRefsExist(node any, defs map[string]any) bool {
	switch val := node.(type) {
	case map[string]any:
		for k, v := range val {
			if k == "$ref" {
				if refStr, ok := v.(string); ok {
					defName := strings.TrimPrefix(refStr, "#/definitions/")
					if _, exists := defs[defName]; !exists {
						return false
					}
				}
				continue
			}
			if !allRefsExist(v, defs) {
				return false
			}
		}
		return true
	case []any:
		for _, v := range val {
			if !allRefsExist(v, defs) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func readJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeSwaggerNode(node any) any {
	switch val := node.(type) {
	case map[string]any:
		if refRaw, hasRef := val["$ref"]; hasRef {
			if refStr, ok := refRaw.(string); ok {
				if idx := strings.Index(refStr, "#/definitions/"); idx != -1 {
					return map[string]any{"$ref": refStr[idx:]}
				}
				return map[string]any{"$ref": refStr}
			}
		}
		out := make(map[string]any, len(val))
		for k, v := range val {
			if k == "discriminator" || k == "readOnly" || k == "example" || k == "operationId" || strings.HasPrefix(k, "x-") {
				continue
			}
			if k == "pattern" {
				if patStr, ok := v.(string); ok {
					if _, err := regexp.Compile(patStr); err != nil {
						continue
					}
				}
			}
			if k == "Enum" {
				k = "enum"
			}
			out[k] = normalizeSwaggerNode(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = normalizeSwaggerNode(v)
		}
		return out
	default:
		return val
	}
}
