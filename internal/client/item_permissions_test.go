// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
