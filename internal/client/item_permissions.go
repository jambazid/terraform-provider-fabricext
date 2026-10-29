// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ItemDirectShare represents a direct item-level permission grant on a Fabric item.
type ItemDirectShare struct {
	PrincipalID            string
	PrincipalType          string // "User", "Group", "ServicePrincipal", "ManagedIdentity"
	Permissions            []string
	RawPermissions         int64
	RawArtifactPermissions int64
	InternalUserID         int
	InternalGroupID        int
	IsServicePrincipal     bool
	Notify                 bool
	NotificationMessage    string
}

type accessRequestBody struct {
	Dashboards []any                `json:"dashboards"`
	Reports    []any                `json:"reports"`
	Workbooks  []any                `json:"workbooks"`
	Models     []any                `json:"models"`
	Datamarts  []any                `json:"datamarts"`
	Artifacts  []artifactGrantEntry `json:"artifacts"`
}

type notifyAccessRequestBody struct {
	Dashboards               []any                `json:"dashboards"`
	Reports                  []any                `json:"reports"`
	Workbooks                []any                `json:"workbooks"`
	Models                   []any                `json:"models"`
	Datamarts                []any                `json:"datamarts"`
	Artifacts                []artifactGrantEntry `json:"artifacts"`
	EmailNotificationMessage string               `json:"emailNotificationMessage"`
}

type artifactGrantEntry struct {
	ID                  int    `json:"id"`
	ArtifactObjectID    string `json:"artifactObjectId"`
	Permissions         int64  `json:"permissions"`
	ArtifactPermissions int64  `json:"artifactPermissions"`
	UserID              *int   `json:"userId,omitempty"`
	GroupID             *int   `json:"groupId,omitempty"`
	UserObjectID        string `json:"userObjectId,omitempty"`
	GroupObjectID       string `json:"groupObjectId,omitempty"`
	IsServicePrincipal  bool   `json:"isServicePrincipal,omitempty"`
}

type artifactAccessResponse struct {
	ArtifactAccessEntities []struct {
		ObjectID    string              `json:"objectId"`
		DisplayName string              `json:"displayName"`
		Detail      []accessDetailEntry `json:"detail"`
	} `json:"artifactAccessEntities"`
}

type accessDetailEntry struct {
	User                string `json:"user"`
	UserType            string `json:"userType"`
	UserID              int    `json:"userId"`
	GroupID             int    `json:"groupId"`
	AADAppID            string `json:"aadAppId"`
	UserObjectID        string `json:"userObjectId"`
	GroupObjectID       string `json:"groupObjectId"`
	GroupType           string `json:"groupType"`
	IsServicePrincipal  bool   `json:"isServicePrincipal"`
	Permissions         int64  `json:"permissions"`
	ArtifactPermissions int64  `json:"artifactPermissions"`
	AccessSource        *int   `json:"accessSource"` // null for direct item grants
}

func (c *FabricClient) getItemAccessLock(workspaceID, itemID string) *sync.Mutex {
	key := workspaceID + "/" + itemID
	val, _ := c.itemAccessLocks.LoadOrStore(key, &sync.Mutex{})
	lock, ok := val.(*sync.Mutex)
	if !ok {
		lock = &sync.Mutex{}
		c.itemAccessLocks.Store(key, lock)
	}
	return lock
}

func (c *FabricClient) getPowerBIToken(ctx context.Context) (string, error) {
	if c.powerBITokenProvider != nil {
		return c.powerBITokenProvider(ctx)
	}
	return "", errors.New("no token provider configured for Power BI metadata API")
}

// DiscoverClusterURI resolves and validates the regional Power BI cluster URI.
func (c *FabricClient) DiscoverClusterURI(ctx context.Context) (string, error) {
	c.clusterURIMu.RLock()
	if c.cachedClusterURI != "" {
		uri := c.cachedClusterURI
		c.clusterURIMu.RUnlock()
		return uri, nil
	}
	c.clusterURIMu.RUnlock()

	c.clusterURIMu.Lock()
	defer c.clusterURIMu.Unlock()
	if c.cachedClusterURI != "" {
		return c.cachedClusterURI, nil
	}

	portalURL := c.powerBIEndpoint
	if portalURL == "" {
		portalURL = "https://app.powerbi.com"
	}

	parsedPortal, err := url.Parse(portalURL)
	if err != nil {
		return "", fmt.Errorf("parsing portal url: %w", err)
	}

	customHost := ""
	if c.powerBIEndpoint != "" {
		customHost = parsedPortal.Hostname()
	}

	if err := validateClusterURI(portalURL, customHost); err != nil {
		return "", fmt.Errorf("invalid portal url %q: %w", portalURL, err)
	}

	if isLoopback(parsedPortal.Hostname()) {
		// Probe loopback test server
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, portalURL+"/", nil)
		if reqErr == nil {
			tok, _ := c.getPowerBIToken(ctx)
			if tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
			resp, doErr := c.httpClient.Do(req)
			if doErr == nil {
				_ = resp.Body.Close()
				for _, sc := range resp.Header.Values("Set-Cookie") {
					if cluster := extractClusterURIFromCookie(sc); cluster != "" {
						cluster = strings.TrimRight(cluster, "/")
						if valErr := validateClusterURI(cluster, parsedPortal.Hostname()); valErr == nil {
							c.cachedClusterURI = cluster
							return cluster, nil
						}
					}
				}
			}
		}
		c.cachedClusterURI = strings.TrimRight(portalURL, "/")
		return c.cachedClusterURI, nil
	}

	// 5-hop redirect discovery
	currentURL := portalURL
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: c.httpClient.Transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	initialHost := parsedPortal.Hostname()
	for hop := 0; hop < 5; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return "", fmt.Errorf("building cluster discovery request: %w", err)
		}
		// Protect bearer token: only send Authorization if target matches initial host
		if req.URL.Hostname() == initialHost {
			tok, err := c.getPowerBIToken(ctx)
			if err == nil && tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("executing cluster discovery request: %w", err)
		}
		_ = resp.Body.Close()

		for _, sc := range resp.Header.Values("Set-Cookie") {
			if cluster := extractClusterURIFromCookie(sc); cluster != "" {
				cluster = strings.TrimRight(cluster, "/")
				if valErr := validateClusterURI(cluster, customHost); valErr == nil {
					c.cachedClusterURI = cluster
					return cluster, nil
				}
			}
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				break
			}
			nextURL, err := url.Parse(loc)
			if err != nil {
				break
			}
			resolved := req.URL.ResolveReference(nextURL)
			if valErr := validateRedirectURL(resolved.String(), customHost); valErr != nil {
				return "", fmt.Errorf("untrusted redirect target %q during cluster discovery: %w", resolved.String(), valErr)
			}
			currentURL = resolved.String()
			continue
		}
		break
	}

	if c.powerBIEndpoint != "" && !strings.HasSuffix(parsedPortal.Hostname(), "powerbi.com") {
		cleanPortal := strings.TrimRight(portalURL, "/")
		if err := validateClusterURI(cleanPortal, customHost); err == nil {
			c.cachedClusterURI = cleanPortal
			return cleanPortal, nil
		}
	}

	return "", fmt.Errorf("failed to discover Power BI cluster URI from %s (no ClusterUri cookie in response)", portalURL)
}

func extractClusterURIFromCookie(cookieHeader string) string {
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "clusteruri=") {
			val := part[len("clusteruri="):]
			val = strings.Trim(val, `"'`)
			if unescaped, err := url.QueryUnescape(val); err == nil {
				val = unescaped
			}
			val = strings.Trim(val, `"'`)
			if idx := strings.LastIndex(val, "http://"); idx != -1 {
				return val[idx:]
			}
			if idx := strings.LastIndex(val, "https://"); idx != -1 {
				return val[idx:]
			}
			if val != "" && !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
				return "https://" + val
			}
			return val
		}
	}
	return ""
}

func validateRedirectURL(raw string, allowedCustomHost string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parsing redirect url: %w", err)
	}
	if u.Scheme != "https" {
		if u.Scheme != "http" || !isLoopback(u.Hostname()) {
			return fmt.Errorf("redirect url scheme must be https, got %q", u.Scheme)
		}
	}
	if u.User != nil {
		return errors.New("redirect url must not contain credentials")
	}
	host := strings.ToLower(u.Hostname())
	if isLoopback(host) {
		return nil
	}
	if allowedCustomHost != "" && host == strings.ToLower(allowedCustomHost) {
		return nil
	}
	allowedSuffixes := []string{
		".analysis.windows.net",
		".analysis.usgovcloudapi.net",
		".analysis.chinacloudapi.cn",
		".pbidedicated.windows.net",
		".powerbi.com",
	}
	for _, suffix := range allowedSuffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return nil
		}
	}
	return fmt.Errorf("redirect host %q is not an authorized Power BI domain", host)
}

func validateClusterURI(raw string, allowedCustomHost string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parsing cluster uri: %w", err)
	}
	if u.Scheme != "https" {
		if u.Scheme != "http" || !isLoopback(u.Hostname()) {
			return fmt.Errorf("cluster uri scheme must be https, got %q", u.Scheme)
		}
	}
	if u.User != nil {
		return errors.New("cluster uri must not contain credentials")
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("cluster uri must not contain path segments")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("cluster uri must not contain query parameters or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if isLoopback(host) {
		return nil
	}
	if allowedCustomHost != "" && host == strings.ToLower(allowedCustomHost) {
		return nil
	}
	allowedSuffixes := []string{
		".analysis.windows.net",
		".analysis.usgovcloudapi.net",
		".analysis.chinacloudapi.cn",
		".pbidedicated.windows.net",
		".powerbi.com",
	}
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return nil
		}
	}
	return fmt.Errorf("cluster uri host %q is not an authorized Power BI domain", host)
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.")
}

func normalizeWorkloadType(itemType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "warehouse", "datawarehouse":
		return "Warehouse", nil
	case "lakehouse":
		return "Lakehouse", nil
	case "sqldatabase", "sql_database", "sqldbnative":
		return "SQLDatabase", nil
	default:
		return "", fmt.Errorf("unsupported item type %q for item permissions", itemType)
	}
}

// EncodePermissions encodes a slice of permission strings into core and artifact bitmasks.
func EncodePermissions(itemType string, perms []string) (coreBitmask int64, artifactBitmask int64, err error) {
	normType, err := normalizeWorkloadType(itemType)
	if err != nil {
		return 0, 0, err
	}

	for _, p := range perms {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "read":
			coreBitmask |= 1
			artifactBitmask |= 1
		case "write":
			if normType != "SQLDatabase" {
				return 0, 0, fmt.Errorf("permission %q is not supported on %s", p, normType)
			}
			coreBitmask |= 2
		case "readdata", "read_data":
			artifactBitmask |= 2
		case "readall", "read_all":
			artifactBitmask |= 4
		case "reshare":
			coreBitmask |= 4
			artifactBitmask |= 8
		case "execute":
			if normType == "Warehouse" {
				return 0, 0, fmt.Errorf("permission %q is not supported on Warehouse", p)
			}
			coreBitmask |= 64
			artifactBitmask |= 64
		default:
			return 0, 0, fmt.Errorf("unsupported permission %q for %s", p, normType)
		}
	}

	if len(perms) > 0 {
		coreBitmask |= 1
		artifactBitmask |= 1
	}

	return coreBitmask, artifactBitmask, nil
}

// DecodePermissions decodes core and artifact bitmasks into a deterministic sorted slice of permission strings.
func DecodePermissions(itemType string, coreBitmask, artifactBitmask int64) ([]string, error) {
	normType, err := normalizeWorkloadType(itemType)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool)

	if (coreBitmask&1 != 0) || (artifactBitmask&1 != 0) {
		permSet["Read"] = true
	}
	if normType == "SQLDatabase" && coreBitmask&2 != 0 {
		permSet["Write"] = true
	}
	if artifactBitmask&2 != 0 {
		permSet["ReadData"] = true
	}
	if artifactBitmask&4 != 0 {
		permSet["ReadAll"] = true
	}
	if (coreBitmask&4 != 0) || (artifactBitmask&8 != 0) {
		permSet["Reshare"] = true
	}
	if normType != "Warehouse" && ((coreBitmask&64 != 0) || (artifactBitmask&64 != 0)) {
		permSet["Execute"] = true
	}

	var out []string
	order := []string{"Read", "Write", "ReadData", "ReadAll", "Reshare", "Execute"}
	for _, p := range order {
		if permSet[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

var bearerTokenRegex = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`)

func scrubAndCapBody(body []byte) string {
	const maxCap = 4096
	s := string(body)
	if len(s) > maxCap {
		s = s[:maxCap] + "... [truncated]"
	}
	return bearerTokenRegex.ReplaceAllString(s, "Bearer [REDACTED]")
}

func (c *FabricClient) setPowerBIHeaders(req *http.Request, token string, isJSON bool) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("x-powerbi-hostenv", "Power BI Web App")
	req.Header.Set("Referer", "https://app.fabric.microsoft.com/")
	req.Header.Set("activityid", uuid.NewString())
	req.Header.Set("requestid", uuid.NewString())
	if isJSON {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
}

func (c *FabricClient) doPowerBIRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	maxRetries := c.maxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseBackoff := c.baseBackoff
	if baseBackoff <= 0 {
		baseBackoff = 250 * time.Millisecond
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("replaying request body: %w", err)
			}
			req.Body = body
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				backoff := baseBackoff * (1 << attempt)
				if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, err
		}

		if shouldRetryStatus(resp.StatusCode) {
			if attempt < maxRetries {
				delay := c.retryDelay(resp.Header.Get("Retry-After"), attempt)
				_ = resp.Body.Close()
				if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
		}

		return resp, nil
	}

	return nil, errors.New("exceeded maximum retries for powerbi request")
}

// GetItemDirectPermissions reads direct item-level permissions for a principal on a Fabric item.
func (c *FabricClient) GetItemDirectPermissions(ctx context.Context, workspaceID, itemID, itemType, principalID string) (*ItemDirectShare, error) {
	cluster, err := c.DiscoverClusterURI(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering cluster uri: %w", err)
	}

	tok, err := c.getPowerBIToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring power bi token: %w", err)
	}

	reqURL := fmt.Sprintf("%s/metadata/access/artifacts/%s", cluster, itemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building artifact access request: %w", err)
	}
	c.setPowerBIHeaders(req, tok, false)

	resp, err := c.doPowerBIRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("executing artifact access request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{
			ResourceType: itemType,
			ResourceID:   itemID,
			WorkspaceID:  workspaceID,
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("reading artifact access response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("powerbi API error (HTTP %d): %s", resp.StatusCode, scrubAndCapBody(body))
	}

	var accessResp artifactAccessResponse
	if err := json.Unmarshal(body, &accessResp); err != nil {
		return nil, fmt.Errorf("unmarshaling artifact access response: %w", err)
	}

	for _, entity := range accessResp.ArtifactAccessEntities {
		for _, detail := range entity.Detail {
			if detail.AccessSource != nil {
				continue // Skip inherited workspace-level assignments
			}
			matched := strings.EqualFold(detail.UserObjectID, principalID) ||
				strings.EqualFold(detail.GroupObjectID, principalID) ||
				strings.EqualFold(detail.AADAppID, principalID)

			if matched {
				decoded, err := DecodePermissions(itemType, detail.Permissions, detail.ArtifactPermissions)
				if err != nil {
					return nil, err
				}
				pType := "User"
				if detail.GroupObjectID != "" {
					pType = "Group"
				} else if detail.IsServicePrincipal || detail.AADAppID != "" {
					pType = "ServicePrincipal"
				}
				return &ItemDirectShare{
					PrincipalID:            principalID,
					PrincipalType:          pType,
					Permissions:            decoded,
					RawPermissions:         detail.Permissions,
					RawArtifactPermissions: detail.ArtifactPermissions,
					InternalUserID:         detail.UserID,
					InternalGroupID:        detail.GroupID,
					IsServicePrincipal:     detail.IsServicePrincipal,
				}, nil
			}
		}
	}

	return nil, nil // No direct share found
}

// UpsertItemDirectPermissions grants or updates direct item-level permissions for a principal.
func (c *FabricClient) UpsertItemDirectPermissions(ctx context.Context, workspaceID, itemID, itemType string, share ItemDirectShare) error {
	lock := c.getItemAccessLock(workspaceID, itemID)
	lock.Lock()
	defer lock.Unlock()

	cluster, err := c.DiscoverClusterURI(ctx)
	if err != nil {
		return fmt.Errorf("discovering cluster uri: %w", err)
	}

	coreBitmask, artifactBitmask, err := EncodePermissions(itemType, share.Permissions)
	if err != nil {
		return fmt.Errorf("encoding permissions: %w", err)
	}

	existing, err := c.GetItemDirectPermissions(ctx, workspaceID, itemID, itemType, share.PrincipalID)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("checking existing item permissions: %w", err)
	}

	method := http.MethodPost
	if existing != nil && !share.Notify {
		method = http.MethodPut
		if existing.InternalUserID != 0 {
			share.InternalUserID = existing.InternalUserID
		}
		if existing.InternalGroupID != 0 {
			share.InternalGroupID = existing.InternalGroupID
		}
	} else if existing != nil && share.Notify {
		method = http.MethodPost
		if existing.InternalUserID != 0 {
			share.InternalUserID = existing.InternalUserID
		}
		if existing.InternalGroupID != 0 {
			share.InternalGroupID = existing.InternalGroupID
		}
	}

	entry := artifactGrantEntry{
		ID:                  1,
		ArtifactObjectID:    itemID,
		Permissions:         coreBitmask,
		ArtifactPermissions: artifactBitmask,
	}

	isSP := share.IsServicePrincipal || strings.EqualFold(share.PrincipalType, "ServicePrincipal") || strings.EqualFold(share.PrincipalType, "ManagedIdentity")
	isGroup := strings.EqualFold(share.PrincipalType, "Group")

	// Strict XOR: Send ONLY internal ID or ONLY Entra Object ID, never both.
	if isGroup {
		if share.InternalGroupID != 0 {
			entry.GroupID = &share.InternalGroupID
		} else {
			entry.GroupObjectID = share.PrincipalID
		}
	} else {
		if share.InternalUserID != 0 {
			entry.UserID = &share.InternalUserID
		} else {
			entry.UserObjectID = share.PrincipalID
		}
		if isSP {
			entry.IsServicePrincipal = true
		}
	}

	var endpointURL string
	var payloadBytes []byte

	if share.Notify {
		endpointURL = fmt.Sprintf("%s/metadata/access/notify", cluster)
		reqBody := notifyAccessRequestBody{
			Dashboards:               []any{},
			Reports:                  []any{},
			Workbooks:                []any{},
			Models:                   []any{},
			Datamarts:                []any{},
			Artifacts:                []artifactGrantEntry{entry},
			EmailNotificationMessage: share.NotificationMessage,
		}
		payloadBytes, err = json.Marshal(reqBody)
	} else {
		endpointURL = fmt.Sprintf("%s/metadata/access", cluster)
		reqBody := accessRequestBody{
			Dashboards: []any{},
			Reports:    []any{},
			Workbooks:  []any{},
			Models:     []any{},
			Datamarts:  []any{},
			Artifacts:  []artifactGrantEntry{entry},
		}
		payloadBytes, err = json.Marshal(reqBody)
	}
	if err != nil {
		return fmt.Errorf("marshaling access request: %w", err)
	}

	tok, err := c.getPowerBIToken(ctx)
	if err != nil {
		return fmt.Errorf("acquiring power bi token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("building access request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payloadBytes)), nil
	}
	c.setPowerBIHeaders(req, tok, true)

	resp, err := c.doPowerBIRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing access request (%s): %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096+1))
		return fmt.Errorf("powerbi API access grant failed (HTTP %d): %s", resp.StatusCode, scrubAndCapBody(body))
	}

	// Read-back verification loop (up to 10 seconds, accelerated in test environments)
	verificationTimeout := 10 * time.Second
	pollInterval := 200 * time.Millisecond
	if c.baseBackoff > 0 && c.baseBackoff < 50*time.Millisecond {
		verificationTimeout = 50 * time.Millisecond
		pollInterval = 5 * time.Millisecond
	}
	deadline := time.Now().Add(verificationTimeout)
	var lastCore, lastArtifact int64
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}

		check, checkErr := c.GetItemDirectPermissions(ctx, workspaceID, itemID, itemType, share.PrincipalID)
		if checkErr == nil && check != nil {
			lastCore = check.RawPermissions
			lastArtifact = check.RawArtifactPermissions
			if check.RawPermissions == coreBitmask && check.RawArtifactPermissions == artifactBitmask {
				return nil
			}
		}

		if sleepErr := sleepWithContext(ctx, pollInterval); sleepErr != nil {
			return sleepErr
		}
	}

	return &SilentPermissionDropError{
		WorkspaceID:      workspaceID,
		ItemID:           itemID,
		PrincipalID:      share.PrincipalID,
		ItemType:         itemType,
		ExpectedCore:     coreBitmask,
		ExpectedArtifact: artifactBitmask,
		ActualCore:       lastCore,
		ActualArtifact:   lastArtifact,
	}
}

// RevokeItemDirectPermissions revokes direct item-level permissions for a principal on a Fabric item.
func (c *FabricClient) RevokeItemDirectPermissions(ctx context.Context, workspaceID, itemID, itemType, principalID string) error {
	lock := c.getItemAccessLock(workspaceID, itemID)
	lock.Lock()
	defer lock.Unlock()

	existing, err := c.GetItemDirectPermissions(ctx, workspaceID, itemID, itemType, principalID)
	if err != nil {
		if IsNotFound(err) {
			return nil // Item already gone
		}
		return fmt.Errorf("reading permissions for revocation: %w", err)
	}
	if existing == nil {
		return nil // Already absent
	}

	cluster, err := c.DiscoverClusterURI(ctx)
	if err != nil {
		return fmt.Errorf("discovering cluster uri: %w", err)
	}

	entry := artifactGrantEntry{
		ID:                  1,
		ArtifactObjectID:    itemID,
		Permissions:         0,
		ArtifactPermissions: 0,
	}

	// Strict XOR: Send ONLY internal ID or ONLY Entra Object ID, never both.
	if strings.EqualFold(existing.PrincipalType, "Group") {
		if existing.InternalGroupID != 0 {
			entry.GroupID = &existing.InternalGroupID
		} else {
			entry.GroupObjectID = principalID
		}
	} else {
		if existing.InternalUserID != 0 {
			entry.UserID = &existing.InternalUserID
		} else {
			entry.UserObjectID = principalID
		}
		if existing.IsServicePrincipal {
			entry.IsServicePrincipal = true
		}
	}

	reqBody := accessRequestBody{
		Dashboards: []any{},
		Reports:    []any{},
		Workbooks:  []any{},
		Models:     []any{},
		Datamarts:  []any{},
		Artifacts:  []artifactGrantEntry{entry},
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling revocation request: %w", err)
	}

	tok, err := c.getPowerBIToken(ctx)
	if err != nil {
		return fmt.Errorf("acquiring power bi token: %w", err)
	}

	endpointURL := fmt.Sprintf("%s/metadata/access", cluster)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("building revocation request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payloadBytes)), nil
	}
	c.setPowerBIHeaders(req, tok, true)

	resp, err := c.doPowerBIRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("executing revocation request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096+1))
		return fmt.Errorf("powerbi API revocation failed (HTTP %d): %s", resp.StatusCode, scrubAndCapBody(body))
	}

	// Read-back verification loop (up to 5 seconds, accelerated in test environments)
	verificationTimeout := 5 * time.Second
	pollInterval := 150 * time.Millisecond
	if c.baseBackoff > 0 && c.baseBackoff < 50*time.Millisecond {
		verificationTimeout = 50 * time.Millisecond
		pollInterval = 5 * time.Millisecond
	}
	deadline := time.Now().Add(verificationTimeout)
	var lastCore, lastArtifact int64
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}

		check, checkErr := c.GetItemDirectPermissions(ctx, workspaceID, itemID, itemType, principalID)
		if checkErr == nil {
			if check == nil || (check.RawPermissions == 0 && check.RawArtifactPermissions == 0) {
				return nil
			}
			lastCore = check.RawPermissions
			lastArtifact = check.RawArtifactPermissions
		}

		if sleepErr := sleepWithContext(ctx, pollInterval); sleepErr != nil {
			return sleepErr
		}
	}

	return &SilentPermissionDropError{
		WorkspaceID:      workspaceID,
		ItemID:           itemID,
		PrincipalID:      principalID,
		ItemType:         itemType,
		ExpectedCore:     0,
		ExpectedArtifact: 0,
		ActualCore:       lastCore,
		ActualArtifact:   lastArtifact,
	}
}

// InvalidateClusterURI clears the cached regional cluster URI.
func (c *FabricClient) InvalidateClusterURI() {
	c.clusterURIMu.Lock()
	defer c.clusterURIMu.Unlock()
	c.cachedClusterURI = ""
}
