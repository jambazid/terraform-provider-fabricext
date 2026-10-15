// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ resource.Resource                = &ItemShareGateResource{}
	_ resource.ResourceWithConfigure   = &ItemShareGateResource{}
	_ resource.ResourceWithImportState = &ItemShareGateResource{}
)

// ItemShareGateResource is a declarative verification waiter for Microsoft Fabric item shares.
type ItemShareGateResource struct {
	client *client.FabricClient
}

// ItemShareGateTimeoutsModel holds optional timeout duration strings.
type ItemShareGateTimeoutsModel struct {
	Create types.String `tfsdk:"create"`
	Read   types.String `tfsdk:"read"`
	Update types.String `tfsdk:"update"`
	Delete types.String `tfsdk:"delete"`
}

// ItemShareGateResourceModel describes the Terraform state for fabricext_item_share_gate.
type ItemShareGateResourceModel struct {
	ID                  types.String                `tfsdk:"id"`
	WorkspaceID         types.String                `tfsdk:"workspace_id"`
	ItemID              types.String                `tfsdk:"item_id"`
	Type                types.String                `tfsdk:"type"`
	PrincipalID         types.String                `tfsdk:"principal_id"`
	PrincipalType       types.String                `tfsdk:"principal_type"`
	ExpectedPermissions types.Set                   `tfsdk:"expected_permissions"`
	MatchType           types.String                `tfsdk:"match_type"`
	Timeouts            *ItemShareGateTimeoutsModel `tfsdk:"timeouts"`
}

// NewItemShareGateResource constructs a new fabricext_item_share_gate resource.
func NewItemShareGateResource() resource.Resource {
	return &ItemShareGateResource{}
}

// Metadata sets the resource type name.
func (r *ItemShareGateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item_share_gate"
}

// Schema defines the schema for fabricext_item_share_gate.
func (r *ItemShareGateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Declarative verification waiter that polls the Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`) until a designated principal holds the expected permissions on an item. It has **no write path**; it never grants or revokes remote permissions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier in the format `{workspace_id}/{item_id}/{principal_id}`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The UUID of the Microsoft Fabric workspace containing the item.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"item_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The UUID of the Microsoft Fabric item.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional item type for route disambiguation (e.g., `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `Report`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"principal_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Microsoft Entra Object ID (UUID) of the principal expected to hold access.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"principal_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional principal type filter (`User`, `Group`, `ServicePrincipal`, `ServicePrincipalProfile`, `EntireTenant`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile", "EntireTenant"),
				},
			},
			"expected_permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of expected permissions that the principal must hold on the item (e.g., `[\"Read\"]` or `[\"Read\", \"ReadData\"]`).",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"match_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("subset"),
				MarkdownDescription: "Matching strategy: `subset` asserts that the principal holds at least the expected permissions (default); `exact` asserts an exact match of permissions.",
				Validators: []validator.String{
					stringvalidator.OneOf("subset", "exact"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": schema.SingleNestedBlock{
				MarkdownDescription: "Timeout configuration for verification operations.",
				Attributes: map[string]schema.Attribute{
					"create": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Timeout for waiting on item share verification during create (e.g. '60m'). Defaults to 60m.",
					},
					"read": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Timeout for reading item share access details (e.g. '10m'). Defaults to 10m.",
					},
					"update": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Timeout for waiting on item share verification during update (e.g. '60m'). Defaults to 60m.",
					},
					"delete": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Timeout for delete state cleanup (e.g. '1m'). Defaults to 1m.",
					},
				},
			},
		},
	}
}

// Configure wires the provider's FabricClient into the resource.
func (r *ItemShareGateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *Data, got: %T", req.ProviderData))
		return
	}
	r.client = pd.Client
}

// Create polls the Fabric Admin API until the principal holds expected permissions.
func (r *ItemShareGateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ItemShareGateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	timeout := 60 * time.Minute
	if plan.Timeouts != nil && !plan.Timeouts.Create.IsNull() && !plan.Timeouts.Create.IsUnknown() && plan.Timeouts.Create.ValueString() != "" {
		parsed, err := time.ParseDuration(plan.Timeouts.Create.ValueString())
		if err != nil || parsed <= 0 {
			resp.Diagnostics.AddAttributeError(path.Root("timeouts").AtName("create"), "Invalid Timeout Duration", fmt.Sprintf("create timeout %q must be a valid positive Go duration string (e.g. '60m')", plan.Timeouts.Create.ValueString()))
			return
		}
		timeout = parsed
	}

	wsID := plan.WorkspaceID.ValueString()
	itemID := plan.ItemID.ValueString()
	principalID := plan.PrincipalID.ValueString()
	matchType := "subset"
	if !plan.MatchType.IsNull() && !plan.MatchType.IsUnknown() && plan.MatchType.ValueString() != "" {
		matchType = plan.MatchType.ValueString()
	}

	var principalTypeFilter string
	if !plan.PrincipalType.IsNull() && !plan.PrincipalType.IsUnknown() {
		principalTypeFilter = plan.PrincipalType.ValueString()
	}

	var itemTypePtr *string
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() && plan.Type.ValueString() != "" {
		v := plan.Type.ValueString()
		itemTypePtr = &v
	}

	var expectedPerms []string
	resp.Diagnostics.Append(plan.ExpectedPermissions.ElementsAs(ctx, &expectedPerms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	detectedType, detectedItemType, err := r.pollForItemShare(pollCtx, wsID, itemID, itemTypePtr, principalID, principalTypeFilter, expectedPerms, matchType, timeout)
	if err != nil {
		resp.Diagnostics.AddError("Item Share Verification Failed", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID))
	if (plan.PrincipalType.IsNull() || plan.PrincipalType.IsUnknown()) && detectedType != "" {
		plan.PrincipalType = types.StringValue(detectedType)
	}
	if (plan.Type.IsNull() || plan.Type.IsUnknown()) && detectedItemType != "" {
		plan.Type = types.StringValue(detectedItemType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read re-verifies the share. If the item or principal access disappears or permissions drift,
// it removes the resource from state so Terraform plans recreation or destroy succeeds cleanly.
func (r *ItemShareGateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ItemShareGateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	timeout := 10 * time.Minute
	if state.Timeouts != nil && !state.Timeouts.Read.IsNull() && !state.Timeouts.Read.IsUnknown() && state.Timeouts.Read.ValueString() != "" {
		parsed, err := time.ParseDuration(state.Timeouts.Read.ValueString())
		if err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	wsID := state.WorkspaceID.ValueString()
	itemID := state.ItemID.ValueString()
	principalID := state.PrincipalID.ValueString()
	matchType := "subset"
	if !state.MatchType.IsNull() && !state.MatchType.IsUnknown() && state.MatchType.ValueString() != "" {
		matchType = state.MatchType.ValueString()
	}

	var principalTypeFilter string
	if !state.PrincipalType.IsNull() && !state.PrincipalType.IsUnknown() {
		principalTypeFilter = state.PrincipalType.ValueString()
	}

	var itemTypePtr *string
	if !state.Type.IsNull() && !state.Type.IsUnknown() && state.Type.ValueString() != "" {
		v := state.Type.ValueString()
		itemTypePtr = &v
	}

	var expectedPerms []string
	resp.Diagnostics.Append(state.ExpectedPermissions.ElementsAs(ctx, &expectedPerms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.ListItemAccessDetails(readCtx, wsID, itemID, itemTypePtr)
	if err != nil {
		if client.IsNotFound(err) {
			// Item was deleted or workspace removed; cleanly remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		var adminErr *client.AdminAPIAccessDeniedError
		if errors.As(err, &adminErr) {
			resp.Diagnostics.AddError("Fabric Admin API Access Denied", adminErr.Error())
			return
		}
		resp.Diagnostics.AddError("Error Reading Microsoft Fabric Item Access Details", err.Error())
		return
	}

	matched, detectedType := checkPrincipalMatch(apiResp.AccessDetails, principalID, principalTypeFilter, expectedPerms, matchType)
	if !matched {
		// Principal share was revoked or permissions drifted; remove from state so plan shows recreation
		resp.State.RemoveResource(ctx)
		return
	}

	if (state.PrincipalType.IsNull() || state.PrincipalType.IsUnknown()) && detectedType != "" {
		state.PrincipalType = types.StringValue(detectedType)
	}
	if (state.Type.IsNull() || state.Type.IsUnknown()) && len(apiResp.AccessDetails) > 0 && apiResp.AccessDetails[0].ItemAccessDetails.Type != "" {
		state.Type = types.StringValue(apiResp.AccessDetails[0].ItemAccessDetails.Type)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update re-polls and re-verifies if expected_permissions or match_type changes.
func (r *ItemShareGateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ItemShareGateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	timeout := 60 * time.Minute
	if plan.Timeouts != nil && !plan.Timeouts.Update.IsNull() && !plan.Timeouts.Update.IsUnknown() && plan.Timeouts.Update.ValueString() != "" {
		parsed, err := time.ParseDuration(plan.Timeouts.Update.ValueString())
		if err != nil || parsed <= 0 {
			resp.Diagnostics.AddAttributeError(path.Root("timeouts").AtName("update"), "Invalid Timeout Duration", fmt.Sprintf("update timeout %q must be a valid positive Go duration string (e.g. '60m')", plan.Timeouts.Update.ValueString()))
			return
		}
		timeout = parsed
	}

	wsID := plan.WorkspaceID.ValueString()
	itemID := plan.ItemID.ValueString()
	principalID := plan.PrincipalID.ValueString()
	matchType := "subset"
	if !plan.MatchType.IsNull() && !plan.MatchType.IsUnknown() && plan.MatchType.ValueString() != "" {
		matchType = plan.MatchType.ValueString()
	}

	var principalTypeFilter string
	if !plan.PrincipalType.IsNull() && !plan.PrincipalType.IsUnknown() {
		principalTypeFilter = plan.PrincipalType.ValueString()
	}

	var itemTypePtr *string
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() && plan.Type.ValueString() != "" {
		v := plan.Type.ValueString()
		itemTypePtr = &v
	}

	var expectedPerms []string
	resp.Diagnostics.Append(plan.ExpectedPermissions.ElementsAs(ctx, &expectedPerms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	detectedType, detectedItemType, err := r.pollForItemShare(pollCtx, wsID, itemID, itemTypePtr, principalID, principalTypeFilter, expectedPerms, matchType, timeout)
	if err != nil {
		resp.Diagnostics.AddError("Item Share Verification Update Failed", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, itemID, principalID))
	if (plan.PrincipalType.IsNull() || plan.PrincipalType.IsUnknown()) && detectedType != "" {
		plan.PrincipalType = types.StringValue(detectedType)
	}
	if (plan.Type.IsNull() || plan.Type.IsUnknown()) && detectedItemType != "" {
		plan.Type = types.StringValue(detectedItemType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the resource from Terraform state with zero network requests.
func (r *ItemShareGateResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

// ImportState imports an existing item share gate by {workspace_id}/{item_id}/{principal_id}.
func (r *ItemShareGateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Expected import identifier in format {workspace_id}/{item_id}/{principal_id}, got: %q", req.ID),
		)
		return
	}

	wsID, itemID, principalID := parts[0], parts[1], parts[2]
	if !uuidRegex.MatchString(wsID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("workspace_id %q must be a valid UUID", wsID))
		return
	}
	if !uuidRegex.MatchString(itemID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("item_id %q must be a valid UUID", itemID))
		return
	}
	if !uuidRegex.MatchString(principalID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("principal_id %q must be a valid UUID", principalID))
		return
	}

	apiResp, err := r.client.ListItemAccessDetails(ctx, wsID, itemID, nil)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Imported Microsoft Fabric Item Access Details", err.Error())
		return
	}

	var matchedEntries []client.ItemAccessDetailsItem
	for _, item := range apiResp.AccessDetails {
		if strings.EqualFold(item.Principal.ID, principalID) {
			matchedEntries = append(matchedEntries, item)
		}
	}

	if len(matchedEntries) == 0 {
		resp.Diagnostics.AddError(
			"Principal Not Found on Item",
			fmt.Sprintf("Principal %q does not hold any access permissions on item %q in workspace %q", principalID, itemID, wsID),
		)
		return
	}

	var detectedItemType string
	if len(matchedEntries) > 0 && matchedEntries[0].ItemAccessDetails.Type != "" {
		detectedItemType = matchedEntries[0].ItemAccessDetails.Type
	}

	foundPrincipalType := matchedEntries[0].Principal.Type

	var grantedUnion []string
	hasPermission := func(target string) bool {
		normTarget := normalizePermission(target)
		for _, g := range grantedUnion {
			if normalizePermission(g) == normTarget {
				return true
			}
		}
		return false
	}

	for _, item := range matchedEntries {
		for _, p := range item.ItemAccessDetails.Permissions {
			if !hasPermission(p) {
				grantedUnion = append(grantedUnion, p)
			}
		}
		for _, ap := range item.ItemAccessDetails.AdditionalPermissions {
			if !hasPermission(ap) {
				grantedUnion = append(grantedUnion, ap)
			}
		}
	}

	expectedSet, sDiags := types.SetValueFrom(ctx, types.StringType, grantedUnion)
	resp.Diagnostics.Append(sDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := ItemShareGateResourceModel{
		ID:                  types.StringValue(req.ID),
		WorkspaceID:         types.StringValue(wsID),
		ItemID:              types.StringValue(itemID),
		Type:                stringOrNull(detectedItemType),
		PrincipalID:         types.StringValue(principalID),
		PrincipalType:       stringOrNull(foundPrincipalType),
		ExpectedPermissions: expectedSet,
		MatchType:           types.StringValue("subset"),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ItemShareGateResource) pollForItemShare(
	ctx context.Context,
	wsID, itemID string,
	itemType *string,
	principalID, principalType string,
	expectedPerms []string,
	matchType string,
	timeout time.Duration,
) (string, string, error) {
	isTestMode := r.client.BaseBackoff() < 50*time.Millisecond
	backoff := 60 * time.Second
	maxBackoff := 300 * time.Second
	if isTestMode {
		backoff = 5 * time.Millisecond
		maxBackoff = 50 * time.Millisecond
	}

	for {
		apiResp, err := r.client.ListItemAccessDetails(ctx, wsID, itemID, itemType)
		if err != nil {
			var adminErr *client.AdminAPIAccessDeniedError
			if errors.As(err, &adminErr) {
				return "", "", adminErr
			}
			var apiErr *client.APIError
			if errors.As(err, &apiErr) {
				if apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusForbidden {
					return "", "", apiErr
				}
			}
			// Transient error or 404 propagation delay
		} else {
			matched, detectedPrincipalType := checkPrincipalMatch(apiResp.AccessDetails, principalID, principalType, expectedPerms, matchType)
			if matched {
				var detectedItemType string
				if len(apiResp.AccessDetails) > 0 {
					detectedItemType = apiResp.AccessDetails[0].ItemAccessDetails.Type
				}
				return detectedPrincipalType, detectedItemType, nil
			}
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return "", "", ctx.Err()
			}
			return "", "", fmt.Errorf(
				"timed out after %s waiting for principal %q to hold expected permissions %v on item %q in workspace %q: share the item in the Microsoft Fabric portal (Manage permissions) with the expected permissions, then re-apply",
				timeout, principalID, expectedPerms, itemID, wsID,
			)
		case <-time.After(backoffWithJitter(backoff, isTestMode)):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func backoffWithJitter(base time.Duration, isTestMode bool) time.Duration {
	if isTestMode {
		return base
	}
	// Add +/- 20% jitter
	jitter := time.Duration(rand.Int63n(int64(base*4/10))) - (base / 5)
	actual := base + jitter
	if actual < 10*time.Second {
		actual = 10 * time.Second
	}
	return actual
}

func normalizePermission(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
}

func checkPrincipalMatch(
	accessDetails []client.ItemAccessDetailsItem,
	principalID, principalType string,
	expectedPerms []string,
	matchType string,
) (bool, string) {
	var matchedEntries []client.ItemAccessDetailsItem
	for _, item := range accessDetails {
		if !strings.EqualFold(item.Principal.ID, principalID) {
			continue
		}
		if principalType != "" && !strings.EqualFold(item.Principal.Type, principalType) {
			continue
		}
		matchedEntries = append(matchedEntries, item)
	}

	if len(matchedEntries) == 0 {
		return false, ""
	}

	detectedPrincipalType := matchedEntries[0].Principal.Type

	var grantedUnion []string
	hasPermission := func(target string) bool {
		normTarget := normalizePermission(target)
		for _, g := range grantedUnion {
			if normalizePermission(g) == normTarget {
				return true
			}
		}
		return false
	}

	for _, item := range matchedEntries {
		for _, p := range item.ItemAccessDetails.Permissions {
			if !hasPermission(p) {
				grantedUnion = append(grantedUnion, p)
			}
		}
		for _, ap := range item.ItemAccessDetails.AdditionalPermissions {
			if !hasPermission(ap) {
				grantedUnion = append(grantedUnion, ap)
			}
		}
	}

	var uniqueExpected []string
	for _, ep := range expectedPerms {
		norm := normalizePermission(ep)
		already := false
		for _, u := range uniqueExpected {
			if normalizePermission(u) == norm {
				already = true
				break
			}
		}
		if !already {
			uniqueExpected = append(uniqueExpected, ep)
		}
	}

	if matchType == "exact" {
		if len(grantedUnion) != len(uniqueExpected) {
			return false, detectedPrincipalType
		}
		for _, ep := range uniqueExpected {
			if !hasPermission(ep) {
				return false, detectedPrincipalType
			}
		}
		return true, detectedPrincipalType
	}

	// Subset match: all expected permissions must be granted
	for _, ep := range uniqueExpected {
		if !hasPermission(ep) {
			return false, detectedPrincipalType
		}
	}
	return true, detectedPrincipalType
}
