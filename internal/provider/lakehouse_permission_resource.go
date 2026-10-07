// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ resource.Resource                = &LakehousePermissionResource{}
	_ resource.ResourceWithConfigure   = &LakehousePermissionResource{}
	_ resource.ResourceWithImportState = &LakehousePermissionResource{}
)

// LakehousePermissionResource manages a OneLake Data Access Role on a Microsoft Fabric Lakehouse.
type LakehousePermissionResource struct {
	client   *client.FabricClient
	tenantID string
}

// LakehousePermissionResourceModel describes the Terraform state model for fabricext_lakehouse_permission.
type LakehousePermissionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	LakehouseName types.String `tfsdk:"lakehouse_name"`
	LakehouseID   types.String `tfsdk:"lakehouse_id"`
	RoleName      types.String `tfsdk:"role_name"`
	Paths         types.Set    `tfsdk:"paths"`
	Actions       types.Set    `tfsdk:"actions"`
	PrincipalIDs  types.Set    `tfsdk:"principal_ids"`
	PrincipalType types.String `tfsdk:"principal_type"`
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
		MarkdownDescription: "Manages a OneLake Data Access Role on a Microsoft Fabric **Lakehouse** to grant read access on specific tables and folders to Microsoft Entra users, groups, service principals, or managed identities.\n\n" +
			"-> **Note:** Microsoft Fabric OneLake Data Access Roles require uniform member types per role resource. To assign mixed member types (e.g., both users and service principals) to the same role, assign access through an Entra ID security `Group`.\n\n" +
			"-> **Note:** Role modifications automatically coordinate via a per-Lakehouse mutex and an `If-Match` ETag optimistic concurrency loop to safely support parallel applies.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite resource identifier in the format `{workspace_id}/{lakehouse_id}/{role_name}`.",
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
				Required:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric Lakehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"lakehouse_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved UUID of the Microsoft Fabric Lakehouse.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the OneLake Data Access Role (must start with a letter and contain only alphanumeric characters or underscores).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(roleNameRegex, "must start with a letter and contain only alphanumeric characters or underscores"),
				},
			},
			"paths": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of OneLake paths (`/Tables/...`, `/Files/...`, or `*`) granted by this role.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"actions": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Read")})),
				MarkdownDescription: "Set of OneLake actions permitted on `paths`. Valid values: `Read`. Defaults to `[\"Read\"]`.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf("Read")),
				},
			},
			"principal_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of Microsoft Entra Object IDs (UUIDs) assigned as members of this OneLake Data Access Role.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(uuidValidator()),
				},
			},
			"principal_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Group"),
				MarkdownDescription: "Microsoft Entra object type for `principal_ids`. Valid values: `User`, `Group`, `ServicePrincipal`, `ManagedIdentity`. Defaults to `Group`. Fabric OneLake Data Access Roles require uniform member types per role resource; to assign mixed member types, use an Entra ID security `Group`. Normalized to TitleCase upon import or read.",
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ManagedIdentity"),
				},
			},
		},
	}
}

// Configure wires the provider's FabricClient and default TenantID into the resource.
func (r *LakehousePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *Data, got: %T", req.ProviderData))
		return
	}
	r.client = pd.Client
	r.tenantID = pd.TenantID
}

func (r *LakehousePermissionResource) buildRolePayload(ctx context.Context, m *LakehousePermissionResourceModel) (client.DataAccessRole, diag.Diagnostics) {
	var diags diag.Diagnostics

	var paths []string
	diags.Append(m.Paths.ElementsAs(ctx, &paths, false)...)
	var actions []string
	diags.Append(m.Actions.ElementsAs(ctx, &actions, false)...)
	var principalIDs []string
	diags.Append(m.PrincipalIDs.ElementsAs(ctx, &principalIDs, false)...)
	if diags.HasError() {
		return client.DataAccessRole{}, diags
	}

	slices.Sort(paths)
	slices.Sort(actions)
	slices.Sort(principalIDs)

	entraMembers := make([]client.MicrosoftEntraMember, 0, len(principalIDs))
	for _, pid := range principalIDs {
		entraMembers = append(entraMembers, client.MicrosoftEntraMember{
			TenantID:   r.tenantID,
			ObjectID:   pid,
			ObjectType: m.PrincipalType.ValueString(),
		})
	}

	return client.DataAccessRole{
		Name: m.RoleName.ValueString(),
		Kind: "Policy",
		DecisionRules: []client.DecisionRule{
			{
				Effect: "Permit",
				Permission: []client.PermissionScope{
					{
						AttributeName:            "Path",
						AttributeValueIncludedIn: paths,
					},
					{
						AttributeName:            "Action",
						AttributeValueIncludedIn: actions,
					},
				},
			},
		},
		Members: &client.Members{
			MicrosoftEntraMembers: entraMembers,
		},
	}, diags
}

// Create upserts the OneLake Data Access Role on the target Lakehouse using ETag RMW.
func (r *LakehousePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	lhName := plan.LakehouseName.ValueString()

	lhID, err := r.client.GetItemIDByName(ctx, wsID, lhName, "Lakehouse")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Lakehouse by Name", err.Error())
		return
	}

	rolePayload, diags := r.buildRolePayload(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpsertDataAccessRole(ctx, wsID, lhID, rolePayload); err != nil {
		resp.Diagnostics.AddError("Unable to Create Lakehouse Data Access Role", err.Error())
		return
	}

	plan.LakehouseID = types.StringValue(lhID)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, lhID, plan.RoleName.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the live Lakehouse OneLake Data Access Role.
func (r *LakehousePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()
	roleName := state.RoleName.ValueString()

	item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Lakehouse Item", err.Error())
		return
	}
	state.LakehouseName = types.StringValue(item.DisplayName)

	role, err := r.client.GetDataAccessRole(ctx, wsID, lhID, roleName)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Lakehouse Data Access Role", err.Error())
		return
	}

	diags := populateLakehouseStateFromRole(ctx, role, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func populateLakehouseStateFromRole(ctx context.Context, role *client.DataAccessRole, state *LakehousePermissionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	paths := []string{}
	actions := []string{}
	for _, rule := range role.DecisionRules {
		for _, scope := range rule.Permission {
			switch scope.AttributeName {
			case "Path":
				paths = append(paths, scope.AttributeValueIncludedIn...)
			case "Action":
				actions = append(actions, scope.AttributeValueIncludedIn...)
			}
		}
	}

	principalIDs := []string{}
	principalType := state.PrincipalType.ValueString()
	if principalType == "" {
		principalType = "Group"
	}
	if role.Members != nil {
		for _, m := range role.Members.MicrosoftEntraMembers {
			principalIDs = append(principalIDs, m.ObjectID)
			if m.ObjectType != "" {
				if norm, err := normalizePrincipalType("Lakehouse", m.ObjectType); err == nil {
					principalType = norm
				} else {
					principalType = m.ObjectType
				}
			}
		}
	}

	pathsSet, d := types.SetValueFrom(ctx, types.StringType, paths)
	diags.Append(d...)
	actionsSet, d := types.SetValueFrom(ctx, types.StringType, actions)
	diags.Append(d...)
	principalsSet, d := types.SetValueFrom(ctx, types.StringType, principalIDs)
	diags.Append(d...)

	state.Paths = pathsSet
	state.Actions = actionsSet
	state.PrincipalIDs = principalsSet
	state.PrincipalType = types.StringValue(principalType)
	return diags
}

// Update modifies the OneLake Data Access Role in-place using ETag RMW.
func (r *LakehousePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LakehousePermissionResourceModel
	var state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rolePayload, diags := r.buildRolePayload(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpsertDataAccessRole(ctx, plan.WorkspaceID.ValueString(), state.LakehouseID.ValueString(), rolePayload); err != nil {
		resp.Diagnostics.AddError("Unable to Update Lakehouse Data Access Role", err.Error())
		return
	}

	plan.LakehouseID = state.LakehouseID
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the managed OneLake Data Access Role while preserving sibling roles.
func (r *LakehousePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDataAccessRole(ctx, state.WorkspaceID.ValueString(), state.LakehouseID.ValueString(), state.RoleName.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Delete Lakehouse Data Access Role", err.Error())
		return
	}
}

// ImportState imports an existing OneLake Data Access Role by {workspace_id}/{lakehouse_id}/{role_name}.
func (r *LakehousePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Expected import identifier in format {workspace_id}/{lakehouse_id}/{role_name}, got: %q", req.ID),
		)
		return
	}

	wsID, lhID, roleName := parts[0], parts[1], parts[2]
	if !uuidRegex.MatchString(wsID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("workspace_id %q must be a valid UUID", wsID))
		return
	}
	if !uuidRegex.MatchString(lhID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("lakehouse_id %q must be a valid UUID", lhID))
		return
	}
	if !roleNameRegex.MatchString(roleName) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("role_name %q must start with a letter and contain only alphanumeric characters or underscores", roleName))
		return
	}

	item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Imported Lakehouse", err.Error())
		return
	}

	role, err := r.client.GetDataAccessRole(ctx, wsID, lhID, roleName)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Imported Lakehouse Data Access Role", err.Error())
		return
	}

	state := LakehousePermissionResourceModel{
		ID:            types.StringValue(req.ID),
		WorkspaceID:   types.StringValue(wsID),
		LakehouseName: types.StringValue(item.DisplayName),
		LakehouseID:   types.StringValue(lhID),
		RoleName:      types.StringValue(roleName),
	}
	resp.Diagnostics.Append(populateLakehouseStateFromRole(ctx, role, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
