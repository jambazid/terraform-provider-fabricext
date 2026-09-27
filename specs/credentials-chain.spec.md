---
name: Native Microsoft Entra ID Credential Chain
description: Ordered azidentity credential resolution chain matching official Fabric provider parity with OIDC fallthrough, in-memory token caching, and secret redaction
targets:
  - ../internal/credentials/chain.go
  - ../internal/credentials/chain_test.go
---

# Native Microsoft Entra ID Credential Chain

## Public Interface

```go
package credentials

const (
    FabricScopePublic       = "https://api.fabric.microsoft.com/.default"
    FabricScopeUSGovernment = "https://api.fabric.microsoft.us/.default"
    FabricScopeChina        = "https://api.fabric.microsoft.cn/.default"
)

type Config struct {
    AccessToken                    string
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
    Environment                    string
    AuxiliaryTenantIDs             []string
    Getenv                         func(string) string
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

## Deterministic Precedence & Fallthrough Rules

- Evaluates credential providers in strict priority matching `microsoft/terraform-provider-fabric`:
  1. `StaticTokenProvider` (`access_token` / `FABRIC_ACCESS_TOKEN`)
  2. `ClientCertificateProvider` (`client_certificate` base64 PKCS#12 bundle or `client_certificate_file_path`)
  3. `ClientSecretProvider` (`tenant_id` + `client_id` + `client_secret` direct or `*_file_path` / `FABRIC_*` / `AZURE_*` / `ARM_*`)
  4. `AzureDevOpsOIDCProvider` (`azure_devops_service_connection_id` + `oidc_request_token` + `oidc_request_url`)
  5. `WorkloadIdentityProvider` (`oidc_token` direct, `oidc_token_file_path`, or `AZURE_FEDERATED_TOKEN_FILE`)
  6. `ManagedIdentityProvider` (`use_msi` / `FABRIC_USE_MSI` / `ARM_USE_MSI`, supporting system-assigned and user-assigned via `client_id`)
  7. `AzureDeveloperCLIProvider` (`use_dev_cli` / `FABRIC_USE_DEV_CLI` via `azidentity.NewAzureDeveloperCLICredential`)
  8. `AzureCLIProvider` (`use_cli` / `FABRIC_USE_CLI` via `azidentity.NewAzureCLICredential`)
  `[@test] ../internal/credentials/chain_test.go::TestChainPrecedenceOrder`
- When `AZURE_TENANT_ID` and `AZURE_CLIENT_ID` are set in the environment without `AZURE_CLIENT_SECRET` (standard GitHub Actions `azure/login` OIDC environment), `ClientSecretProvider` does not fail the chain and cleanly falls through to OIDC and CLI providers
  `[@test] ../internal/credentials/chain_test.go::TestOIDCThreeTupleFallthrough`
- Supports Service Principal Certificate authentication using base64-encoded PKCS#12 bundles or file paths with optional password via `software.sslmate.com/src/go-pkcs12`
  `[@test] ../internal/credentials/chain_test.go::TestClientCertificateProvider_Resolution`
- Supports Azure Developer CLI (`azd`) authentication when `use_dev_cli = true` or `FABRIC_USE_DEV_CLI=true`
  `[@test] ../internal/credentials/chain_test.go::TestAzureDeveloperCLIProvider_Resolution`
- Supports Azure DevOps Workload Identity Federation authentication via `azidentity.NewAzurePipelinesCredential`
  `[@test] ../internal/credentials/chain_test.go::TestAzureDevOpsOIDCProvider_Resolution`
- Resolves credentials from file paths (`client_id_file_path`, `client_secret_file_path`, `tenant_id_file_path`, `oidc_token_file_path`)
  `[@test] ../internal/credentials/chain_test.go::TestFilePathCredentials_Resolution`
- Propagates `environment` to Azure SDK Cloud configuration and sets Fabric audience scopes for `public`, `usgovernment`, and `china`
  `[@test] ../internal/credentials/chain_test.go::TestEnvironmentAndScopeResolution`
- Passes `auxiliary_tenant_ids` to `AdditionallyAllowedTenants` on underlying Azure SDK credentials
  `[@test] ../internal/credentials/chain_test.go::TestAuxiliaryTenantsPropagation`
- When all providers in the chain fail to resolve a token, returns a `*ChainError` wrapping `ErrNoCredentials` that names every attempted provider and its diagnostic reason
  `[@test] ../internal/credentials/chain_test.go::TestErrNoCredentials_ListsAllSources`

## Caching, Refresh & Secret Redaction

- Resolved tokens are cached in memory and reused across concurrent requests until within 2 minutes of `ExpiresOn`, at which point a fresh token is acquired automatically
  `[@test] ../internal/credentials/chain_test.go::TestTokenCachingAndNearExpiryRefresh`
- `Credentials.String()` and `Credentials.GoString()` redact `AccessToken` as `"[REDACTED]"` so raw bearer tokens never leak via `%s`, `%v`, `%+v`, or `%#v` formatting
  `[@test] ../internal/credentials/chain_test.go::TestCredentialsRedaction`
