---
name: Native Microsoft Entra ID Credential Chain
description: Ordered azidentity credential resolution chain matching official Fabric provider parity with OIDC fallthrough, in-memory token caching, and secret redaction
targets:
  - ../internal/credentials/chain.go
  - ../internal/credentials/chain_test.go
---

# Native Entra Credential Chain

## Public Interface

```go
package credentials

const (
    FabricScopePublic       = "https://api.fabric.microsoft.com/.default"
    FabricScopeUSGovernment = "https://api.fabric.microsoft.us/.default"
    FabricScopeChina        = "https://api.fabric.microsoft.cn/.default"

    PowerBIScopePublic       = "https://analysis.windows.net/powerbi/api/.default"
    PowerBIScopeUSGovernment = "https://analysis.usgovcloudapi.net/powerbi/api/.default"
    PowerBIScopeChina        = "https://analysis.chinacloudapi.cn/powerbi/api/.default"
)

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
func (c *Chain) ResolveForScope(ctx context.Context, scope string) (Credentials, error)
func (c *Chain) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)

func ResolvePowerBIPortalURL(env string) string
func ResolvePowerBIAPIBaseURL(env string) string

type TokenPosture struct {
    AuthSource     string
    TokenType      string
    TenantID       string
    AppID          string
    ObjectID       string
    Audience       string
    Roles          []string
    HasTenantRoles bool
    ScpPresent     bool
}

func InspectTokenPosture(rawToken, authSource string) TokenPosture
func (p TokenPosture) Format() string
```

`[@test] ../internal/credentials/chain_test.go::TestStaticProvider_Resolution`
`[@test] ../internal/credentials/chain_test.go::TestStaticProvider_PowerBIScope`
`[@test] ../internal/credentials/chain_test.go::TestChain_ScopeIsolationAndPowerBIScope`
`[@test] ../internal/credentials/chain_test.go::TestInspectTokenPosture_ClassificationAndRedaction`

## Deterministic Precedence & Fallthrough Rules

- Evaluates credential providers in strict priority matching `microsoft/terraform-provider-fabric`:
  1. `StaticTokenProvider` (`access_token` / `FABRIC_ACCESS_TOKEN` for Fabric; `powerbi_access_token` / `FABRIC_POWERBI_ACCESS_TOKEN` for Power BI)
  2. `ClientCertificateProvider` (`client_certificate` base64 PKCS#12 bundle or `client_certificate_path` / `client_certificate_file_path` across `FABRIC_`, `AZURE_`, and `ARM_` prefixes)
  3. `ClientSecretProvider` (`tenant_id` + `client_id` + `client_secret` direct or `*_file_path` / `FABRIC_*` / `AZURE_*` / `ARM_*` file path environment variables)
  4. `AzureDevOpsOIDCProvider` (`azure_devops_service_connection_id` + `oidc_request_token` + `oidc_request_url`, supporting `ARM_ADO_*` and `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, respecting `use_oidc = false`)
  5. `WorkloadIdentityProvider` (GitHub Actions OIDC via `ACTIONS_ID_TOKEN_REQUEST_URL` & `ACTIONS_ID_TOKEN_REQUEST_TOKEN`, generic `oidc_token` direct, `oidc_token_file_path`, or `AZURE_FEDERATED_TOKEN_FILE` with bounded 30s HTTP client and 4 KiB scrubbed errors)
  6. `ManagedIdentityProvider` (`use_msi` / `FABRIC_USE_MSI` / `ARM_USE_MSI`, honoring explicit `use_msi = false` opt-out before auto-detecting `IDENTITY_ENDPOINT`, supporting system-assigned and user-assigned via `client_id`)
  7. `AzureDeveloperCLIProvider` (`use_dev_cli` / `FABRIC_USE_DEV_CLI` / `AZURE_USE_DEV_CLI` / `ARM_USE_DEV_CLI` via `azidentity.NewAzureDeveloperCLICredential`)
  8. `AzureCLIProvider` (`use_cli` / `FABRIC_USE_CLI` / `ARM_USE_CLI` via `azidentity.NewAzureCLICredential`)
  `[@test] ../internal/credentials/chain_test.go::TestWorkloadIdentity_GitHubActionsOIDC`
  `[@test] ../internal/credentials/chain_test.go::TestValidateConfig_MutualExclusion`
- When `AZURE_TENANT_ID` and `AZURE_CLIENT_ID` are set in the environment without `AZURE_CLIENT_SECRET` (standard GitHub Actions `azure/login` OIDC environment), `ClientSecretProvider` does not fail the chain and cleanly falls through to OIDC and CLI providers
  `[@test] ../internal/credentials/chain_test.go::TestOIDCThreeTupleFallthrough`
- Supports Service Principal Certificate authentication using base64-encoded PKCS#12 bundles or file paths (raw binary or base64 text) with optional password via `software.sslmate.com/src/go-pkcs12`
  `[@test] ../internal/credentials/chain_test.go::TestClientCertificateProvider_Resolution`
- Supports Azure Developer CLI (`azd`) authentication when `use_dev_cli = true` or `FABRIC_USE_DEV_CLI=true`, `AZURE_USE_DEV_CLI=true`, `ARM_USE_DEV_CLI=true`
  `[@test] ../internal/credentials/chain_test.go::TestAzureDeveloperCLIProvider_Resolution`
- Supports Azure DevOps Workload Identity Federation authentication via `azidentity.NewAzurePipelinesCredential`
  `[@test] ../internal/credentials/chain_test.go::TestAzureDevOpsOIDCProvider_Resolution`
- Resolves credentials from file paths (`client_id_file_path`, `client_secret_file_path`, `tenant_id_file_path`, `oidc_token_file_path`, `client_certificate_file_path`) and corresponding environment variables
  `[@test] ../internal/credentials/chain_test.go::TestFilePathCredentials_Resolution`
  `[@test] ../internal/credentials/chain_test.go::TestConfig_FilePathEnvFallbacks`
- Propagates `environment` to Azure SDK Cloud configuration and sets Fabric and Power BI audience scopes, portal URLs, and REST API base URLs (`https://api.powerbi.com`, `https://api.powerbigov.us`, `https://api.powerbi.cn`) for `public`, `usgovernment`, and `china`
  `[@test] ../internal/credentials/chain_test.go::TestEnvironmentAndScopeResolution`
- Passes `auxiliary_tenant_ids` (and `FABRIC_AUXILIARY_TENANT_IDS`, `AZURE_AUXILIARY_TENANT_IDS`, `ARM_AUXILIARY_TENANT_IDS`) to `AdditionallyAllowedTenants` on underlying Azure SDK credentials
  `[@test] ../internal/credentials/chain_test.go::TestAuxiliaryTenantsPropagation`
- When all providers in the chain fail to resolve a token, returns a `*ChainError` wrapping `ErrNoCredentials` that names every attempted provider and its diagnostic reason, appending a scope-mismatch hint when only Fabric static tokens were configured for Power BI scope
  `[@test] ../internal/credentials/chain_test.go::TestErrNoCredentials_ListsAllSources`
  `[@test] ../internal/credentials/chain_test.go::TestChain_PowerBIScopeMismatchHint`

## Caching, Refresh & Secret Redaction

- Resolved tokens are cached in memory and reused across concurrent requests until within 2 minutes of `ExpiresOn`, at which point a fresh token is acquired automatically
  `[@test] ../internal/credentials/chain_test.go::TestTokenCachingAndNearExpiryRefresh`
- Isolates in-memory token caches across Entra ID audience scopes (`Chain.ResolveForScope`), ensuring Fabric and Power BI tokens are cached and refreshed independently without cross-scope leakage or cache collisions under concurrent access
  `[@test] ../internal/credentials/chain_test.go::TestChain_ScopeIsolationAndPowerBIScope`
- Resolves static Power BI tokens (`powerbi_access_token` / `FABRIC_POWERBI_ACCESS_TOKEN`) with `SourceStaticPowerBI`, skipping `staticProvider` and falling through to dynamic providers when unset, maintaining strict scope isolation between Fabric and Power BI tokens
  `[@test] ../internal/credentials/chain_test.go::TestStaticProvider_PowerBIScope`
- `Credentials.String()` and `Credentials.GoString()` redact `AccessToken` as `"[REDACTED]"` so raw bearer tokens never leak via `%s`, `%v`, `%+v`, or `%#v` formatting
  `[@test] ../internal/credentials/chain_test.go::TestCredentialsRedaction`
- `InspectTokenPosture` classifies Entra tokens in memory into `app-only` (Service Principal OIDC/Secret/Cert or Managed Identity), `delegated-user` (Azure CLI / interactive user), or `opaque`, extracting non-secret directory identifiers (`tid`, `appid`/`azp`, `oid`, `aud`, sorted `roles`, `scp_present`) matching official `fabric`/`azurerm`/`azuread`/`azapi` redaction boundaries while never retaining raw JWT strings, cryptographic signatures, or human `upn`/`email` claims
  `[@test] ../internal/credentials/chain_test.go::TestInspectTokenPosture_ClassificationAndRedaction`
