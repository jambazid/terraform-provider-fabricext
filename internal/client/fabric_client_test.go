// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
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
