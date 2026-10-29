// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ resource.Resource                = &LakehousePermissionResource{}
	_ resource.ResourceWithConfigure   = &LakehousePermissionResource{}
	_ resource.ResourceWithImportState = &LakehousePermissionResource{}
	_ resource.ResourceWithModifyPlan  = &LakehousePermissionResource{}
)

// LakehousePermissionResource manages direct item-level permissions on a Microsoft Fabric Lakehouse.
type LakehousePermissionResource struct {
	client *client.FabricClient
}

// LakehousePermissionResourceModel describes the Terraform state model for fabricext_lakehouse_permission.
type LakehousePermissionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	LakehouseName types.String `tfsdk:"lakehouse_name"`
	LakehouseID   types.String `tfsdk:"lakehouse_id"`
	PrincipalID   types.String `tfsdk:"principal_id"`
	PrincipalType types.String `tfsdk:"principal_type"`
	Permissions   types.Set    `tfsdk:"permissions"`
}

// NewLakehousePermissionResource constructs a new fabricext_lakehouse_permission resource.
func NewLakehousePermissionResource() resource.Resource {
	return &LakehousePermissionResource{}
}

// Metadata sets the resource type name.
func (r *LakehousePermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lakehouse_permission"
}

// Schema defines the schema for fabricext_lakehouse_permission.
func (r *LakehousePermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages direct item-level permissions on a Microsoft Fabric Lakehouse.",
		MarkdownDescription: "Manages direct item-level permissions on a Microsoft Fabric **Lakehouse** for Microsoft Entra users, groups, service principals, or managed identities.\n\n" +
			"~> **Note:** Manages item-level sharing permissions on a Microsoft Fabric Lakehouse for Microsoft Entra principals via the underlying Power BI metadata service. For storage-level table, folder, RLS, and CLS access control, use `fabricext_lakehouse_data_access_role`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier in the format `{workspace_id}/{lakehouse_id}/{principal_type}/{principal_id}`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the Microsoft Fabric workspace containing the Lakehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"lakehouse_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric Lakehouse. At least one of `lakehouse_name` or `lakehouse_id` must be specified.",
				PlanModifiers: []planmodifier.String{
					RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"lakehouse_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Resolved or explicitly specified UUID of the Microsoft Fabric Lakehouse. At least one of `lakehouse_name` or `lakehouse_id` must be specified.",
				PlanModifiers: []planmodifier.String{
					RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"principal_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Microsoft Entra Object ID (UUID) of the user, group, service principal, or managed identity.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"principal_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of Microsoft Entra principal (`User`, `Group`, `ServicePrincipal`, `ManagedIdentity`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ManagedIdentity"),
				},
			},
			"permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of item permissions granted to the principal. Valid permissions for Lakehouse are `Read`, `ReadData`, `ReadAll`, `Reshare`, and `Execute`.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(
						stringvalidator.OneOf("Read", "ReadData", "ReadAll", "Reshare", "Execute"),
					),
				},
			},
		},
	}
}

// Configure injects the FabricClient from the provider context.
func (r *LakehousePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *Data, got: %T", req.ProviderData))
		return
	}
	r.client = data.Client
}

// ModifyPlan reconciles lakehouse_name and lakehouse_id when replacing resources.
func (r *LakehousePermissionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan, state, config LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reconcileItemIdentifiersPlan(
		config.LakehouseName, config.LakehouseID,
		state.LakehouseName, state.LakehouseID,
		&plan.LakehouseName, &plan.LakehouseID,
	)

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// Create grants direct item-level permissions on the Lakehouse.
func (r *LakehousePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var plan LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	resolvedID, resolvedName, diags := reconcileItemIdentifiersCreate(
		ctx, r.client, "Lakehouse", wsID,
		plan.LakehouseName.ValueString(), plan.LakehouseID.ValueString(),
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	normType, err := normalizePrincipalType("Lakehouse", plan.PrincipalType.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("principal_type"), "Invalid Principal Type", err.Error())
		return
	}

	var perms []string
	resp.Diagnostics.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	share := client.ItemDirectShare{
		PrincipalID:   plan.PrincipalID.ValueString(),
		PrincipalType: normType,
		Permissions:   perms,
	}

	if err := r.client.UpsertItemDirectPermissions(ctx, wsID, resolvedID, "Lakehouse", share); err != nil {
		resp.Diagnostics.AddError("Error Granting Lakehouse Permission", fmt.Sprintf("Failed to grant permissions on Lakehouse %q in workspace %q: %s", resolvedID, wsID, err))
		return
	}

	plan.LakehouseID = types.StringValue(resolvedID)
	plan.LakehouseName = types.StringValue(resolvedName)
	plan.PrincipalType = types.StringValue(normType)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s/%s", wsID, resolvedID, normType, plan.PrincipalID.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the resource state from the Power BI metadata and Fabric APIs.
func (r *LakehousePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()
	principalID := state.PrincipalID.ValueString()

	// 1. Verify item exists by immutable ID
	item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		var typeErr *client.UnsupportedItemTypeError
		if errors.As(err, &typeErr) {
			resp.Diagnostics.AddError("Item Type Mismatch", typeErr.Error())
			return
		}
		resp.Diagnostics.AddError("Error Reading Lakehouse", fmt.Sprintf("Failed to read Lakehouse %q in workspace %q: %s", lhID, wsID, err))
		return
	}

	// Update display name if renamed upstream
	if item.DisplayName != state.LakehouseName.ValueString() {
		state.LakehouseName = types.StringValue(item.DisplayName)
		r.client.InvalidateItemNameCache(wsID, "Lakehouse")
	}

	// 2. Read direct permissions
	share, err := r.client.GetItemDirectPermissions(ctx, wsID, lhID, "Lakehouse", principalID)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Lakehouse Permission", fmt.Sprintf("Failed to read permissions on Lakehouse %q for principal %q: %s", lhID, principalID, err))
		return
	}

	if share == nil {
		// Share was revoked out of band
		resp.State.RemoveResource(ctx)
		return
	}

	managedPerms := filterManagedPermissions(share.Permissions, "Lakehouse")
	if len(managedPerms) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	permElements := make([]types.String, len(managedPerms))
	for i, p := range managedPerms {
		permElements[i] = types.StringValue(p)
	}
	permSet, diags := types.SetValueFrom(ctx, types.StringType, permElements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Permissions = permSet
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates permissions in-place.
func (r *LakehousePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var plan, state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()

	var perms []string
	resp.Diagnostics.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	share := client.ItemDirectShare{
		PrincipalID:   state.PrincipalID.ValueString(),
		PrincipalType: state.PrincipalType.ValueString(),
		Permissions:   perms,
	}

	if err := r.client.UpsertItemDirectPermissions(ctx, wsID, lhID, "Lakehouse", share); err != nil {
		resp.Diagnostics.AddError("Error Updating Lakehouse Permission", fmt.Sprintf("Failed to update permissions on Lakehouse %q: %s", lhID, err))
		return
	}

	state.Permissions = plan.Permissions
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete revokes direct permissions on the Lakehouse.
func (r *LakehousePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()
	principalID := state.PrincipalID.ValueString()

	if err := r.client.RevokeItemDirectPermissions(ctx, wsID, lhID, "Lakehouse", principalID); err != nil {
		if client.IsNotFound(err) {
			return // Item already deleted
		}
		resp.Diagnostics.AddError("Error Revoking Lakehouse Permission", fmt.Sprintf("Failed to revoke permissions on Lakehouse %q for principal %q: %s", lhID, principalID, err))
		return
	}
}

// ImportState imports a lakehouse permission resource from composite ID.
func (r *LakehousePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	wsID, lhID, principalType, principalID, err := parseItemPermissionImportID(req.ID, "Lakehouse")
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import Identifier", err.Error())
		return
	}

	lhName := ""
	if !uuidRegex.MatchString(lhID) {
		lhName = lhID
		resolvedID, err := r.client.GetItemIDByName(ctx, wsID, "Lakehouse", lhName)
		if err != nil {
			resp.Diagnostics.AddError("Error Resolving Lakehouse", fmt.Sprintf("Failed to resolve lakehouse %q in workspace %q: %s", lhName, wsID, err))
			return
		}
		lhID = resolvedID
	}

	canonicalID := fmt.Sprintf("%s/%s/%s/%s", wsID, lhID, principalType, principalID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), canonicalID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), wsID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("lakehouse_id"), lhID)...)
	if lhName != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("lakehouse_name"), lhName)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_type"), principalType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_id"), principalID)...)
}
