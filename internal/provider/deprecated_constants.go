// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

const (
	// deprecatedItemSharingMessage explains that item-level sharing endpoints do not exist
	// in the public Microsoft Fabric REST API and points practitioners to portal & T-SQL alternatives.
	deprecatedItemSharingMessage = "This resource was tombstoned in v0.3.0 and is scheduled for removal in v0.4.0. The underlying " +
		"Microsoft Fabric REST routes (/permissions, /grantPermissions, /revokePermissions) do not exist in the public " +
		"Fabric REST API. For Warehouses, sharing is currently available only through the Fabric portal UI, and fine-grained " +
		"data access must be configured using in-database T-SQL controls (GRANT/DENY/REVOKE). For SQL Databases, sharing " +
		"must be performed via the Share quick action in the Fabric portal, with fine-grained access controlled via SQL roles " +
		"or Purview policies. Creating or updating this resource fails without making network calls. Removing it from " +
		"configuration is safe and makes no API calls."

	deprecatedItemSharingCreateErrorSummary = "Resource Not Supported by Microsoft Fabric"

	deprecatedItemSharingCreateErrorDetail = deprecatedItemSharingMessage + "\n\n" +
		"Automated Alternative:\n" +
		"1. For fine-grained access control on tables and schemas, execute T-SQL GRANT/DENY statements against the database endpoint.\n" +
		"2. To allow an Entra principal without workspace roles to connect, share the item via the Fabric portal interface with minimum permissions."
)
