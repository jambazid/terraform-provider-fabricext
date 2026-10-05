---
name: Native Microsoft Entra ID Credential Chain
description: Ordered azidentity credential resolution chain with OIDC fallthrough, in-memory token caching, and secret redaction
targets:
  - ../internal/credentials/chain.go
  - ../internal/credentials/chain_test.go
---

# Native Microsoft Entra ID Credential Chain

## Public Interface

```go
package credentials

const FabricScope = "https://api.fabric.microsoft.com/.default"

type Config struct {
    AccessToken  string
    TenantID     string
    ClientID     string
    ClientSecret string
    UseOIDC      bool
    UseMSI       bool
    UseCLI       *bool
    Getenv       func(string) string
}

type Credentials struct {
    AccessToken string
    ExpiresOn   time.Time
    Source      string
}

func (c Credentials) String() string
func (c Credentials) GoString() string

type Chain struct { ... }

func NewChain(cfg Config, opts ...Option) *Chain
func (c *Chain) Resolve(ctx context.Context) (Credentials, error)
func (c *Chain) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
```

`[@test] ../internal/credentials/chain_test.go::TestStaticProvider_Resolution`

## Deterministic Precedence & OIDC Fallthrough Rules

- Evaluates credential providers in strict order: `StaticTokenProvider` (`access_token` / `FABRIC_ACCESS_TOKEN`) $\rightarrow$ `ClientSecretProvider` (`tenant_id` + `client_id` + `client_secret` / `FABRIC_*` / `AZURE_*` / `ARM_*`) $\rightarrow$ `WorkloadIdentityProvider` (`AZURE_FEDERATED_TOKEN_FILE`) $\rightarrow$ `ManagedIdentityProvider` (`use_msi` / `FABRIC_USE_MSI` / `AZURE_USE_MSI`) $\rightarrow$ `AzureCLIProvider` (`azidentity.NewAzureCLICredential`)
  `[@test] ../internal/credentials/chain_test.go::TestChainPrecedenceOrder`
- When `AZURE_TENANT_ID` and `AZURE_CLIENT_ID` are set in the environment without `AZURE_CLIENT_SECRET` (standard GitHub Actions `azure/login` OIDC environment), `ClientSecretProvider` does not fail the chain and cleanly falls through to `WorkloadIdentityProvider` and `AzureCLIProvider`
  `[@test] ../internal/credentials/chain_test.go::TestOIDCThreeTupleFallthrough`
- When all providers in the chain fail to resolve a token, returns a `*ChainError` wrapping `ErrNoCredentials` that names every attempted provider and its diagnostic reason
  `[@test] ../internal/credentials/chain_test.go::TestErrNoCredentials_ListsAllSources`

## Caching, Refresh & Secret Redaction

- Resolved tokens are cached in memory and reused across concurrent requests until within 2 minutes of `ExpiresOn`, at which point a fresh token is acquired automatically
  `[@test] ../internal/credentials/chain_test.go::TestTokenCachingAndNearExpiryRefresh`
- `Credentials.String()` and `Credentials.GoString()` redact `AccessToken` as `"[REDACTED]"` so raw bearer tokens never leak via `%s`, `%v`, `%+v`, or `%#v` formatting
  `[@test] ../internal/credentials/chain_test.go::TestCredentialsRedaction`
