// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &WarehousePermissionResource{}
	_ resource.ResourceWithConfigure   = &WarehousePermissionResource{}
	_ resource.ResourceWithImportState = &WarehousePermissionResource{}
)

// WarehousePermissionResource manages item-level permissions on a Microsoft Fabric Warehouse.
type WarehousePermissionResource struct {
	client *client.FabricClient
}

// WarehousePermissionResourceModel describes the Terraform state model for fabricext_warehouse_permission.
type WarehousePermissionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	WarehouseName types.String `tfsdk:"warehouse_name"`
	WarehouseID   types.String `tfsdk:"warehouse_id"`
	PrincipalID   types.String `tfsdk:"principal_id"`
	PrincipalType types.String `tfsdk:"principal_type"`
	RoleType      types.String `tfsdk:"role_type"`
}

// NewWarehousePermissionResource constructs a new fabricext_warehouse_permission resource.
func NewWarehousePermissionResource() resource.Resource {
	return &WarehousePermissionResource{}
}

// Metadata sets the resource type name.
func (r *WarehousePermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_permission"
}

// Schema defines the schema for fabricext_warehouse_permission.
func (r *WarehousePermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages item-level sharing and permissions on a Microsoft Fabric **Warehouse** (`read`, `write`, `reshare`) for a Microsoft Entra principal.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite resource identifier in the format `{workspace_id}/{warehouse_id}/{principal_type}/{principal_id}`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the Microsoft Fabric workspace containing the Warehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"warehouse_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric Warehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"warehouse_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved UUID of the Microsoft Fabric Warehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Microsoft Entra Object ID (UUID) of the principal receiving access.",
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
				Default:             stringdefault.StaticString("Group"),
				MarkdownDescription: "Microsoft Entra principal type. Valid values: `User`, `Group`, `ServicePrincipal`, `ServicePrincipalProfile`. Defaults to `Group`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile"),
				},
			},
			"role_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Permission role to grant on the Warehouse. Valid values: `read` (`Read`), `write` (`Read`, `Write`), `reshare` (`Read`, `Reshare`).",
				Validators: []validator.String{
					stringvalidator.OneOf("read", "write", "reshare"),
				},
			},
		},
	}
}

// Configure wires the provider's FabricClient into the resource.
func (r *WarehousePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create grants the requested Warehouse permissions and populates Terraform state.
func (r *WarehousePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WarehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	whName := plan.WarehouseName.ValueString()
	principal := client.Principal{
		ID:   plan.PrincipalID.ValueString(),
		Type: plan.PrincipalType.ValueString(),
	}

	whID, err := r.client.GetItemIDByName(ctx, wsID, whName, "Warehouse")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Warehouse by Name", err.Error())
		return
	}

	perms, err := client.ExpandRolePermissions("Warehouse", plan.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Warehouse Role Type", err.Error())
		return
	}

	if err := r.client.GrantItemPermissions(ctx, wsID, whID, "Warehouse", principal, perms); err != nil {
		resp.Diagnostics.AddError("Unable to Grant Warehouse Permissions", err.Error())
		return
	}

	plan.WarehouseID = types.StringValue(whID)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s/%s", wsID, whID, principal.Type, principal.ID))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the live Fabric Warehouse permission assignments.
func (r *WarehousePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WarehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	whID := state.WarehouseID.ValueString()
	principalID := state.PrincipalID.ValueString()
	principalType := state.PrincipalType.ValueString()

	item, err := r.client.GetItemByID(ctx, wsID, whID, "Warehouse")
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Warehouse Item", err.Error())
		return
	}
	state.WarehouseName = types.StringValue(item.DisplayName)

	perms, err := r.client.GetItemPermissions(ctx, wsID, whID, "Warehouse", principalID, principalType)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Warehouse Permissions", err.Error())
		return
	}

	roleType, err := client.CollapseRolePermissions(perms)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Warehouse Permissions to Role Type", err.Error())
		return
	}

	state.RoleType = types.StringValue(roleType)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update reconciles Warehouse permissions in-place using set-difference grant and revoke calls.
func (r *WarehousePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WarehousePermissionResourceModel
	var state WarehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldPerms, err := client.ExpandRolePermissions("Warehouse", state.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Previous Warehouse Role Type", err.Error())
		return
	}
	newPerms, err := client.ExpandRolePermissions("Warehouse", plan.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Target Warehouse Role Type", err.Error())
		return
	}

	principal := client.Principal{
		ID:   plan.PrincipalID.ValueString(),
		Type: plan.PrincipalType.ValueString(),
	}

	if err := r.client.UpdateItemPermissions(ctx, plan.WorkspaceID.ValueString(), state.WarehouseID.ValueString(), "Warehouse", principal, oldPerms, newPerms); err != nil {
		resp.Diagnostics.AddError("Unable to Update Warehouse Permissions", err.Error())
		return
	}

	plan.WarehouseID = state.WarehouseID
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete revokes the managed Warehouse permissions from the target principal.
func (r *WarehousePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WarehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	perms, err := client.ExpandRolePermissions("Warehouse", state.RoleType.ValueString())
	if err != nil {
		perms = []string{"Read", "Write", "Reshare"}
	}

	principal := client.Principal{
		ID:   state.PrincipalID.ValueString(),
		Type: state.PrincipalType.ValueString(),
	}

	if err := r.client.RevokeItemPermissions(ctx, state.WorkspaceID.ValueString(), state.WarehouseID.ValueString(), "Warehouse", principal, perms); err != nil {
		resp.Diagnostics.AddError("Unable to Revoke Warehouse Permissions", err.Error())
		return
	}
}

// ImportState imports an existing Warehouse permission binding by {workspace_id}/{warehouse_id}/{principal_type}/{principal_id}.
func (r *WarehousePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Expected import identifier in format {workspace_id}/{warehouse_id}/{principal_type}/{principal_id}, got: %q", req.ID),
		)
		return
	}

	wsID, whID, principalType, principalID := parts[0], parts[1], parts[2], parts[3]

	item, err := r.client.GetItemByID(ctx, wsID, whID, "Warehouse")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Imported Warehouse", err.Error())
		return
	}

	perms, err := r.client.GetItemPermissions(ctx, wsID, whID, "Warehouse", principalID, principalType)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Imported Warehouse Permissions", err.Error())
		return
	}
	roleType, err := client.CollapseRolePermissions(perms)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Imported Warehouse Permissions", err.Error())
		return
	}

	state := WarehousePermissionResourceModel{
		ID:            types.StringValue(req.ID),
		WorkspaceID:   types.StringValue(wsID),
		WarehouseName: types.StringValue(item.DisplayName),
		WarehouseID:   types.StringValue(whID),
		PrincipalID:   types.StringValue(principalID),
		PrincipalType: types.StringValue(principalType),
		RoleType:      types.StringValue(roleType),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
