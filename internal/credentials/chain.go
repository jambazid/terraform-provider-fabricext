// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package credentials implements the Microsoft Entra ID credential chain for the Fabric API,
// providing 100% authentication parity with the official Microsoft Fabric provider.
package credentials

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	// FabricScopePublic is the Entra ID scope for the Public Microsoft Fabric REST API.
	FabricScopePublic = "https://api.fabric.microsoft.com/.default"
	// FabricScopeUSGovernment is the Entra ID scope for US Government Microsoft Fabric.
	FabricScopeUSGovernment = "https://api.fabric.microsoft.us/.default"
	// FabricScopeChina is the Entra ID scope for China Microsoft Fabric.
	FabricScopeChina = "https://api.fabric.microsoft.cn/.default"

	// FabricScope is preserved for backwards-compatibility referencing the public scope.
	FabricScope = FabricScopePublic

	// PowerBIScopePublic is the Entra ID scope for the Public Power BI Service.
	PowerBIScopePublic = "https://analysis.windows.net/powerbi/api/.default"
	// PowerBIScopeUSGovernment is the Entra ID scope for US Government Power BI Service.
	PowerBIScopeUSGovernment = "https://analysis.usgovcloudapi.net/powerbi/api/.default"
	// PowerBIScopeChina is the Entra ID scope for China Power BI Service.
	PowerBIScopeChina = "https://analysis.chinacloudapi.cn/powerbi/api/.default"

	// PowerBIScope is preserved referencing the public scope.
	PowerBIScope = PowerBIScopePublic

	// SourceStatic identifies credentials resolved from a static bearer token.
	SourceStatic = "static_access_token"
	// SourceStaticPowerBI identifies credentials resolved from a static Power BI bearer token.
	SourceStaticPowerBI = "static_powerbi_access_token"
	// SourceClientCertificate identifies credentials resolved via Service Principal client certificate.
	SourceClientCertificate = "service_principal_client_certificate"
	// SourceClientSecret identifies credentials resolved via Service Principal client secret.
	SourceClientSecret = "service_principal_client_secret"
	// SourceAzureDevOpsOIDC identifies credentials resolved via Azure DevOps Workload Identity Federation.
	SourceAzureDevOpsOIDC = "azure_devops_workload_identity_federation"
	// SourceWorkloadIdentity identifies credentials resolved via OIDC federated token.
	SourceWorkloadIdentity = "workload_identity_oidc"
	// SourceManagedIdentity identifies credentials resolved via Azure Managed Identity.
	SourceManagedIdentity = "managed_identity"
	// SourceAzureDeveloperCLI identifies credentials resolved via the Azure Developer CLI.
	SourceAzureDeveloperCLI = "azure_developer_cli"
	// SourceAzureCLI identifies credentials resolved via the Azure CLI session.
	SourceAzureCLI = "azure_cli"

	refreshWindow = 2 * time.Minute
)

// ErrNoCredentials indicates that no provider in the credential chain was able to resolve a token.
var ErrNoCredentials = errors.New("no valid Microsoft Fabric credentials found in credential chain")

// ResolveFabricScope returns the canonical Entra ID token scope for the given cloud environment.
func ResolveFabricScope(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return FabricScopeUSGovernment
	case "china":
		return FabricScopeChina
	default:
		return FabricScopePublic
	}
}

// ResolvePowerBIScope returns the canonical Entra ID token scope for Power BI in the given environment.
func ResolvePowerBIScope(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return PowerBIScopeUSGovernment
	case "china":
		return PowerBIScopeChina
	default:
		return PowerBIScopePublic
	}
}

// ResolvePowerBIPortalURL returns the bootstrap portal URL for cluster discovery.
func ResolvePowerBIPortalURL(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return "https://app.powerbigov.us"
	case "china":
		return "https://app.powerbi.cn"
	default:
		return "https://app.powerbi.com"
	}
}

// ResolvePowerBIAPIBaseURL returns the public REST API gateway base URL for Power BI.
func ResolvePowerBIAPIBaseURL(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return "https://api.powerbigov.us"
	case "china":
		return "https://api.powerbi.cn"
	default:
		return "https://api.powerbi.com"
	}
}

// TokenPosture holds non-sensitive metadata describing caller identity and token capabilities
// for diagnostic error output, strictly following official provider redaction boundaries.
type TokenPosture struct {
	AuthSource     string   `json:"auth_source"`
	TokenType      string   `json:"token_type"` // "app-only", "delegated-user", or "opaque"
	TenantID       string   `json:"tenant_id,omitempty"`
	AppID          string   `json:"app_id,omitempty"`
	ObjectID       string   `json:"object_id,omitempty"`
	Audience       string   `json:"audience,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	HasTenantRoles bool     `json:"has_tenant_roles"`
	ScpPresent     bool     `json:"scp_present"`
}

// Format returns a concise, single-line diagnostic string suitable for default Terraform Error: output.
func (p TokenPosture) Format() string {
	var parts []string
	if p.AuthSource != "" {
		parts = append(parts, fmt.Sprintf("auth_source=%s", p.AuthSource))
	}
	if p.TokenType != "" {
		parts = append(parts, fmt.Sprintf("token_type=%s", p.TokenType))
	}
	if p.TenantID != "" {
		parts = append(parts, fmt.Sprintf("tid=%s", p.TenantID))
	}
	if p.AppID != "" {
		parts = append(parts, fmt.Sprintf("appid=%s", p.AppID))
	}
	if p.ObjectID != "" {
		parts = append(parts, fmt.Sprintf("oid=%s", p.ObjectID))
	}
	if p.Audience != "" {
		parts = append(parts, fmt.Sprintf("aud=%s", p.Audience))
	}
	if len(p.Roles) > 0 {
		parts = append(parts, fmt.Sprintf("roles=[%s]", strings.Join(p.Roles, ", ")))
	} else {
		parts = append(parts, "roles=none")
	}
	parts = append(parts, fmt.Sprintf("scp_present=%t", p.ScpPresent))
	return strings.Join(parts, ", ")
}

type rawTokenClaims struct {
	Audience   interface{} `json:"aud"`
	IDType     string      `json:"idtyp"`
	AppID      string      `json:"appid"`
	Azp        string      `json:"azp"`
	AppIDAcr   string      `json:"appidacr"`
	ObjectID   string      `json:"oid"`
	TenantID   string      `json:"tid"`
	Roles      []string    `json:"roles"`
	Scopes     string      `json:"scp"`
	Upn        string      `json:"upn"`
	UniqueName string      `json:"unique_name"`
	Email      string      `json:"email"`
}

// InspectTokenPosture decodes non-sensitive JWT token claims in memory without retaining the raw token
// or PII (e.g. human UPN/email), classifying caller identity and token posture for diagnostic reporting.
func InspectTokenPosture(rawToken, authSource string) TokenPosture {
	posture := TokenPosture{
		AuthSource: authSource,
		TokenType:  "opaque",
	}

	trimmedToken := strings.TrimSpace(rawToken)
	parts := strings.Split(trimmedToken, ".")
	if len(parts) == 3 {
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			payloadBytes, err = base64.RawStdEncoding.DecodeString(parts[1])
		}
		if err != nil {
			payloadBytes, err = base64.URLEncoding.DecodeString(parts[1])
		}
		if err == nil {
			var raw rawTokenClaims
			if jsonErr := json.Unmarshal(payloadBytes, &raw); jsonErr == nil {
				posture.TenantID = raw.TenantID
				if raw.AppID != "" {
					posture.AppID = raw.AppID
				} else if raw.Azp != "" {
					posture.AppID = raw.Azp
				}
				posture.ObjectID = raw.ObjectID

				switch v := raw.Audience.(type) {
				case string:
					posture.Audience = v
				case []interface{}:
					var auds []string
					for _, item := range v {
						if s, ok := item.(string); ok && s != "" {
							auds = append(auds, s)
						}
					}
					posture.Audience = strings.Join(auds, ",")
				}

				if len(raw.Roles) > 0 {
					roles := make([]string, len(raw.Roles))
					copy(roles, raw.Roles)
					sort.Strings(roles)
					posture.Roles = roles
					for _, r := range roles {
						if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r)), "tenant.") {
							posture.HasTenantRoles = true
							break
						}
					}
				}

				if strings.TrimSpace(raw.Scopes) != "" {
					posture.ScpPresent = true
				}

				hasUserSignals := raw.IDType == "user" ||
					posture.ScpPresent ||
					strings.TrimSpace(raw.Upn) != "" ||
					strings.TrimSpace(raw.UniqueName) != "" ||
					strings.TrimSpace(raw.Email) != ""

				switch {
				case raw.IDType == "app":
					posture.TokenType = "app-only"
				case hasUserSignals:
					posture.TokenType = "delegated-user"
				case raw.AppIDAcr == "1" || raw.AppIDAcr == "2" || len(posture.Roles) > 0 || (posture.ObjectID != "" && posture.AppID != ""):
					posture.TokenType = "app-only"
				}
			}
		}
	}

	// Fallback classification if token_type could not be conclusively determined from claims
	if posture.TokenType == "opaque" && authSource != "" {
		src := strings.ToLower(authSource)
		switch {
		case strings.Contains(src, "oidc"),
			strings.Contains(src, "secret"),
			strings.Contains(src, "certificate"),
			strings.Contains(src, "cert"),
			strings.Contains(src, "msi"),
			strings.Contains(src, "managed_identity"):
			posture.TokenType = "app-only"
		case strings.Contains(src, "cli"),
			strings.Contains(src, "dev_cli"):
			posture.TokenType = "delegated-user"
		}
	}

	return posture
}

// ResolvePowerBIAllowedHostSuffix returns the expected domain suffix for cluster discovery.
func ResolvePowerBIAllowedHostSuffix(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return "analysis.usgovcloudapi.net"
	case "china":
		return "analysis.chinacloudapi.cn"
	default:
		return "analysis.windows.net"
	}
}

// ResolveCloudConfiguration returns the azcore cloud.Configuration for the given environment name.
func ResolveCloudConfiguration(env string) cloud.Configuration {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "usgovernment":
		return cloud.AzureGovernment
	case "china":
		return cloud.AzureChina
	default:
		return cloud.AzurePublic
	}
}

// AttemptError records the failure or skip status of an individual credential provider.
type AttemptError struct {
	Source  string
	Skipped bool
	Err     error
}

// ChainError aggregates every credential provider attempted when the chain fails.
type ChainError struct {
	Attempts  []AttemptError
	ScopeHint string
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
	base := fmt.Sprintf("%s; tried [%s]", ErrNoCredentials.Error(), strings.Join(parts, "; "))
	if e.ScopeHint != "" {
		base += "; " + e.ScopeHint
	}
	return base
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

// Config configures the Microsoft Entra ID credential chain matching official Fabric provider options.
type Config struct {
	AccessToken                    string
	PowerBIAccessToken             string
	PowerBIEndpoint                string
	TenantID                       string
	TenantIDFilePath               string
	ClientID                       string
	ClientIDFilePath               string
	ClientSecret                   string
	ClientSecretFilePath           string
	ClientCertificate              string
	ClientCertificateFilePath      string
	ClientCertificatePassword      string
	UseMSI                         bool
	UseOIDC                        *bool
	OIDCToken                      string
	OIDCTokenFilePath              string
	OIDCRequestToken               string
	OIDCRequestURL                 string
	AzureDevOpsServiceConnectionID string
	UseCLI                         *bool
	UseDevCLI                      *bool
	Environment                    string // "public", "usgovernment", "china"
	AuxiliaryTenantIDs             []string
	Getenv                         func(string) string
	Transport                      policy.Transporter
}

// String masks sensitive secrets in the configuration.
func (c Config) String() string {
	return fmt.Sprintf("Config{TenantID:%q, ClientID:%q, Environment:%q, UseMSI:%v, HasSecret:%v, HasCert:%v, HasPowerBIToken:%v, PowerBIEndpoint:%q}",
		c.TenantID, c.ClientID, c.Environment, c.UseMSI, c.ClientSecret != "", c.ClientCertificate != "" || c.ClientCertificateFilePath != "", c.PowerBIAccessToken != "", c.PowerBIEndpoint)
}

// GoString masks sensitive secrets in the configuration when formatted with %#v.
func (c Config) GoString() string {
	return c.String()
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

func (c Config) readFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("reading credential file %q: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ResolveTenantID returns the configured or discovered Microsoft Entra ID tenant UUID.
func (c Config) ResolveTenantID() (string, error) {
	return c.resolveTenantID()
}

func (c Config) resolveTenantID() (string, error) {
	if v := strings.TrimSpace(c.TenantID); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(c.TenantIDFilePath); v != "" {
		return c.readFile(v)
	}
	if v := c.firstEnv("FABRIC_TENANT_ID", "AZURE_TENANT_ID", "ARM_TENANT_ID"); v != "" {
		return v, nil
	}
	if v := c.firstEnv("FABRIC_TENANT_ID_FILE_PATH", "AZURE_TENANT_ID_FILE_PATH", "ARM_TENANT_ID_FILE_PATH"); v != "" {
		return c.readFile(v)
	}
	return "", nil
}

func (c Config) resolveClientID() (string, error) {
	if v := strings.TrimSpace(c.ClientID); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(c.ClientIDFilePath); v != "" {
		return c.readFile(v)
	}
	if v := c.firstEnv("FABRIC_CLIENT_ID", "AZURE_CLIENT_ID", "ARM_CLIENT_ID"); v != "" {
		return v, nil
	}
	if v := c.firstEnv("FABRIC_CLIENT_ID_FILE_PATH", "AZURE_CLIENT_ID_FILE_PATH", "ARM_CLIENT_ID_FILE_PATH"); v != "" {
		return c.readFile(v)
	}
	return "", nil
}

func (c Config) resolveClientSecret() (string, error) {
	if v := strings.TrimSpace(c.ClientSecret); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(c.ClientSecretFilePath); v != "" {
		return c.readFile(v)
	}
	if v := c.firstEnv("FABRIC_CLIENT_SECRET", "AZURE_CLIENT_SECRET", "ARM_CLIENT_SECRET"); v != "" {
		return v, nil
	}
	if v := c.firstEnv("FABRIC_CLIENT_SECRET_FILE_PATH", "AZURE_CLIENT_SECRET_FILE_PATH", "ARM_CLIENT_SECRET_FILE_PATH"); v != "" {
		return c.readFile(v)
	}
	return "", nil
}

func (c Config) resolveEnvironment() string {
	if v := strings.TrimSpace(c.Environment); v != "" {
		return v
	}
	if v := c.firstEnv("FABRIC_ENVIRONMENT", "AZURE_ENVIRONMENT", "ARM_ENVIRONMENT"); v != "" {
		return v
	}
	return "public"
}

func (c Config) resolveAuxiliaryTenants() []string {
	if len(c.AuxiliaryTenantIDs) > 0 {
		return c.AuxiliaryTenantIDs
	}
	if v := c.firstEnv("FABRIC_AUXILIARY_TENANT_IDS", "AZURE_AUXILIARY_TENANT_IDS", "ARM_AUXILIARY_TENANT_IDS"); v != "" {
		var ids []string
		for _, part := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
		return ids
	}
	return nil
}

// Provider represents a single credential resolution strategy in the chain.
type Provider interface {
	Name() string
	Resolve(ctx context.Context) (Credentials, bool, error)
}

// ScopedProvider allows a credential provider to resolve tokens for a specific Entra ID resource scope.
type ScopedProvider interface {
	Provider
	ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error)
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
	cfg       Config
	providers []Provider
	now       func() time.Time

	mu     sync.RWMutex
	cached map[string]Credentials
}

// NewChain constructs the unified Microsoft Fabric credential chain with 100% official provider parity:
// StaticToken -> ClientCertificate -> ClientSecret -> AzureDevOpsOIDC -> WorkloadIdentity -> ManagedIdentity -> AzureDeveloperCLI -> AzureCLI.
func NewChain(cfg Config, opts ...Option) *Chain {
	c := &Chain{
		cfg:    cfg,
		now:    time.Now,
		cached: make(map[string]Credentials),
		providers: []Provider{
			&staticProvider{cfg: cfg},
			&clientCertificateProvider{cfg: cfg},
			&clientSecretProvider{cfg: cfg},
			&azureDevOpsOIDCProvider{cfg: cfg},
			&workloadIdentityProvider{cfg: cfg},
			&managedIdentityProvider{cfg: cfg},
			&azureDeveloperCLIProvider{cfg: cfg},
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
	var modes []string
	if cfg.UseOIDC != nil && *cfg.UseOIDC {
		modes = append(modes, "use_oidc")
	}
	if cfg.UseMSI {
		modes = append(modes, "use_msi")
	}
	if cfg.UseDevCLI != nil && *cfg.UseDevCLI {
		modes = append(modes, "use_dev_cli")
	}
	if cfg.UseCLI != nil && *cfg.UseCLI {
		modes = append(modes, "use_cli")
	}
	if len(modes) > 1 {
		return fmt.Errorf("only one of %s can be enabled at a time", strings.Join(modes, ", "))
	}

	hasSecret := strings.TrimSpace(cfg.ClientSecret) != "" || strings.TrimSpace(cfg.ClientSecretFilePath) != ""
	hasCert := strings.TrimSpace(cfg.ClientCertificate) != "" || strings.TrimSpace(cfg.ClientCertificateFilePath) != ""

	if hasSecret || hasCert {
		tenantID, err := cfg.resolveTenantID()
		if err != nil {
			return fmt.Errorf("resolving tenant id: %w", err)
		}
		clientID, err := cfg.resolveClientID()
		if err != nil {
			return fmt.Errorf("resolving client id: %w", err)
		}
		if tenantID == "" || clientID == "" {
			return errors.New("service principal configuration requires tenant_id and client_id (or corresponding file paths / env vars) to be configured")
		}
	}
	return nil
}

// Resolve returns a cached valid token for the default Fabric scope or evaluates the credential chain in order.
func (c *Chain) Resolve(ctx context.Context) (Credentials, error) {
	return c.ResolveForScope(ctx, ResolveFabricScope(c.cfg.resolveEnvironment()))
}

// ResolveForScope returns a cached valid token for the specified Entra ID scope or evaluates the credential chain.
func (c *Chain) ResolveForScope(ctx context.Context, scope string) (Credentials, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(c.cfg.resolveEnvironment())
	}
	now := c.now()

	c.mu.RLock()
	if cred, ok := c.cached[scope]; ok && cred.AccessToken != "" && cred.ExpiresOn.After(now.Add(refreshWindow)) {
		c.mu.RUnlock()
		return cred, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	now = c.now()
	if cred, ok := c.cached[scope]; ok && cred.AccessToken != "" && cred.ExpiresOn.After(now.Add(refreshWindow)) {
		return cred, nil
	}

	var attempts []AttemptError
	for _, p := range c.providers {
		if err := ctx.Err(); err != nil {
			return Credentials{}, err
		}
		var creds Credentials
		var skipped bool
		var err error
		if sp, ok := p.(ScopedProvider); ok {
			creds, skipped, err = sp.ResolveForScope(ctx, scope)
		} else {
			creds, skipped, err = p.Resolve(ctx)
		}
		if err == nil && strings.TrimSpace(creds.AccessToken) != "" {
			if creds.Source == "" {
				creds.Source = p.Name()
			}
			if creds.ExpiresOn.IsZero() {
				creds.ExpiresOn = now.Add(1 * time.Hour)
			}
			c.cached[scope] = creds
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

	var scopeHint string
	if strings.Contains(strings.ToLower(scope), "powerbi") {
		hasFabricToken := strings.TrimSpace(c.cfg.AccessToken) != "" || c.cfg.firstEnv("FABRIC_ACCESS_TOKEN") != ""
		hasPBIToken := strings.TrimSpace(c.cfg.PowerBIAccessToken) != "" || c.cfg.firstEnv("FABRIC_POWERBI_ACCESS_TOKEN") != ""
		if hasFabricToken && !hasPBIToken {
			scopeHint = "hint: a static Fabric access token (FABRIC_ACCESS_TOKEN) was configured but Power BI metadata operations require a separate Power BI token (FABRIC_POWERBI_ACCESS_TOKEN) or a dynamic credential modality (OIDC, SPN, MSI, CLI)"
		}
	}

	return Credentials{}, &ChainError{Attempts: attempts, ScopeHint: scopeHint}
}

// GetToken implements azcore.TokenCredential so Chain can be used directly with Azure SDK pipelines.
func (c *Chain) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scope := ""
	if len(opts.Scopes) > 0 {
		scope = opts.Scopes[0]
	}
	creds, err := c.ResolveForScope(ctx, scope)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{
		Token:     creds.AccessToken,
		ExpiresOn: creds.ExpiresOn,
	}, nil
}

// -----------------------------------------------------------------------------
// Priority 1: Static Access Token
// -----------------------------------------------------------------------------

type staticProvider struct {
	cfg Config
}

func (p *staticProvider) Name() string { return SourceStatic }

func (p *staticProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *staticProvider) ResolveForScope(_ context.Context, scope string) (Credentials, bool, error) {
	isPBI := strings.Contains(strings.ToLower(scope), "powerbi")
	if isPBI {
		tok := strings.TrimSpace(p.cfg.PowerBIAccessToken)
		if tok == "" {
			tok = p.cfg.firstEnv("FABRIC_POWERBI_ACCESS_TOKEN")
		}
		if tok != "" {
			return Credentials{
				AccessToken: tok,
				Source:      SourceStaticPowerBI,
			}, false, nil
		}
		return Credentials{}, true, errors.New("neither powerbi_access_token attribute nor FABRIC_POWERBI_ACCESS_TOKEN env var is set")
	}

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

// -----------------------------------------------------------------------------
// Priority 2: Service Principal Certificate
// -----------------------------------------------------------------------------

type clientCertificateProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID string, certs []*x509.Certificate, key crypto.PrivateKey) (azcore.AccessToken, error)
}

func (p *clientCertificateProvider) Name() string { return SourceClientCertificate }

func (p *clientCertificateProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *clientCertificateProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, err
	}
	clientID, err := p.cfg.resolveClientID()
	if err != nil {
		return Credentials{}, false, err
	}

	b64Cert := strings.TrimSpace(p.cfg.ClientCertificate)
	if b64Cert == "" {
		b64Cert = p.cfg.firstEnv("FABRIC_CLIENT_CERTIFICATE", "AZURE_CLIENT_CERTIFICATE", "ARM_CLIENT_CERTIFICATE")
	}
	certPath := strings.TrimSpace(p.cfg.ClientCertificateFilePath)
	if certPath == "" {
		certPath = p.cfg.firstEnv(
			"FABRIC_CLIENT_CERTIFICATE_FILE_PATH",
			"AZURE_CLIENT_CERTIFICATE_FILE_PATH",
			"ARM_CLIENT_CERTIFICATE_FILE_PATH",
			"FABRIC_CLIENT_CERTIFICATE_PATH",
			"AZURE_CLIENT_CERTIFICATE_PATH",
			"ARM_CLIENT_CERTIFICATE_PATH",
		)
	}
	password := p.cfg.ClientCertificatePassword
	if password == "" {
		password = p.cfg.firstEnv("FABRIC_CLIENT_CERTIFICATE_PASSWORD", "AZURE_CLIENT_CERTIFICATE_PASSWORD", "ARM_CLIENT_CERTIFICATE_PASSWORD")
	}

	if b64Cert == "" && certPath == "" {
		return Credentials{}, true, errors.New("neither client_certificate nor client_certificate_file_path is configured")
	}
	if tenantID == "" || clientID == "" {
		return Credentials{}, true, errors.New("client certificate configured but tenant_id or client_id is missing")
	}

	var certBytes []byte
	if b64Cert != "" {
		data, err := base64.StdEncoding.DecodeString(b64Cert)
		if err != nil {
			return Credentials{}, false, fmt.Errorf("decoding client_certificate base64: %w", err)
		}
		certBytes = data
	} else {
		data, err := os.ReadFile(certPath)
		if err != nil {
			return Credentials{}, false, fmt.Errorf("reading client_certificate_file_path %q: %w", certPath, err)
		}
		certBytes = data
	}

	key, cert, _, err := pkcs12.DecodeChain(certBytes, password)
	if err != nil {
		// Attempt base64 decoding in case the file contains base64-encoded PKCS#12 text.
		if b64Data, b64Err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(certBytes))); b64Err == nil {
			key, cert, _, err = pkcs12.DecodeChain(b64Data, password)
		}
	}
	if err != nil {
		return Credentials{}, false, fmt.Errorf("decoding PKCS#12 certificate bundle: %w", err)
	}
	if cert == nil {
		return Credentials{}, false, errors.New("no certificate found in PKCS#12 bundle")
	}
	certs := []*x509.Certificate{cert}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID, clientID, certs, key)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceClientCertificate,
		}, false, nil
	}

	cred, err := azidentity.NewClientCertificateCredential(tenantID, clientID, certs, key, &azidentity.ClientCertificateCredentialOptions{
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		ClientOptions: azcore.ClientOptions{
			Cloud:     ResolveCloudConfiguration(p.cfg.resolveEnvironment()),
			Transport: p.cfg.Transport,
		},
	})

	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity ClientCertificateCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via ClientCertificateCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceClientCertificate,
	}, false, nil
}

// -----------------------------------------------------------------------------
// Priority 3: Service Principal Client Secret
// -----------------------------------------------------------------------------

type clientSecretProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID, clientSecret string) (azcore.AccessToken, error)
}

func (p *clientSecretProvider) Name() string { return SourceClientSecret }

func (p *clientSecretProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *clientSecretProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	if err := ValidateConfig(p.cfg); err != nil {
		return Credentials{}, false, err
	}

	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, err
	}
	clientID, err := p.cfg.resolveClientID()
	if err != nil {
		return Credentials{}, false, err
	}
	clientSecret, err := p.cfg.resolveClientSecret()
	if err != nil {
		return Credentials{}, false, err
	}

	// OIDC 3-Tuple Fallthrough Law: require all 3 values before attempting ClientSecretCredential
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

	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, &azidentity.ClientSecretCredentialOptions{
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		ClientOptions: azcore.ClientOptions{
			Cloud:     ResolveCloudConfiguration(p.cfg.resolveEnvironment()),
			Transport: p.cfg.Transport,
		},
	})

	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity ClientSecretCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via ClientSecretCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceClientSecret,
	}, false, nil
}

// -----------------------------------------------------------------------------
// Priority 4: Azure DevOps Workload Identity Federation (OIDC)
// -----------------------------------------------------------------------------

type azureDevOpsOIDCProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID, connID, sysToken string) (azcore.AccessToken, error)
}

func (p *azureDevOpsOIDCProvider) Name() string { return SourceAzureDevOpsOIDC }

func (p *azureDevOpsOIDCProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *azureDevOpsOIDCProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	if p.cfg.UseOIDC != nil && !*p.cfg.UseOIDC {
		return Credentials{}, true, errors.New("use_oidc is explicitly disabled")
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_OIDC", "AZURE_USE_OIDC", "ARM_USE_OIDC"), "false") {
		return Credentials{}, true, errors.New("use_oidc is explicitly disabled via env")
	}

	connID := strings.TrimSpace(p.cfg.AzureDevOpsServiceConnectionID)
	if connID == "" {
		connID = p.cfg.firstEnv(
			"FABRIC_AZURE_DEVOPS_SERVICE_CONNECTION_ID",
			"ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID",
			"ARM_OIDC_AZURE_SERVICE_CONNECTION_ID",
		)
	}
	if connID == "" {
		return Credentials{}, true, errors.New("azure_devops_service_connection_id is not set")
	}

	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, err
	}
	clientID, err := p.cfg.resolveClientID()
	if err != nil {
		return Credentials{}, false, err
	}
	sysToken := strings.TrimSpace(p.cfg.OIDCRequestToken)
	if sysToken == "" {
		sysToken = p.cfg.firstEnv(
			"FABRIC_OIDC_REQUEST_TOKEN",
			"ACTIONS_ID_TOKEN_REQUEST_TOKEN",
			"SYSTEM_ACCESSTOKEN",
			"ARM_OIDC_REQUEST_TOKEN",
		)
	}

	if tenantID == "" || clientID == "" || sysToken == "" {
		return Credentials{}, true, errors.New("azure devops oidc requires tenant_id, client_id, and oidc_request_token")
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID, clientID, connID, sysToken)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceAzureDevOpsOIDC,
		}, false, nil
	}

	cred, err := azidentity.NewAzurePipelinesCredential(tenantID, clientID, connID, sysToken, &azidentity.AzurePipelinesCredentialOptions{
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		ClientOptions: azcore.ClientOptions{
			Cloud:     ResolveCloudConfiguration(p.cfg.resolveEnvironment()),
			Transport: p.cfg.Transport,
		},
	})

	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity AzurePipelinesCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via AzurePipelinesCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceAzureDevOpsOIDC,
	}, false, nil
}

// -----------------------------------------------------------------------------
// Priority 5: Workload Identity (OIDC Assertion / Federated Token File)
// -----------------------------------------------------------------------------

type workloadIdentityProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID, clientID, token string) (azcore.AccessToken, error)
}

func (p *workloadIdentityProvider) Name() string { return SourceWorkloadIdentity }

func (p *workloadIdentityProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *workloadIdentityProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	if p.cfg.UseOIDC != nil && !*p.cfg.UseOIDC {
		return Credentials{}, true, errors.New("workload identity disabled via use_oidc=false")
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_OIDC", "AZURE_USE_OIDC", "ARM_USE_OIDC"), "false") {
		return Credentials{}, true, errors.New("workload identity disabled via USE_OIDC=false")
	}

	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, err
	}
	clientID, err := p.cfg.resolveClientID()
	if err != nil {
		return Credentials{}, false, err
	}

	// 1. Direct OIDC Token
	directToken := strings.TrimSpace(p.cfg.OIDCToken)
	if directToken == "" {
		directToken = p.cfg.firstEnv("FABRIC_OIDC_TOKEN", "AZURE_OIDC_TOKEN", "ARM_OIDC_TOKEN")
	}
	if directToken == "" && strings.TrimSpace(p.cfg.OIDCTokenFilePath) != "" {
		data, err := p.cfg.readFile(p.cfg.OIDCTokenFilePath)
		if err != nil {
			return Credentials{}, false, err
		}
		directToken = data
	}
	if directToken == "" {
		if path := p.cfg.firstEnv("FABRIC_OIDC_TOKEN_FILE_PATH", "AZURE_OIDC_TOKEN_FILE_PATH", "ARM_OIDC_TOKEN_FILE_PATH"); path != "" {
			data, err := p.cfg.readFile(path)
			if err != nil {
				return Credentials{}, false, err
			}
			directToken = data
		}
	}

	// 2. OIDC Request URL + Request Token exchange
	reqURL := strings.TrimSpace(p.cfg.OIDCRequestURL)
	if reqURL == "" {
		reqURL = p.cfg.firstEnv(
			"FABRIC_OIDC_REQUEST_URL",
			"ACTIONS_ID_TOKEN_REQUEST_URL",
			"SYSTEM_OIDCREQUESTURI",
			"ARM_OIDC_REQUEST_URL",
		)
	}
	reqToken := strings.TrimSpace(p.cfg.OIDCRequestToken)
	if reqToken == "" {
		reqToken = p.cfg.firstEnv(
			"FABRIC_OIDC_REQUEST_TOKEN",
			"ACTIONS_ID_TOKEN_REQUEST_TOKEN",
			"SYSTEM_ACCESSTOKEN",
			"ARM_OIDC_REQUEST_TOKEN",
		)
	}

	// 3. Azure Federated Token File
	tokenFile := p.cfg.firstEnv("AZURE_FEDERATED_TOKEN_FILE")

	explicitOIDC := (p.cfg.UseOIDC != nil && *p.cfg.UseOIDC) ||
		strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_OIDC", "AZURE_USE_OIDC", "ARM_USE_OIDC"), "true")

	if directToken == "" && (reqURL == "" || reqToken == "") && tokenFile == "" {
		if explicitOIDC {
			return Credentials{}, false, errors.New("use_oidc was explicitly enabled but OIDC request parameters or assertion token are missing (in GitHub Actions, ensure 'permissions: id-token: write' is granted; otherwise set FABRIC_OIDC_TOKEN or tenant_id/client_id)")
		}
		return Credentials{}, true, errors.New("no OIDC token, token file, or request URI configured")
	}
	if tenantID == "" || clientID == "" {
		if explicitOIDC {
			return Credentials{}, false, errors.New("use_oidc was explicitly enabled but tenant_id or client_id is missing")
		}
		return Credentials{}, true, errors.New("OIDC token configured but tenant_id or client_id is missing")
	}

	if p.tokenFetcher != nil {
		assertion := directToken
		if assertion == "" && reqURL != "" && reqToken != "" {
			fetched, err := fetchOIDCAssertion(ctx, reqURL, reqToken, p.cfg.Transport)
			if err != nil {
				return Credentials{}, false, err
			}
			assertion = fetched
		}
		tok, err := p.tokenFetcher(ctx, tenantID, clientID, assertion)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceWorkloadIdentity,
		}, false, nil
	}

	cloudCfg := ResolveCloudConfiguration(p.cfg.resolveEnvironment())

	// Handle direct token or URL assertion
	if directToken != "" || (reqURL != "" && reqToken != "") {
		assertionFunc := func(c context.Context) (string, error) {
			if directToken != "" {
				return directToken, nil
			}
			return fetchOIDCAssertion(c, reqURL, reqToken, p.cfg.Transport)
		}

		cred, err := azidentity.NewClientAssertionCredential(tenantID, clientID, assertionFunc, &azidentity.ClientAssertionCredentialOptions{
			AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
			ClientOptions: azcore.ClientOptions{
				Cloud:     cloudCfg,
				Transport: p.cfg.Transport,
			},
		})
		if err != nil {
			return Credentials{}, false, fmt.Errorf("creating azidentity ClientAssertionCredential: %w", err)
		}
		tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
		if err != nil {
			return Credentials{}, false, fmt.Errorf("acquiring token via ClientAssertionCredential: %w", err)
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceWorkloadIdentity,
		}, false, nil
	}

	// Handle AZURE_FEDERATED_TOKEN_FILE
	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		TenantID:                   tenantID,
		ClientID:                   clientID,
		TokenFilePath:              tokenFile,
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		ClientOptions: azcore.ClientOptions{
			Cloud:     cloudCfg,
			Transport: p.cfg.Transport,
		},
	})

	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity WorkloadIdentityCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via WorkloadIdentityCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceWorkloadIdentity,
	}, false, nil
}

var oidcSecretRegex = regexp.MustCompile(`(?i)(?:Bearer\s+|ey[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.)[A-Za-z0-9\-._~+/]+=*`)

func scrubSensitiveOIDC(body []byte) string {
	const maxCap = 4096
	s := string(body)
	if len(s) > maxCap {
		s = s[:maxCap] + "... [truncated]"
	}
	return oidcSecretRegex.ReplaceAllString(s, "[REDACTED]")
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func fetchOIDCAssertion(ctx context.Context, requestURL, requestToken string, transport ...policy.Transporter) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("building OIDC assertion request: %w", err)
	}

	if req.URL.Scheme != "https" && (req.URL.Scheme != "http" || !isLoopbackHost(req.URL.Hostname())) {
		return "", fmt.Errorf("OIDC request URL must use https scheme (got %q)", req.URL.Scheme)
	}
	if req.URL.User != nil {
		return "", errors.New("OIDC request URL must not contain user credentials")
	}

	q, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return "", fmt.Errorf("parsing OIDC request query: %w", err)
	}
	if q.Get("audience") == "" {
		q.Set("audience", "api://AzureADTokenExchange")
		req.URL.RawQuery = q.Encode()
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+requestToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp *http.Response
	if len(transport) > 0 && transport[0] != nil {
		resp, err = transport[0].Do(req)
	} else {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err = client.Do(req)
	}
	if err != nil {
		return "", fmt.Errorf("executing OIDC assertion request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("reading OIDC assertion response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("received HTTP status %d from OIDC provider: %s", resp.StatusCode, scrubSensitiveOIDC(body))
	}

	var tokenResp struct {
		Value *string `json:"value"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("unmarshaling OIDC assertion response: %w", err)
	}
	if tokenResp.Value == nil || *tokenResp.Value == "" {
		return "", errors.New("empty JWT assertion received from OIDC provider")
	}
	return *tokenResp.Value, nil
}

// -----------------------------------------------------------------------------
// Priority 6: Azure Managed Identity (MSI)
// -----------------------------------------------------------------------------

type managedIdentityProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, clientID string) (azcore.AccessToken, error)
}

func (p *managedIdentityProvider) Name() string { return SourceManagedIdentity }

func (p *managedIdentityProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *managedIdentityProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_MSI", "AZURE_USE_MSI", "ARM_USE_MSI"), "false") {
		return Credentials{}, true, errors.New("managed identity explicitly disabled via USE_MSI=false")
	}
	useMSI := p.cfg.UseMSI ||
		strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_MSI", "AZURE_USE_MSI", "ARM_USE_MSI"), "true") ||
		p.cfg.firstEnv("IDENTITY_ENDPOINT", "MSI_ENDPOINT") != ""
	if !useMSI {
		return Credentials{}, true, errors.New("managed identity not enabled (use_msi=false and IDENTITY_ENDPOINT unset)")
	}

	clientID, err := p.cfg.resolveClientID()
	if err != nil {
		return Credentials{}, false, fmt.Errorf("resolving client id for managed identity: %w", err)
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

	opts := &azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud:     ResolveCloudConfiguration(p.cfg.resolveEnvironment()),
			Transport: p.cfg.Transport,
		},
	}

	if clientID != "" {
		opts.ID = azidentity.ClientID(clientID)
	}

	cred, err := azidentity.NewManagedIdentityCredential(opts)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity ManagedIdentityCredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via ManagedIdentityCredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceManagedIdentity,
	}, false, nil
}

// -----------------------------------------------------------------------------
// Priority 7: Azure Developer CLI (azd)
// -----------------------------------------------------------------------------

type azureDeveloperCLIProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID string) (azcore.AccessToken, error)
}

func (p *azureDeveloperCLIProvider) Name() string { return SourceAzureDeveloperCLI }

func (p *azureDeveloperCLIProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *azureDeveloperCLIProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	useDevCLI := false
	if p.cfg.UseDevCLI != nil {
		useDevCLI = *p.cfg.UseDevCLI
	} else if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_DEV_CLI", "AZURE_USE_DEV_CLI", "ARM_USE_DEV_CLI"), "true") {
		useDevCLI = true
	}

	if !useDevCLI {
		return Credentials{}, true, errors.New("azure developer cli not enabled (use_dev_cli=false)")
	}

	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, fmt.Errorf("resolving tenant id for azure developer cli: %w", err)
	}

	if p.tokenFetcher != nil {
		tok, err := p.tokenFetcher(ctx, tenantID)
		if err != nil {
			return Credentials{}, false, err
		}
		return Credentials{
			AccessToken: tok.Token,
			ExpiresOn:   tok.ExpiresOn,
			Source:      SourceAzureDeveloperCLI,
		}, false, nil
	}

	opts := &azidentity.AzureDeveloperCLICredentialOptions{
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		TenantID:                   tenantID,
	}
	cred, err := azidentity.NewAzureDeveloperCLICredential(opts)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity AzureDeveloperCLICredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via AzureDeveloperCLICredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceAzureDeveloperCLI,
	}, false, nil
}

// -----------------------------------------------------------------------------
// Priority 8: Azure CLI (az login)
// -----------------------------------------------------------------------------

type azureCLIProvider struct {
	cfg          Config
	tokenFetcher func(ctx context.Context, tenantID string) (azcore.AccessToken, error)
}

func (p *azureCLIProvider) Name() string { return SourceAzureCLI }

func (p *azureCLIProvider) Resolve(ctx context.Context) (Credentials, bool, error) {
	return p.ResolveForScope(ctx, ResolveFabricScope(p.cfg.resolveEnvironment()))
}

func (p *azureCLIProvider) ResolveForScope(ctx context.Context, scope string) (Credentials, bool, error) {
	if strings.TrimSpace(scope) == "" {
		scope = ResolveFabricScope(p.cfg.resolveEnvironment())
	}
	if p.cfg.UseCLI != nil && !*p.cfg.UseCLI {
		return Credentials{}, true, errors.New("azure cli disabled via use_cli=false")
	}
	if strings.EqualFold(p.cfg.firstEnv("FABRIC_USE_CLI", "AZURE_USE_CLI", "ARM_USE_CLI"), "false") {
		return Credentials{}, true, errors.New("azure cli disabled via USE_CLI=false")
	}

	tenantID, err := p.cfg.resolveTenantID()
	if err != nil {
		return Credentials{}, false, fmt.Errorf("resolving tenant id for azure cli: %w", err)
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

	opts := &azidentity.AzureCLICredentialOptions{
		AdditionallyAllowedTenants: p.cfg.resolveAuxiliaryTenants(),
		TenantID:                   tenantID,
	}
	cred, err := azidentity.NewAzureCLICredential(opts)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("creating azidentity AzureCLICredential: %w", err)
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return Credentials{}, false, fmt.Errorf("acquiring token via AzureCLICredential: %w", err)
	}
	return Credentials{
		AccessToken: tok.Token,
		ExpiresOn:   tok.ExpiresOn,
		Source:      SourceAzureCLI,
	}, false, nil
}
