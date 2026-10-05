// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
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

func TestUpdateItemPermissions_DowngradeRevokesExcessPrivileges(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	dbID := "44444444-4444-4444-4444-444444444444"
	principal := Principal{
		ID:   "55555555-5555-5555-5555-555555555555",
		Type: "Group",
	}

	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "app_sqldb",
		Type:        "SQLDatabase",
	})

	c := newTestClient(t, srv)
	ctx := context.Background()

	// Initial grant: read, read_data, write
	if err := c.GrantItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal, []string{"read", "read_data", "write"}); err != nil {
		t.Fatalf("GrantItemPermissions failed: %v", err)
	}

	// Downgrade to ["read"] -> must revoke ["ReadData", "Write"] before granting ["Read"].
	if err := c.UpdateItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal, []string{"read", "read_data", "write"}, []string{"read"}); err != nil {
		t.Fatalf("UpdateItemPermissions failed: %v", err)
	}

	seq := srv.OperationSequence()
	if !slices.Equal(seq, []string{"grant", "revoke", "grant"}) {
		t.Fatalf("expected operation sequence [grant, revoke, grant], got %v", seq)
	}
	if !slices.Equal(srv.LastRevokedPermissions(), []string{"ReadData", "Write"}) {
		t.Fatalf("expected revoked permissions [ReadData Write], got %v", srv.LastRevokedPermissions())
	}

	gotPerms, err := c.GetItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal.ID, principal.Type)
	if err != nil {
		t.Fatalf("GetItemPermissions failed: %v", err)
	}
	if !slices.Equal(gotPerms, []string{"read"}) {
		t.Fatalf("expected final permissions [read], got %v", gotPerms)
	}

	// Revoke all and confirm idempotent 404 handling on second revoke.
	if err := c.RevokeItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal, []string{"read"}); err != nil {
		t.Fatalf("first RevokeItemPermissions failed: %v", err)
	}
	if err := c.RevokeItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal, []string{"read"}); err != nil {
		t.Fatalf("second RevokeItemPermissions should treat 404 as idempotent nil, got: %v", err)
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
