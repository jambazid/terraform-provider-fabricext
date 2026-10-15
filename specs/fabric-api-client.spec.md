---
name: Microsoft Fabric REST API Client and OpenAPI Contract Mock
description: Paginated item lookup cache, Retry-After HTTP backoff, Lakehouse ETag RMW concurrency, and kin-openapi contract validation
targets:
  - ../internal/client/fabric_client.go
  - ../internal/client/fabric_client_test.go
  - ../internal/testutil/fabricmock/server.go
  - ../internal/testutil/fabricmock/server_test.go
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
- `fabricmock.Server` loads the vendored Swagger 2.0 specs in `specs/openapi/` via `kin-openapi` (`openapi2conv.ToV3` + `openapi3filter`) and validates every HTTP request and response at runtime
  `[@test] ../internal/testutil/fabricmock/server_test.go::TestServer_OpenAPIContractValidation`

## Security Error Disambiguation

- `listDataAccessRolesWithETag` intercepts HTTP 404 and 400 responses from `GET /dataAccessRoles` and probes `GetItemByID(ctx, workspaceID, lakehouseID, "")` with an empty type filter:
  - If `GetItemByID` returns `NotFoundError`: returns `&NotFoundError{ResourceType: "Lakehouse"}`.
  - If `GetItemByID` returns non-404 transient errors (HTTP 429, 5xx, context timeout): propagates `itemErr` directly without masking.
  - If `GetItemByID` returns an existing item with `Type == "Lakehouse"`: returns `&OneLakeSecurityNotEnabledError{WorkspaceID: workspaceID, LakehouseID: lakehouseID}`.
  - If `GetItemByID` returns an existing item with `Type != "Lakehouse"`: returns `&UnsupportedItemTypeError{WorkspaceID: workspaceID, ItemID: lakehouseID, ActualType: item.Type}`.
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ErrorDisambiguation`
