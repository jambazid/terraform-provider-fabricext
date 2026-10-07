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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestConfig_ResolversAndFileHelpers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tenantFile := filepath.Join(tmpDir, "tenant.txt")
	clientFile := filepath.Join(tmpDir, "client.txt")
	secretFile := filepath.Join(tmpDir, "secret.txt")

	if err := os.WriteFile(tenantFile, []byte("tenant-from-file\n"), 0o600); err != nil {
		t.Fatalf("write tenant file: %v", err)
	}
	if err := os.WriteFile(clientFile, []byte("client-from-file\n"), 0o600); err != nil {
		t.Fatalf("write client file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("secret-from-file\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	t.Run("ResolveTenantID direct, file, and env fallbacks", func(t *testing.T) {
		t.Parallel()
		cfgDirect := Config{TenantID: "direct-tenant"}
		val, err := cfgDirect.ResolveTenantID()
		if err != nil || val != "direct-tenant" {
			t.Fatalf("expected direct-tenant, got %q (err: %v)", val, err)
		}

		cfgFile := Config{TenantIDFilePath: tenantFile}
		val, err = cfgFile.ResolveTenantID()
		if err != nil || val != "tenant-from-file" {
			t.Fatalf("expected tenant-from-file, got %q (err: %v)", val, err)
		}

		cfgBadFile := Config{TenantIDFilePath: filepath.Join(tmpDir, "nonexistent.txt")}
		if _, err := cfgBadFile.ResolveTenantID(); err == nil {
			t.Fatal("expected error reading nonexistent tenant file")
		}

		cfgEnv := Config{
			Getenv: func(k string) string {
				if k == "ARM_TENANT_ID" {
					return "arm-tenant-id"
				}
				return ""
			},
		}
		val, err = cfgEnv.ResolveTenantID()
		if err != nil || val != "arm-tenant-id" {
			t.Fatalf("expected arm-tenant-id, got %q", val)
		}
	})

	t.Run("resolveClientID direct, file, and env fallbacks", func(t *testing.T) {
		t.Parallel()
		cfgDirect := Config{ClientID: "direct-client"}
		val, err := cfgDirect.resolveClientID()
		if err != nil || val != "direct-client" {
			t.Fatalf("expected direct-client, got %q", val)
		}

		cfgFile := Config{ClientIDFilePath: clientFile}
		val, err = cfgFile.resolveClientID()
		if err != nil || val != "client-from-file" {
			t.Fatalf("expected client-from-file, got %q", val)
		}

		cfgEnv := Config{
			Getenv: func(k string) string {
				if k == "AZURE_CLIENT_ID" {
					return "azure-client-id"
				}
				return ""
			},
		}
		val, err = cfgEnv.resolveClientID()
		if err != nil || val != "azure-client-id" {
			t.Fatalf("expected azure-client-id, got %q", val)
		}
	})

	t.Run("resolveClientSecret direct, file, and env fallbacks", func(t *testing.T) {
		t.Parallel()
		cfgDirect := Config{ClientSecret: "direct-secret"}
		val, err := cfgDirect.resolveClientSecret()
		if err != nil || val != "direct-secret" {
			t.Fatalf("expected direct-secret, got %q", val)
		}

		cfgFile := Config{ClientSecretFilePath: secretFile}
		val, err = cfgFile.resolveClientSecret()
		if err != nil || val != "secret-from-file" {
			t.Fatalf("expected secret-from-file, got %q", val)
		}

		cfgEnv := Config{
			Getenv: func(k string) string {
				if k == "FABRIC_CLIENT_SECRET" {
					return "fabric-client-secret"
				}
				return ""
			},
		}
		val, err = cfgEnv.resolveClientSecret()
		if err != nil || val != "fabric-client-secret" {
			t.Fatalf("expected fabric-client-secret, got %q", val)
		}
	})

	t.Run("resolveEnvironment and resolveAuxiliaryTenants", func(t *testing.T) {
		t.Parallel()
		cfgEnvExplicit := Config{Environment: "usgovernment"}
		if got := cfgEnvExplicit.resolveEnvironment(); got != "usgovernment" {
			t.Fatalf("expected usgovernment, got %q", got)
		}

		cfgEnvFallback := Config{
			Getenv: func(k string) string {
				if k == "ARM_ENVIRONMENT" {
					return "china"
				}
				return ""
			},
		}
		if got := cfgEnvFallback.resolveEnvironment(); got != "china" {
			t.Fatalf("expected china, got %q", got)
		}

		cfgEnvDefault := Config{Getenv: func(string) string { return "" }}
		if got := cfgEnvDefault.resolveEnvironment(); got != "public" {
			t.Fatalf("expected default public, got %q", got)
		}

		cfgAuxDirect := Config{AuxiliaryTenantIDs: []string{"aux-1", "aux-2"}}
		if got := cfgAuxDirect.resolveAuxiliaryTenants(); len(got) != 2 || got[0] != "aux-1" {
			t.Fatalf("expected aux-1, aux-2, got %v", got)
		}

		cfgAuxEnv := Config{
			Getenv: func(k string) string {
				if k == "FABRIC_AUXILIARY_TENANT_IDS" {
					return " aux-a , aux-b , "
				}
				return ""
			},
		}
		got := cfgAuxEnv.resolveAuxiliaryTenants()
		if len(got) != 2 || got[0] != "aux-a" || got[1] != "aux-b" {
			t.Fatalf("expected trimmed aux-a, aux-b, got %v", got)
		}
	})
}

func TestConfig_ValidateConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       Config
		wantError bool
	}{
		{
			name:      "empty config is valid (allows cli/default)",
			cfg:       Config{Getenv: func(string) string { return "" }},
			wantError: false,
		},
		{
			name: "client secret without client id",
			cfg: Config{
				ClientSecret: "secret-123",
				Getenv:       func(string) string { return "" },
			},
			wantError: true,
		},
		{
			name: "client certificate without client id",
			cfg: Config{
				ClientCertificateFilePath: "cert.pfx",
				Getenv:                    func(string) string { return "" },
			},
			wantError: true,
		},
		{
			name: "client secret with valid tenant and client id",
			cfg: Config{
				ClientID:     "client-1",
				TenantID:     "tenant-1",
				ClientSecret: "secret-1",
				Getenv:       func(string) string { return "" },
			},
			wantError: false,
		},
		{
			name: "client secret with invalid tenant id file path",
			cfg: Config{
				ClientSecret:     "secret-1",
				TenantIDFilePath: "/nonexistent/tenant/path.txt",
				Getenv:           func(string) string { return "" },
			},
			wantError: true,
		},
		{
			name: "client secret with invalid client id file path",
			cfg: Config{
				ClientSecret:     "secret-1",
				TenantID:         "tenant-1",
				ClientIDFilePath: "/nonexistent/client/path.txt",
				Getenv:           func(string) string { return "" },
			},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateConfig(tc.cfg)
			if (err != nil) != tc.wantError {
				t.Fatalf("expected error: %v, got %v", tc.wantError, err)
			}
		})
	}
}

func TestWorkloadIdentity_OIDCAssertionAndFile(t *testing.T) {
	t.Parallel()

	t.Run("fetchOIDCAssertion from HTTP mock server", func(t *testing.T) {
		t.Parallel()

		var requestBearer string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestBearer = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": "mocked-github-actions-jwt",
			})
		}))
		defer server.Close()

		token, err := fetchOIDCAssertion(context.Background(), server.URL, "actions-token-abc")
		if err != nil {
			t.Fatalf("fetchOIDCAssertion failed: %v", err)
		}
		if token != "mocked-github-actions-jwt" {
			t.Fatalf("expected mocked-github-actions-jwt, got %q", token)
		}
		if requestBearer != "Bearer actions-token-abc" {
			t.Fatalf("expected Authorization header Bearer actions-token-abc, got %q", requestBearer)
		}
	})

	t.Run("fetchOIDCAssertion HTTP error response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		}))
		defer server.Close()

		if _, err := fetchOIDCAssertion(context.Background(), server.URL, "tok"); err == nil {
			t.Fatal("expected error on HTTP 500 response")
		}
	})

	t.Run("fetchOIDCAssertion empty token value", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": "",
			})
		}))
		defer server.Close()

		if _, err := fetchOIDCAssertion(context.Background(), server.URL, "tok"); err == nil {
			t.Fatal("expected error on empty token value")
		}
	})

	t.Run("fetchOIDCAssertion unmarshal error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not-json"))
		}))
		defer server.Close()

		if _, err := fetchOIDCAssertion(context.Background(), server.URL, "tok"); err == nil {
			t.Fatal("expected error on unmarshal error")
		}
	})

	t.Run("workload identity from assertion file on disk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tokenPath := filepath.Join(tmpDir, "oidc-token.jwt")
		if err := os.WriteFile(tokenPath, []byte("disk-jwt-token-xyz"), 0o600); err != nil {
			t.Fatalf("write token file: %v", err)
		}

		cfg := Config{
			ClientID:          "00000000-0000-0000-0000-000000000002",
			TenantID:          "00000000-0000-0000-0000-000000000001",
			OIDCTokenFilePath: tokenPath,
			Getenv:            func(string) string { return "" },
		}

		p := &workloadIdentityProvider{
			cfg: cfg,
			tokenFetcher: func(context.Context, string, string, string) (azcore.AccessToken, error) {
				return azcore.AccessToken{
					Token:     "mocked-workload-token",
					ExpiresOn: time.Now().Add(1 * time.Hour),
				}, nil
			},
		}
		creds, skipped, err := p.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("expected workloadIdentityProvider to resolve (skipped: %v, err: %v)", skipped, err)
		}
		if creds.AccessToken != "mocked-workload-token" {
			t.Fatalf("expected mocked-workload-token, got %q", creds.AccessToken)
		}
	})

	t.Run("workload identity missing tenant or client returns skipped", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			OIDCToken: "some-token",
			Getenv:    func(string) string { return "" },
		}

		p := &workloadIdentityProvider{cfg: cfg}
		_, skipped, err := p.Resolve(context.Background())
		if !skipped || err == nil {
			t.Fatal("expected skipped true and error when tenant/client missing")
		}
	})
}

func TestClientCertificate_ParsingAndErrors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1001),
		Subject:      pkix.Name{CommonName: "fabric-test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	// 1. Valid PKCS#12 (.pfx)
	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, "pfx-secret")
	if err != nil {
		t.Fatalf("encode pfx: %v", err)
	}
	pfxPath := filepath.Join(tmpDir, "cert.pfx")
	if err := os.WriteFile(pfxPath, pfxData, 0o600); err != nil {
		t.Fatalf("write pfx: %v", err)
	}

	// 2. Valid PEM without password
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	pemPriv := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pemData := make([]byte, 0, len(pemCert)+len(pemPriv))
	pemData = append(pemData, pemCert...)
	pemData = append(pemData, pemPriv...)
	pemPath := filepath.Join(tmpDir, "cert.pem")
	if err := os.WriteFile(pemPath, pemData, 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}

	t.Run("valid PKCS#12 certificate resolves matched", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:                  "00000000-0000-0000-0000-000000000002",
			TenantID:                  "00000000-0000-0000-0000-000000000001",
			ClientCertificateFilePath: pfxPath,
			ClientCertificatePassword: "pfx-secret",
			Getenv:                    func(string) string { return "" },
		}
		p := &clientCertificateProvider{
			cfg: cfg,
			tokenFetcher: func(context.Context, string, string, []*x509.Certificate, crypto.PrivateKey) (azcore.AccessToken, error) {
				return azcore.AccessToken{
					Token:     "mocked-pfx-cert-token",
					ExpiresOn: time.Now().Add(1 * time.Hour),
				}, nil
			},
		}
		creds, skipped, err := p.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("unexpected error resolving pfx: %v (skipped: %v)", err, skipped)
		}
		if creds.AccessToken != "mocked-pfx-cert-token" {
			t.Fatalf("expected mocked-pfx-cert-token, got %q", creds.AccessToken)
		}
	})

	t.Run("valid PKCS#12 certificate with live fallback Transport", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:                  "00000000-0000-0000-0000-000000000002",
			TenantID:                  "00000000-0000-0000-0000-000000000001",
			ClientCertificateFilePath: pfxPath,
			ClientCertificatePassword: "pfx-secret",
			Getenv:                    func(string) string { return "" },
			Transport:                 &mockOAuthTransport{token: "mock-cert-live-token"},
		}
		p := &clientCertificateProvider{
			cfg: cfg,
		}
		creds, skipped, err := p.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("unexpected error resolving pfx live: %v (skipped: %v)", err, skipped)
		}
		if creds.AccessToken != "mock-cert-live-token" {
			t.Fatalf("expected mock-cert-live-token, got %q", creds.AccessToken)
		}
	})

	t.Run("non-PKCS12 PEM certificate returns decode error", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:                  "00000000-0000-0000-0000-000000000002",
			TenantID:                  "00000000-0000-0000-0000-000000000001",
			ClientCertificateFilePath: pemPath,
			Getenv:                    func(string) string { return "" },
		}
		p := &clientCertificateProvider{
			cfg: cfg,
		}
		_, _, err := p.Resolve(context.Background())
		if err == nil {
			t.Fatalf("expected error decoding non-PKCS12 PEM, got nil")
		}
	})

	t.Run("nonexistent cert path returns error", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:                  "00000000-0000-0000-0000-000000000002",
			TenantID:                  "00000000-0000-0000-0000-000000000001",
			ClientCertificateFilePath: "/nonexistent/path/cert.pfx",
			Getenv:                    func(string) string { return "" },
		}
		p := &clientCertificateProvider{cfg: cfg}
		_, _, err := p.Resolve(context.Background())
		if err == nil {
			t.Fatal("expected error on nonexistent cert path")
		}
	})

	t.Run("wrong password on pfx returns error", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:                  "00000000-0000-0000-0000-000000000002",
			TenantID:                  "00000000-0000-0000-0000-000000000001",
			ClientCertificateFilePath: pfxPath,
			ClientCertificatePassword: "wrong-password",
			Getenv:                    func(string) string { return "" },
		}
		p := &clientCertificateProvider{cfg: cfg}
		_, _, err := p.Resolve(context.Background())
		if err == nil {
			t.Fatal("expected error for wrong pfx password")
		}
	})

	t.Run("invalid base64 returns decode error", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientID:          "00000000-0000-0000-0000-000000000002",
			TenantID:          "00000000-0000-0000-0000-000000000001",
			ClientCertificate: "!!!not-valid-base64!!!",
			Getenv:            func(string) string { return "" },
		}
		p := &clientCertificateProvider{cfg: cfg}
		_, _, err := p.Resolve(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid base64 client certificate")
		}
	})

	t.Run("missing client_id returns skipped", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			ClientCertificateFilePath: pfxPath,
			Getenv:                    func(string) string { return "" },
		}
		p := &clientCertificateProvider{cfg: cfg}
		_, skipped, err := p.Resolve(context.Background())
		if !skipped || err == nil {
			t.Fatalf("expected skipped=true and error when client_id is missing, got skipped=%v, err=%v", skipped, err)
		}
	})
}

func TestManagedIdentity_OptionsAndFallback(t *testing.T) {
	t.Parallel()

	t.Run("use_msi false skips provider", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			UseMSI: false,
			Getenv: func(string) string { return "" },
		}
		p := &managedIdentityProvider{cfg: cfg}
		_, skipped, err := p.Resolve(context.Background())
		if err == nil || !skipped {
			t.Fatal("expected skipped true and error when use_msi is false")
		}
	})

	t.Run("use_msi true builds managed identity credential with tokenFetcher", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			UseMSI:   true,
			ClientID: "00000000-0000-0000-0000-000000000002",
			Getenv:   func(string) string { return "" },
		}
		p := &managedIdentityProvider{
			cfg: cfg,
			tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
				return azcore.AccessToken{
					Token:     "mocked-msi-token",
					ExpiresOn: time.Now().Add(1 * time.Hour),
				}, nil
			},
		}
		creds, skipped, err := p.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("unexpected error resolving MSI: %v (skipped: %v)", err, skipped)
		}
		if creds.AccessToken != "mocked-msi-token" {
			t.Fatalf("expected mocked-msi-token, got %q", creds.AccessToken)
		}
	})

	t.Run("IDENTITY_ENDPOINT env var enables managed identity", func(t *testing.T) {
		t.Parallel()
		cfg := Config{
			Getenv: func(k string) string {
				if k == "IDENTITY_ENDPOINT" {
					return "http://127.0.0.1:40381/token"
				}
				return ""
			},
		}
		p := &managedIdentityProvider{
			cfg: cfg,
			tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
				return azcore.AccessToken{
					Token:     "mocked-msi-env-token",
					ExpiresOn: time.Now().Add(1 * time.Hour),
				}, nil
			},
		}
		creds, skipped, err := p.Resolve(context.Background())
		if err != nil || skipped {
			t.Fatalf("unexpected error resolving MSI via env: %v (skipped: %v)", err, skipped)
		}
		if creds.AccessToken != "mocked-msi-env-token" {
			t.Fatalf("expected mocked-msi-env-token, got %q", creds.AccessToken)
		}
	})
}

func TestChain_GetToken(t *testing.T) {
	t.Parallel()

	chain := NewChain(Config{
		AccessToken: "test-token-fixed",
		Getenv:      func(string) string { return "" },
	})

	tok, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{})
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if tok.Token != "test-token-fixed" {
		t.Fatalf("expected test-token-fixed, got %q", tok.Token)
	}
}

func TestResolveCloudConfiguration(t *testing.T) {
	t.Parallel()

	if cfg := ResolveCloudConfiguration("usgovernment"); cfg.ActiveDirectoryAuthorityHost == "" {
		t.Error("expected non-empty authority host for usgovernment")
	}
	if cfg := ResolveCloudConfiguration("china"); cfg.ActiveDirectoryAuthorityHost == "" {
		t.Error("expected non-empty authority host for china")
	}
	if cfg := ResolveCloudConfiguration("public"); cfg.ActiveDirectoryAuthorityHost == "" {
		t.Error("expected non-empty authority host for public")
	}
	if cfg := ResolveCloudConfiguration(""); cfg.ActiveDirectoryAuthorityHost == "" {
		t.Error("expected default public configuration for empty env")
	}
}

func TestClientSecretProvider_TokenFetcher(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TenantID:     "00000000-0000-0000-0000-000000000001",
		ClientID:     "00000000-0000-0000-0000-000000000002",
		ClientSecret: "secret-val",
		Getenv:       func(string) string { return "" },
	}

	p := &clientSecretProvider{
		cfg: cfg,
		tokenFetcher: func(context.Context, string, string, string) (azcore.AccessToken, error) {
			return azcore.AccessToken{
				Token:     "mocked-secret-tok",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds, skipped, err := p.Resolve(context.Background())
	if err != nil || skipped {
		t.Fatalf("unexpected resolve error: %v, skipped: %v", err, skipped)
	}
	if creds.AccessToken != "mocked-secret-tok" {
		t.Errorf("expected mocked-secret-tok, got %q", creds.AccessToken)
	}

	pMissing := &clientSecretProvider{
		cfg: Config{Getenv: func(string) string { return "" }},
	}
	_, skippedMissing, errMissing := pMissing.Resolve(context.Background())
	if !skippedMissing || errMissing == nil {
		t.Errorf("expected skipped=true when client secret not configured, got skipped=%v, err=%v", skippedMissing, errMissing)
	}

	pErr := &clientSecretProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("simulated fetcher error")
		},
	}
	_, _, errFetcher := pErr.Resolve(context.Background())
	if errFetcher == nil {
		t.Error("expected error from simulated fetcher")
	}

	// File path errors during resolve
	pBadTenantFile := &clientSecretProvider{
		cfg: Config{
			TenantIDFilePath: "/nonexistent/tenant.txt",
			ClientID:         "client-1",
			ClientSecret:     "secret-1",
			Getenv:           func(string) string { return "" },
		},
	}
	_, _, errBadTenantFile := pBadTenantFile.Resolve(context.Background())
	if errBadTenantFile == nil {
		t.Error("expected error from invalid tenant file path")
	}

	pBadClientFile := &clientSecretProvider{
		cfg: Config{
			TenantID:         "tenant-1",
			ClientIDFilePath: "/nonexistent/client.txt",
			ClientSecret:     "secret-1",
			Getenv:           func(string) string { return "" },
		},
	}
	_, _, errBadClientFile := pBadClientFile.Resolve(context.Background())
	if errBadClientFile == nil {
		t.Error("expected error from invalid client file path")
	}

	pBadSecretFile := &clientSecretProvider{
		cfg: Config{
			TenantID:             "tenant-1",
			ClientID:             "client-1",
			ClientSecretFilePath: "/nonexistent/secret.txt",
			Getenv:               func(string) string { return "" },
		},
	}
	_, _, errBadSecretFile := pBadSecretFile.Resolve(context.Background())
	if errBadSecretFile == nil {
		t.Error("expected error from invalid secret file path")
	}

	// Live fallback branch with custom Transport returning mock token
	mockTransport := &mockOAuthTransport{
		token: "mock-oauth-live-token",
	}
	pLive := &clientSecretProvider{
		cfg: Config{
			TenantID:     "00000000-0000-0000-0000-000000000001",
			ClientID:     "00000000-0000-0000-0000-000000000002",
			ClientSecret: "secret-val",
			Getenv:       func(string) string { return "" },
			Transport:    mockTransport,
		},
	}
	liveCreds, liveSkipped, liveErr := pLive.Resolve(context.Background())
	if liveErr != nil || liveSkipped {
		t.Fatalf("unexpected live resolve error: %v, skipped: %v", liveErr, liveSkipped)
	}
	if liveCreds.AccessToken != "mock-oauth-live-token" {
		t.Errorf("expected mock-oauth-live-token, got %q", liveCreds.AccessToken)
	}
}

type mockOAuthTransport struct {
	token string
}

func (m *mockOAuthTransport) Do(req *http.Request) (*http.Response, error) {
	var respBody string
	if strings.Contains(req.URL.Path, "openid-configuration") {
		respBody = `{"issuer":"https://login.microsoftonline.com/00000000-0000-0000-0000-000000000001/v2.0","token_endpoint":"https://login.microsoftonline.com/00000000-0000-0000-0000-000000000001/oauth2/v2.0/token","authorization_endpoint":"https://login.microsoftonline.com/00000000-0000-0000-0000-000000000001/oauth2/v2.0/authorize"}`
	} else {
		respBody = fmt.Sprintf(`{"access_token":%q,"expires_in":3600,"token_type":"Bearer"}`, m.token)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:          io.NopCloser(strings.NewReader(respBody)),
		ContentLength: int64(len(respBody)),
		Request:       req,
	}, nil
}

func TestAzureCLIProvider_TokenFetcher(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TenantID: "00000000-0000-0000-0000-000000000001",
		Getenv:   func(string) string { return "" },
	}

	p := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{
				Token:     "mocked-cli-tok",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds, skipped, err := p.Resolve(context.Background())
	if err != nil || skipped {
		t.Fatalf("unexpected resolve error: %v, skipped: %v", err, skipped)
	}
	if creds.AccessToken != "mocked-cli-tok" {
		t.Errorf("expected mocked-cli-tok, got %q", creds.AccessToken)
	}

	pFetcherErr := &azureCLIProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("simulated cli error")
		},
	}
	_, _, errCliFetcher := pFetcherErr.Resolve(context.Background())
	if errCliFetcher == nil {
		t.Error("expected error from simulated cli fetcher")
	}

	pBadTenantFile := &azureCLIProvider{
		cfg: Config{
			TenantIDFilePath: "/nonexistent/tenant.txt",
			Getenv:           func(string) string { return "" },
		},
	}
	_, _, errBadTenantFile := pBadTenantFile.Resolve(context.Background())
	if errBadTenantFile == nil {
		t.Error("expected error from invalid tenant file in cli provider")
	}
}

func TestAzureDevOpsOIDCProvider_TokenFetcher(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TenantID:                       "00000000-0000-0000-0000-000000000001",
		ClientID:                       "00000000-0000-0000-0000-000000000002",
		AzureDevOpsServiceConnectionID: "conn-123",
		OIDCRequestToken:               "token-xyz",
		Getenv:                         func(string) string { return "" },
	}

	p := &azureDevOpsOIDCProvider{
		cfg: cfg,
		tokenFetcher: func(context.Context, string, string, string, string) (azcore.AccessToken, error) {
			return azcore.AccessToken{
				Token:     "mocked-ado-oidc-tok",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds, skipped, err := p.Resolve(context.Background())
	if err != nil || skipped {
		t.Fatalf("unexpected resolve error: %v, skipped: %v", err, skipped)
	}
	if creds.AccessToken != "mocked-ado-oidc-tok" {
		t.Errorf("expected mocked-ado-oidc-tok, got %q", creds.AccessToken)
	}

	// Missing connection ID
	pMissingConn := &azureDevOpsOIDCProvider{
		cfg: Config{Getenv: func(string) string { return "" }},
	}
	_, skippedConn, errConn := pMissingConn.Resolve(context.Background())
	if !skippedConn || errConn == nil {
		t.Errorf("expected skipped=true on missing connection ID, got skipped=%v, err=%v", skippedConn, errConn)
	}

	// Missing client or tenant ID
	pMissingCreds := &azureDevOpsOIDCProvider{
		cfg: Config{
			AzureDevOpsServiceConnectionID: "conn-123",
			Getenv:                         func(string) string { return "" },
		},
	}
	_, skippedCreds, errCreds := pMissingCreds.Resolve(context.Background())
	if !skippedCreds || errCreds == nil {
		t.Errorf("expected skipped=true on missing client/tenant ID, got skipped=%v, err=%v", skippedCreds, errCreds)
	}

	// Simulated tokenFetcher error
	pFetcherErr := &azureDevOpsOIDCProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("simulated ADO fetcher error")
		},
	}
	_, _, errAdoFetcher := pFetcherErr.Resolve(context.Background())
	if errAdoFetcher == nil {
		t.Error("expected error from simulated ADO fetcher")
	}

	// File path errors during resolve
	pBadTenantFile := &azureDevOpsOIDCProvider{
		cfg: Config{
			AzureDevOpsServiceConnectionID: "conn-123",
			TenantIDFilePath:               "/nonexistent/tenant.txt",
			ClientID:                       "client-1",
			OIDCRequestToken:               "token-xyz",
			Getenv:                         func(string) string { return "" },
		},
	}
	_, _, errBadTenantFile := pBadTenantFile.Resolve(context.Background())
	if errBadTenantFile == nil {
		t.Error("expected error from invalid tenant file path in ADO provider")
	}

	pBadClientFile := &azureDevOpsOIDCProvider{
		cfg: Config{
			AzureDevOpsServiceConnectionID: "conn-123",
			TenantID:                       "tenant-1",
			ClientIDFilePath:               "/nonexistent/client.txt",
			OIDCRequestToken:               "token-xyz",
			Getenv:                         func(string) string { return "" },
		},
	}
	_, _, errBadClientFile := pBadClientFile.Resolve(context.Background())
	if errBadClientFile == nil {
		t.Error("expected error from invalid client file path in ADO provider")
	}
}

func TestWorkloadIdentityProvider_TokenFetcher(t *testing.T) {
	t.Parallel()

	cfg := Config{
		TenantID:  "00000000-0000-0000-0000-000000000001",
		ClientID:  "00000000-0000-0000-0000-000000000002",
		OIDCToken: "oidc-tok-raw",
		Getenv:    func(string) string { return "" },
	}

	p := &workloadIdentityProvider{
		cfg: cfg,
		tokenFetcher: func(context.Context, string, string, string) (azcore.AccessToken, error) {
			return azcore.AccessToken{
				Token:     "mocked-workload-id-tok",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds, skipped, err := p.Resolve(context.Background())
	if err != nil || skipped {
		t.Fatalf("unexpected resolve error: %v, skipped: %v", err, skipped)
	}
	if creds.AccessToken != "mocked-workload-id-tok" {
		t.Errorf("expected mocked-workload-id-tok, got %q", creds.AccessToken)
	}

	pMissingAll := &workloadIdentityProvider{
		cfg: Config{Getenv: func(string) string { return "" }},
	}
	_, skippedAll, errAll := pMissingAll.Resolve(context.Background())
	if !skippedAll || errAll == nil {
		t.Errorf("expected skipped=true when no OIDC token configured, got skipped=%v, err=%v", skippedAll, errAll)
	}

	pMissingClient := &workloadIdentityProvider{
		cfg: Config{
			OIDCToken: "tok",
			Getenv:    func(string) string { return "" },
		},
	}
	_, skippedClient, errClient := pMissingClient.Resolve(context.Background())
	if !skippedClient || errClient == nil {
		t.Errorf("expected skipped=true when client/tenant ID missing for OIDC, got skipped=%v, err=%v", skippedClient, errClient)
	}

	// Simulated tokenFetcher error
	pFetcherErr := &workloadIdentityProvider{
		cfg: cfg,
		tokenFetcher: func(_ context.Context, _, _, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("simulated workload id error")
		},
	}
	_, _, errOidcFetcher := pFetcherErr.Resolve(context.Background())
	if errOidcFetcher == nil {
		t.Error("expected error from simulated workload identity fetcher")
	}

	// File reading errors during resolve
	pBadTenantFile := &workloadIdentityProvider{
		cfg: Config{
			TenantIDFilePath: "/nonexistent/tenant.txt",
			ClientID:         "client-1",
			OIDCToken:        "tok",
			Getenv:           func(string) string { return "" },
		},
	}
	_, _, errBadTenantFile := pBadTenantFile.Resolve(context.Background())
	if errBadTenantFile == nil {
		t.Error("expected error from invalid tenant file in workload id provider")
	}

	pBadClientFile := &workloadIdentityProvider{
		cfg: Config{
			TenantID:         "tenant-1",
			ClientIDFilePath: "/nonexistent/client.txt",
			OIDCToken:        "tok",
			Getenv:           func(string) string { return "" },
		},
	}
	_, _, errBadClientFile := pBadClientFile.Resolve(context.Background())
	if errBadClientFile == nil {
		t.Error("expected error from invalid client file in workload id provider")
	}

	pBadTokenFile := &workloadIdentityProvider{
		cfg: Config{
			TenantID:          "tenant-1",
			ClientID:          "client-1",
			OIDCTokenFilePath: "/nonexistent/oidc.txt",
			Getenv:            func(string) string { return "" },
		},
	}
	_, _, errBadTokenFile := pBadTokenFile.Resolve(context.Background())
	if errBadTokenFile == nil {
		t.Error("expected error from invalid token file in workload id provider")
	}
}

func TestChain_ResolveContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewChain(Config{Getenv: func(string) string { return "" }})
	_, err := c.Resolve(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCLIProviders_DisabledExplicitly(t *testing.T) {
	t.Parallel()

	f := false
	pCLI := &azureCLIProvider{
		cfg: Config{
			UseCLI: &f,
			Getenv: func(string) string { return "" },
		},
	}
	_, skipped, err := pCLI.Resolve(context.Background())
	if !skipped || err == nil {
		t.Errorf("expected azureCLIProvider to be skipped when use_cli=false, got skipped=%v, err=%v", skipped, err)
	}

	pDevCLI := &azureDeveloperCLIProvider{
		cfg: Config{
			UseDevCLI: &f,
			Getenv:    func(string) string { return "" },
		},
	}
	_, skippedDev, errDev := pDevCLI.Resolve(context.Background())
	if !skippedDev || errDev == nil {
		t.Errorf("expected azureDeveloperCLIProvider to be skipped when use_dev_cli=false, got skipped=%v, err=%v", skippedDev, errDev)
	}
}

func TestConfigAndErrors_StringAndEdgeCases(t *testing.T) {
	// Not t.Parallel() because t.Setenv is used below for SYSTEM_OIDCREQUESTURI

	// 1. Config String() and GoString()
	cfg := Config{
		TenantID:     "tenant-123",
		ClientID:     "client-456",
		ClientSecret: "super-secret",
		Environment:  "public",
		UseMSI:       true,
	}
	str := cfg.String()
	if str != cfg.GoString() {
		t.Errorf("expected GoString() to match String(), got %q vs %q", cfg.GoString(), str)
	}
	if str == "" {
		t.Error("expected non-empty config string")
	}

	// 2. Credentials String() and GoString()
	cred := Credentials{
		Source:      SourceStatic,
		AccessToken: "secret-token",
		ExpiresOn:   time.Now(),
	}
	credStr := cred.String()
	if credStr != cred.GoString() {
		t.Errorf("expected cred GoString() to match String(), got %q vs %q", cred.GoString(), credStr)
	}

	// 3. ChainError Error() and Unwrap()
	chainErr := &ChainError{
		Attempts: []AttemptError{
			{Source: "static", Skipped: true, Err: errors.New("not set")},
			{Source: "cli", Skipped: false, Err: errors.New("timeout")},
		},
	}
	if !errors.Is(chainErr, ErrNoCredentials) {
		t.Errorf("expected chainErr to unwrap to ErrNoCredentials")
	}
	if errStr := chainErr.Error(); errStr == "" {
		t.Error("expected non-empty chain error string")
	}

	// 4. AzureCLI / AzureDevCLI disabled via env var
	pCLIEnv := &azureCLIProvider{
		cfg: Config{
			Getenv: func(k string) string {
				if k == "FABRIC_USE_CLI" {
					return "false"
				}
				return ""
			},
		},
	}
	_, skippedCLI, _ := pCLIEnv.Resolve(context.Background())
	if !skippedCLI {
		t.Error("expected azureCLIProvider to be skipped when FABRIC_USE_CLI=false")
	}

	// 5. AzureDevCLI enabled via FABRIC_USE_DEV_CLI=true with tokenFetcher
	pDevCLIEnv := &azureDeveloperCLIProvider{
		cfg: Config{
			TenantID: "00000000-0000-0000-0000-000000000001",
			Getenv: func(k string) string {
				if k == "FABRIC_USE_DEV_CLI" {
					return "true"
				}
				return ""
			},
		},
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{
				Token:     "mocked-dev-cli-tok",
				ExpiresOn: time.Now().Add(1 * time.Hour),
			}, nil
		},
	}
	devCreds, devSkipped, devErr := pDevCLIEnv.Resolve(context.Background())
	if devErr != nil || devSkipped || devCreds.AccessToken != "mocked-dev-cli-tok" {
		t.Errorf("expected dev cli token, got creds=%v, skipped=%v, err=%v", devCreds, devSkipped, devErr)
	}

	// 6. AzureDevCLI error cases
	pDevCLIFetcherErr := &azureDeveloperCLIProvider{
		cfg: Config{
			TenantID: "00000000-0000-0000-0000-000000000001",
			Getenv: func(k string) string {
				if k == "FABRIC_USE_DEV_CLI" {
					return "true"
				}
				return ""
			},
		},
		tokenFetcher: func(_ context.Context, _ string) (azcore.AccessToken, error) {
			return azcore.AccessToken{}, errors.New("simulated dev cli error")
		},
	}
	_, _, errDevCliFetcher := pDevCLIFetcherErr.Resolve(context.Background())
	if errDevCliFetcher == nil {
		t.Error("expected error from simulated dev cli fetcher")
	}

	pDevCLIBadTenantFile := &azureDeveloperCLIProvider{
		cfg: Config{
			TenantIDFilePath: "/nonexistent/tenant.txt",
			Getenv: func(k string) string {
				if k == "FABRIC_USE_DEV_CLI" {
					return "true"
				}
				return ""
			},
		},
	}
	_, _, errDevBadTenant := pDevCLIBadTenantFile.Resolve(context.Background())
	if errDevBadTenant == nil {
		t.Error("expected error from bad tenant file in dev cli provider")
	}

	// 7. WorkloadIdentity disabled via env var
	pWorkloadEnv := &workloadIdentityProvider{
		cfg: Config{
			Getenv: func(k string) string {
				if k == "FABRIC_USE_OIDC" {
					return "false"
				}
				return ""
			},
		},
	}
	_, skippedWorkload, _ := pWorkloadEnv.Resolve(context.Background())
	if !skippedWorkload {
		t.Error("expected workloadIdentityProvider to be skipped when FABRIC_USE_OIDC=false")
	}

	// 8. Live fallback branches using mockOAuthTransport
	mockTransport := &mockOAuthTransport{token: "mock-live-token"}

	// AzureDevOps OIDC live fallback
	pAdoLive := &azureDevOpsOIDCProvider{
		cfg: Config{
			TenantID:                       "00000000-0000-0000-0000-000000000001",
			ClientID:                       "00000000-0000-0000-0000-000000000002",
			AzureDevOpsServiceConnectionID: "conn-123",
			OIDCRequestToken:               "sys-token-abc",
			Getenv: func(k string) string {
				if k == "SYSTEM_OIDCREQUESTURI" {
					return "https://vstoken.dev.azure.com/token"
				}
				return ""
			},
			Transport: mockTransport,
		},
	}
	t.Setenv("SYSTEM_OIDCREQUESTURI", "https://vstoken.dev.azure.com/token")
	adoCreds, adoSkipped, adoErr := pAdoLive.Resolve(context.Background())
	if adoErr != nil || adoSkipped {
		t.Fatalf("unexpected ADO live error: %v, skipped: %v", adoErr, adoSkipped)
	}
	if adoCreds.AccessToken != "mock-live-token" {
		t.Errorf("expected mock-live-token, got %q", adoCreds.AccessToken)
	}

	// Workload Identity live fallback (direct token)
	pWorkloadLive := &workloadIdentityProvider{
		cfg: Config{
			TenantID:  "00000000-0000-0000-0000-000000000001",
			ClientID:  "00000000-0000-0000-0000-000000000002",
			OIDCToken: "fed-token-abc",
			Getenv:    func(string) string { return "" },
			Transport: mockTransport,
		},
	}
	wlCreds, wlSkipped, wlErr := pWorkloadLive.Resolve(context.Background())
	if wlErr != nil || wlSkipped {
		t.Fatalf("unexpected Workload ID live error: %v, skipped: %v", wlErr, wlSkipped)
	}
	if wlCreds.AccessToken != "mock-live-token" {
		t.Errorf("expected mock-live-token, got %q", wlCreds.AccessToken)
	}

	// Workload Identity live fallback with AZURE_FEDERATED_TOKEN_FILE
	tokFilePath := filepath.Join(t.TempDir(), "azure_fed_token.jwt")
	if err := os.WriteFile(tokFilePath, []byte("jwt-content-placeholder"), 0o600); err != nil {
		t.Fatalf("write jwt file: %v", err)
	}
	pWorkloadTokenFile := &workloadIdentityProvider{
		cfg: Config{
			TenantID: "00000000-0000-0000-0000-000000000001",
			ClientID: "00000000-0000-0000-0000-000000000002",
			Getenv: func(k string) string {
				if k == "AZURE_FEDERATED_TOKEN_FILE" {
					return tokFilePath
				}
				return ""
			},
			Transport: mockTransport,
		},
	}
	wlFedCreds, wlFedSkipped, wlFedErr := pWorkloadTokenFile.Resolve(context.Background())
	if wlFedErr != nil || wlFedSkipped {
		t.Fatalf("unexpected Workload ID token file live error: %v, skipped: %v", wlFedErr, wlFedSkipped)
	}
	if wlFedCreds.AccessToken != "mock-live-token" {
		t.Errorf("expected mock-live-token, got %q", wlFedCreds.AccessToken)
	}

	// Managed Identity live fallback
	pMsiLive := &managedIdentityProvider{
		cfg: Config{
			UseMSI:    true,
			ClientID:  "00000000-0000-0000-0000-000000000002",
			Getenv:    func(string) string { return "" },
			Transport: mockTransport,
		},
	}
	msiCreds, msiSkipped, msiErr := pMsiLive.Resolve(context.Background())
	if msiErr != nil || msiSkipped {
		t.Fatalf("unexpected MSI live error: %v, skipped: %v", msiErr, msiSkipped)
	}
	if msiCreds.AccessToken != "mock-live-token" {
		t.Errorf("expected mock-live-token, got %q", msiCreds.AccessToken)
	}
}

func TestFetchOIDCAssertion(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-request-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("audience") != "api://AzureADTokenExchange" {
			http.Error(w, "bad audience", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"value": "jwt-assertion-token-value",
		})
	}))
	defer ts.Close()

	ctx := context.Background()

	// 1. Success case
	tok, err := fetchOIDCAssertion(ctx, ts.URL, "test-request-token")
	if err != nil {
		t.Fatalf("unexpected error fetching OIDC assertion: %v", err)
	}
	if tok != "jwt-assertion-token-value" {
		t.Errorf("expected jwt-assertion-token-value, got %q", tok)
	}

	// 2. HTTP error case
	_, errHTTP := fetchOIDCAssertion(ctx, ts.URL, "wrong-token")
	if errHTTP == nil {
		t.Fatal("expected error on wrong token")
	}

	// 3. Invalid json server
	tsBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer tsBadJSON.Close()

	_, errBadJSON := fetchOIDCAssertion(ctx, tsBadJSON.URL, "token")
	if errBadJSON == nil {
		t.Fatal("expected error on bad json")
	}

	// 4. Empty value
	tsEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":""}`))
	}))
	defer tsEmpty.Close()

	_, errEmpty := fetchOIDCAssertion(ctx, tsEmpty.URL, "token")
	if errEmpty == nil {
		t.Fatal("expected error on empty value")
	}
}

func TestCLIProviders_LiveFallbackErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context to guarantee immediate non-network failure in azidentity GetToken

	// 1. AzureDeveloperCLI fallback GetToken error
	pDev := &azureDeveloperCLIProvider{
		cfg: Config{
			TenantID: "00000000-0000-0000-0000-000000000001",
			Getenv: func(k string) string {
				if k == "FABRIC_USE_DEV_CLI" {
					return "true"
				}
				return ""
			},
		},
	}
	_, _, errDev := pDev.Resolve(ctx)
	if errDev == nil {
		t.Error("expected error from dev cli on canceled context")
	}

	// 2. AzureCLI fallback GetToken error
	pCLI := &azureCLIProvider{
		cfg: Config{
			TenantID: "00000000-0000-0000-0000-000000000001",
			Getenv:   func(string) string { return "" },
		},
	}
	_, _, errCLI := pCLI.Resolve(ctx)
	if errCLI == nil {
		t.Error("expected error from azure cli on canceled context")
	}
}
