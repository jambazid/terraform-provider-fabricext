---
name: Microsoft Fabric REST API Client and OpenAPI Contract Mock
description: Paginated item lookup cache, Retry-After HTTP backoff, Lakehouse ETag RMW concurrency, and kin-openapi contract validation
targets:
  - ../internal/client/fabric_client.go
  - ../internal/client/fabric_client_test.go
  - ../internal/client/item_permissions.go
  - ../internal/client/item_permissions_test.go
  - ../internal/testutil/fabricmock/server.go
  - ../internal/testutil/fabricmock/server_test.go
  - ../internal/testutil/pbimock/server.go
  - ../internal/testutil/pbimock/server_test.go
---

# Fabric REST Client Spec

## Client Interface

```go
package client

type FabricClient struct { ... }

func NewFabricClient(cfg Config) (*FabricClient, error)
func (c *FabricClient) GetItemIDByName(ctx context.Context, workspaceID, displayName, itemType string) (string, error)
func (c *FabricClient) GetItemByID(ctx context.Context, workspaceID, itemID, itemType string) (*Item, error)

func (c *FabricClient) UpsertDataAccessRole(ctx context.Context, workspaceID, lakehouseID string, role DataAccessRole) error
func (c *FabricClient) GetDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) (*DataAccessRole, error)
func (c *FabricClient) DeleteDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) error
func (c *FabricClient) DiscoverClusterURI(ctx context.Context) (string, error)
```

`[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`

## Paginated Discovery & Cache

- `GetItemIDByName` queries `GET /v1/workspaces/{workspaceId}/items?type={itemType}`, follows `continuationToken` across all pages, and caches results keyed strictly on `(workspaceID, itemType, displayName)` so companion `SQLEndpoint` items sharing a display name with a `Lakehouse` or `SQLDatabase` never collide
  `[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`
- On a cache miss for a previously cached `(workspaceID, itemType)`, the client evicts the cached entry once and re-fetches from the API before returning `*NotFoundError`
  `[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`

## Lakehouse Roles Concurrency & Pagination

- `ListDataAccessRoles` traverses all pages using `continuationToken` query parameters across the client's configured base URL until `continuationToken` is empty, asserting that the response header `ETag` remains identical across all pages; if an ETag change is detected between pages, the scan aborts as a concurrency conflict and triggers a full retry
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_PaginationAndETagConsistency`
- `normalizeETag` normalizes HTTP `ETag` headers for strong comparison in `If-Match`, rejecting empty strings, weak tags (`W/"..."`), and malformed quotes, while quoting unquoted tokens (`etag-v1` $\rightarrow$ `"etag-v1"`)
  `[@test] ../internal/client/fabric_client_test.go::TestNormalizeETag_Invariants`
- `UpsertDataAccessRole` and `DeleteDataAccessRole` acquire a per-Lakehouse mutex (`workspaceID + "/" + lakehouseID`), `GET` the current `/dataAccessRoles` slice and `ETag` header across all pages, mutate only the target `role_name` (preserving `DefaultReader` and sibling roles), and `PUT` the updated document with normalized `If-Match: <ETag>`
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ConcurrentRMWAndETagConflictRetry`
- When a concurrent modification causes HTTP `412 Precondition Failed` or `409 Conflict`, the client retries the `GET` $\rightarrow$ mutate $\rightarrow$ `PUT` cycle up to `MaxETagRetries` without losing sibling roles
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ConcurrentRMWAndETagConflictRetry`

## Retry Backoff & Validation

- HTTP `429 Too Many Requests` responses honor the `Retry-After` header; transient `502`/`503`/`504` responses retry with exponential backoff respecting `ctx.Done()`
  `[@test] ../internal/client/fabric_client_test.go::TestRetryAfter429And503Backoff`
- `fabricmock.Server` loads the vendored Swagger 2.0 specs in `specs/fabric/` via `kin-openapi` (`openapi2conv.ToV3` + `openapi3filter`) and validates every HTTP request and response at runtime, including `GET /v1/workspaces/{workspaceId}`, `GET /v1/workspaces`, and optional `home-cluster-uri` response headers
  `[@test] ../internal/testutil/fabricmock/server_test.go::TestServer_OpenAPIContractValidation`
- `pbimock.Server` loads `specs/powerbi/metadata/openapi.json` via `kin-openapi` (`openapi3filter`) and validates every HTTP request and response at runtime, including `GET /v1.0/myorg/groups`, root integer `id` on `GET /metadata/access/artifacts/{itemId}`, and `X-PowerBI-Error-Info` + `RequestId` headers on `401`/`403` error simulation
  `[@test] ../internal/testutil/pbimock/server_test.go::TestServer_OpenAPIContractValidation`

## Security Error Disambiguation

- `listDataAccessRolesWithETag` intercepts HTTP 404 and 400 responses from `GET /dataAccessRoles` and probes `GetItemByID(ctx, workspaceID, lakehouseID, "")` with an empty type filter:
  - If `GetItemByID` returns `NotFoundError`: returns `&NotFoundError{ResourceType: "Lakehouse"}`.
  - If `GetItemByID` returns non-404 transient errors (HTTP 429, 5xx, context timeout): propagates `itemErr` directly without masking.
  - If `GetItemByID` returns an existing item with `Type == "Lakehouse"`: returns `&OneLakeSecurityNotEnabledError{WorkspaceID: workspaceID, LakehouseID: lakehouseID}`.
  - If `GetItemByID` returns an existing item with `Type != "Lakehouse"`: returns `&UnsupportedItemTypeError{WorkspaceID: workspaceID, ItemID: lakehouseID, ActualType: item.Type}`.
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ErrorDisambiguation`

## Admin Item Access Details & Throttling

- `ListItemAccessDetails` queries `GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users` with optional query parameter `type`, parses the `ItemAccessDetailsResponse` payload, and handles HTTP 429 `Retry-After` rate limit headers
  `[@test] ../internal/client/fabric_client_test.go::TestListItemAccessDetails_SuccessAndRateLimitRetry`
- When calling `ListItemAccessDetails` returns HTTP 401 Unauthorized or HTTP 403 Forbidden, the client returns a structured `*AdminAPIAccessDeniedError` containing `fault_class` (`ServicePrincipalTenantRoleConflict` when `Tenant.*` application roles are present, otherwise `AdminAPIAccessDenied`), `x_powerbi_error`, `request_id`, and `caller_posture` (`tid`, `appid`, `oid`, `aud`, `roles`, `token_type`, `auth_source`)
  `[@test] ../internal/client/fabric_client_test.go::TestListItemAccessDetails_AdminForbiddenError`
  `[@test] ../internal/client/fabric_client_test.go::TestListItemAccessDetails_SPNTenantRoleConflict401`

## Item Permissions & Cluster Discovery

- `EncodePermissions` and `DecodePermissions` provide a deterministic bijection between high-level permission tokens (`Read`, `ReadData`, `ReadAll`, `Reshare`, `Execute`) and low-level Core and Artifact bitmasks across Warehouses, Lakehouses, and SQL Databases, encoding `Execute` strictly as `coreBitmask |= 64`
  `[@test] ../internal/client/item_permissions_test.go::TestItemPermissions_BitmaskBijection`
- `DiscoverClusterURI` bypasses bootstrap discovery when `powerbi_endpoint` is configured with a regional cluster URL, resolves the Power BI token upfront, inspects caller token posture, and conditionally routes cluster discovery by token type:
  - `delegated-user` tokens execute portal cookie discovery first (`GET {portalURL}/` sending only `Authorization: Bearer <token>`, preserving `Authorization` on same-origin redirects, collecting comma-joined `Set-Cookie` headers, and extracting URL-unescaped `ClusterUri`) with automatic fallback to `home-cluster-uri` discovery if portal bootstrap fails.
  - `app-only` Service Principal and Managed Identity tokens execute REST `home-cluster-uri` discovery exclusively (passive `FabricClient.doJSON` `2xx` header $\rightarrow$ `GET {fabricEndpoint}/v1/workspaces/{workspaceId}` $\rightarrow$ `GET {pbiAPIBase}/v1.0/myorg/groups?$top=1`), skipping interactive `app.powerbi.com` completely (`PortalRequestCount() == 0`).
  - Cached regional cluster hosts are validated via `validateBackendClusterURI` (permitting sovereign `.powerbigov.us` and `.powerbi.cn` on portal/gateway hops while preventing portal or API gateway hosts from being cached as regional backend clusters).
  `[@test] ../internal/client/item_permissions_test.go::TestExtractClusterURIFromCookie`
  `[@test] ../internal/client/item_permissions_test.go::TestDiscoverClusterURI_5HopRedirect`
  `[@test] ../internal/client/item_permissions_test.go::TestDiscoverClusterURI_ConditionalTokenRouting`
- `GetItemDirectPermissions`, `UpsertItemDirectPermissions`, and `RevokeItemDirectPermissions` manage direct item permissions via undocumented Power BI metadata endpoints (`/metadata/access` and `/metadata/access/artifacts/{itemId}`), serializing explicit JSON `null` fields for unused principal properties, capturing root integer `id` (`InternalArtifactID`) and `detail[].aadAppId` on GET for subsequent PUT calls, isolating direct shares from workspace inheritance, and verifying propagation via superset read-back polling on POST and managed-mask polling on PUT
  `[@test] ../internal/client/item_permissions_test.go::TestItemPermissions_LifecycleWithPBIMock`
  `[@test] ../internal/client/item_permissions_test.go::TestItemPermissions_InternalArtifactIDAndAADAppID`
- Non-2xx Power BI metadata and cluster discovery responses return structured default error diagnostics containing `fault_class` (`ServicePrincipalTenantRoleConflict`, `ServicePrincipalMetadataAccessRejected`, `ClusterDiscoveryExhausted`), `x_powerbi_error`, `request_id`, `activity_id`, `cluster_source`, `caller_posture` (`tid`, `appid`, `oid`, `aud`, `roles`, `scp_present`, `token_type`, `auth_source`), and token-scrubbed `response_body` (reading error bodies across all `GET`/`POST`/`PUT` paths while redacting raw JWTs, secrets, cookies, and human `upn`/`email` claims)
  `[@test] ../internal/client/item_permissions_test.go::TestItemPermissions_RichAuthErrorDiagnostics`
- Item permission mutations acquire per-item mutexes (`workspaceID + "/" + itemID`), preventing race conditions and payload clobbering across concurrent goroutines
  `[@test] ../internal/client/item_permissions_test.go::TestItemPermissions_ConcurrentAccessSafety`
