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
	_ resource.Resource                = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithConfigure   = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithImportState = &SQLDatabasePermissionResource{}
)

// SQLDatabasePermissionResource manages item-level permissions on a Microsoft Fabric SQL Database.
type SQLDatabasePermissionResource struct {
	client *client.FabricClient
}

// SQLDatabasePermissionResourceModel describes the Terraform state model for fabricext_sql_database_permission.
type SQLDatabasePermissionResourceModel struct {
	ID              types.String `tfsdk:"id"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	SQLDatabaseName types.String `tfsdk:"sql_database_name"`
	SQLDatabaseID   types.String `tfsdk:"sql_database_id"`
	PrincipalID     types.String `tfsdk:"principal_id"`
	PrincipalType   types.String `tfsdk:"principal_type"`
	RoleType        types.String `tfsdk:"role_type"`
}

// NewSQLDatabasePermissionResource constructs a new fabricext_sql_database_permission resource.
func NewSQLDatabasePermissionResource() resource.Resource {
	return &SQLDatabasePermissionResource{}
}

// Metadata sets the resource type name.
func (r *SQLDatabasePermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sql_database_permission"
}

// Schema defines the schema for fabricext_sql_database_permission.
func (r *SQLDatabasePermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages item-level sharing and permissions on a Microsoft Fabric **SQL Database** (`read`, `read_data`, `read_spark`, `write`, `reshare`) for a Microsoft Entra principal.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite resource identifier in the format `{workspace_id}/{sql_database_id}/{principal_type}/{principal_id}`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the Microsoft Fabric workspace containing the SQL Database.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"sql_database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric SQL Database.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"sql_database_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved UUID of the Microsoft Fabric SQL Database.",
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
				MarkdownDescription: "Microsoft Entra principal type. Valid values: `User`, `Group`, `ServicePrincipal`, `ServicePrincipalProfile`. Defaults to `Group`. Normalized to TitleCase upon import or read.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ServicePrincipalProfile"),
				},
			},
			"role_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Permission role to grant on the SQL Database. Valid values: `read` (`Read`), `read_data` (`Read`, `ReadData`), `read_spark` (`Read`, `ReadAll`, `SubscribeOneLakeEvents`), `write` (`Read`, `Write`), `reshare` (`Read`, `Reshare`). When reading existing permissions from Fabric, multi-permission assignments are collapsed according to precedence: `write` > `read_spark` > `read_data` > `reshare` > `read`.",
				Validators: []validator.String{
					stringvalidator.OneOf("read", "read_data", "read_spark", "write", "reshare"),
				},
			},
		},
	}
}

// Configure wires the provider's FabricClient into the resource.
func (r *SQLDatabasePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create grants the requested SQL Database permissions and populates Terraform state.
func (r *SQLDatabasePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	dbName := plan.SQLDatabaseName.ValueString()
	principal := client.Principal{
		ID:   plan.PrincipalID.ValueString(),
		Type: plan.PrincipalType.ValueString(),
	}

	dbID, err := r.client.GetItemIDByName(ctx, wsID, dbName, "SQLDatabase")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve SQL Database by Name", err.Error())
		return
	}

	perms, err := client.ExpandRolePermissions("SQLDatabase", plan.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid SQL Database Role Type", err.Error())
		return
	}

	if err := r.client.GrantItemPermissions(ctx, wsID, dbID, "SQLDatabase", principal, perms); err != nil {
		resp.Diagnostics.AddError("Unable to Grant SQL Database Permissions", err.Error())
		return
	}

	plan.SQLDatabaseID = types.StringValue(dbID)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s/%s", wsID, dbID, principal.Type, principal.ID))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the live Fabric SQL Database permission assignments.
func (r *SQLDatabasePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	dbID := state.SQLDatabaseID.ValueString()
	principalID := state.PrincipalID.ValueString()
	principalType := state.PrincipalType.ValueString()

	item, err := r.client.GetItemByID(ctx, wsID, dbID, "SQLDatabase")
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read SQL Database Item", err.Error())
		return
	}
	state.SQLDatabaseName = types.StringValue(item.DisplayName)

	perms, err := r.client.GetItemPermissions(ctx, wsID, dbID, "SQLDatabase", principalID, principalType)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read SQL Database Permissions", err.Error())
		return
	}

	roleType, err := client.CollapseRolePermissions(perms)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map SQL Database Permissions to Role Type", err.Error())
		return
	}

	state.RoleType = types.StringValue(roleType)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update reconciles SQL Database permissions in-place using set-difference grant and revoke calls.
func (r *SQLDatabasePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SQLDatabasePermissionResourceModel
	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldPerms, err := client.ExpandRolePermissions("SQLDatabase", state.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Previous SQL Database Role Type", err.Error())
		return
	}
	newPerms, err := client.ExpandRolePermissions("SQLDatabase", plan.RoleType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Target SQL Database Role Type", err.Error())
		return
	}

	principal := client.Principal{
		ID:   plan.PrincipalID.ValueString(),
		Type: plan.PrincipalType.ValueString(),
	}

	if err := r.client.UpdateItemPermissions(ctx, plan.WorkspaceID.ValueString(), state.SQLDatabaseID.ValueString(), "SQLDatabase", principal, oldPerms, newPerms); err != nil {
		resp.Diagnostics.AddError("Unable to Update SQL Database Permissions", err.Error())
		return
	}

	plan.SQLDatabaseID = state.SQLDatabaseID
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete revokes the managed SQL Database permissions from the target principal.
func (r *SQLDatabasePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	perms, err := client.ExpandRolePermissions("SQLDatabase", state.RoleType.ValueString())
	if err != nil {
		perms = []string{"Read", "ReadData", "ReadAll", "SubscribeOneLakeEvents", "Write", "Reshare"}
	}

	principal := client.Principal{
		ID:   state.PrincipalID.ValueString(),
		Type: state.PrincipalType.ValueString(),
	}

	if err := r.client.RevokeItemPermissions(ctx, state.WorkspaceID.ValueString(), state.SQLDatabaseID.ValueString(), "SQLDatabase", principal, perms); err != nil {
		resp.Diagnostics.AddError("Unable to Revoke SQL Database Permissions", err.Error())
		return
	}
}

// ImportState imports an existing SQL Database permission binding by {workspace_id}/{sql_database_id}/{principal_type}/{principal_id}.
func (r *SQLDatabasePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Expected import identifier in format {workspace_id}/{sql_database_id}/{principal_type}/{principal_id}, got: %q", req.ID),
		)
		return
	}

	wsID, dbID, rawPrincipalType, principalID := parts[0], parts[1], parts[2], parts[3]
	if !uuidRegex.MatchString(wsID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("workspace_id %q must be a valid UUID", wsID))
		return
	}
	if !uuidRegex.MatchString(dbID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("sql_database_id %q must be a valid UUID", dbID))
		return
	}
	if !uuidRegex.MatchString(principalID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("principal_id %q must be a valid UUID", principalID))
		return
	}
	principalType, err := normalizePrincipalType("SQLDatabase", rawPrincipalType)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import Identifier", err.Error())
		return
	}

	item, err := r.client.GetItemByID(ctx, wsID, dbID, "SQLDatabase")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Imported SQL Database", err.Error())
		return
	}

	perms, err := r.client.GetItemPermissions(ctx, wsID, dbID, "SQLDatabase", principalID, principalType)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Imported SQL Database Permissions", err.Error())
		return
	}
	roleType, err := client.CollapseRolePermissions(perms)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Imported SQL Database Permissions", err.Error())
		return
	}

	canonicalID := fmt.Sprintf("%s/%s/%s/%s", wsID, dbID, principalType, principalID)
	state := SQLDatabasePermissionResourceModel{
		ID:              types.StringValue(canonicalID),
		WorkspaceID:     types.StringValue(wsID),
		SQLDatabaseName: types.StringValue(item.DisplayName),
		SQLDatabaseID:   types.StringValue(dbID),
		PrincipalID:     types.StringValue(principalID),
		PrincipalType:   types.StringValue(principalType),
		RoleType:        types.StringValue(roleType),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
