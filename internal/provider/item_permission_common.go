// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

type requiresReplaceIfConfiguredModifier struct{}

func (m requiresReplaceIfConfiguredModifier) Description(_ context.Context) string {
	return "Requires replacement if this attribute is explicitly configured and its value changes."
}

func (m requiresReplaceIfConfiguredModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m requiresReplaceIfConfiguredModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If configuration does not specify this attribute (computed), do not require replacement on upstream rename.
	if req.ConfigValue.IsNull() {
		return
	}
	if req.StateValue.IsNull() {
		return
	}
	if req.PlanValue.Equal(req.StateValue) {
		return
	}
	resp.RequiresReplace = true
}

// RequiresReplaceIfConfigured creates a plan modifier that only requires replacement if the attribute was explicitly configured in HCL.
func RequiresReplaceIfConfigured() planmodifier.String {
	return requiresReplaceIfConfiguredModifier{}
}

func reconcileItemIdentifiersCreate(ctx context.Context, c *client.FabricClient, itemType, wsID, configuredName, configuredID string) (resolvedID, resolvedName string, diags diag.Diagnostics) {
	hasID := strings.TrimSpace(configuredID) != ""
	hasName := strings.TrimSpace(configuredName) != ""

	if !hasID && !hasName {
		diags.AddError(
			fmt.Sprintf("Missing %s Identifier", itemType),
			fmt.Sprintf("At least one of %s_name or %s_id must be configured.", strings.ToLower(itemType), strings.ToLower(itemType)),
		)
		return "", "", diags
	}

	if hasID {
		item, err := c.GetItemByID(ctx, wsID, configuredID, itemType)
		if err != nil {
			if client.IsNotFound(err) {
				diags.AddError(
					fmt.Sprintf("%s Not Found", itemType),
					fmt.Sprintf("%s with ID %q was not found in workspace %q.", itemType, configuredID, wsID),
				)
				return "", "", diags
			}
			var typeErr *client.UnsupportedItemTypeError
			if errors.As(err, &typeErr) {
				diags.AddError(
					"Item Type Mismatch",
					typeErr.Error(),
				)
				return "", "", diags
			}
			diags.AddError(
				fmt.Sprintf("Error Resolving %s", itemType),
				fmt.Sprintf("Failed to retrieve %s with ID %q in workspace %q: %s", itemType, configuredID, wsID, err),
			)
			return "", "", diags
		}
		if hasName && !strings.EqualFold(item.DisplayName, configuredName) {
			diags.AddError(
				fmt.Sprintf("%s Identifier Mismatch", itemType),
				fmt.Sprintf("Configured %s_name %q does not match display name %q of %s with ID %q.", strings.ToLower(itemType), configuredName, item.DisplayName, itemType, configuredID),
			)
			return "", "", diags
		}
		return item.ID, item.DisplayName, diags
	}

	// Resolve by name
	id, err := c.GetItemIDByName(ctx, wsID, configuredName, itemType)
	if err != nil {
		if client.IsNotFound(err) {
			diags.AddError(
				fmt.Sprintf("%s Not Found", itemType),
				fmt.Sprintf("%s with display name %q was not found in workspace %q.", itemType, configuredName, wsID),
			)
			return "", "", diags
		}
		diags.AddError(
			fmt.Sprintf("Error Resolving %s", itemType),
			fmt.Sprintf("Failed to retrieve %s with display name %q in workspace %q: %s", itemType, configuredName, wsID, err),
		)
		return "", "", diags
	}
	return id, configuredName, diags
}

func filterManagedPermissions(perms []string, itemType string) []string {
	var filtered []string
	for _, p := range perms {
		norm := strings.ToLower(strings.TrimSpace(p))
		switch norm {
		case "read":
			filtered = append(filtered, "Read")
		case "readdata", "read_data":
			filtered = append(filtered, "ReadData")
		case "readall", "read_all":
			filtered = append(filtered, "ReadAll")
		case "reshare":
			filtered = append(filtered, "Reshare")
		case "execute":
			if strings.EqualFold(itemType, "Lakehouse") || strings.EqualFold(itemType, "SQLDatabase") {
				filtered = append(filtered, "Execute")
			}
		case "write":
			if strings.EqualFold(itemType, "SQLDatabase") {
				filtered = append(filtered, "Write")
			}
		}
	}
	return filtered
}

func parseItemPermissionImportID(id string, itemType string) (wsID, itemID, principalType, principalID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid import ID %q: expected format {workspace_id}/{%s_id_or_name}/{principal_type}/{principal_id}", id, strings.ToLower(itemType))
	}
	wsID = strings.TrimSpace(parts[0])
	itemID = strings.TrimSpace(parts[1])
	principalType = strings.TrimSpace(parts[2])
	principalID = strings.TrimSpace(parts[3])

	if !uuidRegex.MatchString(wsID) {
		return "", "", "", "", fmt.Errorf("invalid workspace_id %q: must be a valid UUID", wsID)
	}
	if itemID == "" {
		return "", "", "", "", fmt.Errorf("invalid %s identifier: cannot be empty", strings.ToLower(itemType))
	}
	if !uuidRegex.MatchString(principalID) {
		return "", "", "", "", fmt.Errorf("invalid principal_id %q: must be a valid UUID", principalID)
	}
	normType, err := normalizePrincipalType(itemType, principalType)
	if err != nil {
		return "", "", "", "", err
	}
	return wsID, itemID, normType, principalID, nil
}
