// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestFabricClient_InvalidateClusterURI(t *testing.T) {
	t.Parallel()
	c, err := client.NewFabricClient(client.Config{
		Endpoint: "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) {
			return "tok", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	c.InvalidateClusterURI()
}

func TestScrubAndCapBody(t *testing.T) {
	t.Parallel()

	input := []byte("error occurred: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-IDN context")
	scrubbed := client.ScrubAndCapBodyForTest(input)
	if strings.Contains(scrubbed, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("expected Bearer token to be redacted, got: %s", scrubbed)
	}

	large := make([]byte, 5000)
	for i := range large {
		large[i] = 'X'
	}
	capped := client.ScrubAndCapBodyForTest(large)
	if len(capped) > 4200 || !strings.Contains(capped, "[truncated]") {
		t.Errorf("expected large body to be capped with truncation marker, length: %d", len(capped))
	}
}

func TestDiscoverClusterURI_InvalidPortal(t *testing.T) {
	t.Parallel()

	// 1. Invalid portal URL syntax
	c, err := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://attacker.com/evil",
		TokenProvider:   func(context.Context) (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = c.DiscoverClusterURI(context.Background())
	if err == nil {
		t.Fatal("expected error on untrusted powerbi endpoint")
	}
}

func TestGetItemDirectPermissions_NotFoundAndMissingPrincipal(t *testing.T) {
	t.Parallel()

	srv, err := pbimock.NewServer()
	if err != nil {
		t.Fatalf("failed to create pbimock server: %v", err)
	}
	defer srv.Close()

	c, err := client.NewFabricClient(client.Config{
		Endpoint:             "https://api.fabric.microsoft.com",
		PowerBIEndpoint:      srv.URL,
		TokenProvider:        func(context.Context) (string, error) { return "mock-token", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "mock-token", nil },
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	wsID := "11111111-1111-1111-1111-111111111111"
	missingItem := "99999999-9999-9999-9999-999999999999"

	// 1. Missing principal returns nil permissions and nil error
	perms, err := c.GetItemDirectPermissions(ctx, wsID, missingItem, "Warehouse", "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("unexpected error querying missing principal: %v", err)
	}
	if perms != nil {
		t.Fatalf("expected nil permissions for absent principal, got: %v", perms)
	}

	// 2. RevokeItemDirectPermissions on absent principal returns nil cleanly (idempotent absent)
	err = c.RevokeItemDirectPermissions(ctx, wsID, missingItem, "Warehouse", "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("expected nil error revoking absent principal, got: %v", err)
	}
}

type mockRoundTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestDiscoverClusterURI_5HopRedirect(t *testing.T) {
	t.Parallel()

	step := 0
	rt := &mockRoundTripper{
		roundTrip: func(_ *http.Request) (*http.Response, error) {
			step++
			if step == 1 {
				// 1st request -> 302 redirect to another app.powerbi.com path
				resp := &http.Response{
					StatusCode: http.StatusFound,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("")),
				}
				resp.Header.Set("Location", "https://app.powerbi.com/step2")
				return resp, nil
			}
			// 2nd request -> returns Set-Cookie with cluster URI
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Add("Set-Cookie", "ClusterUri=https://wabi-west-us.analysis.windows.net/; Path=/; Secure")
			return resp, nil
		},
	}

	c, err := client.NewFabricClient(client.Config{
		Endpoint:             "https://api.fabric.microsoft.com",
		PowerBIEndpoint:      "https://app.powerbi.com",
		HTTPClient:           &http.Client{Transport: rt},
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "pbi-tok", nil },
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	uri, err := c.DiscoverClusterURI(context.Background())
	if err != nil {
		t.Fatalf("failed to discover cluster: %v", err)
	}
	if uri != "https://wabi-west-us.analysis.windows.net" {
		t.Fatalf("expected cluster URI https://wabi-west-us.analysis.windows.net, got %q", uri)
	}
}

func TestDoPowerBIRequest_RetriesAndReplay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	attempt := 0
	rt := &mockRoundTripper{
		roundTrip: func(_ *http.Request) (*http.Response, error) {
			attempt++
			if attempt == 1 {
				// 429 rate limit on first attempt
				resp := &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("rate limited")),
				}
				resp.Header.Set("Retry-After", "0")
				return resp, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
			}, nil
		},
	}

	c, err := client.NewFabricClient(client.Config{
		Endpoint:      "https://api.fabric.microsoft.com",
		HTTPClient:    &http.Client{Transport: rt},
		TokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:   1 * time.Millisecond,
		MaxRetries:    2,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	bodyContent := "test body"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://example.com/test", strings.NewReader(bodyContent))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(bodyContent)), nil
	}

	resp, err := c.DoPowerBIRequestForTest(ctx, req)
	if err != nil {
		t.Fatalf("expected success on retry, got: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", resp.StatusCode)
	}
}

func TestValidateRedirectURL_EdgeCases(t *testing.T) {
	t.Parallel()

	// 1. Invalid URL parse
	if err := client.ValidateRedirectURLForTest("::bad-url", ""); err == nil {
		t.Error("expected error on unparseable URL")
	}

	// 2. Non-HTTPS scheme (not loopback)
	if err := client.ValidateRedirectURLForTest("http://app.powerbi.com", ""); err == nil {
		t.Error("expected error on non-https scheme")
	}

	// 3. Credentials in redirect URL
	if err := client.ValidateRedirectURLForTest("https://user:pass@app.powerbi.com", ""); err == nil {
		t.Error("expected error on credentials in redirect URL")
	}

	// 4. Loopback HTTP allowed
	if err := client.ValidateRedirectURLForTest("http://127.0.0.1:8080/auth", ""); err != nil {
		t.Errorf("unexpected error on loopback: %v", err)
	}
	if err := client.ValidateRedirectURLForTest("http://localhost:8080/auth", ""); err != nil {
		t.Errorf("unexpected error on localhost: %v", err)
	}

	// 5. Allowed custom host
	if err := client.ValidateRedirectURLForTest("https://custom.local:9000/step", "custom.local"); err != nil {
		t.Errorf("unexpected error on allowed custom host: %v", err)
	}

	// 6. Sovereign clouds and Power BI domains
	sovereignURLs := []string{
		"https://app.powerbi.com/landing",
		"https://usgov.analysis.usgovcloudapi.net/step",
		"https://china.analysis.chinacloudapi.cn/step",
		"https://dedicated.pbidedicated.windows.net/step",
		"https://wabi.analysis.windows.net/step",
	}
	for _, u := range sovereignURLs {
		if err := client.ValidateRedirectURLForTest(u, ""); err != nil {
			t.Errorf("unexpected error on sovereign URL %s: %v", u, err)
		}
	}

	// 7. Unauthorized domain
	if err := client.ValidateRedirectURLForTest("https://attacker.com/evil", ""); err == nil {
		t.Error("expected error on unauthorized domain")
	}
}

func TestValidateClusterURI_EdgeCases(t *testing.T) {
	t.Parallel()

	// 1. Invalid scheme
	if err := client.ValidateClusterURIForTest("ftp://wabi.analysis.windows.net", ""); err == nil {
		t.Error("expected error on ftp scheme")
	}

	// 2. User info
	if err := client.ValidateClusterURIForTest("https://user:pass@wabi.analysis.windows.net", ""); err == nil {
		t.Error("expected error on userinfo")
	}

	// 3. Path segments
	if err := client.ValidateClusterURIForTest("https://wabi.analysis.windows.net/some/path", ""); err == nil {
		t.Error("expected error on path segments")
	}

	// 4. Query params and fragments
	if err := client.ValidateClusterURIForTest("https://wabi.analysis.windows.net?q=1", ""); err == nil {
		t.Error("expected error on query parameters")
	}
	if err := client.ValidateClusterURIForTest("https://wabi.analysis.windows.net#frag", ""); err == nil {
		t.Error("expected error on fragment")
	}

	// 5. Loopback and allowed custom host
	if err := client.ValidateClusterURIForTest("http://127.0.0.1:8080", ""); err != nil {
		t.Errorf("unexpected error on loopback: %v", err)
	}
	if err := client.ValidateClusterURIForTest("https://custom.local:8080", "custom.local"); err != nil {
		t.Errorf("unexpected error on allowed custom host: %v", err)
	}

	// 6. Unauthorized domain
	if err := client.ValidateClusterURIForTest("https://attacker.com", ""); err == nil {
		t.Error("expected error on unauthorized domain")
	}
}

func TestErrorTypes_Formatting(t *testing.T) {
	t.Parallel()

	// AdminAPIAccessDeniedError without and with message
	err1 := &client.AdminAPIAccessDeniedError{
		WorkspaceID: "ws-1",
		ItemID:      "item-1",
	}
	if !strings.Contains(err1.Error(), "caller must be a Fabric Administrator") {
		t.Errorf("unexpected error string: %s", err1.Error())
	}

	err2 := &client.AdminAPIAccessDeniedError{
		WorkspaceID: "ws-1",
		ItemID:      "item-1",
		Message:     "upstream 403 Forbidden",
	}
	if !strings.Contains(err2.Error(), "upstream error: upstream 403 Forbidden") {
		t.Errorf("unexpected error string: %s", err2.Error())
	}

	// SilentPermissionDropError
	err3 := &client.SilentPermissionDropError{
		WorkspaceID:      "ws-1",
		ItemID:           "item-1",
		PrincipalID:      "p-1",
		ItemType:         "Warehouse",
		ExpectedCore:     3,
		ExpectedArtifact: 0,
		ActualCore:       1,
		ActualArtifact:   0,
	}
	if !strings.Contains(err3.Error(), "silent permission drop") {
		t.Errorf("unexpected error string: %s", err3.Error())
	}
}

func TestExpandRolePermissions_AllBranches(t *testing.T) {
	t.Parallel()

	// Read
	perms, err := client.ExpandRolePermissions("Lakehouse", "Read")
	if err != nil || len(perms) != 1 || perms[0] != "Read" {
		t.Errorf("unexpected Read: %v, %v", perms, err)
	}

	// Read_Data on Lakehouse vs Warehouse
	perms, err = client.ExpandRolePermissions("Lakehouse", "read_data")
	if err != nil || len(perms) != 2 {
		t.Errorf("unexpected read_data on Lakehouse: %v, %v", perms, err)
	}
	_, err = client.ExpandRolePermissions("Warehouse", "read_data")
	if err == nil {
		t.Error("expected error on read_data for Warehouse")
	}

	// Read_Spark on Lakehouse vs Warehouse
	perms, err = client.ExpandRolePermissions("Lakehouse", "read_spark")
	if err != nil || len(perms) != 3 {
		t.Errorf("unexpected read_spark on Lakehouse: %v, %v", perms, err)
	}
	_, err = client.ExpandRolePermissions("Warehouse", "read_spark")
	if err == nil {
		t.Error("expected error on read_spark for Warehouse")
	}

	// Write
	perms, err = client.ExpandRolePermissions("Lakehouse", "write")
	if err != nil || len(perms) != 2 {
		t.Errorf("unexpected write: %v, %v", perms, err)
	}

	// Reshare
	perms, err = client.ExpandRolePermissions("Lakehouse", "reshare")
	if err != nil || len(perms) != 2 {
		t.Errorf("unexpected reshare: %v, %v", perms, err)
	}

	// Invalid
	_, err = client.ExpandRolePermissions("Lakehouse", "unknown_role")
	if err == nil {
		t.Error("expected error on unknown role")
	}
}

func TestGetItemAccessLock_EdgeCoverage(t *testing.T) {
	t.Parallel()

	c, err := client.NewFabricClient(client.Config{
		Endpoint:      "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Normal load/store
	lock1 := c.GetItemAccessLockForTest("ws-1", "item-1")
	if lock1 == nil {
		t.Fatal("expected non-nil lock")
	}

	// Fallback when map has non-*sync.Mutex
	c.GetItemAccessLocksMapForTest().Store("ws-corrupted/item-corrupted", "invalid-type")
	lock2 := c.GetItemAccessLockForTest("ws-corrupted", "item-corrupted")
	if lock2 == nil {
		t.Fatal("expected non-nil lock on fallback")
	}
}

func TestItemPermissions_EdgeCasesAndCoverage(t *testing.T) {
	t.Parallel()

	// 1. ScrubAndCapBodyForTest
	scrubbed := client.ScrubAndCapBodyForTest([]byte("Error: Bearer secret-token-12345 unauthorized"))
	if !strings.Contains(scrubbed, "Bearer [REDACTED]") {
		t.Errorf("expected Bearer token redacted, got %s", scrubbed)
	}

	largeBody := make([]byte, 5000)
	for i := range largeBody {
		largeBody[i] = 'a'
	}
	capped := client.ScrubAndCapBodyForTest(largeBody)
	if !strings.HasSuffix(capped, "... [truncated]") || len(capped) > 4150 {
		t.Errorf("expected capped body, got length %d", len(capped))
	}

	// 2. ExtractClusterURIFromCookieForTest
	cookie1 := "foo=bar; ClusterUri=wabi-us-north-central-redirect.analysis.windows.net=https://wabi-us-north-central.analysis.windows.net/; baz=qux"
	uri1 := client.ExtractClusterURIFromCookieForTest(cookie1)
	if uri1 != "https://wabi-us-north-central.analysis.windows.net/" {
		t.Errorf("expected parsed URI, got %q", uri1)
	}

	cookieEmpty := "foo=bar; baz=qux"
	if uriEmpty := client.ExtractClusterURIFromCookieForTest(cookieEmpty); uriEmpty != "" {
		t.Errorf("expected empty URI, got %q", uriEmpty)
	}

	// 3. doPowerBIRequest retries on 429
	retryCount := 0
	retryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", "http://127.0.0.1:1"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if retryCount == 0 {
			retryCount++
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer retryServer.Close()

	c, err := client.NewFabricClient(client.Config{
		Endpoint:             "https://api.fabric.microsoft.com",
		PowerBIEndpoint:      retryServer.URL,
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:          1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed client creation: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, retryServer.URL+"/test-retry", nil)
	resp, err := c.DoPowerBIRequestForTest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on DoPowerBIRequest: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after retry, got %d", resp.StatusCode)
	}
	if retryCount != 1 {
		t.Errorf("expected 1 retry, got %d", retryCount)
	}

	// 3b. doPowerBIRequest retries on network error with request body replay
	networkAttempt := 0
	netClient, _ := client.NewFabricClient(client.Config{
		Endpoint:      "https://api.fabric.microsoft.com",
		TokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:   1 * time.Millisecond,
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				if networkAttempt == 0 {
					networkAttempt++
					return nil, errors.New("simulated network glitch")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	})
	reqNet, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.fabric.microsoft.com/test", strings.NewReader("sample payload"))
	respNet, err := netClient.DoPowerBIRequestForTest(context.Background(), reqNet)
	if err != nil {
		t.Fatalf("unexpected error on network retry: %v", err)
	}
	_ = respNet.Body.Close()
	if respNet.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on network retry, got %d", respNet.StatusCode)
	}

	// 4. DiscoverClusterURI edge cases
	t.Run("DiscoverClusterURI unexpected status", func(t *testing.T) {
		t.Parallel()
		srv200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK) // Not 302
		}))
		defer srv200.Close()

		c200, _ := client.NewFabricClient(client.Config{
			Endpoint:        "https://api.fabric.microsoft.com",
			PowerBIEndpoint: srv200.URL + "/invalid-path", // Path causes fallback validation to fail
			TokenProvider:   func(context.Context) (string, error) { return "tok", nil },
		})
		_, err := c200.DiscoverClusterURI(context.Background())
		if err == nil {
			t.Fatal("expected error on 200 without redirect")
		}
	})

	t.Run("DiscoverClusterURI 302 without cookie", func(t *testing.T) {
		t.Parallel()
		srv302 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "https://app.powerbi.com/landing")
			w.WriteHeader(http.StatusFound)
		}))
		defer srv302.Close()

		c302, _ := client.NewFabricClient(client.Config{
			Endpoint:        "https://api.fabric.microsoft.com",
			PowerBIEndpoint: srv302.URL + "/invalid-path",
			TokenProvider:   func(context.Context) (string, error) { return "tok", nil },
		})
		_, err := c302.DiscoverClusterURI(context.Background())
		if err == nil {
			t.Fatal("expected error on 302 without Set-Cookie")
		}
	})

	// 5. GetItemDirectPermissions edge cases
	t.Run("GetItemDirectPermissions 404", func(t *testing.T) {
		t.Parallel()
		var pbi404 *httptest.Server
		pbi404 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbi404.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer pbi404.Close()

		c404, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbi404.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		})
		_, err := c404.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "p-1")
		if !client.IsNotFound(err) {
			t.Fatalf("expected NotFoundError, got %v", err)
		}
	})

	t.Run("GetItemDirectPermissions 500", func(t *testing.T) {
		t.Parallel()
		var pbi500 *httptest.Server
		pbi500 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbi500.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal error"}`))
		}))
		defer pbi500.Close()

		c500, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbi500.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
			MaxRetries:           0,
		})
		_, err := c500.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "p-1")
		if err == nil {
			t.Fatal("expected error on 500 response")
		}
	})

	t.Run("GetItemDirectPermissions principal type resolution and inheritance filtering", func(t *testing.T) {
		t.Parallel()
		var pbiMock *httptest.Server
		pbiMock = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiMock.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{
				"artifactAccessEntities": [
					{
						"objectId": "item-1",
						"detail": [
							{
								"userObjectId": "user-inherited",
								"permissions": 1,
								"artifactPermissions": 1,
								"accessSource": 1
							},
							{
								"userObjectId": "user-direct",
								"permissions": 1,
								"artifactPermissions": 3,
								"accessSource": null
							},
							{
								"groupObjectId": "group-direct",
								"permissions": 1,
								"artifactPermissions": 15,
								"accessSource": null
							},
							{
								"aadAppId": "sp-direct",
								"permissions": 1,
								"artifactPermissions": 1,
								"isServicePrincipal": true,
								"accessSource": null
							}
						]
					}
				]
			}`)
		}))
		defer pbiMock.Close()

		cMock, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiMock.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		})

		// 1. Inherited row is ignored (returns nil share)
		shareInherited, err := cMock.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "user-inherited")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shareInherited != nil {
			t.Fatalf("expected nil share for inherited row, got %v", shareInherited)
		}

		// 2. User direct share
		shareUser, err := cMock.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "user-direct")
		if err != nil || shareUser == nil {
			t.Fatalf("unexpected error on user direct share: %v", err)
		}
		if shareUser.PrincipalType != "User" {
			t.Errorf("expected PrincipalType User, got %s", shareUser.PrincipalType)
		}

		// 3. Group direct share
		shareGroup, err := cMock.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "group-direct")
		if err != nil || shareGroup == nil {
			t.Fatalf("unexpected error on group direct share: %v", err)
		}
		if shareGroup.PrincipalType != "Group" {
			t.Errorf("expected PrincipalType Group, got %s", shareGroup.PrincipalType)
		}

		// 4. ServicePrincipal direct share
		shareSP, err := cMock.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "sp-direct")
		if err != nil || shareSP == nil {
			t.Fatalf("unexpected error on SP direct share: %v", err)
		}
		if shareSP.PrincipalType != "ServicePrincipal" {
			t.Errorf("expected PrincipalType ServicePrincipal, got %s", shareSP.PrincipalType)
		}
	})

	// 6. RevokeItemDirectPermissions already revoked returns nil
	t.Run("RevokeItemDirectPermissions already revoked returns nil", func(t *testing.T) {
		t.Parallel()
		var pbiEmpty *httptest.Server
		pbiEmpty = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiEmpty.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"artifactAccessEntities":[{"objectId":"item-1","detail":[]}]}`)
		}))
		defer pbiEmpty.Close()

		cEmpty, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiEmpty.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		})
		err := cEmpty.RevokeItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "non-existent-user")
		if err != nil {
			t.Fatalf("expected nil error on revoking non-existent permission, got %v", err)
		}
	})

	// 7. UpsertItemDirectPermissions encode error and SilentPermissionDropError
	t.Run("UpsertItemDirectPermissions encode error", func(t *testing.T) {
		t.Parallel()
		cBad, _ := client.NewFabricClient(client.Config{
			Endpoint:        "https://api.fabric.microsoft.com",
			PowerBIEndpoint: "http://127.0.0.1:8080",
			TokenProvider:   func(context.Context) (string, error) { return "tok", nil },
		})
		err := cBad.UpsertItemDirectPermissions(context.Background(), "ws-1", "item-1", "UnknownWorkload", client.ItemDirectShare{
			PrincipalID:   "p-1",
			PrincipalType: "User",
			Permissions:   []string{"Read"},
		})
		if err == nil {
			t.Fatal("expected error on invalid item type")
		}
	})

	t.Run("UpsertItemDirectPermissions silent drop timeout", func(t *testing.T) {
		t.Parallel()
		var pbiDrop *httptest.Server
		pbiDrop = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiDrop.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"artifactAccessEntities":[{"objectId":"item-1","detail":[]}]}`)
		}))
		defer pbiDrop.Close()

		cDrop, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiDrop.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
			BaseBackoff:          1 * time.Millisecond,
		})

		err := cDrop.UpsertItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", client.ItemDirectShare{
			PrincipalID:   "user-1",
			PrincipalType: "User",
			Permissions:   []string{"Read"},
		})
		if err == nil {
			t.Fatal("expected error on silent drop timeout")
		}
		var dropErr *client.SilentPermissionDropError
		if !errors.As(err, &dropErr) {
			t.Fatalf("expected *SilentPermissionDropError, got %T: %v", err, err)
		}
	})

	t.Run("RevokeItemDirectPermissions verification timeout", func(t *testing.T) {
		t.Parallel()
		var pbiRevokeTimeout *httptest.Server
		pbiRevokeTimeout = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiRevokeTimeout.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"artifactAccessEntities":[{"objectId":"item-1","detail":[{"userObjectId":"user-1","permissions":1,"artifactPermissions":1}]}]}`)
		}))
		defer pbiRevokeTimeout.Close()

		cRevTimeout, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiRevokeTimeout.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
			BaseBackoff:          1 * time.Millisecond,
		})

		err := cRevTimeout.RevokeItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "user-1")
		if err == nil {
			t.Fatal("expected error on revocation verification timeout")
		}
		var dropErr *client.SilentPermissionDropError
		if !errors.As(err, &dropErr) {
			t.Fatalf("expected *SilentPermissionDropError on revoke verification timeout, got %T: %v", err, err)
		}
	})

	// 8. Upsert and Revoke with various principal types and HTTP errors
	t.Run("UpsertItemDirectPermissions and Revoke edge branches", func(t *testing.T) {
		t.Parallel()

		mockSrv, err := pbimock.NewServer()
		if err != nil {
			t.Fatalf("start pbimock: %v", err)
		}
		defer mockSrv.Close()

		var lastAction string
		var lastPrincipal string
		mockSrv.OnMutation = func(action, _, principalID string, _ []string) {
			lastAction = action
			lastPrincipal = principalID
		}

		cMock, _ := client.NewFabricClient(client.Config{
			Endpoint:             mockSrv.URL,
			PowerBIEndpoint:      mockSrv.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
			BaseBackoff:          1 * time.Millisecond,
		})

		wsID := "00000000-0000-0000-0000-000000000001"
		itemID := "00000000-0000-0000-0000-000000000002"

		// A. Grant new user without notify -> POST /metadata/access
		err = cMock.UpsertItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", client.ItemDirectShare{
			PrincipalID:   "00000000-0000-0000-0000-000000000010",
			PrincipalType: "User",
			Permissions:   []string{"Read"},
			Notify:        false,
		})
		if err != nil {
			t.Fatalf("granting new user: %v", err)
		}
		if lastAction != "grant" || lastPrincipal != "00000000-0000-0000-0000-000000000010" {
			t.Errorf("unexpected mutation: %s %s", lastAction, lastPrincipal)
		}

		// B. Update existing user without notify -> PUT /metadata/access
		err = cMock.UpsertItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", client.ItemDirectShare{
			PrincipalID:   "00000000-0000-0000-0000-000000000010",
			PrincipalType: "User",
			Permissions:   []string{"Read", "ReadData"},
			Notify:        false,
		})
		if err != nil {
			t.Fatalf("updating user: %v", err)
		}

		// C. Grant group with notify and message -> POST /metadata/access/notify
		err = cMock.UpsertItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", client.ItemDirectShare{
			PrincipalID:         "00000000-0000-0000-0000-000000000020",
			PrincipalType:       "Group",
			Permissions:         []string{"Read"},
			Notify:              true,
			NotificationMessage: "Welcome team",
		})
		if err != nil {
			t.Fatalf("granting group with notify: %v", err)
		}

		// D. Grant ServicePrincipal
		err = cMock.UpsertItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", client.ItemDirectShare{
			PrincipalID:   "00000000-0000-0000-0000-000000000030",
			PrincipalType: "ServicePrincipal",
			Permissions:   []string{"Read"},
			Notify:        false,
		})
		if err != nil {
			t.Fatalf("granting SP: %v", err)
		}

		// E. Revoke Group
		err = cMock.RevokeItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", "00000000-0000-0000-0000-000000000020")
		if err != nil {
			t.Fatalf("revoking group: %v", err)
		}

		// F. Revoke ServicePrincipal
		err = cMock.RevokeItemDirectPermissions(context.Background(), wsID, itemID, "Warehouse", "00000000-0000-0000-0000-000000000030")
		if err != nil {
			t.Fatalf("revoking SP: %v", err)
		}

		// G. HTTP 500 error on Upsert and Revoke
		var pbiFail *httptest.Server
		pbiFail = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiFail.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"artifactAccessEntities":[{"objectId":"item-fail","detail":[{"userObjectId":"user-fail","permissions":1,"artifactPermissions":1}]}]}`)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"server error"}`)
		}))
		defer pbiFail.Close()

		cFail, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiFail.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
			BaseBackoff:          1 * time.Millisecond,
		})

		err = cFail.UpsertItemDirectPermissions(context.Background(), "ws-1", "item-fail", "Warehouse", client.ItemDirectShare{
			PrincipalID:   "user-fail",
			PrincipalType: "User",
			Permissions:   []string{"Read"},
		})
		if err == nil {
			t.Fatal("expected error on failing endpoint during upsert")
		}

		err = cFail.RevokeItemDirectPermissions(context.Background(), "ws-1", "item-fail", "Warehouse", "user-fail")
		if err == nil {
			t.Fatal("expected error on failing endpoint during revoke")
		}
	})

	// 9. GetItemDirectPermissions unmarshal error
	t.Run("GetItemDirectPermissions unmarshal and decode errors", func(t *testing.T) {
		t.Parallel()

		var pbiBadJSON *httptest.Server
		pbiBadJSON = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Set-Cookie", fmt.Sprintf("ClusterUri=wabi-test=%s/", pbiBadJSON.URL))
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `invalid-json-body`)
		}))
		defer pbiBadJSON.Close()

		cBadJSON, _ := client.NewFabricClient(client.Config{
			Endpoint:             "https://api.fabric.microsoft.com",
			PowerBIEndpoint:      pbiBadJSON.URL,
			TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
			PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		})

		_, err := cBadJSON.GetItemDirectPermissions(context.Background(), "ws-1", "item-1", "Warehouse", "user-1")
		if err == nil {
			t.Fatal("expected unmarshal error on bad json")
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestItemPermissions_DiscoveryAndErrorCoverage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. DiscoverClusterURI redirect to untrusted target
	cUntrusted, _ := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://app.powerbi.com",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{"https://evil.attacker.com/malicious"}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}),
		},
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	_, errUntrusted := cUntrusted.DiscoverClusterURI(ctx)
	if errUntrusted == nil || !strings.Contains(errUntrusted.Error(), "untrusted redirect target") {
		t.Fatalf("expected untrusted redirect target error, got: %v", errUntrusted)
	}

	// 2. DiscoverClusterURI redirect with empty Location header
	cEmptyLoc, _ := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://app.powerbi.com",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}),
		},
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	_, errEmptyLoc := cEmptyLoc.DiscoverClusterURI(ctx)
	if errEmptyLoc == nil {
		t.Fatal("expected error on empty location redirect")
	}

	// 3. DiscoverClusterURI portal without cookie that is not valid cluster URI
	cNoCookie, _ := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://app.powerbi.com",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			}),
		},
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	_, errNoCookie := cNoCookie.DiscoverClusterURI(ctx)
	if errNoCookie == nil || !strings.Contains(errNoCookie.Error(), "no ClusterUri cookie in response") {
		t.Fatalf("expected no ClusterUri cookie error, got: %v", errNoCookie)
	}

	// 3b. DiscoverClusterURI multi-hop redirect with valid ClusterUri cookie
	hopCount := 0
	cRedirectSuccess, _ := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://app.powerbi.com",
		HTTPClient: &http.Client{
			Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
				hopCount++
				if hopCount == 1 {
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{"https://wabi-west-us-redirect.analysis.windows.net/powerbi/metadata"}},
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}
				h := http.Header{}
				h.Add("Set-Cookie", `ClusterUri="https://wabi-west-us-redirect.analysis.windows.net/"`)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     h,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			}),
		},
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
	})
	clusterFound, errRedirectSuccess := cRedirectSuccess.DiscoverClusterURI(ctx)
	if errRedirectSuccess != nil {
		t.Fatalf("unexpected error on multi-hop discovery: %v", errRedirectSuccess)
	}
	if clusterFound != "https://wabi-west-us-redirect.analysis.windows.net" {
		t.Fatalf("expected cluster https://wabi-west-us-redirect.analysis.windows.net, got %q", clusterFound)
	}

	// 4. RevokeItemDirectPermissions token acquisition error
	cTokenErr, _ := client.NewFabricClient(client.Config{
		Endpoint:        "https://api.fabric.microsoft.com",
		PowerBIEndpoint: "https://wabi-west-us-redirect.analysis.windows.net",
		TokenProvider:   func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) {
			return "", errors.New("boom pbi token")
		},
	})
	errToken := cTokenErr.RevokeItemDirectPermissions(ctx, "ws-1", "item-1", "Warehouse", "user-1")
	if errToken == nil || !strings.Contains(errToken.Error(), "boom pbi token") {
		t.Fatalf("expected token error on RevokeItemDirectPermissions, got: %v", errToken)
	}

	// 5. RevokeItemDirectPermissions HTTP 500 error
	pbiRevoke500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/metadata/access") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal revocation error"))
			return
		}
		// artifact access returns user
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"artifactAccessEntities": [{
				"objectId": "item-1",
				"detail": [{
					"userObjectId": "user-1",
					"permissions": 1,
					"artifactPermissions": 0,
					"userId": 42
				}]
			}]
		}`))
	}))
	defer pbiRevoke500.Close()

	cRevoke500, _ := client.NewFabricClient(client.Config{
		Endpoint:             "https://api.fabric.microsoft.com",
		PowerBIEndpoint:      pbiRevoke500.URL,
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:          1 * time.Millisecond,
		MaxRetries:           1,
	})
	errRevoke500 := cRevoke500.RevokeItemDirectPermissions(ctx, "ws-1", "item-1", "Warehouse", "user-1")
	if errRevoke500 == nil || !strings.Contains(errRevoke500.Error(), "powerbi API revocation failed (HTTP 500)") {
		t.Fatalf("expected revocation failed 500, got: %v", errRevoke500)
	}

	// 6. RevokeItemDirectPermissions existing lookup failure (non-404)
	pbiLookup500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("lookup error"))
	}))
	defer pbiLookup500.Close()

	cLookup500, _ := client.NewFabricClient(client.Config{
		Endpoint:             "https://api.fabric.microsoft.com",
		PowerBIEndpoint:      pbiLookup500.URL,
		TokenProvider:        func(context.Context) (string, error) { return "tok", nil },
		PowerBITokenProvider: func(context.Context) (string, error) { return "tok", nil },
		BaseBackoff:          1 * time.Millisecond,
		MaxRetries:           1,
	})
	errLookup500 := cLookup500.RevokeItemDirectPermissions(ctx, "ws-1", "item-1", "Warehouse", "user-1")
	if errLookup500 == nil || !strings.Contains(errLookup500.Error(), "reading permissions for revocation") {
		t.Fatalf("expected reading permissions for revocation error, got: %v", errLookup500)
	}

	// 7. UpsertItemDirectPermissions existing lookup failure (non-404)
	errUpsertLookup500 := cLookup500.UpsertItemDirectPermissions(ctx, "ws-1", "item-1", "Warehouse", client.ItemDirectShare{
		PrincipalID:   "user-1",
		PrincipalType: "User",
		Permissions:   []string{"Read"},
	})
	if errUpsertLookup500 == nil || !strings.Contains(errUpsertLookup500.Error(), "checking existing item permissions") {
		t.Fatalf("expected checking existing item permissions error on upsert, got: %v", errUpsertLookup500)
	}
}
