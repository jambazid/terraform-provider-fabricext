// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package credentials implements the Microsoft Entra ID credential chain for the Fabric API.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	// FabricScope is the Microsoft Entra ID token scope for the Microsoft Fabric REST API.
	FabricScope = "https://api.fabric.microsoft.com/.default"

	// SourceStatic identifies credentials resolved from a static bearer token.
	SourceStatic = "static_access_token"
	// SourceClientSecret identifies credentials resolved via Service Principal client secret.
	SourceClientSecret = "service_principal_client_secret"
	// SourceWorkloadIdentity identifies credentials resolved via OIDC federated token.
	SourceWorkloadIdentity = "workload_identity_oidc"
	// SourceManagedIdentity identifies credentials resolved via Azure Managed Identity.
	SourceManagedIdentity = "managed_identity"
	// SourceAzureCLI identifies credentials resolved via the Azure CLI session.
	SourceAzureCLI = "azure_cli"

	refreshWindow = 2 * time.Minute
)

// ErrNoCredentials indicates that no provider in the credential chain was able to resolve a token.
var ErrNoCredentials = errors.New("no valid Microsoft Fabric credentials found in credential chain")

// AttemptError records the failure of an individual credential source within the chain.
type AttemptError struct {
	Source  string
	Skipped bool
	Err     error
}

// ChainError aggregates every credential provider attempted when the chain fails.
type ChainError struct {
	Attempts []AttemptError
}

func (e *ChainError) Error() string {
	var parts []string
	for _, a := range e.Attempts {
		status := "failed"
		if a.Skipped {
			status = "skipped"
		}
		parts = append(parts, fmt.Sprintf("%s (%s: %v)", a.Source, status, a.Err))
	}
	return fmt.Sprintf("%s; tried [%s]", ErrNoCredentials.Error(), strings.Join(parts, "; "))
}

func (e *ChainError) Unwrap() error {
	return ErrNoCredentials
}

// Credentials holds a resolved bearer token and its metadata with built-in string redaction.
type Credentials struct {
	AccessToken string
	ExpiresOn   time.Time
	Source      string
}

// String masks the raw bearer token to prevent accidental exposure in logs or diagnostics.
func (c Credentials) String() string {
	return fmt.Sprintf("Credentials{Source:%q, AccessToken:\"[REDACTED]\", ExpiresOn:%q}", c.Source, c.ExpiresOn.Format(time.RFC3339))
}

// GoString masks the raw bearer token when formatted with %#v.
func (c Credentials) GoString() string {
	return c.String()
}

// Config configures the Microsoft Entra ID credential chain.
type Config struct {
	AccessToken  string
	TenantID     string
	ClientID     string
	ClientSecret string
	UseMSI       bool
	UseOIDC      *bool
	UseCLI       *bool
	Getenv       func(string) string
}

func (c Config) getenv(key string) string {
	if c.Getenv != nil {
		return strings.TrimSpace(c.Getenv(key))
	}
	return strings.TrimSpace(os.Getenv(key))
}

func (c Config) firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := c.getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Provider represents a single credential resolution strategy in the chain.
type Provider interface {
	Name() string
	Resolve(ctx context.Context) (Credentials, bool, error)
}

// Option customizes a Chain instance.
type Option func(*Chain)

// WithProviders overrides the default provider sequence (used in unit tests).
func WithProviders(providers ...Provider) Option {
	return func(c *Chain) {
		c.providers = providers
	}
}

// WithClock overrides the clock function used for token expiry checks.
func WithClock(now func() time.Time) Option {
	return func(c *Chain) {
		c.now = now
	}
}

// Chain evaluates an ordered sequence of credential providers and caches the active token.
type Chain struct {
	providers []Provider
	now       func() time.Time

	mu     sync.RWMutex
	cached *Credentials
}

// NewChain constructs the ordered Microsoft Fabric credential chain:
// StaticToken -> ClientSecret -> WorkloadIdentity (OIDC) -> ManagedIdentity -> AzureCLI.
func NewChain(cfg Config, opts ...Option) *Chain {
	c := &Chain{
		now: time.Now,
		providers: []Provider{
			&staticProvider{cfg: cfg},
			&clientSecretProvider{cfg: cfg},
			&workloadIdentityProvider{cfg: cfg},
			&managedIdentityProvider{cfg: cfg},
			&azureCLIProvider{cfg: cfg},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ValidateConfig checks for invalid partial explicit configuration in the provider HCL block.
func ValidateConfig(cfg Config) error {
	explicitCount := 0
	if strings.TrimSpace(cfg.TenantID) != "" {
		explicitCount++
	}
	if strings.TrimSpace(cfg.ClientID) != "" {
		explicitCount++
	}
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		explicitCount++
	}
	// If client_secret is explicitly set in HCL without tenant_id and client_id, fail fast.
	if strings.TrimSpace(cfg.ClientSecret) != "" && explicitCount < 3 {
		return errors.New("partial service principal configuration in provider block: tenant_id, client_id, and client_secret must all be set when client_secret is configured")
	}
	return nil
}

// Resolve returns a cached valid token or evaluates the credential chain in order.
func (c *Chain) Resolve(ctx context.Context) (Credentials, error) {
	now := c.now()

	c.mu.RLock()
	if c.cached != nil && c.cached.AccessToken != "" && c.cached.ExpiresOn.After(now.Add(refreshWindow)) {
		creds := *c.cached
		c.mu.RUnlock()
		return creds, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	now = c.now()
	if c.cached != nil && c.cached.AccessToken != "" && c.cached.ExpiresOn.After(now.Add(refreshWindow)) {
		return *c.cached, nil
	}

	var attempts []AttemptError
	for _, p := range c.providers {
		if err := ctx.Err(); err != nil {
			return Credentials{}, err
		}
		creds, skipped, err := p.Resolve(ctx)
		if err == nil && strings.TrimSpace(creds.AccessToken) != "" {
			if creds.Source == "" {
				creds.Source = p.Name()
			}
			if creds.ExpiresOn.IsZero() {
				creds.ExpiresOn = now.Add(1 * time.Hour)
			}
			c.cached = &creds
			return creds, nil
		}
		if err == nil {
			err = errors.New("returned empty access token")
		}
		attempts = append(attempts, AttemptError{
			Source:  p.Name(),
			Skipped: skipped,
			Err:     err,
		})
	}

	return Credentials{}, &ChainError{Attempts: attempts}
}

// GetToken implements azcore.TokenCredential so Chain can be used directly with Azure SDK pipelines.
func (c *Chain) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	creds, err := c.Resolve(ctx)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{
		Token:     creds.AccessToken,
		ExpiresOn: creds.ExpiresOn,
	}, nil
}

type staticProvider struct {
	cfg Config
}

func (p *staticProvider) Name() string { return SourceStatic }

func (p *staticProvider) Resolve(_ context.Context) (Credentials, bool, error) {
	tok := strings.TrimSpace(p.cfg.AccessToken)
	if tok == "" {
		tok = p.cfg.firstEnv("FABRIC_ACCESS_TOKEN")
	}
	if tok == "" {
		return Credentials{}, true, errors.New("neither access_token attribute nor FABRIC_ACCESS_TOKEN env var is set")
	}
	return Credentials{
		AccessToken: tok,
		Source:      SourceStatic,
	}, false, nil
}

type clientSecretProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID, clientSecret string) (azcore.AccessToken, error)
}

func (p *clientSecretProvider) Name() string { return SourceClientSecret }

func (p *clientSecretProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	if err := ValidateConfig(p.cfg); err != nil {
		return Credentials{}, false, err
	}

	tenantID := strings.TrimSpace(p.cfg.TenantID)
	if tenantID == "" {
		tenantID = p.cfg.firstEnv("FABRIC_TENANT_ID", "AZURE_TENANT_ID", "ARM_TENANT_ID")
	}
	clientID := strings.TrimSpace(p.cfg.ClientID)
	if clientID == "" {
		clientID = p.cfg.firstEnv("FABRIC_CLIENT_ID", "AZURE_CLIENT_ID", "ARM_CLIENT_ID")
	}
	clientSecret := strings.TrimSpace(p.cfg.ClientSecret)
	if clientSecret == "" {
		clientSecret = p.cfg.firstEnv("FABRIC_CLIENT_SECRET", "AZURE_CLIENT_SECRET", "ARM_CLIENT_SECRET")
	}

	// OIDC 3-Tuple Fallthrough Law: require all 3 values before attempting ClientSecretCredential
	// so GitHub Actions OIDC environments (which set AZURE_TENANT_ID and AZURE_CLIENT_ID without AZURE_CLIENT_SECRET)
	// cleanly fall through to WorkloadIdentity / AzureCLI.
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return Credentials{}, true, errors.New("complete 3-tuple (tenant_id, client_id, client_secret) not present; skipping for OIDC/CLI fallthrough")
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID, clientID, clientSecret)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceClientSecret,
		}, false, nil
	}

	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity ClientSecretCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{FabricScope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via ClientSecretCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceClientSecret,
	}, false, nil
}

type workloadIdentityProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID, tokenFile string) (azcore.AccessToken, error)
}

func (p *workloadIdentityProvider) Name() string { return SourceWorkloadIdentity }

func (p *workloadIdentityProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	if p.cfg.UseOIDC != nil && !*p.cfg.UseOIDC {
		return Credentials{}, true, errors.New("workload identity disabled via use_oidc=false")
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_OIDC", "AZURE_USE_OIDC", "ARM_USE_OIDC"), "false") {
		return Credentials{}, true, errors.New("workload identity disabled via USE_OIDC=false")
	}

	tenantID := strings.TrimSpace(p.cfg.TenantID)
	if tenantID == "" {
		tenantID = p.cfg.firstEnv("FABRIC_TENANT_ID", "AZURE_TENANT_ID", "ARM_TENANT_ID")
	}
	clientID := strings.TrimSpace(p.cfg.ClientID)
	if clientID == "" {
		clientID = p.cfg.firstEnv("FABRIC_CLIENT_ID", "AZURE_CLIENT_ID", "ARM_CLIENT_ID")
	}
	tokenFile := p.cfg.firstEnv("AZURE_FEDERATED_TOKEN_FILE")

	if tenantID == "" || clientID == "" || tokenFile == "" {
		return Credentials{}, true, errors.New("AZURE_TENANT_ID, AZURE_CLIENT_ID, and AZURE_FEDERATED_TOKEN_FILE not all set")
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID, clientID, tokenFile)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceWorkloadIdentity,
		}, false, nil
	}

	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		TenantID:      tenantID,
		ClientID:      clientID,
		TokenFilePath: tokenFile,
	})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity WorkloadIdentityCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{FabricScope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via WorkloadIdentityCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceWorkloadIdentity,
	}, false, nil
}

type managedIdentityProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, clientID string) (azcore.AccessToken, error)
}

func (p *managedIdentityProvider) Name() string { return SourceManagedIdentity }

func (p *managedIdentityProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	useMSI := p.cfg.UseMSI ||
		strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_MSI", "AZURE_USE_MSI", "ARM_USE_MSI"), "true") ||
		p.cfg.firstEnv("IDENTITY_ENDPOINT", "MSI_ENDPOINT") != ""
	if !useMSI {
		return Credentials{}, true, errors.New("managed identity not enabled (use_msi=false and IDENTITY_ENDPOINT unset)")
	}

	clientID := strings.TrimSpace(p.cfg.ClientID)
	if clientID == "" {
		clientID = p.cfg.firstEnv("FABRIC_CLIENT_ID", "AZURE_CLIENT_ID", "ARM_CLIENT_ID")
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, clientID)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceManagedIdentity,
		}, false, nil
	}

	var opts *azidentity.ManagedIdentityCredentialOptions
	if clientID != "" {
		opts = &azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(clientID),
		}
	}
	cred, err := azidentity.NewManagedIdentityCredential(opts)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity ManagedIdentityCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{FabricScope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via ManagedIdentityCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceManagedIdentity,
	}, false, nil
}

type azureCLIProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID string) (azcore.AccessToken, error)
}

func (p *azureCLIProvider) Name() string { return SourceAzureCLI }

func (p *azureCLIProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	if p.cfg.UseCLI != nil && !*p.cfg.UseCLI {
		return Credentials{}, true, errors.New("azure cli disabled via use_cli=false")
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_CLI", "AZURE_USE_CLI", "ARM_USE_CLI"), "false") {
		return Credentials{}, true, errors.New("azure cli disabled via USE_CLI=false")
	}

	tenantID := strings.TrimSpace(p.cfg.TenantID)
	if tenantID == "" {
		tenantID = p.cfg.firstEnv("FABRIC_TENANT_ID", "AZURE_TENANT_ID", "ARM_TENANT_ID")
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceAzureCLI,
		}, false, nil
	}

	var opts *azidentity.AzureCLICredentialOptions
	if tenantID != "" {
		opts = &azidentity.AzureCLICredentialOptions{
			TenantID: tenantID,
		}
	}
	cred, err := azidentity.NewAzureCLICredential(opts)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity AzureCLICredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{FabricScope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via AzureCLICredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceAzureCLI,
	}, false, nil
}
