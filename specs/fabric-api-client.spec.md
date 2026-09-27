---
name: Microsoft Fabric REST API Client and OpenAPI Contract Mock
description: Paginated item lookup cache, Retry-After HTTP backoff, Warehouse/SQLDatabase permission CRUD, Lakehouse ETag RMW concurrency, and kin-openapi contract validation
targets:
  - ../internal/client/fabric_client.go
  - ../internal/client/fabric_client_test.go
  - ../internal/testutil/fabricmock/server.go
  - ../internal/testutil/fabricmock/server_test.go
---

# Microsoft Fabric REST API Client and OpenAPI Contract Mock

## Client Interface

```go
package client

type FabricClient struct { ... }

func NewFabricClient(cfg Config) (*FabricClient, error)
func (c *FabricClient) GetItemIDByName(ctx context.Context, workspaceID, displayName, itemType string) (string, error)
func (c *FabricClient) GetItemByID(ctx context.Context, workspaceID, itemID, itemType string) (*Item, error)

func (c *FabricClient) GrantItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, perms []string) error
func (c *FabricClient) GetItemPermissions(ctx context.Context, workspaceID, itemID, itemType, principalID, principalType string) ([]string, error)
func (c *FabricClient) UpdateItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, oldPerms, newPerms []string) error
func (c *FabricClient) RevokeItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, perms []string) error

func (c *FabricClient) UpsertDataAccessRole(ctx context.Context, workspaceID, lakehouseID string, role DataAccessRole) error
func (c *FabricClient) GetDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) (*DataAccessRole, error)
func (c *FabricClient) DeleteDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) error
```

`[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`

## Paginated Item Discovery & Type-Isolated Cache

- `GetItemIDByName` queries `GET /v1/workspaces/{workspaceId}/items?type={itemType}`, follows `continuationToken` across all pages, and caches results keyed strictly on `(workspaceID, itemType, displayName)` so companion `SQLEndpoint` items sharing a display name with a `Lakehouse` or `SQLDatabase` never collide
  `[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`
- On a cache miss for a previously cached `(workspaceID, itemType)`, the client evicts the cached entry once and re-fetches from the API before returning `*NotFoundError`
  `[@test] ../internal/client/fabric_client_test.go::TestGetItemIDByName_PaginationTypeIsolationAndCacheRefresh`

## Item Permission Downgrade Revocation (Warehouse & SQLDatabase)

- `UpdateItemPermissions` computes `toRevoke = oldPerms \ newPerms` and invokes `POST .../revokePermissions` before `POST .../grantPermissions` so role downgrades (e.g., `"write"` $\rightarrow$ `"read"` or `"read_data"` $\rightarrow$ `"read"`) remove excess privileges
  `[@test] ../internal/client/fabric_client_test.go::TestUpdateItemPermissions_DowngradeRevokesExcessPrivileges`
- `RevokeItemPermissions` treats HTTP `404 Not Found` as idempotent success
  `[@test] ../internal/client/fabric_client_test.go::TestUpdateItemPermissions_DowngradeRevokesExcessPrivileges`

## Lakehouse OneLake Data Access Roles Concurrency & ETag RMW

- `UpsertDataAccessRole` and `DeleteDataAccessRole` acquire a per-Lakehouse mutex (`workspaceID + "/" + lakehouseID`), `GET` the current `/dataAccessRoles` slice and `ETag` header, mutate only the target `role_name` (preserving `DefaultReader` and sibling roles), and `PUT` the updated document with `If-Match: <ETag>`
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ConcurrentRMWAndETagConflictRetry`
- When a concurrent modification causes HTTP `412 Precondition Failed` or `409 Conflict`, the client retries the `GET` $\rightarrow$ mutate $\rightarrow$ `PUT` cycle up to `MaxETagRetries` without losing sibling roles
  `[@test] ../internal/client/fabric_client_test.go::TestLakehouseDataAccessRoles_ConcurrentRMWAndETagConflictRetry`

## HTTP `429` `Retry-After`, 5xx Backoff & OpenAPI Contract Validation

- HTTP `429 Too Many Requests` responses honor the `Retry-After` header; transient `502`/`503`/`504` responses retry with exponential backoff respecting `ctx.Done()`
  `[@test] ../internal/client/fabric_client_test.go::TestRetryAfter429And503Backoff`
- `fabricmock.Server` loads the vendored Swagger 2.0 specs in `specs/openapi/` via `kin-openapi` (`openapi2conv.ToV3` + `openapi3filter`) and validates every HTTP request and response at runtime
  `[@test] ../internal/testutil/fabricmock/server_test.go::TestServer_OpenAPIContractValidation`
