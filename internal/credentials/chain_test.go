// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package credentials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

func TestStaticProvider_Resolution(t *testing.T) {
	t.Parallel()

	t.Run("explicit access_token attribute takes precedence over env", func(t *testing.T) {
		t.Parallel()
		chain := NewChain(Config{
			AccessToken: "explicit-token-123",
			Getenv: func(k string) string {
				if k == "FABRIC_ACCESS_TOKEN" {
					return "env-token-456"
				}
				return ""
			},
		})
		creds, err := chain.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.AccessToken != "explicit-token-123" {
			t.Fatalf("expected explicit-token-123, got %q", creds.AccessToken)
		}
		if creds.Source != SourceStatic {
			t.Fatalf("expected source %q, got %q", SourceStatic, creds.Source)
		}
	})

	t.Run("falls back to FABRIC_ACCESS_TOKEN env var", func(t *testing.T) {
		t.Parallel()
		chain := NewChain(Config{
			Getenv: func(k string) string {
				if k == "FABRIC_ACCESS_TOKEN" {
					return "env-token-456"
				}
				return ""
			},
		})
		creds, err := chain.Resolve(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if creds.AccessToken != "env-token-456" {
			t.Fatalf("expected env-token-456, got %q", creds.AccessToken)
		}
	})
}

func TestOIDCThreeTupleFallthrough(t *testing.T) {
	t.Parallel()

	// Simulate GitHub Actions azure/login OIDC where AZURE_TENANT_ID and AZURE_CLIENT_ID
	// are exported without AZURE_CLIENT_SECRET. ClientSecretProvider MUST skip cleanly
	// and fall through to WorkloadIdentity / AzureCLI.
	env := map[string]string{
		"AZURE_TENANT_ID": "00000000-0000-0000-0000-000000000001",
		"AZURE_CLIENT_ID": "00000000-0000-0000-0000-000000000002",
	}
	cfg := Config{
		Getenv: func(k string) string { return env[k] },
	}

	cs := &clientSecretProvider{cfg: cfg}
	wi := &workloadIdentityProvider{cfg: cfg}
	cli := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, tenantID string) (azcore.AccessToken, error) {
			if tenantID != "00000000-0000-0000-0000-000000000001" {
				return azcore.AccessToken{}, fmt.Errorf("unexpected tenantID: %s", tenantID)
			}
			return azcore.AccessToken{
				Token:     "cli-oidc-token-789",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	chain := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, cs, wi, cli))
	creds, err := chain.Resolve(context.Background())
	if err != nil {
		t.Fatalf("expected clean fallthrough to AzureCLIProvider, got error: %v", err)
	}
	if creds.Source != SourceAzureCLI {
		t.Fatalf("expected source %q, got %q", SourceAzureCLI, creds.Source)
	}
	if creds.AccessToken != "cli-oidc-token-789" {
		t.Fatalf("expected token cli-oidc-token-789, got %q", creds.AccessToken)
	}
}

func TestChainPrecedenceOrder(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"AZURE_TENANT_ID":            "00000000-0000-0000-0000-000000000001",
		"AZURE_CLIENT_ID":            "00000000-0000-0000-0000-000000000002",
		"AZURE_CLIENT_SECRET":        "super-secret",
		"AZURE_FEDERATED_TOKEN_FILE": "/tmp/oidc-token",
		"AZURE_USE_MSI":              "true",
	}
	cfg := Config{
		Getenv: func(k string) string { return env[k] },
	}

	cs := &clientSecretProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "spn-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}
	wi := &workloadIdentityProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "oidc-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}
	msi := &managedIdentityProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "msi-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}
	cli := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "cli-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}

	chain := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, cs, wi, msi, cli))
	creds, err := chain.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Source != SourceClientSecret || creds.AccessToken != "spn-token" {
		t.Fatalf("expected ClientSecret to win over WorkloadIdentity, ManagedIdentity, and AzureCLI, got %+v", creds)
	}

	// Verify WorkloadIdentity wins over ManagedIdentity and AzureCLI when ClientSecret is absent.
	delete(env, "AZURE_CLIENT_SECRET")
	chainWI := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, cs, wi, msi, cli))
	credsWI, err := chainWI.Resolve(context.Background())
	if err != nil || credsWI.Source != SourceWorkloadIdentity {
		t.Fatalf("expected WorkloadIdentity to win over ManagedIdentity, got %+v (err=%v)", credsWI, err)
	}

	// Verify ManagedIdentity wins over AzureCLI when WorkloadIdentity is absent.
	delete(env, "AZURE_FEDERATED_TOKEN_FILE")
	chainMSI := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, cs, wi, msi, cli))
	credsMSI, err := chainMSI.Resolve(context.Background())
	if err != nil || credsMSI.Source != SourceManagedIdentity {
		t.Fatalf("expected ManagedIdentity to win over AzureCLI, got %+v (err=%v)", credsMSI, err)
	}

	// Verify UseCLI = false skips AzureCLIProvider.
	delete(env, "AZURE_USE_MSI")
	useCLIFalse := false
	cfgNoCLI := Config{
		UseCLI: &useCLIFalse,
		Getenv: func(k string) string { return env[k] },
	}
	cliDisabled := &azureCLIProvider{cfg: cfgNoCLI, tokenFetcher: cli.tokenFetcher}
	chainDisabled := NewChain(cfgNoCLI, WithProviders(cliDisabled))
	if _, err := chainDisabled.Resolve(context.Background()); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("expected ErrNoCredentials when UseCLI is false, got %v", err)
	}
}

func TestErrNoCredentials_ListsAllSources(t *testing.T) {
	t.Parallel()

	cfg := Config{Getenv: func(string) string { return "" }}
	cli := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("az login session expired")
		},
	}

	chain := NewChain(cfg, WithProviders(
		&staticProvider{cfg: cfg},
		&clientSecretProvider{cfg: cfg},
		&workloadIdentityProvider{cfg: cfg},
		&managedIdentityProvider{cfg: cfg},
		cli,
	))

	_, err := chain.Resolve(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("expected errors.Is(err, ErrNoCredentials), got: %v", err)
	}
	msg := err.Error()
	for _, expectedSource := range []string{
		SourceStatic,
		SourceClientSecret,
		SourceWorkloadIdentity,
		SourceManagedIdentity,
		SourceAzureCLI,
	} {
		if !strings.Contains(msg, expectedSource) {
			t.Errorf("expected error message to mention source %q, got: %s", expectedSource, msg)
		}
	}
}

func TestCredentialsRedaction(t *testing.T) {
	t.Parallel()

	secret := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.super-secret-bearer-token"
	creds := Credentials{
		AccessToken: secret,
		ExpiresOn:   time.Date(2026, 9, 27, 22, 0, 0, 0, time.UTC),
		Source:      SourceStatic,
	}

	for _, formatted := range []string{
		creds.String(),
		creds.GoString(),
		fmt.Sprintf("%v", creds),
		fmt.Sprintf("%+v", creds),
		fmt.Sprintf("%#v", creds),
	} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("secret bearer token leaked in formatted output: %s", formatted)
		}
		if !strings.Contains(formatted, "[REDACTED]") {
			t.Fatalf("expected [REDACTED] in formatted output: %s", formatted)
		}
	}
}

func TestTokenCachingAndNearExpiryRefresh(t *testing.T) {
	t.Parallel()

	currentTime := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	nowFunc := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return currentTime
	}

	var calls atomic.Int32
	cfg := Config{Getenv: func(string) string { return "" }}
	cli := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			n := calls.Add(1)
			return azcore.AccessToken{
				Token:     fmt.Sprintf("token-v%d", n),
				ExpiresOn: nowFunc().Add(30 * time.Minute),
			}, nil
		},
	}

	chain := NewChain(cfg, WithProviders(cli), WithClock(nowFunc))

	// Run 10 concurrent GetToken calls — should only invoke tokenFetcher once.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{FabricScope}})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tok.Token != "token-v1" {
				t.Errorf("expected token-v1, got %s", tok.Token)
			}
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 underlying token fetch, got %d", got)
	}

	// Advance clock to within 90 seconds of expiry (< 2m refreshWindow) and verify refresh.
	mu.Lock()
	currentTime = currentTime.Add(29 * time.Minute)
	mu.Unlock()

	creds, err := chain.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on refresh: %v", err)
	}
	if creds.AccessToken != "token-v2" {
		t.Fatalf("expected refreshed token-v2, got %q", creds.AccessToken)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 underlying token fetches after expiry advance, got %d", got)
	}
}
