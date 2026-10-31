// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	fabadmin "github.com/microsoft/fabric-sdk-go/fabric/admin"

	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

func newTestClient(t *testing.T, srv *fabricmock.Server) *FabricClient {
	t.Helper()
	c, err := NewFabricClient(Config{
		Endpoint: srv.URL(),
		TokenProvider: func(_ context.Context) (string, error) {
			return "mock-bearer-token", nil
		},
		BaseBackoff: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create FabricClient: %v", err)
	}
	return c
}

func TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	srv.SetItemsPageSize(1)

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222221"
	sqlEpID := "22222222-2222-2222-2222-222222222222"
	wh1ID := "33333333-3333-3333-3333-333333333331"
	wh2ID := "33333333-3333-3333-3333-333333333332"

	// Register a Lakehouse and a companion SQLEndpoint sharing the exact same displayName.
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "shared_analytics_name",
		Type:        "Lakehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          sqlEpID,
		WorkspaceID: wsID,
		DisplayName: "shared_analytics_name",
		Type:        "SQLEndpoint",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          wh1ID,
		WorkspaceID: wsID,
		DisplayName: "wh_one",
		Type:        "Warehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          wh2ID,
		WorkspaceID: wsID,
		DisplayName: "wh_two",
		Type:        "Warehouse",
	})

	c := newTestClient(t, srv)
	ctx := context.Background()

	// 1. Type isolation: querying Lakehouse must never return the companion SQLEndpoint ID.
	gotLH, err := c.GetItemIDByName(ctx, wsID, "shared_analytics_name", "Lakehouse")
	if err != nil {
		t.Fatalf("unexpected error resolving Lakehouse: %v", err)
	}
	if gotLH != lhID {
		t.Fatalf("expected Lakehouse ID %s, got %s", lhID, gotLH)
	}

	// 2. Paginated lookup across 2 pages for Warehouse.
	gotWH2, err := c.GetItemIDByName(ctx, wsID, "wh_two", "Warehouse")
	if err != nil {
		t.Fatalf("unexpected error resolving wh_two across pages: %v", err)
	}
	if gotWH2 != wh2ID {
		t.Fatalf("expected wh_two ID %s, got %s", wh2ID, gotWH2)
	}
	callsAfterFirstLookup := srv.ListItemsCallCount(wsID, "Warehouse")

	// Cached lookup for wh_one should not trigger any new HTTP calls.
	gotWH1, err := c.GetItemIDByName(ctx, wsID, "wh_one", "Warehouse")
	if err != nil || gotWH1 != wh1ID {
		t.Fatalf("expected cached wh_one ID %s, got %s (err=%v)", wh1ID, gotWH1, err)
	}
	if srv.ListItemsCallCount(wsID, "Warehouse") != callsAfterFirstLookup {
		t.Fatalf("expected cache hit with 0 additional HTTP calls")
	}

	// 3. Add a new Warehouse after initial cache population; cache miss must evict once and re-fetch.
	wh3ID := "33333333-3333-3333-3333-333333333333"
	srv.UpsertItem(fabricmock.Item{
		ID:          wh3ID,
		WorkspaceID: wsID,
		DisplayName: "wh_newly_created",
		Type:        "Warehouse",
	})
	gotWH3, err := c.GetItemIDByName(ctx, wsID, "wh_newly_created", "Warehouse")
	if err != nil || gotWH3 != wh3ID {
		t.Fatalf("expected cache-miss eviction to find wh_newly_created (%s), got %s (err=%v)", wh3ID, gotWH3, err)
	}

	// 4. Non-existent item returns *NotFoundError.
	_, err = c.GetItemIDByName(ctx, wsID, "does_not_exist", "Warehouse")
	if !IsNotFound(err) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
}

func TestLakehouseDataAccessRoles_ConcurrentRMWAndETagConflictRetry(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "66666666-6666-6666-6666-666666666666"
	tenantID := "77777777-7777-7777-7777-777777777777"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "bronze_lh",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	// Inject 2 ETag 412 Precondition Failed conflicts to verify retry + sibling preservation.
	srv.InjectETagConflicts(wsID, lhID, 2)

	c := newTestClient(t, srv)
	ctx := context.Background()

	const numRoles = 5
	var wg sync.WaitGroup
	errCh := make(chan error, numRoles)

	for i := range numRoles {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			roleName := fmt.Sprintf("CustomRole%d", idx)
			principalID := fmt.Sprintf("88888888-8888-8888-8888-%012d", idx+1)
			role := BuildLakehouseRole(
				wsID,
				lhID,
				roleName,
				[]string{fmt.Sprintf("Tables/table_%d", idx)},
				[]string{"Read"},
				tenantID,
				principalID,
				"Group",
			)
			if err := c.UpsertDataAccessRole(ctx, wsID, lhID, role); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent UpsertDataAccessRole failed: %v", err)
	}

	// Verify DefaultReader, ConcurrentSiblingRole, and all 5 CustomRoles exist intact.
	allRoles := srv.GetDataAccessRoles(wsID, lhID)
	names := make([]string, 0, len(allRoles))
	for _, r := range allRoles {
		names = append(names, r.Name)
	}
	if !slices.Contains(names, "DefaultReader") {
		t.Fatalf("DefaultReader role was lost during RMW: %v", names)
	}
	if !slices.Contains(names, "ConcurrentSiblingRole") {
		t.Fatalf("ConcurrentSiblingRole injected during 412 conflict was lost: %v", names)
	}
	for i := range numRoles {
		expected := fmt.Sprintf("CustomRole%d", i)
		if !slices.Contains(names, expected) {
			t.Fatalf("expected role %q in final Lakehouse roles, got %v", expected, names)
		}
	}

	// Delete one role with an injected 412 conflict and verify siblings remain.
	srv.InjectETagConflicts(wsID, lhID, 1)
	if err := c.DeleteDataAccessRole(ctx, wsID, lhID, "CustomRole0"); err != nil {
		t.Fatalf("DeleteDataAccessRole failed on 412 retry: %v", err)
	}
	if _, err := c.GetDataAccessRole(ctx, wsID, lhID, "CustomRole0"); !IsNotFound(err) {
		t.Fatalf("expected CustomRole0 to be NotFound after delete, got %v", err)
	}
	if _, err := c.GetDataAccessRole(ctx, wsID, lhID, "CustomRole1"); err != nil {
		t.Fatalf("expected CustomRole1 to remain intact after deleting CustomRole0: %v", err)
	}

	// Verify defaultMaxETagRetries exhaustion returns an error when conflicts exceed defaultMaxETagRetries.
	srv.InjectETagConflicts(wsID, lhID, defaultMaxETagRetries+2)
	exhaustRole := BuildLakehouseRole(wsID, lhID, "ExhaustRole", []string{"Tables/t"}, []string{"Read"}, tenantID, "88888888-8888-8888-8888-000000000099", "Group")
	if err := c.UpsertDataAccessRole(ctx, wsID, lhID, exhaustRole); err == nil {
		t.Fatalf("expected error when ETag conflicts exceed defaultMaxETagRetries (%d)", defaultMaxETagRetries)
	}
}

func TestRetryAfter429And503Backoff(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "99999999-9999-9999-9999-999999999999"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "throttled_wh",
		Type:        "Warehouse",
	})

	c := newTestClient(t, srv)
	ctx := context.Background()

	// 1. Inject two HTTP 429 responses with Retry-After: 1 before succeeding.
	srv.InjectThrottle(http.StatusTooManyRequests, 1, 2)

	item, err := c.GetItemByID(ctx, wsID, whID, "Warehouse")
	if err != nil {
		t.Fatalf("expected GetItemByID to succeed after 429 retries, got: %v", err)
	}
	if item.ID != whID {
		t.Fatalf("expected item ID %s, got %s", whID, item.ID)
	}

	// 2. Inject two HTTP 503 Service Unavailable responses before succeeding.
	srv.InjectThrottle(http.StatusServiceUnavailable, 0, 2)

	item503, err := c.GetItemByID(ctx, wsID, whID, "Warehouse")
	if err != nil {
		t.Fatalf("expected GetItemByID to succeed after 503 retries, got: %v", err)
	}
	if item503.ID != whID {
		t.Fatalf("expected item ID %s after 503 retry, got %s", whID, item503.ID)
	}

	// 3. Verify ctx.Done() aborts retry sleep immediately when context is cancelled.
	cancelCtx, cancel := context.WithCancel(context.Background())
	srv.InjectThrottle(http.StatusTooManyRequests, 5, 2)
	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()
	if _, err := c.GetItemByID(cancelCtx, wsID, whID, "Warehouse"); err == nil {
		t.Fatalf("expected error when context is cancelled during Retry-After sleep")
	}
}

func TestCollapseRolePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perms    []string
		expected string
		wantErr  bool
	}{
		{
			name:     "single read",
			perms:    []string{"Read"},
			expected: "read",
		},
		{
			name:     "read and write maps to write",
			perms:    []string{"Read", "Write"},
			expected: "write",
		},
		{
			name:     "write takes precedence over reshare and read",
			perms:    []string{"Read", "Write", "Reshare"},
			expected: "write",
		},
		{
			name:     "read_spark precedence over read_data and read",
			perms:    []string{"Read", "ReadAll", "ReadData"},
			expected: "read_spark",
		},
		{
			name:     "subscribe_onelake_events collapses to read_spark",
			perms:    []string{"Read", "SubscribeOneLakeEvents"},
			expected: "read_spark",
		},
		{
			name:     "read_data collapses to read_data",
			perms:    []string{"Read", "ReadData"},
			expected: "read_data",
		},
		{
			name:     "reshare collapses to reshare",
			perms:    []string{"Read", "Reshare"},
			expected: "reshare",
		},
		{
			name:    "unknown permission produces error",
			perms:   []string{"SuperAdmin"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CollapseRolePermissions(tc.perms)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestCollapseItemRolePermissions_Exhaustive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		itemType string
		perms    []string
		expected string
		wantErr  bool
	}{
		// Warehouse
		{name: "Warehouse write", itemType: "Warehouse", perms: []string{"Read", "Write"}, expected: "write"},
		{name: "Warehouse reshare", itemType: "Warehouse", perms: []string{"Read", "Reshare"}, expected: "reshare"},
		{name: "Warehouse read", itemType: "Warehouse", perms: []string{"Read"}, expected: "read"},
		{name: "Warehouse unmapped permission ReadData", itemType: "Warehouse", perms: []string{"ReadData"}, wantErr: true},
		{name: "Warehouse unknown permission", itemType: "Warehouse", perms: []string{"Unknown"}, wantErr: true},

		// SQLDatabase
		{name: "SQLDatabase write", itemType: "SQLDatabase", perms: []string{"Read", "Write"}, expected: "write"},
		{name: "SQLDatabase reshare", itemType: "SQLDatabase", perms: []string{"Read", "Reshare"}, expected: "reshare"},
		{name: "SQLDatabase read_data", itemType: "SQLDatabase", perms: []string{"Read", "ReadData"}, expected: "read_data"},
		{name: "SQLDatabase read", itemType: "SQLDatabase", perms: []string{"Read"}, expected: "read"},
		{name: "SQLDatabase unknown permission", itemType: "SQLDatabase", perms: []string{"Unknown"}, wantErr: true},

		// Default / Lakehouse
		{name: "Default Lakehouse read_spark", itemType: "Lakehouse", perms: []string{"Read", "ReadAll"}, expected: "read_spark"},
		{name: "Default Lakehouse unknown", itemType: "Lakehouse", perms: []string{"Unknown"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CollapseItemRolePermissions(tc.itemType, tc.perms)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestItemPermissionsBasePath_Exhaustive(t *testing.T) {
	t.Parallel()

	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"

	whPath, err := itemPermissionsBasePath(wsID, itemID, "Warehouse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if whPath != fmt.Sprintf("/v1/workspaces/%s/warehouses/%s", wsID, itemID) {
		t.Fatalf("unexpected warehouse path: %s", whPath)
	}

	sqlPath, err := itemPermissionsBasePath(wsID, itemID, "SQLDatabase")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sqlPath != fmt.Sprintf("/v1/workspaces/%s/sqlDatabases/%s", wsID, itemID) {
		t.Fatalf("unexpected sql path: %s", sqlPath)
	}

	_, err = itemPermissionsBasePath(wsID, itemID, "Lakehouse")
	if err == nil {
		t.Fatal("expected error for unsupported item type Lakehouse")
	}
}

func TestGetLakehouseMutex_Concurrency(t *testing.T) {
	t.Parallel()

	c, err := NewFabricClient(Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	var testMu sync.Mutex
	var active [3]int32
	counts := make(map[string]int)
	ptrs := make(map[string]*sync.Mutex)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bucket := idx % 3
			key := fmt.Sprintf("lh-%d", bucket)
			mu := c.getLakehouseMutex("ws-1", key)

			mu.Lock()
			cur := atomic.AddInt32(&active[bucket], 1)
			if cur > 1 {
				t.Errorf("expected at most 1 active goroutine for key %s, got %d", key, cur)
			}
			time.Sleep(50 * time.Microsecond)
			atomic.AddInt32(&active[bucket], -1)
			mu.Unlock()

			testMu.Lock()
			counts[key]++
			if existing, ok := ptrs[key]; ok {
				if existing != mu {
					t.Errorf("expected same mutex pointer for key %s", key)
				}
			} else {
				ptrs[key] = mu
			}
			testMu.Unlock()
		}(i)
	}
	wg.Wait()

	testMu.Lock()
	defer testMu.Unlock()
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("lh-%d", i)
		if counts[key] == 0 {
			t.Errorf("expected non-zero count for %s", key)
		}
	}
}

func TestLakehouseDataAccessRoles_ErrorDisambiguation(t *testing.T) {
	t.Parallel()

	wsID := "11111111-1111-1111-1111-111111111111"
	nonExistentID := "99999999-9999-9999-9999-999999999999"
	unenabledLHID := "22222222-2222-2222-2222-222222222222"
	warehouseID := "33333333-3333-3333-3333-333333333333"
	enabledLHID := "44444444-4444-4444-4444-444444444444"

	srv := fabricmock.NewServer(t)

	// Register unenabled Lakehouse (default lakehouseSecurityEnabled is false)
	srv.UpsertItem(fabricmock.Item{
		ID:          unenabledLHID,
		WorkspaceID: wsID,
		DisplayName: "unenabled_lakehouse",
		Type:        "Lakehouse",
	})

	// Register a Warehouse
	srv.UpsertItem(fabricmock.Item{
		ID:          warehouseID,
		WorkspaceID: wsID,
		DisplayName: "some_warehouse",
		Type:        "Warehouse",
	})

	// Register an enabled Lakehouse
	srv.UpsertItem(fabricmock.Item{
		ID:          enabledLHID,
		WorkspaceID: wsID,
		DisplayName: "enabled_lakehouse",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, enabledLHID)

	c := newTestClient(t, srv)
	ctx := context.Background()

	t.Run("non-existent item returns NotFoundError", func(t *testing.T) {
		_, _, err := c.ListDataAccessRoles(ctx, wsID, nonExistentID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var nfErr *NotFoundError
		if !errors.As(err, &nfErr) {
			t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
		}
		if nfErr.ResourceType != "Lakehouse" || nfErr.ResourceID != nonExistentID {
			t.Errorf("unexpected NotFoundError fields: %+v", nfErr)
		}
	})

	t.Run("unenabled lakehouse returns OneLakeSecurityNotEnabledError", func(t *testing.T) {
		_, _, err := c.ListDataAccessRoles(ctx, wsID, unenabledLHID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var secErr *OneLakeSecurityNotEnabledError
		if !errors.As(err, &secErr) {
			t.Fatalf("expected *OneLakeSecurityNotEnabledError, got %T: %v", err, err)
		}
		if secErr.LakehouseID != unenabledLHID || secErr.WorkspaceID != wsID {
			t.Errorf("unexpected OneLakeSecurityNotEnabledError fields: %+v", secErr)
		}
	})

	t.Run("non-lakehouse item returns UnsupportedItemTypeError", func(t *testing.T) {
		_, _, err := c.ListDataAccessRoles(ctx, wsID, warehouseID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var typeErr *UnsupportedItemTypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("expected *UnsupportedItemTypeError, got %T: %v", err, err)
		}
		if typeErr.ItemID != warehouseID || typeErr.ActualType != "Warehouse" {
			t.Errorf("unexpected UnsupportedItemTypeError fields: %+v", typeErr)
		}
	})

	t.Run("enabled lakehouse succeeds", func(t *testing.T) {
		roles, etag, err := c.ListDataAccessRoles(ctx, wsID, enabledLHID)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if etag == "" {
			t.Error("expected non-empty etag")
		}
		_ = roles
	})

	t.Run("fallback probe error is propagated directly", func(t *testing.T) {
		transientSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == fmt.Sprintf("/v1/workspaces/%s/items/%s/dataAccessRoles", wsID, "transient-id") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorCode":"ItemNotFound","message":"Item not found"}`))
				return
			}
			if r.URL.Path == fmt.Sprintf("/v1/workspaces/%s/items/%s", wsID, "transient-id") {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"errorCode":"RateLimitExceeded","message":"Too many requests"}`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer transientSrv.Close()

		transientClient, err := NewFabricClient(Config{
			Endpoint: transientSrv.URL,
			TokenProvider: func(_ context.Context) (string, error) {
				return "token", nil
			},
			MaxRetries:  0,
			BaseBackoff: 1 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("create transient client: %v", err)
		}

		_, _, err = transientClient.ListDataAccessRoles(ctx, wsID, "transient-id")
		if err == nil {
			t.Fatal("expected rate limit error, got nil")
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", apiErr.StatusCode)
		}
	})
}

func TestItemPermissionsClient_DirectOperations(t *testing.T) {
	t.Parallel()

	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	var lastMethod, lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path

		if strings.HasSuffix(r.URL.Path, "/permissions") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"value": [
					{
						"principal": {
							"id": %q,
							"type": "User"
						},
						"permissions": ["Read", "Write"]
					}
				]
			}`, principalID)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/grantPermissions") || strings.HasSuffix(r.URL.Path, "/revokePermissions") {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := NewFabricClient(Config{
		Endpoint: srv.URL,
		TokenProvider: func(context.Context) (string, error) {
			return "mock-token", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	p := Principal{ID: principalID, Type: "User"}

	// 1. GrantItemPermissions
	if err := c.GrantItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"read", "write"}); err != nil {
		t.Fatalf("GrantItemPermissions failed: %v", err)
	}
	if lastMethod != http.MethodPost || !strings.Contains(lastPath, "/grantPermissions") {
		t.Errorf("unexpected grant call: %s %s", lastMethod, lastPath)
	}

	// 2. GetItemPermissions match
	perms, err := c.GetItemPermissions(ctx, wsID, itemID, "Warehouse", principalID, "User")
	if err != nil {
		t.Fatalf("GetItemPermissions failed: %v", err)
	}
	if len(perms) != 2 || perms[0] != "read" || perms[1] != "write" {
		t.Errorf("unexpected permissions returned: %v", perms)
	}

	// 3. GetItemPermissions not found
	_, err = c.GetItemPermissions(ctx, wsID, itemID, "Warehouse", "missing-principal", "User")
	if err == nil {
		t.Fatal("expected error for missing principal")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound(err), got %v", err)
	}

	// 4. UpdateItemPermissions
	if err := c.UpdateItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"read", "write"}, []string{"read"}); err != nil {
		t.Fatalf("UpdateItemPermissions failed: %v", err)
	}

	// 5. RevokeItemPermissions
	if err := c.RevokeItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"write"}); err != nil {
		t.Fatalf("RevokeItemPermissions failed: %v", err)
	}

	// 6. Error cases for invalid item type or permissions
	if err := c.GrantItemPermissions(ctx, wsID, itemID, "InvalidType", p, []string{"read"}); err == nil {
		t.Fatal("expected error for InvalidType in GrantItemPermissions")
	}
	if err := c.GrantItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"invalid-perm"}); err == nil {
		t.Fatal("expected error for invalid-perm in GrantItemPermissions")
	}
	if _, err := c.GetItemPermissions(ctx, wsID, itemID, "InvalidType", principalID, "User"); err == nil {
		t.Fatal("expected error for InvalidType in GetItemPermissions")
	}
	if err := c.UpdateItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"bad"}, []string{"read"}); err == nil {
		t.Fatal("expected error for bad oldPerms in UpdateItemPermissions")
	}
	if err := c.UpdateItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"read"}, []string{"bad"}); err == nil {
		t.Fatal("expected error for bad newPerms in UpdateItemPermissions")
	}
	if err := c.RevokeItemPermissions(ctx, wsID, itemID, "InvalidType", p, []string{"read"}); err == nil {
		t.Fatal("expected error for InvalidType in RevokeItemPermissions")
	}
	if err := c.RevokeItemPermissions(ctx, wsID, itemID, "Warehouse", p, []string{"bad"}); err == nil {
		t.Fatal("expected error for bad perms in RevokeItemPermissions")
	}

	// 7. Test error stringers
	typeErr := &UnsupportedItemTypeError{
		WorkspaceID: wsID,
		ItemID:      itemID,
		ActualType:  "Warehouse",
	}
	if !strings.Contains(typeErr.Error(), "only supported on Lakehouses") {
		t.Errorf("unexpected error string: %s", typeErr.Error())
	}

	secErr := &OneLakeSecurityNotEnabledError{
		WorkspaceID: wsID,
		LakehouseID: itemID,
		Message:     "custom message",
	}
	if secErr.Error() != "custom message" {
		t.Errorf("unexpected custom message: %s", secErr.Error())
	}
}

func TestNormalizeETag_Invariants(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			expectError: true,
		},
		{
			name:        "weak etag lowercase",
			input:       `w/"etag-v1"`,
			expectError: true,
		},
		{
			name:        "weak etag uppercase",
			input:       `W/"etag-v1"`,
			expectError: true,
		},
		{
			name:        "wildcard",
			input:       "*",
			expectError: true,
		},
		{
			name:        "already quoted etag",
			input:       `"etag-v1"`,
			expected:    `"etag-v1"`,
			expectError: false,
		},
		{
			name:        "unquoted etag",
			input:       `etag-v1`,
			expected:    `"etag-v1"`,
			expectError: false,
		},
		{
			name:        "unquoted with leading trailing whitespace",
			input:       `  etag-v1  `,
			expected:    `"etag-v1"`,
			expectError: false,
		},
		{
			name:        "unbalanced leading quote",
			input:       `"etag-v1`,
			expectError: true,
		},
		{
			name:        "unbalanced trailing quote",
			input:       `etag-v1"`,
			expectError: true,
		},
		{
			name:        "rogue interior quotes",
			input:       `"etag"v1"`,
			expectError: true,
		},
		{
			name:        "rogue interior quotes unquoted",
			input:       `etag"v1`,
			expectError: true,
		},
		{
			name:        "empty quoted string",
			input:       `""`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeETag(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("normalizeETag(%q) expected error, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Fatalf("normalizeETag(%q) unexpected error: %v", tc.input, err)
				}
				if got != tc.expected {
					t.Fatalf("normalizeETag(%q) = %q, expected %q", tc.input, got, tc.expected)
				}
			}
		})
	}
}

func TestLakehouseDataAccessRoles_PaginationAndETagConsistency(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	// Force page size of 1 to ensure multi-page retrieval across 3 roles
	srv.SetDataAccessRolesPageSize(1)

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222221"

	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "analytics_lakehouse",
		Type:        "Lakehouse",
	})
	srv.EnableLakehouseSecurity(wsID, lhID)

	c := newTestClient(t, srv)
	ctx := context.Background()

	// 1. Initial lakehouse has DefaultReader (1 role). Let's upsert 2 more roles.
	role1 := DataAccessRole{
		Name: "RoleAlpha",
		Kind: "Policy",
		DecisionRules: []DecisionRule{
			{
				Effect: "Permit",
				Permission: []PermissionScope{
					{AttributeName: "Path", AttributeValueIncludedIn: []string{"Tables/alpha"}},
					{AttributeName: "Action", AttributeValueIncludedIn: []string{"Read"}},
				},
			},
		},
		Members: &Members{
			MicrosoftEntraMembers: []MicrosoftEntraMember{
				{ObjectID: "33333333-3333-3333-3333-333333333333", ObjectType: "Group", TenantID: "99999999-9999-9999-9999-999999999999"},
			},
		},
	}
	if err := c.UpsertDataAccessRole(ctx, wsID, lhID, role1); err != nil {
		t.Fatalf("UpsertDataAccessRole(RoleAlpha) failed: %v", err)
	}

	role2 := DataAccessRole{
		Name: "RoleBeta",
		Kind: "Policy",
		DecisionRules: []DecisionRule{
			{
				Effect: "Permit",
				Permission: []PermissionScope{
					{AttributeName: "Path", AttributeValueIncludedIn: []string{"Tables/beta"}},
					{AttributeName: "Action", AttributeValueIncludedIn: []string{"Read"}},
				},
			},
		},
		Members: &Members{
			MicrosoftEntraMembers: []MicrosoftEntraMember{
				{ObjectID: "44444444-4444-4444-4444-444444444444", ObjectType: "User", TenantID: "99999999-9999-9999-9999-999999999999"},
			},
		},
	}
	if err := c.UpsertDataAccessRole(ctx, wsID, lhID, role2); err != nil {
		t.Fatalf("UpsertDataAccessRole(RoleBeta) failed: %v", err)
	}

	// 2. Fetch single role via GetDataAccessRole (exercises multi-page listDataAccessRolesWithETag)
	gotAlpha, err := c.GetDataAccessRole(ctx, wsID, lhID, "RoleAlpha")
	if err != nil {
		t.Fatalf("GetDataAccessRole(RoleAlpha) failed: %v", err)
	}
	if gotAlpha.Name != "RoleAlpha" {
		t.Errorf("expected role name RoleAlpha, got %s", gotAlpha.Name)
	}

	gotBeta, err := c.GetDataAccessRole(ctx, wsID, lhID, "RoleBeta")
	if err != nil {
		t.Fatalf("GetDataAccessRole(RoleBeta) failed: %v", err)
	}
	if gotBeta.Name != "RoleBeta" {
		t.Errorf("expected role name RoleBeta, got %s", gotBeta.Name)
	}

	// 3. Verify DeleteDataAccessRole correctly reads paginated list and deletes role
	if err := c.DeleteDataAccessRole(ctx, wsID, lhID, "RoleAlpha"); err != nil {
		t.Fatalf("DeleteDataAccessRole(RoleAlpha) failed: %v", err)
	}

	// 4. Verify RoleAlpha is gone, RoleBeta still exists
	if _, err := c.GetDataAccessRole(ctx, wsID, lhID, "RoleAlpha"); !IsNotFound(err) {
		t.Fatalf("expected NotFoundError for RoleAlpha, got %v", err)
	}
	if _, err := c.GetDataAccessRole(ctx, wsID, lhID, "RoleBeta"); err != nil {
		t.Fatalf("expected RoleBeta to persist after RoleAlpha deletion: %v", err)
	}
}

func TestLakehouseDataAccessRoles_PaginationETagMismatchAndPageCap(t *testing.T) {
	t.Parallel()

	wsID := "11111111-1111-1111-1111-111111111111"
	lhID := "22222222-2222-2222-2222-222222222221"

	t.Run("etag mismatch across pages triggers conflict retry", func(t *testing.T) {
		page1Calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contToken := r.URL.Query().Get("continuationToken")
			w.Header().Set("Content-Type", "application/json")

			if contToken == "" {
				page1Calls++
				// First page
				w.Header().Set("ETag", `"etag-v1"`)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"value": [{"id": "r1", "name": "Role1", "kind": "Policy", "decisionRules": []}],
					"continuationToken": "token-page-2"
				}`))
				return
			}

			// Page 2: on first try, return mismatched ETag; on second attempt, return consistent ETag
			if page1Calls == 1 {
				w.Header().Set("ETag", `"etag-v2-mismatch"`)
			} else {
				w.Header().Set("ETag", `"etag-v1"`)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"value": [{"id": "r2", "name": "Role2", "kind": "Policy", "decisionRules": []}]
			}`))
		}))
		defer srv.Close()

		c, err := NewFabricClient(Config{
			Endpoint: srv.URL,
			TokenProvider: func(context.Context) (string, error) {
				return "token", nil
			},
			BaseBackoff: 1 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("NewFabricClient failed: %v", err)
		}

		roles, etag, err := c.ListDataAccessRoles(context.Background(), wsID, lhID)
		if err != nil {
			t.Fatalf("ListDataAccessRoles failed on etag mismatch retry: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("expected 2 roles, got %d", len(roles))
		}
		if etag != `"etag-v1"` {
			t.Fatalf("expected etag-v1, got %s", etag)
		}
		if page1Calls < 2 {
			t.Fatalf("expected at least 2 attempts due to page ETag mismatch, got %d", page1Calls)
		}
	})

	t.Run("pagination cycle returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"etag-v1"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"value": [{"id": "r1", "name": "Role1", "kind": "Policy", "decisionRules": []}],
				"continuationToken": "infinite-loop"
			}`))
		}))
		defer srv.Close()

		c, err := NewFabricClient(Config{
			Endpoint: srv.URL,
			TokenProvider: func(context.Context) (string, error) {
				return "token", nil
			},
		})
		if err != nil {
			t.Fatalf("NewFabricClient failed: %v", err)
		}

		_, _, err = c.ListDataAccessRoles(context.Background(), wsID, lhID)
		if err == nil || !strings.Contains(err.Error(), "detected cycle in pagination") {
			t.Fatalf("expected cycle error, got: %v", err)
		}
	})
}

func TestListItemAccessDetails_SuccessAndRateLimitRetry(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	c := newTestClient(t, srv)
	ctx := context.Background()

	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"
	principalName := "Data Engineers Group"
	principalType := fabadmin.PrincipalTypeGroup
	groupType := fabadmin.GroupTypeSecurityGroup
	itemTypeStr := fabadmin.ItemTypeWarehouse

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetItemAccessDetails(wsID, itemID, &fabadmin.ItemAccessDetailsResponse{
		AccessDetails: []fabadmin.ItemAccessDetails{
			{
				Principal: &fabadmin.GroupPrincipal{
					ID:          &principalID,
					DisplayName: &principalName,
					Type:        &principalType,
					GroupDetails: &fabadmin.GroupPrincipalGroupDetails{
						GroupType: &groupType,
					},
				},
				ItemAccessDetails: &fabadmin.ItemAccessDetail{
					Type:                  &itemTypeStr,
					Permissions:           []fabadmin.ItemPermissions{fabadmin.ItemPermissionsRead},
					AdditionalPermissions: []string{"ReadData"},
				},
			},
		},
	})

	// Inject 429 rate limit to verify Retry-After handling
	srv.InjectThrottle(http.StatusTooManyRequests, 1, 1)

	typeParam := "Warehouse"
	resp, err := c.ListItemAccessDetails(ctx, wsID, itemID, &typeParam)
	if err != nil {
		t.Fatalf("expected ListItemAccessDetails to succeed after retry, got: %v", err)
	}

	if len(resp.AccessDetails) != 1 {
		t.Fatalf("expected 1 access detail, got %d", len(resp.AccessDetails))
	}

	detail := resp.AccessDetails[0]
	if detail.Principal.ID != principalID {
		t.Errorf("expected principal ID %s, got %s", principalID, detail.Principal.ID)
	}
	if detail.Principal.DisplayName != principalName {
		t.Errorf("expected principal display name %s, got %s", principalName, detail.Principal.DisplayName)
	}
	if detail.Principal.Type != "Group" {
		t.Errorf("expected principal type Group, got %s", detail.Principal.Type)
	}
	if detail.Principal.GroupDetails == nil || detail.Principal.GroupDetails.GroupType != "SecurityGroup" {
		t.Errorf("expected groupType SecurityGroup, got %+v", detail.Principal.GroupDetails)
	}
	if !slices.Contains(detail.ItemAccessDetails.Permissions, "Read") {
		t.Errorf("expected permissions to contain Read, got %v", detail.ItemAccessDetails.Permissions)
	}
	if !slices.Contains(detail.ItemAccessDetails.AdditionalPermissions, "ReadData") {
		t.Errorf("expected additionalPermissions to contain ReadData, got %v", detail.ItemAccessDetails.AdditionalPermissions)
	}
}

func TestListItemAccessDetails_AdminForbiddenError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	c := newTestClient(t, srv)
	ctx := context.Background()

	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})

	srv.SetAdminForbidden(wsID, itemID, true)

	resp, err := c.ListItemAccessDetails(ctx, wsID, itemID, nil)
	if err == nil {
		t.Fatalf("expected error from ListItemAccessDetails when forbidden, got resp: %+v", resp)
	}

	var adminErr *AdminAPIAccessDeniedError
	if !errors.As(err, &adminErr) {
		t.Fatalf("expected AdminAPIAccessDeniedError, got %T: %v", err, err)
	}

	if adminErr.WorkspaceID != wsID || adminErr.ItemID != itemID {
		t.Errorf("expected workspaceID %s and itemID %s, got %+v", wsID, itemID, adminErr)
	}

	errStr := adminErr.Error()
	if !strings.Contains(errStr, "Fabric Administrator") || !strings.Contains(errStr, "Service principals can access read-only admin APIs") {
		t.Errorf("error string does not cite required tenant settings: %s", errStr)
	}
}

func TestListItemAccessDetails_SPNTenantRoleConflict401(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	itemID := "22222222-2222-2222-2222-222222222222"

	srv.UpsertItem(fabricmock.Item{
		ID:          itemID,
		WorkspaceID: wsID,
		DisplayName: "gold_warehouse",
		Type:        "Warehouse",
	})
	srv.SetAdminUnauthorizedErrorInfo(wsID, itemID, "PowerBINotAuthorizedException")

	// Create synthetic SPN token with Tenant.ReadWrite.All role
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"idtyp":"app","appid":"app-spn-uuid","oid":"spn-obj-uuid","tid":"tenant-uuid","roles":["Tenant.ReadWrite.All"]}`))
	spnToken := header + "." + claims + ".signature"

	c, err := NewFabricClient(Config{
		Endpoint: srv.URL(),
		TokenProvider: func(_ context.Context) (string, error) {
			return spnToken, nil
		},
		FabricAuthSource: "oidc",
		BaseBackoff:      5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create FabricClient: %v", err)
	}

	ctx := context.Background()
	resp, err := c.ListItemAccessDetails(ctx, wsID, itemID, nil)
	if err == nil {
		t.Fatalf("expected error from ListItemAccessDetails when unauthorized, got resp: %+v", resp)
	}

	var adminErr *AdminAPIAccessDeniedError
	if !errors.As(err, &adminErr) {
		t.Fatalf("expected AdminAPIAccessDeniedError, got %T: %v", err, err)
	}

	if adminErr.FaultClass != "ServicePrincipalTenantRoleConflict" {
		t.Errorf("expected FaultClass 'ServicePrincipalTenantRoleConflict', got %q", adminErr.FaultClass)
	}
	if adminErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected StatusCode 401, got %d", adminErr.StatusCode)
	}
	if adminErr.XPowerBIError != "PowerBINotAuthorizedException" {
		t.Errorf("expected XPowerBIError 'PowerBINotAuthorizedException', got %q", adminErr.XPowerBIError)
	}
	if !strings.Contains(adminErr.CallerPosture, "roles=[Tenant.ReadWrite.All]") {
		t.Errorf("expected CallerPosture to contain roles=[Tenant.ReadWrite.All], got: %s", adminErr.CallerPosture)
	}
	if !strings.Contains(adminErr.CallerPosture, "appid=app-spn-uuid") {
		t.Errorf("expected CallerPosture to contain appid=app-spn-uuid, got: %s", adminErr.CallerPosture)
	}

	errStr := adminErr.Error()
	if !strings.Contains(errStr, "fault_class=ServicePrincipalTenantRoleConflict") {
		t.Errorf("expected Error() to contain fault_class=ServicePrincipalTenantRoleConflict, got: %s", errStr)
	}
}

func TestRetryDelay_RFC9110HTTPDate(t *testing.T) {
	t.Parallel()

	c := &FabricClient{
		baseBackoff: 100 * time.Millisecond,
	}

	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	delay := c.retryDelay(future, 1)
	if delay < 5*time.Second || delay > 12*time.Second {
		t.Errorf("expected delay approximately 10s, got %v", delay)
	}

	past := time.Now().Add(-10 * time.Second).UTC().Format(http.TimeFormat)
	delayPast := c.retryDelay(past, 1)
	if delayPast != 0 {
		t.Errorf("expected 0 delay for past HTTP-date, got %v", delayPast)
	}

	numericDelay := c.retryDelay("15", 1)
	if numericDelay != 15*time.Second {
		t.Errorf("expected 15s delay for integer seconds, got %v", numericDelay)
	}
}

func TestFabricClient_PermissionMethodsAndCoverage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. NewFabricClient endpoint and token provider defaults/errors
	cDefault, err := NewFabricClient(Config{
		TokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error creating client with default endpoint: %v", err)
	}
	if cDefault.baseURL != DefaultEndpoint {
		t.Errorf("expected default endpoint %s, got %s", DefaultEndpoint, cDefault.baseURL)
	}

	_, errNoToken := NewFabricClient(Config{})
	if errNoToken == nil {
		t.Fatal("expected error on NewFabricClient with nil token provider and chain")
	}

	// 2. getLakehouseMutex caching
	mu1 := cDefault.getLakehouseMutex("ws1", "lh1")
	mu2 := cDefault.getLakehouseMutex("ws1", "lh1")
	if mu1 != mu2 {
		t.Errorf("expected identical mutex pointer for same lakehouse, got %p vs %p", mu1, mu2)
	}
	mu3 := cDefault.getLakehouseMutex("ws1", "lh2")
	if mu1 == mu3 {
		t.Errorf("expected different mutex for different lakehouse")
	}

	// 3. GetItemPermissions, UpdateItemPermissions, RevokeItemPermissions with mock server
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	principalID := "33333333-3333-3333-3333-333333333333"

	mockHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/permissions") && r.Method == http.MethodGet:
			if strings.Contains(path, "missing-wh") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"value": [
					{
						"principal": {
							"id": "33333333-3333-3333-3333-333333333333",
							"type": "User"
						},
						"permissions": ["Read", "ReadWrite"]
					},
					{
						"principal": {
							"id": "44444444-4444-4444-4444-444444444444",
							"type": "User"
						},
						"permissions": ["UnknownPermission"]
					}
				]
			}`))
		case strings.HasSuffix(path, "/grantPermissions") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(path, "/revokePermissions") && r.Method == http.MethodPost:
			if strings.Contains(path, "missing-revoke") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		case strings.Contains(path, "/admin/workspaces/") && strings.Contains(path, "/items/") && strings.HasSuffix(path, "/users"):
			// Verify type query parameter passed
			if r.URL.Query().Get("type") != "Warehouse" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessDetails": []}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockHTTP.Close()

	c, err := NewFabricClient(Config{
		Endpoint:      mockHTTP.URL,
		TokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:   5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// 3a. GetItemPermissions invalid itemType
	_, errInvalidType := c.GetItemPermissions(ctx, wsID, whID, "Lakehouse", principalID, "User")
	if errInvalidType == nil {
		t.Fatal("expected error on GetItemPermissions with unsupported item type")
	}

	// 3b. GetItemPermissions item 404
	_, errNotFound := c.GetItemPermissions(ctx, wsID, "missing-wh", "Warehouse", principalID, "User")
	if errNotFound == nil || !IsNotFound(errNotFound) {
		t.Fatalf("expected NotFoundError on 404 item: %v", errNotFound)
	}

	// 3c. GetItemPermissions match
	perms, err := c.GetItemPermissions(ctx, wsID, whID, "Warehouse", principalID, "User")
	if err != nil {
		t.Fatalf("unexpected error getting item permissions: %v", err)
	}
	if len(perms) == 0 || !slices.Contains(perms, "read") {
		t.Errorf("expected permissions to contain read, got %v", perms)
	}

	// 3d. GetItemPermissions principal with only unrecognized permissions
	_, errUnknown := c.GetItemPermissions(ctx, wsID, whID, "Warehouse", "44444444-4444-4444-4444-444444444444", "User")
	if errUnknown == nil || !IsNotFound(errUnknown) {
		t.Fatalf("expected NotFoundError for principal with unknown perms: %v", errUnknown)
	}

	// 3e. GetItemPermissions principal not in list
	_, errMissingPrincipal := c.GetItemPermissions(ctx, wsID, whID, "Warehouse", "99999999-9999-9999-9999-999999999999", "User")
	if errMissingPrincipal == nil || !IsNotFound(errMissingPrincipal) {
		t.Fatalf("expected NotFoundError for non-existent principal: %v", errMissingPrincipal)
	}

	// 3f. UpdateItemPermissions invalid old/new permissions
	p := Principal{ID: principalID, Type: "User"}
	errBadOld := c.UpdateItemPermissions(ctx, wsID, whID, "Warehouse", p, []string{"BadPerm"}, []string{"read"})
	if errBadOld == nil {
		t.Fatal("expected error on invalid old permission")
	}
	errBadNew := c.UpdateItemPermissions(ctx, wsID, whID, "Warehouse", p, []string{"read"}, []string{"BadPerm"})
	if errBadNew == nil {
		t.Fatal("expected error on invalid new permission")
	}

	// 3g. UpdateItemPermissions success (downgrade revoking read_data and granting read)
	errUpdate := c.UpdateItemPermissions(ctx, wsID, whID, "Warehouse", p, []string{"read", "read_data"}, []string{"read"})
	if errUpdate != nil {
		t.Fatalf("unexpected error updating item permissions: %v", errUpdate)
	}

	// 3h. RevokeItemPermissions 404 is idempotent
	errRevoke404 := c.RevokeItemPermissions(ctx, wsID, "missing-revoke", "Warehouse", p, []string{"read"})
	if errRevoke404 != nil {
		t.Fatalf("expected 404 revoke to succeed idempotently: %v", errRevoke404)
	}

	// 3i. RevokeItemPermissions invalid itemType
	errRevokeBadType := c.RevokeItemPermissions(ctx, wsID, whID, "InvalidItemType", p, []string{"read"})
	if errRevokeBadType == nil {
		t.Fatal("expected error revoking on invalid item type")
	}

	// 3j. ListItemAccessDetails with itemType query filter
	itemTypeFilter := "Warehouse"
	_, errAccess := c.ListItemAccessDetails(ctx, wsID, whID, &itemTypeFilter)
	if errAccess != nil {
		t.Fatalf("unexpected error listing access details with type filter: %v", errAccess)
	}
}
