// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/pbimock"
)

func TestItemPermissions_BitmaskBijection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		itemType    string
		perms       []string
		expCore     int64
		expArtifact int64
		expDecoded  []string
		expErr      bool
	}{
		{
			itemType:    "Warehouse",
			perms:       []string{"Read"},
			expCore:     1,
			expArtifact: 1,
			expDecoded:  []string{"Read"},
		},
		{
			itemType:    "Warehouse",
			perms:       []string{"Read", "ReadData"},
			expCore:     1,
			expArtifact: 3, // 1 | 2
			expDecoded:  []string{"Read", "ReadData"},
		},
		{
			itemType:    "Warehouse",
			perms:       []string{"Read", "ReadData", "ReadAll", "Reshare"},
			expCore:     5,  // 1 | 4
			expArtifact: 15, // 1 | 2 | 4 | 8
			expDecoded:  []string{"Read", "ReadData", "ReadAll", "Reshare"},
		},
		{
			itemType: "Warehouse",
			perms:    []string{"Execute"},
			expErr:   true, // Execute not allowed on Warehouse
		},
		{
			itemType:    "Lakehouse",
			perms:       []string{"Read", "ReadData", "Execute"},
			expCore:     65, // 1 | 64
			expArtifact: 67, // 1 | 2 | 64
			expDecoded:  []string{"Read", "ReadData", "Execute"},
		},
		{
			itemType:    "SQLDatabase",
			perms:       []string{"Read", "ReadData", "ReadAll", "Reshare", "Execute"},
			expCore:     69, // 1 | 4 | 64
			expArtifact: 79, // 1 | 2 | 4 | 8 | 64
			expDecoded:  []string{"Read", "ReadData", "ReadAll", "Reshare", "Execute"},
		},
		{
			itemType: "UnknownType",
			perms:    []string{"Read"},
			expErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.itemType+"_"+strings.Join(tc.perms, "_"), func(t *testing.T) {
			t.Parallel()

			core, artifact, err := client.EncodePermissions(tc.itemType, tc.perms)
			if tc.expErr {
				if err == nil {
					t.Fatalf("expected error for %s with perms %v, got nil", tc.itemType, tc.perms)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected encode error: %v", err)
			}
			if core != tc.expCore || artifact != tc.expArtifact {
				t.Fatalf("expected bitmasks core=%d, artifact=%d; got core=%d, artifact=%d", tc.expCore, tc.expArtifact, core, artifact)
			}

			decoded, err := client.DecodePermissions(tc.itemType, core, artifact)
			if err != nil {
				t.Fatalf("unexpected decode error: %v", err)
			}
			if strings.Join(decoded, ",") != strings.Join(tc.expDecoded, ",") {
				t.Fatalf("expected decoded %v, got %v", tc.expDecoded, decoded)
			}
		})
	}
}

func TestItemPermissions_LifecycleWithPBIMock(t *testing.T) {
	t.Parallel()

	mockSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer mockSrv.Close()

	c, err := client.NewFabricClient(client.Config{
		Endpoint:        mockSrv.URL,
		PowerBIEndpoint: mockSrv.URL,
		TokenProvider: func(_ context.Context) (string, error) {
			return "mock-fabric-token", nil
		},
		PowerBITokenProvider: func(_ context.Context) (string, error) {
			return "mock-pbi-token", nil
		},
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("creating fabric client: %v", err)
	}

	ctx := context.Background()
	wsID := "00000000-0000-0000-0000-000000000001"
	itemID := "00000000-0000-0000-0000-000000000002"
	principalID := "00000000-0000-0000-0000-000000000003"

	// 1. Initially absent
	existing, err := c.GetItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID)
	if err != nil {
		t.Fatalf("unexpected error on initial read: %v", err)
	}
	if existing != nil {
		t.Fatalf("expected no initial permissions, got %+v", existing)
	}

	// 2. Grant permissions (POST /metadata/access)
	share := client.ItemDirectShare{
		PrincipalID:   principalID,
		PrincipalType: "User",
		Permissions:   []string{"Read", "ReadData"},
	}
	if err := c.UpsertItemDirectPermissions(ctx, wsID, itemID, "Warehouse", share); err != nil {
		t.Fatalf("granting permissions: %v", err)
	}

	// 3. Read back
	granted, err := c.GetItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID)
	if err != nil {
		t.Fatalf("reading granted permissions: %v", err)
	}
	if granted == nil {
		t.Fatalf("expected granted permissions, got nil")
	}
	if strings.Join(granted.Permissions, ",") != "Read,ReadData" {
		t.Fatalf("expected permissions [Read, ReadData], got %v", granted.Permissions)
	}

	// 4. In-place update (PUT /metadata/access)
	share.Permissions = []string{"Read", "ReadData", "ReadAll", "Reshare"}
	if err := c.UpsertItemDirectPermissions(ctx, wsID, itemID, "Warehouse", share); err != nil {
		t.Fatalf("updating permissions: %v", err)
	}

	updated, err := c.GetItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID)
	if err != nil {
		t.Fatalf("reading updated permissions: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected updated permissions, got nil")
	}
	if strings.Join(updated.Permissions, ",") != "Read,ReadData,ReadAll,Reshare" {
		t.Fatalf("expected permissions [Read, ReadData, ReadAll, Reshare], got %v", updated.Permissions)
	}

	// 5. Revoke permissions (PUT with 0/0)
	if err := c.RevokeItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID); err != nil {
		t.Fatalf("revoking permissions: %v", err)
	}

	// 6. Verify absent after revoke
	afterRevoke, err := c.GetItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID)
	if err != nil {
		t.Fatalf("reading after revocation: %v", err)
	}
	if afterRevoke != nil {
		t.Fatalf("expected nil after revocation, got %+v", afterRevoke)
	}

	// 7. Revoke again is idempotent
	if err := c.RevokeItemDirectPermissions(ctx, wsID, itemID, "Warehouse", principalID); err != nil {
		t.Fatalf("idempotent revoke failed: %v", err)
	}
}

func TestItemPermissions_ConcurrentAccessSafety(t *testing.T) {
	t.Parallel()

	mockSrv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("starting pbimock: %v", err)
	}
	defer mockSrv.Close()

	c, err := client.NewFabricClient(client.Config{
		Endpoint:        mockSrv.URL,
		PowerBIEndpoint: mockSrv.URL,
		TokenProvider: func(_ context.Context) (string, error) {
			return "mock-token", nil
		},
		PowerBITokenProvider: func(_ context.Context) (string, error) {
			return "mock-pbi-token", nil
		},
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		t.Fatalf("creating fabric client: %v", err)
	}

	ctx := context.Background()
	wsID := "00000000-0000-0000-0000-000000000001"
	itemID := "00000000-0000-0000-0000-000000000002"

	var wg sync.WaitGroup
	errs := make(chan error, 15)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		principalID := "00000000-0000-0000-0000-00000000000" + string(rune('0'+i))
		go func(pID string) {
			defer wg.Done()
			share := client.ItemDirectShare{
				PrincipalID:   pID,
				PrincipalType: "User",
				Permissions:   []string{"Read", "ReadData"},
			}
			if err := c.UpsertItemDirectPermissions(ctx, wsID, itemID, "Lakehouse", share); err != nil {
				errs <- fmt.Errorf("upsert %s: %w", pID, err)
			}
			if _, err := c.GetItemDirectPermissions(ctx, wsID, itemID, "Lakehouse", pID); err != nil {
				errs <- fmt.Errorf("get %s: %w", pID, err)
			}
			if err := c.RevokeItemDirectPermissions(ctx, wsID, itemID, "Lakehouse", pID); err != nil {
				errs <- fmt.Errorf("revoke %s: %w", pID, err)
			}
		}(principalID)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestValidateClusterURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		customHost string
		wantErr    bool
	}{
		{"valid public commercial", "https://wabi-north-europe-redirect.analysis.windows.net", "", false},
		{"valid usgov", "https://wabi-usgov-redirect.analysis.usgovcloudapi.net", "", false},
		{"valid china", "https://wabi-china-redirect.analysis.chinacloudapi.cn", "", false},
		{"valid loopback http", "http://127.0.0.1:8080", "", false},
		{"valid loopback localhost", "http://localhost:9000", "", false},
		{"valid custom host", "https://my-proxy.corp.internal", "my-proxy.corp.internal", false},
		{"reject http non-loopback", "http://wabi-redirect.analysis.windows.net", "", true},
		{"reject credentials", "https://user:pass@wabi-redirect.analysis.windows.net", "", true},
		{"reject path segments", "https://wabi-redirect.analysis.windows.net/admin/metadata", "", true},
		{"reject query params", "https://wabi-redirect.analysis.windows.net?foo=bar", "", true},
		{"reject fragment", "https://wabi-redirect.analysis.windows.net#frag", "", true},
		{"reject unauthorized domain", "https://evil-analysis.windows.net.attacker.com", "", true},
		{"reject custom host mismatch", "https://other-proxy.corp.internal", "my-proxy.corp.internal", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := client.ValidateClusterURIForTest(tt.raw, tt.customHost)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateClusterURI(%q, %q) error = %v, wantErr = %v", tt.raw, tt.customHost, err, tt.wantErr)
			}
		})
	}
}

func TestExtractClusterURIFromCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cookie string
		want   string
	}{
		{
			name:   "standard cookie",
			cookie: "ClusterUri=https://wabi-north-europe.analysis.windows.net/; Path=/; Secure",
			want:   "https://wabi-north-europe.analysis.windows.net/",
		},
		{
			name:   "quoted cookie",
			cookie: `ClusterUri="https://wabi-west-us.analysis.windows.net/"; Path=/`,
			want:   "https://wabi-west-us.analysis.windows.net/",
		},
		{
			name:   "percent-encoded cookie",
			cookie: "ClusterUri=https%3A%2F%2Fwabi-east-us.analysis.windows.net%2F; Path=/",
			want:   "https://wabi-east-us.analysis.windows.net/",
		},
		{
			name:   "bare host without scheme",
			cookie: "ClusterUri=wabi-south-central.analysis.windows.net; Path=/",
			want:   "https://wabi-south-central.analysis.windows.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := client.ExtractClusterURIFromCookieForTest(tt.cookie)
			if got != tt.want {
				t.Errorf("ExtractClusterURIFromCookie(%q) = %q, want %q", tt.cookie, got, tt.want)
			}
		})
	}
}

func TestSQLDatabase_WritePermissionEncodingDecoding(t *testing.T) {
	t.Parallel()

	core, artifact, err := client.EncodePermissions("SQLDatabase", []string{"Read", "Write", "Execute"})
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if core&2 == 0 {
		t.Errorf("expected write bit 2 set in core (%d)", core)
	}

	decoded, err := client.DecodePermissions("SQLDatabase", core, artifact)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	hasWrite := false
	for _, p := range decoded {
		if p == "Write" {
			hasWrite = true
		}
	}
	if !hasWrite {
		t.Errorf("expected 'Write' in decoded permissions: %v", decoded)
	}
}
