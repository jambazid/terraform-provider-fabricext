// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package credentials

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
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

	certProv := &clientCertificateProvider{
		cfg: cfg,
	}
	cs := &clientSecretProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "spn-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}
	ado := &azureDevOpsOIDCProvider{
		cfg: cfg,
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
	azd := &azureDeveloperCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "azd-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}
	cli := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{Token: "cli-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}

	chain := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, certProv, cs, ado, wi, msi, azd, cli))
	creds, err := chain.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Source != SourceClientSecret || creds.AccessToken != "spn-token" {
		t.Fatalf("expected ClientSecret to win over WorkloadIdentity, ManagedIdentity, and AzureCLI, got %+v", creds)
	}

	// Verify WorkloadIdentity wins over ManagedIdentity and AzureCLI when ClientSecret is absent.
	delete(env, "AZURE_CLIENT_SECRET")
	chainWI := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, certProv, cs, ado, wi, msi, azd, cli))
	credsWI, err := chainWI.Resolve(context.Background())
	if err != nil || credsWI.Source != SourceWorkloadIdentity {
		t.Fatalf("expected WorkloadIdentity to win over ManagedIdentity, got %+v (err=%v)", credsWI, err)
	}

	// Verify ManagedIdentity wins over AzureCLI when WorkloadIdentity is absent.
	delete(env, "AZURE_FEDERATED_TOKEN_FILE")
	chainMSI := NewChain(cfg, WithProviders(&staticProvider{cfg: cfg}, certProv, cs, ado, wi, msi, azd, cli))
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

func createTestPKCS12Bundle(t *testing.T, password string) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-cert",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("creating self-signed cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parsing cert: %v", err)
	}

	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, password)
	if err != nil {
		t.Fatalf("encoding PKCS#12 bundle: %v", err)
	}
	return pfxData
}

func TestClientCertificateProvider_Resolution(t *testing.T) {
	t.Parallel()

	password := "SecretPass123!"
	pfxData := createTestPKCS12Bundle(t, password)
	b64Data := base64.StdEncoding.EncodeToString(pfxData)

	t.Run("resolves base64 client_certificate", func(t *testing.T) {
		t.Parallel()
		var certsPassed []*x509.Certificate
		prov := &clientCertificateProvider{
			cfg: Config{
				TenantID:                  "00000000-0000-0000-0000-000000000001",
				ClientID:                  "00000000-0000-0000-0000-000000000002",
				ClientCertificate:         b64Data,
				ClientCertificatePassword: password,
			},
			tokenFetcher: func(_ context.Context, _, _ string, certs []*x509.Certificate, _ crypto.PrivateKey) (azcore.AccessToken, error) {
				certsPassed = certs
				return azcore.AccessToken{Token: "cert-token-123", ExpiresOn: time.Now().Add(time.Hour)}, nil
			},
		}

		creds, skipped, err := prov.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("expected resolution, got err=%v skipped=%v", err, skipped)
		}
		if creds.AccessToken != "cert-token-123" {
			t.Fatalf("unexpected token: %s", creds.AccessToken)
		}
		if len(certsPassed) != 1 || certsPassed[0].Subject.CommonName != "test-cert" {
			t.Fatalf("unexpected cert common name: %v", certsPassed)
		}
	})

	t.Run("resolves client_certificate_file_path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		pfxPath := filepath.Join(tmpDir, "cert.pfx")
		if err := os.WriteFile(pfxPath, pfxData, 0o600); err != nil {
			t.Fatalf("writing cert file: %v", err)
		}

		prov := &clientCertificateProvider{
			cfg: Config{
				TenantID:                  "00000000-0000-0000-0000-000000000001",
				ClientID:                  "00000000-0000-0000-0000-000000000002",
				ClientCertificateFilePath: pfxPath,
				ClientCertificatePassword: password,
			},
			tokenFetcher: func(_ context.Context, _, _ string, _ []*x509.Certificate, _ crypto.PrivateKey) (azcore.AccessToken, error) {
				return azcore.AccessToken{Token: "file-cert-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
			},
		}

		creds, skipped, err := prov.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("expected resolution, got err=%v skipped=%v", err, skipped)
		}
		if creds.AccessToken != "file-cert-token" {
			t.Fatalf("unexpected token: %s", creds.AccessToken)
		}
	})
}

func TestAzureDeveloperCLIProvider_Resolution(t *testing.T) {
	t.Parallel()

	t.Run("skipped when use_dev_cli is not enabled", func(t *testing.T) {
		t.Parallel()
		prov := &azureDeveloperCLIProvider{
			cfg: Config{Getenv: func(string) string { return "" }},
		}
		_, skipped, err := prov.Resolve(context.Background())
		if !skipped || err == nil {
			t.Fatalf("expected skipped=true, got skipped=%v err=%v", skipped, err)
		}
	})

	t.Run("resolves when use_dev_cli is true", func(t *testing.T) {
		t.Parallel()
		useDevCLI := true
		prov := &azureDeveloperCLIProvider{
			cfg: Config{
				TenantID:  "00000000-0000-0000-0000-000000000001",
				UseDevCLI: &useDevCLI,
			},
			tokenFetcher: func(_ context.Context, tenantID string) (azcore.AccessToken, error) {
				if tenantID != "00000000-0000-0000-0000-000000000001" {
					return azcore.AccessToken{}, fmt.Errorf("unexpected tenant: %s", tenantID)
				}
				return azcore.AccessToken{Token: "azd-test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
			},
		}
		creds, skipped, err := prov.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("expected resolution, got err=%v skipped=%v", err, skipped)
		}
		if creds.AccessToken != "azd-test-token" || creds.Source != SourceAzureDeveloperCLI {
			t.Fatalf("unexpected creds: %+v", creds)
		}
	})
}

func TestAzureDevOpsOIDCProvider_Resolution(t *testing.T) {
	t.Parallel()

	t.Run("skipped when azure_devops_service_connection_id is missing", func(t *testing.T) {
		t.Parallel()
		prov := &azureDevOpsOIDCProvider{
			cfg: Config{Getenv: func(string) string { return "" }},
		}
		_, skipped, err := prov.Resolve(context.Background())
		if !skipped || err == nil {
			t.Fatalf("expected skipped=true, got skipped=%v err=%v", skipped, err)
		}
	})

	t.Run("resolves with service connection and system token", func(t *testing.T) {
		t.Parallel()
		prov := &azureDevOpsOIDCProvider{
			cfg: Config{
				TenantID:                       "00000000-0000-0000-0000-000000000001",
				ClientID:                       "00000000-0000-0000-0000-000000000002",
				AzureDevOpsServiceConnectionID: "conn-12345",
				OIDCRequestToken:               "sys-token-abc",
			},
			tokenFetcher: func(_ context.Context, _, _, connID, sysToken string) (azcore.AccessToken, error) {
				if connID != "conn-12345" || sysToken != "sys-token-abc" {
					return azcore.AccessToken{}, fmt.Errorf("unexpected args: conn=%s sys=%s", connID, sysToken)
				}
				return azcore.AccessToken{Token: "ado-token-xyz", ExpiresOn: time.Now().Add(time.Hour)}, nil
			},
		}

		creds, skipped, err := prov.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("expected resolution, got err=%v skipped=%v", err, skipped)
		}
		if creds.AccessToken != "ado-token-xyz" || creds.Source != SourceAzureDevOpsOIDC {
			t.Fatalf("unexpected creds: %+v", creds)
		}
	})
}

func TestFilePathCredentials_Resolution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tenantFile := filepath.Join(tmpDir, "tenant.txt")
	clientFile := filepath.Join(tmpDir, "client.txt")
	secretFile := filepath.Join(tmpDir, "secret.txt")

	if err := os.WriteFile(tenantFile, []byte("file-tenant-id\n"), 0o600); err != nil {
		t.Fatalf("writing tenant file: %v", err)
	}
	if err := os.WriteFile(clientFile, []byte("file-client-id\n"), 0o600); err != nil {
		t.Fatalf("writing client file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("file-client-secret\n"), 0o600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}

	cfg := Config{
		TenantIDFilePath:     tenantFile,
		ClientIDFilePath:     clientFile,
		ClientSecretFilePath: secretFile,
	}

	prov := &clientSecretProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, tenantID, clientID, clientSecret string) (azcore.AccessToken, error) {
			if tenantID != "file-tenant-id" || clientID != "file-client-id" || clientSecret != "file-client-secret" {
				return azcore.AccessToken{}, fmt.Errorf("unexpected file content: %s, %s, %s", tenantID, clientID, clientSecret)
			}
			return azcore.AccessToken{Token: "file-secret-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
		},
	}

	creds, skipped, err := prov.Resolve(context.Background())
	if err != nil || skipped {
		t.Fatalf("expected resolution, got err=%v skipped=%v", err, skipped)
	}
	if creds.AccessToken != "file-secret-token" {
		t.Fatalf("unexpected token: %s", creds.AccessToken)
	}
}

func TestEnvironmentAndScopeResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env           string
		expectedScope string
	}{
		{"public", FabricScopePublic},
		{"", FabricScopePublic},
		{"usgovernment", FabricScopeUSGovernment},
		{"china", FabricScopeChina},
		{"CHINA", FabricScopeChina},
	}

	for _, tt := range tests {
		scope := ResolveFabricScope(tt.env)
		if scope != tt.expectedScope {
			t.Errorf("ResolveFabricScope(%q) = %q; expected %q", tt.env, scope, tt.expectedScope)
		}
	}
}

func TestAuxiliaryTenantsPropagation(t *testing.T) {
	t.Parallel()

	cfg := Config{
		AuxiliaryTenantIDs: []string{"tenant-a", "tenant-b"},
	}
	tenants := cfg.resolveAuxiliaryTenants()
	if len(tenants) != 2 || tenants[0] != "tenant-a" || tenants[1] != "tenant-b" {
		t.Fatalf("unexpected tenants: %v", tenants)
	}

	cfgEnv := Config{
		Getenv: func(k string) string {
			if k == "FABRIC_AUXILIARY_TENANT_IDS" {
				return "tenant-1, tenant-2 "
			}
			return ""
		},
	}
	envTenants := cfgEnv.resolveAuxiliaryTenants()
	if len(envTenants) != 2 || envTenants[0] != "tenant-1" || envTenants[1] != "tenant-2" {
		t.Fatalf("unexpected env tenants: %v", envTenants)
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
		&clientCertificateProvider{cfg: cfg},
		&clientSecretProvider{cfg: cfg},
		&azureDevOpsOIDCProvider{cfg: cfg},
		&workloadIdentityProvider{cfg: cfg},
		&managedIdentityProvider{cfg: cfg},
		&azureDeveloperCLIProvider{cfg: cfg},
		cli,
	))

	_, err := chain.Resolve(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("expected errors.Is(err, ErrNoCredentials), got: %v", err)
	}
	msg := err.Error()
	for _, expectedSource := range []string{
		SourceStatic,
		SourceClientCertificate,
		SourceClientSecret,
		SourceAzureDevOpsOIDC,
		SourceWorkloadIdentity,
		SourceManagedIdentity,
		SourceAzureDeveloperCLI,
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

	cfg := Config{
		ClientSecret:      "raw-client-secret",
		ClientCertificate: "raw-certificate-base64",
	}
	if strings.Contains(cfg.String(), "raw-client-secret") || strings.Contains(cfg.GoString(), "raw-client-secret") {
		t.Fatalf("Config leaked secret in String/GoString: %s", cfg.String())
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
