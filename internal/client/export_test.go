// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package client

// ValidateClusterURIForTest exports validateClusterURI for package-internal testing.
func ValidateClusterURIForTest(raw string, allowedCustomHost string) error {
	return validateClusterURI(raw, allowedCustomHost)
}

// ExtractClusterURIFromCookieForTest exports extractClusterURIFromCookie for testing.
func ExtractClusterURIFromCookieForTest(cookieHeader string) string {
	return extractClusterURIFromCookie(cookieHeader)
}
