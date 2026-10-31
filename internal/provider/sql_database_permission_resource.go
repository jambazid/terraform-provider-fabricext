// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ resource.Resource                   = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithConfigure      = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithImportState    = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithModifyPlan     = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithValidateConfig = &SQLDatabasePermissionResource{}
)

// SQLDatabasePermissionResource manages direct item-level permissions on a Microsoft Fabric SQL Database.
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
	Permissions     types.Set    `tfsdk:"permissions"`
	Notify          types.Bool   `tfsdk:"notify"`
	Message         types.String `tfsdk:"message"`
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
		Description: "Manages direct item-level permissions on a Microsoft Fabric SQL Database.",
		MarkdownDescription: "Manages direct item-level permissions on a Microsoft Fabric **SQL Database** for Microsoft Entra users, groups, service principals, or managed identities.\n\n" +
			"~> **Note:** Manages item-level permissions on a Microsoft Fabric SQL Database for Microsoft Entra principals via the underlying Power BI metadata service.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier in the format `{workspace_id}/{sql_database_id}/{principal_type}/{principal_id}`.",
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
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric SQL Database. At least one of `sql_database_name` or `sql_database_id` must be specified.",
				PlanModifiers: []planmodifier.String{
					RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"sql_database_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Resolved or explicitly specified UUID of the Microsoft Fabric SQL Database. At least one of `sql_database_name` or `sql_database_id` must be specified.",
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
				MarkdownDescription: "Set of item permissions granted to the principal. Must include `Read`. Valid permissions for SQL Database are `Read`, `Write`, `ReadData`, `ReadAll`, `Reshare`, and `Execute`.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(
						stringvalidator.OneOf("Read", "Write", "ReadData", "ReadAll", "Reshare", "Execute"),
					),
				},
			},
			"notify": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to send an email notification when granting or updating item permissions. Defaults to `false`.",
			},
			"message": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Custom email notification message sent when `notify` is set to `true`. Requires `notify = true`.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// ValidateConfig enforces that message requires notify = true and permissions contains Read.
func (r *SQLDatabasePermissionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.Permissions.IsNull() && !config.Permissions.IsUnknown() {
		var perms []string
		resp.Diagnostics.Append(config.Permissions.ElementsAs(ctx, &perms, false)...)
		if !resp.Diagnostics.HasError() {
			hasRead := false
			for _, p := range perms {
				if strings.EqualFold(p, "Read") {
					hasRead = true
					break
				}
			}
			if !hasRead {
				resp.Diagnostics.AddAttributeError(
					path.Root("permissions"),
					"Invalid Permissions Configuration",
					"The 'permissions' set must contain 'Read'. Microsoft Fabric requires 'Read' permissions as the foundation for any item-level access grant.",
				)
			}
		}
	}

	if !config.Message.IsNull() && !config.Message.IsUnknown() && config.Message.ValueString() != "" {
		if !config.Notify.IsUnknown() && (config.Notify.IsNull() || !config.Notify.ValueBool()) {
			resp.Diagnostics.AddAttributeError(
				path.Root("message"),
				"Invalid Configuration",
				"The 'message' attribute requires 'notify = true'. Either set 'notify = true' or remove 'message'.",
			)
		}
	}
}

// Configure injects the FabricClient from the provider context.
func (r *SQLDatabasePermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan reconciles sql_database_name and sql_database_id when replacing resources.
func (r *SQLDatabasePermissionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan, state, config SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reconcileItemIdentifiersPlan(
		config.SQLDatabaseName, config.SQLDatabaseID,
		state.SQLDatabaseName, state.SQLDatabaseID,
		&plan.SQLDatabaseName, &plan.SQLDatabaseID,
	)

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// Create grants direct item-level permissions on the SQL Database.
func (r *SQLDatabasePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var plan SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	resolvedID, resolvedName, diags := reconcileItemIdentifiersCreate(
		ctx, r.client, "SQLDatabase", wsID,
		plan.SQLDatabaseName.ValueString(), plan.SQLDatabaseID.ValueString(),
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	normType, err := normalizePrincipalType("SQLDatabase", plan.PrincipalType.ValueString())
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
		PrincipalID:         plan.PrincipalID.ValueString(),
		PrincipalType:       normType,
		Permissions:         perms,
		Notify:              plan.Notify.ValueBool(),
		NotificationMessage: plan.Message.ValueString(),
	}

	if err := r.client.UpsertItemDirectPermissions(ctx, wsID, resolvedID, "SQLDatabase", share); err != nil {
		resp.Diagnostics.AddError("Error Granting SQL Database Permission", fmt.Sprintf("Failed to grant permissions on SQL Database %q in workspace %q: %s", resolvedID, wsID, err))
		return
	}

	plan.SQLDatabaseID = types.StringValue(resolvedID)
	plan.SQLDatabaseName = types.StringValue(resolvedName)
	plan.PrincipalType = types.StringValue(normType)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s/%s", wsID, resolvedID, normType, plan.PrincipalID.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the resource state from the Power BI metadata and Fabric APIs.
func (r *SQLDatabasePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	dbID := state.SQLDatabaseID.ValueString()
	principalID := state.PrincipalID.ValueString()

	// 1. Verify item exists by immutable ID
	item, err := r.client.GetItemByID(ctx, wsID, dbID, "SQLDatabase")
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
		resp.Diagnostics.AddError("Error Reading SQL Database", fmt.Sprintf("Failed to read SQL Database %q in workspace %q: %s", dbID, wsID, err))
		return
	}

	// Update display name if renamed upstream
	if item.DisplayName != state.SQLDatabaseName.ValueString() {
		state.SQLDatabaseName = types.StringValue(item.DisplayName)
		r.client.InvalidateItemNameCache(wsID, "SQLDatabase")
	}

	// 2. Read direct permissions
	share, err := r.client.GetItemDirectPermissions(ctx, wsID, dbID, "SQLDatabase", principalID)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading SQL Database Permission", fmt.Sprintf("Failed to read permissions on SQL Database %q for principal %q: %s", dbID, principalID, err))
		return
	}

	if share == nil {
		// Share was revoked out of band
		resp.State.RemoveResource(ctx)
		return
	}

	managedPerms := filterManagedPermissions(share.Permissions, "SQLDatabase")
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
func (r *SQLDatabasePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var plan, state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	dbID := state.SQLDatabaseID.ValueString()

	var perms []string
	resp.Diagnostics.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	share := client.ItemDirectShare{
		PrincipalID:         state.PrincipalID.ValueString(),
		PrincipalType:       state.PrincipalType.ValueString(),
		Permissions:         perms,
		Notify:              plan.Notify.ValueBool(),
		NotificationMessage: plan.Message.ValueString(),
	}

	if err := r.client.UpsertItemDirectPermissions(ctx, wsID, dbID, "SQLDatabase", share); err != nil {
		resp.Diagnostics.AddError("Error Updating SQL Database Permission", fmt.Sprintf("Failed to update permissions on SQL Database %q: %s", dbID, err))
		return
	}

	state.Permissions = plan.Permissions
	state.Notify = plan.Notify
	state.Message = plan.Message
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete revokes direct permissions on the SQL Database.
func (r *SQLDatabasePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wsID := state.WorkspaceID.ValueString()
	dbID := state.SQLDatabaseID.ValueString()
	principalID := state.PrincipalID.ValueString()

	if err := r.client.RevokeItemDirectPermissions(ctx, wsID, dbID, "SQLDatabase", principalID); err != nil {
		if client.IsNotFound(err) {
			return // Item already deleted
		}
		resp.Diagnostics.AddError("Error Revoking SQL Database Permission", fmt.Sprintf("Failed to revoke permissions on SQL Database %q for principal %q: %s", dbID, principalID, err))
		return
	}
}

// ImportState imports a SQL database permission resource from composite ID.
func (r *SQLDatabasePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client Not Configured", "Expected configured client, but received nil.")
		return
	}

	wsID, dbID, principalType, principalID, err := parseItemPermissionImportID(req.ID, "SQLDatabase")
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import Identifier", err.Error())
		return
	}

	dbName := ""
	if !uuidRegex.MatchString(dbID) {
		dbName = dbID
		resolvedID, err := r.client.GetItemIDByName(ctx, wsID, dbName, "SQLDatabase")
		if err != nil {
			resp.Diagnostics.AddError("Error Resolving SQL Database", fmt.Sprintf("Failed to resolve SQL database %q in workspace %q: %s", dbName, wsID, err))
			return
		}
		dbID = resolvedID
	}

	canonicalID := fmt.Sprintf("%s/%s/%s/%s", wsID, dbID, principalType, principalID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), canonicalID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), wsID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sql_database_id"), dbID)...)
	if dbName != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sql_database_name"), dbName)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_type"), principalType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_id"), principalID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("notify"), false)...)
}
