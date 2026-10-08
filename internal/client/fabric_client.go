// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package client implements the Microsoft Fabric REST/RPC HTTP client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jambazid/terraform-provider-fabricext/internal/credentials"
)

const (
	// DefaultEndpoint is the base URL for the Microsoft Fabric REST API.
	DefaultEndpoint = "https://api.fabric.microsoft.com"

	defaultTimeout        = 60 * time.Second
	defaultMaxRetries     = 5
	defaultMaxETagRetries = 10
	defaultBaseBackoff    = 100 * time.Millisecond
)

// Item represents a Microsoft Fabric workspace item returned by the Items API.
type Item struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// Principal identifies a Microsoft Entra object in item permission calls.
type Principal struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// PermissionScope defines a Path or Action attribute scope in a OneLake Data Access Role.
type PermissionScope struct {
	AttributeName            string   `json:"attributeName"`
	AttributeValueIncludedIn []string `json:"attributeValueIncludedIn"`
}

// ColumnConstraint defines a column-level security constraint (CLS) applied to a table.
type ColumnConstraint struct {
	TablePath    string   `json:"tablePath"`
	ColumnNames  []string `json:"columnNames"`
	ColumnAction []string `json:"columnAction"`
	ColumnEffect string   `json:"columnEffect"`
}

// RowConstraint defines a row-level security predicate (RLS) applied to a table.
type RowConstraint struct {
	TablePath string `json:"tablePath"`
	Value     string `json:"value"`
}

// Constraints defines row-level and column-level security constraints applied to tables.
type Constraints struct {
	Columns []ColumnConstraint `json:"columns,omitempty"`
	Rows    []RowConstraint    `json:"rows,omitempty"`
}

// DecisionRule defines the effect, permission scopes, and optional constraints of a OneLake Data Access Role.
type DecisionRule struct {
	Effect      string            `json:"effect,omitempty"`
	Permission  []PermissionScope `json:"permission"`
	Constraints *Constraints      `json:"constraints,omitempty"`
}

// FabricItemMember defines workspace item-access inheritance for a Data Access Role.
type FabricItemMember struct {
	ItemAccess []string `json:"itemAccess"`
	SourcePath string   `json:"sourcePath"`
}

// MicrosoftEntraMember defines an explicit Entra object member of a Data Access Role.
type MicrosoftEntraMember struct {
	TenantID   string `json:"tenantId"`
	ObjectID   string `json:"objectId"`
	ObjectType string `json:"objectType,omitempty"`
}

// Members holds both Fabric item members and Microsoft Entra members of a role.
type Members struct {
	FabricItemMembers     []FabricItemMember     `json:"fabricItemMembers,omitempty"`
	MicrosoftEntraMembers []MicrosoftEntraMember `json:"microsoftEntraMembers,omitempty"`
}

// DataAccessRole represents a Lakehouse OneLake Data Access Role.
type DataAccessRole struct {
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name"`
	Kind          string         `json:"kind,omitempty"`
	DecisionRules []DecisionRule `json:"decisionRules"`
	Members       *Members       `json:"members,omitempty"`
}

// NotFoundError indicates that a requested Fabric item, permission assignment, or role does not exist.
type NotFoundError struct {
	ResourceType string
	ResourceID   string
	WorkspaceID  string
	Message      string
}

func (e *NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("%s %q not found in workspace %q", e.ResourceType, e.ResourceID, e.WorkspaceID)
}

// IsNotFound reports whether err or any error in its chain is a *NotFoundError or HTTP 404 *APIError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

// APIError represents a structured error response returned by the Microsoft Fabric REST API.
type APIError struct {
	StatusCode  int
	ErrorCode   string `json:"errorCode"`
	Message     string `json:"message"`
	RequestID   string `json:"requestId"`
	IsRetriable bool   `json:"isRetriable"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("fabric API error (HTTP %d, code %s): %s", e.StatusCode, e.ErrorCode, e.Message)
}

// Config configures a FabricClient instance.
type Config struct {
	Endpoint        string
	HTTPClient      *http.Client
	CredentialChain *credentials.Chain
	TokenProvider   func(ctx context.Context) (string, error)
	MaxRetries      int
	MaxETagRetries  int
	BaseBackoff     time.Duration
}

type itemCacheKey struct {
	workspaceID string
	itemType    string
}

// FabricClient is a thread-safe Microsoft Fabric REST API client.
type FabricClient struct {
	baseURL        string
	httpClient     *http.Client
	tokenProvider  func(ctx context.Context) (string, error)
	maxRetries     int
	maxETagRetries int
	baseBackoff    time.Duration

	cacheMu        sync.Mutex
	itemCache      map[itemCacheKey]map[string]string
	lakehouseLocks sync.Map // map[string]*sync.Mutex keyed by workspaceID + "/" + lakehouseID
}

// NewFabricClient creates a new FabricClient with normalized endpoint and retry defaults.
func NewFabricClient(cfg Config) (*FabricClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/v1")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	tokenProvider := cfg.TokenProvider
	if tokenProvider == nil && cfg.CredentialChain != nil {
		chain := cfg.CredentialChain
		tokenProvider = func(ctx context.Context) (string, error) {
			creds, err := chain.Resolve(ctx)
			if err != nil {
				return "", err
			}
			return creds.AccessToken, nil
		}
	}
	if tokenProvider == nil {
		return nil, errors.New("fabric client requires either CredentialChain or TokenProvider")
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	maxETagRetries := cfg.MaxETagRetries
	if maxETagRetries <= 0 {
		maxETagRetries = defaultMaxETagRetries
	}
	baseBackoff := cfg.BaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = defaultBaseBackoff
	}

	return &FabricClient{
		baseURL:        endpoint,
		httpClient:     httpClient,
		tokenProvider:  tokenProvider,
		maxRetries:     maxRetries,
		maxETagRetries: maxETagRetries,
		baseBackoff:    baseBackoff,
		itemCache:      make(map[itemCacheKey]map[string]string),
	}, nil
}

// ToAPIPermission converts a Terraform HCL permission token (or API enum value) to the Fabric REST API enum value.
func ToAPIPermission(perm string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(perm)) {
	case "read":
		return "Read", nil
	case "read_data", "readdata":
		return "ReadData", nil
	case "read_spark", "readall", "read_all":
		return "ReadAll", nil
	case "subscribe_onelake_events", "subscribeonelakeevents":
		return "SubscribeOneLakeEvents", nil
	case "write":
		return "Write", nil
	case "reshare":
		return "Reshare", nil
	default:
		return "", fmt.Errorf("unsupported permission value %q", perm)
	}
}

// FromAPIPermission converts a Fabric REST API permission enum value to its Terraform HCL token.
func FromAPIPermission(apiPerm string) (string, bool) {
	switch strings.TrimSpace(apiPerm) {
	case "Read", "read":
		return "read", true
	case "ReadData", "read_data":
		return "read_data", true
	case "ReadAll", "read_spark":
		return "read_spark", true
	case "SubscribeOneLakeEvents", "subscribe_onelake_events":
		return "subscribe_onelake_events", true
	case "Write", "write":
		return "write", true
	case "Reshare", "reshare":
		return "reshare", true
	default:
		return "", false
	}
}

// ExpandRolePermissions expands a Terraform role_type into the corresponding Fabric API permission slice.
func ExpandRolePermissions(itemType, roleType string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(roleType)) {
	case "read":
		return []string{"Read"}, nil
	case "read_data":
		if strings.EqualFold(itemType, "Warehouse") {
			return nil, fmt.Errorf("role_type %q is not supported for Warehouse", roleType)
		}
		return []string{"Read", "ReadData"}, nil
	case "read_spark":
		if strings.EqualFold(itemType, "Warehouse") {
			return nil, fmt.Errorf("role_type %q is not supported for Warehouse", roleType)
		}
		return []string{"Read", "ReadAll", "SubscribeOneLakeEvents"}, nil
	case "write":
		return []string{"Read", "Write"}, nil
	case "reshare":
		return []string{"Read", "Reshare"}, nil
	default:
		return nil, fmt.Errorf("unsupported role_type %q for %s", roleType, itemType)
	}
}

// CollapseItemRolePermissions maps a principal's granted permissions slice back to the canonical Terraform role_type
// for the specified Fabric item type.
// For Warehouse: write > reshare > read.
// For SQLDatabase and default: write > read_spark > read_data > reshare > read.
func CollapseItemRolePermissions(itemType string, perms []string) (string, error) {
	var normalized []string
	for _, p := range perms {
		if mapped, ok := FromAPIPermission(p); ok {
			if !slices.Contains(normalized, mapped) {
				normalized = append(normalized, mapped)
			}
		} else {
			lower := strings.ToLower(strings.TrimSpace(p))
			if lower != "" && !slices.Contains(normalized, lower) {
				normalized = append(normalized, lower)
			}
		}
	}

	if strings.EqualFold(itemType, "Warehouse") {
		switch {
		case slices.Contains(normalized, "write"):
			return "write", nil
		case slices.Contains(normalized, "reshare"):
			return "reshare", nil
		case slices.Contains(normalized, "read"):
			return "read", nil
		default:
			return "", fmt.Errorf("unable to map permissions %v to a known canonical Warehouse role_type", perms)
		}
	}

	switch {
	case slices.Contains(normalized, "write"):
		return "write", nil
	case slices.Contains(normalized, "read_spark") || slices.Contains(normalized, "subscribe_onelake_events") || slices.Contains(normalized, "readall"):
		return "read_spark", nil
	case slices.Contains(normalized, "read_data"):
		return "read_data", nil
	case slices.Contains(normalized, "reshare"):
		return "reshare", nil
	case slices.Contains(normalized, "read"):
		return "read", nil
	default:
		return "", fmt.Errorf("unable to map permissions %v to a known canonical role_type", perms)
	}
}

// CollapseRolePermissions maps a principal's granted permissions slice back to the canonical Terraform role_type.
// It delegates to CollapseItemRolePermissions with an empty item type for general compatibility.
func CollapseRolePermissions(perms []string) (string, error) {
	return CollapseItemRolePermissions("", perms)
}

// BuildLakehouseRole constructs a OneLake Data Access Role object for a single Entra principal.
func BuildLakehouseRole(_, _, roleName string, scopedPaths, actions []string, tenantID, principalID, principalType string) DataAccessRole {
	paths := slices.Clone(scopedPaths)
	if len(paths) == 0 {
		paths = []string{"*"}
	}
	acts := slices.Clone(actions)
	if len(acts) == 0 {
		acts = []string{"Read"}
	}
	return DataAccessRole{
		Name: roleName,
		Kind: "Policy",
		DecisionRules: []DecisionRule{
			{
				Effect: "Permit",
				Permission: []PermissionScope{
					{
						AttributeName:            "Path",
						AttributeValueIncludedIn: paths,
					},
					{
						AttributeName:            "Action",
						AttributeValueIncludedIn: acts,
					},
				},
			},
		},
		Members: &Members{
			MicrosoftEntraMembers: []MicrosoftEntraMember{
				{
					TenantID:   tenantID,
					ObjectID:   principalID,
					ObjectType: principalType,
				},
			},
		},
	}
}

// GetItemIDByName resolves an item's UUID by (workspaceID, displayName, itemType) using a
// type-isolated cache with a single cache-miss eviction and refresh before returning *NotFoundError.
func (c *FabricClient) GetItemIDByName(ctx context.Context, workspaceID, displayName, itemType string) (string, error) {
	key := itemCacheKey{workspaceID: workspaceID, itemType: itemType}

	c.cacheMu.Lock()
	byName, wasCached := c.itemCache[key]
	if wasCached {
		if id, ok := byName[displayName]; ok {
			c.cacheMu.Unlock()
			return id, nil
		}
		// Cache miss on a previously cached (workspaceID, itemType): evict once and re-fetch below.
		delete(c.itemCache, key)
	}
	c.cacheMu.Unlock()

	freshMap, err := c.fetchWorkspaceItemsByType(ctx, workspaceID, itemType)
	if err != nil {
		return "", err
	}

	c.cacheMu.Lock()
	c.itemCache[key] = freshMap
	id, ok := freshMap[displayName]
	c.cacheMu.Unlock()

	if !ok {
		return "", &NotFoundError{
			ResourceType: itemType,
			ResourceID:   displayName,
			WorkspaceID:  workspaceID,
			Message:      fmt.Sprintf("%s with display name %q not found in workspace %q", itemType, displayName, workspaceID),
		}
	}
	return id, nil
}

func (c *FabricClient) fetchWorkspaceItemsByType(ctx context.Context, workspaceID, itemType string) (map[string]string, error) {
	result := make(map[string]string)
	contToken := ""

	for {
		q := url.Values{}
		if itemType != "" {
			q.Set("type", itemType)
		}
		if contToken != "" {
			q.Set("continuationToken", contToken)
		}
		path := fmt.Sprintf("/v1/workspaces/%s/items", workspaceID)
		if encoded := q.Encode(); encoded != "" {
			path += "?" + encoded
		}

		var page struct {
			Value             []Item `json:"value"`
			ContinuationToken string `json:"continuationToken"`
		}
		if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &page); err != nil {
			return nil, err
		}

		for _, it := range page.Value {
			if itemType == "" || strings.EqualFold(it.Type, itemType) {
				result[it.DisplayName] = it.ID
			}
		}

		if page.ContinuationToken == "" {
			break
		}
		contToken = page.ContinuationToken
	}

	return result, nil
}

// GetItemByID retrieves an item by ID and verifies its type when itemType is non-empty.
func (c *FabricClient) GetItemByID(ctx context.Context, workspaceID, itemID, itemType string) (*Item, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/items/%s", workspaceID, itemID)
	var item Item
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &item); err != nil {
		if IsNotFound(err) {
			return nil, &NotFoundError{
				ResourceType: itemType,
				ResourceID:   itemID,
				WorkspaceID:  workspaceID,
			}
		}
		return nil, err
	}
	if itemType != "" && !strings.EqualFold(item.Type, itemType) {
		return nil, &NotFoundError{
			ResourceType: itemType,
			ResourceID:   itemID,
			WorkspaceID:  workspaceID,
			Message:      fmt.Sprintf("item %q in workspace %q has type %q, expected %q", itemID, workspaceID, item.Type, itemType),
		}
	}
	return &item, nil
}

func itemPermissionsBasePath(workspaceID, itemID, itemType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "warehouse":
		return fmt.Sprintf("/v1/workspaces/%s/warehouses/%s", workspaceID, itemID), nil
	case "sqldatabase", "sql_database":
		return fmt.Sprintf("/v1/workspaces/%s/sqlDatabases/%s", workspaceID, itemID), nil
	default:
		return "", fmt.Errorf("unsupported item type %q for item permissions endpoint", itemType)
	}
}

func toAPIPermissionSlice(perms []string) ([]string, error) {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		apiPerm, err := ToAPIPermission(p)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, apiPerm) {
			out = append(out, apiPerm)
		}
	}
	return out, nil
}

// GrantItemPermissions grants permissions on a Warehouse or SQLDatabase to a Microsoft Entra principal.
func (c *FabricClient) GrantItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, perms []string) error {
	basePath, err := itemPermissionsBasePath(workspaceID, itemID, itemType)
	if err != nil {
		return err
	}
	apiPerms, err := toAPIPermissionSlice(perms)
	if err != nil {
		return err
	}

	body := map[string]any{
		"principal":   principal,
		"permissions": apiPerms,
	}
	_, err = c.doJSON(ctx, http.MethodPost, basePath+"/grantPermissions", nil, body, nil)
	return err
}

// GetItemPermissions reads the current permissions for a principal on a Warehouse or SQLDatabase,
// returning *NotFoundError if either the item or the principal's permission assignment is absent.
func (c *FabricClient) GetItemPermissions(ctx context.Context, workspaceID, itemID, itemType, principalID, principalType string) ([]string, error) {
	basePath, err := itemPermissionsBasePath(workspaceID, itemID, itemType)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Value []struct {
			Principal   Principal `json:"principal"`
			Permissions []string  `json:"permissions"`
		} `json:"value"`
	}
	if _, err := c.doJSON(ctx, http.MethodGet, basePath+"/permissions", nil, nil, &resp); err != nil {
		if IsNotFound(err) {
			return nil, &NotFoundError{
				ResourceType: itemType,
				ResourceID:   itemID,
				WorkspaceID:  workspaceID,
			}
		}
		return nil, err
	}

	for _, assignment := range resp.Value {
		if strings.EqualFold(assignment.Principal.ID, principalID) &&
			(principalType == "" || assignment.Principal.Type == "" || strings.EqualFold(assignment.Principal.Type, principalType)) {
			var hclPerms []string
			for _, p := range assignment.Permissions {
				if mapped, ok := FromAPIPermission(p); ok && !slices.Contains(hclPerms, mapped) {
					hclPerms = append(hclPerms, mapped)
				}
			}
			slices.Sort(hclPerms)
			if len(hclPerms) == 0 {
				break
			}
			return hclPerms, nil
		}
	}

	return nil, &NotFoundError{
		ResourceType: itemType + "Permission",
		ResourceID:   principalID,
		WorkspaceID:  workspaceID,
		Message:      fmt.Sprintf("permissions for principal %q (%s) on %s %q not found in workspace %q", principalID, principalType, itemType, itemID, workspaceID),
	}
}

// UpdateItemPermissions computes the set difference (oldPerms \ newPerms), revokes removed
// privileges first, and then grants newPerms so permission downgrades never leave excess privileges.
func (c *FabricClient) UpdateItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, oldPerms, newPerms []string) error {
	oldAPI, err := toAPIPermissionSlice(oldPerms)
	if err != nil {
		return err
	}
	newAPI, err := toAPIPermissionSlice(newPerms)
	if err != nil {
		return err
	}

	var toRevoke []string
	for _, oldP := range oldAPI {
		if !slices.Contains(newAPI, oldP) {
			toRevoke = append(toRevoke, oldP)
		}
	}

	if len(toRevoke) > 0 {
		if err := c.RevokeItemPermissions(ctx, workspaceID, itemID, itemType, principal, toRevoke); err != nil {
			return err
		}
	}

	return c.GrantItemPermissions(ctx, workspaceID, itemID, itemType, principal, newAPI)
}

// RevokeItemPermissions revokes the specified permissions from a principal on a Warehouse or SQLDatabase,
// treating HTTP 404 Not Found as idempotent success.
func (c *FabricClient) RevokeItemPermissions(ctx context.Context, workspaceID, itemID, itemType string, principal Principal, perms []string) error {
	basePath, err := itemPermissionsBasePath(workspaceID, itemID, itemType)
	if err != nil {
		return err
	}
	apiPerms, err := toAPIPermissionSlice(perms)
	if err != nil {
		return err
	}

	body := map[string]any{
		"principal":   principal,
		"permissions": apiPerms,
	}
	_, err = c.doJSON(ctx, http.MethodPost, basePath+"/revokePermissions", nil, body, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

func (c *FabricClient) getLakehouseMutex(workspaceID, lakehouseID string) *sync.Mutex {
	key := workspaceID + "/" + lakehouseID
	val, _ := c.lakehouseLocks.LoadOrStore(key, &sync.Mutex{})
	mu, ok := val.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
	}
	return mu
}

func (c *FabricClient) listDataAccessRolesWithETag(ctx context.Context, workspaceID, lakehouseID string) ([]DataAccessRole, string, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/items/%s/dataAccessRoles", workspaceID, lakehouseID)
	var resp struct {
		Value []DataAccessRole `json:"value"`
	}
	headers, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &resp)
	if err != nil {
		if IsNotFound(err) {
			return nil, "", &NotFoundError{
				ResourceType: "Lakehouse",
				ResourceID:   lakehouseID,
				WorkspaceID:  workspaceID,
			}
		}
		return nil, "", err
	}
	etag := headers.Get("ETag")
	if etag == "" {
		etag = headers.Get("Etag")
	}
	return resp.Value, etag, nil
}

// UpsertDataAccessRole performs a mutex-serialized, ETag-guarded Read-Modify-Write on a Lakehouse's
// OneLake Data Access Roles, preserving DefaultReader and all sibling roles.
func (c *FabricClient) UpsertDataAccessRole(ctx context.Context, workspaceID, lakehouseID string, role DataAccessRole) error {
	mu := c.getLakehouseMutex(workspaceID, lakehouseID)
	mu.Lock()
	defer mu.Unlock()

	if role.Kind == "" {
		role.Kind = "Policy"
	}

	path := fmt.Sprintf("/v1/workspaces/%s/items/%s/dataAccessRoles", workspaceID, lakehouseID)

	for attempt := 0; attempt < c.maxETagRetries; attempt++ {
		roles, etag, err := c.listDataAccessRolesWithETag(ctx, workspaceID, lakehouseID)
		if err != nil {
			return err
		}

		updated := make([]DataAccessRole, 0, len(roles)+1)
		found := false
		for _, existing := range roles {
			if strings.EqualFold(existing.Name, role.Name) {
				merged := role
				if merged.ID == "" {
					merged.ID = existing.ID
				}
				updated = append(updated, merged)
				found = true
			} else {
				updated = append(updated, existing)
			}
		}
		if !found {
			updated = append(updated, role)
		}

		reqHeaders := make(http.Header)
		if etag != "" {
			reqHeaders.Set("If-Match", etag)
		}

		body := map[string]any{
			"value": updated,
		}
		_, err = c.doJSON(ctx, http.MethodPut, path, reqHeaders, body, nil)
		if err == nil {
			return nil
		}

		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusPreconditionFailed || apiErr.StatusCode == http.StatusConflict) {
			if err := sleepWithContext(ctx, c.retryDelay("", attempt)); err != nil {
				return err
			}
			continue
		}
		return err
	}

	return fmt.Errorf("exceeded maximum ETag retries (%d) updating Lakehouse %s Data Access Role %q", c.maxETagRetries, lakehouseID, role.Name)
}

// GetDataAccessRole fetches a single named OneLake Data Access Role from a Lakehouse.
func (c *FabricClient) GetDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) (*DataAccessRole, error) {
	roles, _, err := c.listDataAccessRolesWithETag(ctx, workspaceID, lakehouseID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if strings.EqualFold(r.Name, roleName) {
			copyRole := r
			return &copyRole, nil
		}
	}
	return nil, &NotFoundError{
		ResourceType: "DataAccessRole",
		ResourceID:   roleName,
		WorkspaceID:  workspaceID,
		Message:      fmt.Sprintf("OneLake Data Access Role %q not found on Lakehouse %q in workspace %q", roleName, lakehouseID, workspaceID),
	}
}

// DeleteDataAccessRole removes a single named OneLake Data Access Role from a Lakehouse using
// a mutex-serialized, ETag-guarded Read-Modify-Write while preserving all sibling roles.
func (c *FabricClient) DeleteDataAccessRole(ctx context.Context, workspaceID, lakehouseID, roleName string) error {
	mu := c.getLakehouseMutex(workspaceID, lakehouseID)
	mu.Lock()
	defer mu.Unlock()

	path := fmt.Sprintf("/v1/workspaces/%s/items/%s/dataAccessRoles", workspaceID, lakehouseID)

	for attempt := 0; attempt < c.maxETagRetries; attempt++ {
		roles, etag, err := c.listDataAccessRolesWithETag(ctx, workspaceID, lakehouseID)
		if err != nil {
			if IsNotFound(err) {
				return nil
			}
			return err
		}

		remaining := make([]DataAccessRole, 0, len(roles))
		found := false
		for _, existing := range roles {
			if strings.EqualFold(existing.Name, roleName) {
				found = true
				continue
			}
			remaining = append(remaining, existing)
		}
		if !found {
			return nil
		}

		reqHeaders := make(http.Header)
		if etag != "" {
			reqHeaders.Set("If-Match", etag)
		}

		body := map[string]any{
			"value": remaining,
		}
		_, err = c.doJSON(ctx, http.MethodPut, path, reqHeaders, body, nil)
		if err == nil {
			return nil
		}

		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusPreconditionFailed || apiErr.StatusCode == http.StatusConflict) {
			if err := sleepWithContext(ctx, c.retryDelay("", attempt)); err != nil {
				return err
			}
			continue
		}
		return err
	}

	return fmt.Errorf("exceeded maximum ETag retries (%d) deleting Lakehouse %s Data Access Role %q", c.maxETagRetries, lakehouseID, roleName)
}

func (c *FabricClient) doJSON(ctx context.Context, method, path string, headers http.Header, reqBody any, respOut any) (http.Header, error) {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
	}

	fullURL := c.baseURL + path

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		token, err := c.tokenProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve bearer token: %w", err)
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("create HTTP request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, vals := range headers {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute HTTP request %s %s: %w", method, path, err)
		}

		respBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read HTTP response body: %w", readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if respOut != nil && len(respBytes) > 0 {
				if err := json.Unmarshal(respBytes, respOut); err != nil {
					return nil, fmt.Errorf("decode response JSON from %s %s: %w", method, path, err)
				}
			}
			return resp.Header, nil
		}

		if shouldRetryStatus(resp.StatusCode) && attempt < c.maxRetries {
			delay := c.retryDelay(resp.Header.Get("Retry-After"), attempt)
			if err := sleepWithContext(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  http.StatusText(resp.StatusCode),
			Message:    string(respBytes),
		}
		if len(respBytes) > 0 {
			_ = json.Unmarshal(respBytes, apiErr)
		}
		if resp.StatusCode == http.StatusNotFound {
			return resp.Header, &NotFoundError{
				ResourceType: "FabricResource",
				ResourceID:   path,
				Message:      apiErr.Error(),
			}
		}
		return resp.Header, apiErr
	}

	return nil, fmt.Errorf("exceeded max retries (%d) for %s %s", c.maxRetries, method, path)
}

func shouldRetryStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

func (c *FabricClient) retryDelay(retryAfterHeader string, attempt int) time.Duration {
	if retryAfterHeader != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfterHeader)); err == nil && secs > 0 {
			// Cap Retry-After during unit/acc tests if BaseBackoff was configured below 50ms.
			if c.baseBackoff < 50*time.Millisecond {
				return time.Duration(secs) * c.baseBackoff
			}
			return time.Duration(secs) * time.Second
		}
	}
	shift := min(attempt, 5)
	base := c.baseBackoff * time.Duration(1<<shift)
	jitter := (base / 4) * time.Duration((attempt%3)+1) / 3
	return base + jitter
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
