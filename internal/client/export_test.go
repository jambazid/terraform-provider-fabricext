// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"sync"
)

// ValidateClusterURIForTest exports validateClusterURI for package-internal testing.
func ValidateClusterURIForTest(raw string, allowedCustomHost string) error {
	return validateClusterURI(raw, allowedCustomHost)
}

// ValidateRedirectURLForTest exports validateRedirectURL for package-internal testing.
func ValidateRedirectURLForTest(raw string, allowedCustomHost string) error {
	return validateRedirectURL(raw, allowedCustomHost)
}

// ExtractClusterURIFromCookieForTest exports extractClusterURIFromCookie for testing.
func ExtractClusterURIFromCookieForTest(cookieHeader string) string {
	return extractClusterURIFromCookie(cookieHeader)
}

// ScrubAndCapBodyForTest exports scrubAndCapBody for unit testing.
func ScrubAndCapBodyForTest(b []byte) string {
	return scrubAndCapBody(b)
}

// DoPowerBIRequestForTest exports doPowerBIRequest for unit testing.
func (c *FabricClient) DoPowerBIRequestForTest(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.doPowerBIRequest(ctx, req)
}

// GetItemAccessLockForTest exports getItemAccessLock for package-internal testing.
func (c *FabricClient) GetItemAccessLockForTest(workspaceID, itemID string) *sync.Mutex {
	return c.getItemAccessLock(workspaceID, itemID)
}

// GetItemAccessLocksMapForTest exports itemAccessLocks for package-internal testing.
func (c *FabricClient) GetItemAccessLocksMapForTest() *sync.Map {
	return &c.itemAccessLocks
}
