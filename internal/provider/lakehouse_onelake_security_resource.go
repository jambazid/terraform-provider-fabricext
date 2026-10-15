// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                   = &LakehouseOnelakeSecurityResource{}
	_ resource.ResourceWithConfigure      = &LakehouseOnelakeSecurityResource{}
	_ resource.ResourceWithImportState    = &LakehouseOnelakeSecurityResource{}
	_ resource.ResourceWithValidateConfig = &LakehouseOnelakeSecurityResource{}
	_ resource.ResourceWithModifyPlan     = &LakehouseOnelakeSecurityResource{}
)

// LakehouseOnelakeSecurityResourceModel represents the schema model for fabricext_lakehouse_onelake_security.
type LakehouseOnelakeSecurityResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	WorkspaceID             types.String `tfsdk:"workspace_id"`
	LakehouseID             types.String `tfsdk:"lakehouse_id"`
	LakehouseName           types.String `tfsdk:"lakehouse_name"`
	RemoveDefaultReaderRole types.Bool   `tfsdk:"remove_default_reader_role"`
	DefaultReaderRoleExists types.Bool   `tfsdk:"default_reader_role_exists"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
}

// LakehouseOnelakeSecurityResource manages OneLake Data Access Security enablement verification
// and DefaultReader role purging on a Microsoft Fabric Lakehouse.
type LakehouseOnelakeSecurityResource struct {
	client   *client.FabricClient
	tenantID string
}

// NewLakehouseOnelakeSecurityResource creates a new resource instance.
func NewLakehouseOnelakeSecurityResource() resource.Resource {
	return &LakehouseOnelakeSecurityResource{}
}

// Metadata sets the resource type name.
func (r *LakehouseOnelakeSecurityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lakehouse_onelake_security"
}

// Schema defines the schema for fabricext_lakehouse_onelake_security.
func (r *LakehouseOnelakeSecurityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Verifies that OneLake Data Access Security is enabled on a Microsoft Fabric Lakehouse and optionally purges the auto-provisioned DefaultReader role to enforce zero-trust least-privilege access control.",
		MarkdownDescription: "Verifies that OneLake Data Access Security is enabled on a Microsoft Fabric **Lakehouse** and optionally purges the auto-provisioned `DefaultReader` role to enforce zero-trust least-privilege access control.\n\n" +
			"~> **Note:** Microsoft Fabric currently does not support enabling OneLake Data Access Security via public REST API. Enablement must be performed once in the Fabric portal via **Lakehouse ribbon -> Manage OneLake security**. This resource validates enablement and manages the `DefaultReader` role lifecycle.\n\n" +
			"~> **Important:** When OneLake security is enabled, Fabric automatically creates a `DefaultReader` role granting read access across all tables to anyone with workspace `ReadAll` permissions. Leaving this role active undermines fine-grained RLS and CLS permissions. By default, `remove_default_reader_role = true` automatically removes this role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite resource identifier in the format `{workspace_id}/{lakehouse_id}`.",
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
			"lakehouse_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "UUID of the Microsoft Fabric Lakehouse. At least one of `lakehouse_id` or `lakehouse_name` must be configured.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"lakehouse_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the Microsoft Fabric Lakehouse. At least one of `lakehouse_id` or `lakehouse_name` must be configured.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"remove_default_reader_role": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether to automatically remove the default `DefaultReader` role created by Microsoft Fabric when OneLake security is enabled. Defaults to `true` to enforce zero-trust least-privilege access control.",
			},
			"default_reader_role_exists": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the `DefaultReader` role currently exists on the Lakehouse.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether OneLake Data Access Security is enabled on the Lakehouse. Always true when managed by this resource.",
			},
		},
	}
}

// Configure wires the provider's FabricClient and default TenantID into the resource.
func (r *LakehouseOnelakeSecurityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig validates that either lakehouse_id or lakehouse_name is configured.
func (r *LakehouseOnelakeSecurityResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config LakehouseOnelakeSecurityResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.LakehouseID.IsNull() && config.LakehouseName.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Lakehouse Identifier",
			"At least one of 'lakehouse_id' or 'lakehouse_name' must be configured.",
		)
	}
}

// ModifyPlan reconciles identifiers and prevents reversion of remove_default_reader_role.
func (r *LakehouseOnelakeSecurityResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan LakehouseOnelakeSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !req.State.Raw.IsNull() && !req.Config.Raw.IsNull() {
		var state LakehouseOnelakeSecurityResourceModel
		var config LakehouseOnelakeSecurityResourceModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
		if !resp.Diagnostics.HasError() {
			reconcileItemIdentifiersPlan(
				config.LakehouseName, config.LakehouseID,
				state.LakehouseName, state.LakehouseID,
				&plan.LakehouseName, &plan.LakehouseID,
			)

			// Reversion guard: true -> false
			if !state.RemoveDefaultReaderRole.IsNull() && !state.RemoveDefaultReaderRole.IsUnknown() && state.RemoveDefaultReaderRole.ValueBool() &&
				!plan.RemoveDefaultReaderRole.IsNull() && !plan.RemoveDefaultReaderRole.IsUnknown() && !plan.RemoveDefaultReaderRole.ValueBool() {
				resp.Diagnostics.AddError(
					"Invalid Attribute Modification",
					"Cannot revert 'remove_default_reader_role' from true to false: Microsoft Fabric does not provide an API to restore the purged DefaultReader role. If access for all ReadAll holders is needed, define an explicit fabricext_lakehouse_data_access_role resource.",
				)
			}

			// Drift self-healing: if DefaultReader was recreated out-of-band, plan the purge
			if !state.DefaultReaderRoleExists.IsNull() && !state.DefaultReaderRoleExists.IsUnknown() && state.DefaultReaderRoleExists.ValueBool() &&
				!plan.RemoveDefaultReaderRole.IsNull() && !plan.RemoveDefaultReaderRole.IsUnknown() && plan.RemoveDefaultReaderRole.ValueBool() {
				plan.DefaultReaderRoleExists = types.BoolValue(false)
			}
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// Create verifies OneLake security enablement, optionally purges DefaultReader, and sets state.
func (r *LakehouseOnelakeSecurityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LakehouseOnelakeSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config LakehouseOnelakeSecurityResourceModel
	if !req.Config.Raw.IsNull() {
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	wsID := plan.WorkspaceID.ValueString()
	hasConfigID := !config.LakehouseID.IsNull() && !config.LakehouseID.IsUnknown() && config.LakehouseID.ValueString() != ""
	hasConfigName := !config.LakehouseName.IsNull() && !config.LakehouseName.IsUnknown() && config.LakehouseName.ValueString() != ""
	if !hasConfigID && !hasConfigName {
		config.LakehouseID = plan.LakehouseID
		config.LakehouseName = plan.LakehouseName
		hasConfigID = !config.LakehouseID.IsNull() && !config.LakehouseID.IsUnknown() && config.LakehouseID.ValueString() != ""
		hasConfigName = !config.LakehouseName.IsNull() && !config.LakehouseName.IsUnknown() && config.LakehouseName.ValueString() != ""
	}

	var lhID string
	var lhName string

	switch {
	case hasConfigID && hasConfigName:
		lhID = config.LakehouseID.ValueString()
		lhName = config.LakehouseName.ValueString()
		item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
		if err != nil {
			resp.Diagnostics.AddError("Unable to Resolve Lakehouse by ID", err.Error())
			return
		}
		if item.DisplayName != lhName {
			resp.Diagnostics.AddError(
				"Conflicting Lakehouse Identifiers",
				fmt.Sprintf("Specified lakehouse_name %q does not match display name %q of lakehouse_id %q.", lhName, item.DisplayName, lhID),
			)
			return
		}
		plan.LakehouseID = types.StringValue(lhID)
		plan.LakehouseName = types.StringValue(item.DisplayName)
	case hasConfigID:
		lhID = config.LakehouseID.ValueString()
		item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
		if err != nil {
			resp.Diagnostics.AddError("Unable to Resolve Lakehouse by ID", err.Error())
			return
		}
		lhName = item.DisplayName
		plan.LakehouseID = types.StringValue(lhID)
		plan.LakehouseName = types.StringValue(lhName)
	case hasConfigName:
		lhName = config.LakehouseName.ValueString()
		resolvedID, err := r.client.GetItemIDByName(ctx, wsID, lhName, "Lakehouse")
		if err != nil {
			resp.Diagnostics.AddError("Unable to Resolve Lakehouse by Name", err.Error())
			return
		}
		lhID = resolvedID
		plan.LakehouseID = types.StringValue(lhID)
		plan.LakehouseName = types.StringValue(lhName)
	default:
		resp.Diagnostics.AddError("Missing Lakehouse Identifier", "Either lakehouse_id or lakehouse_name must be configured.")
		return
	}

	// Verify OneLake Security Enablement
	roles, _, err := r.client.ListDataAccessRoles(ctx, wsID, lhID)
	if err != nil {
		var notEnabledErr *client.OneLakeSecurityNotEnabledError
		if errors.As(err, &notEnabledErr) {
			resp.Diagnostics.AddError(
				"OneLake Data Access Security Not Enabled",
				notEnabledErr.Error(),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to Verify OneLake Data Access Security", err.Error())
		return
	}

	// Locate DefaultReader role
	var defaultReaderRoleName string
	for _, role := range roles {
		if strings.EqualFold(role.Name, "DefaultReader") {
			defaultReaderRoleName = role.Name
			break
		}
	}
	defaultReaderExists := defaultReaderRoleName != ""

	// Remove DefaultReader if configured
	if plan.RemoveDefaultReaderRole.ValueBool() && defaultReaderExists {
		if err := r.client.DeleteDataAccessRole(ctx, wsID, lhID, defaultReaderRoleName); err != nil {
			resp.Diagnostics.AddError("Failed to Remove DefaultReader Role", err.Error())
			return
		}
		defaultReaderExists = false
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", wsID, lhID))
	plan.WorkspaceID = types.StringValue(wsID)
	plan.LakehouseID = types.StringValue(lhID)
	plan.LakehouseName = types.StringValue(lhName)
	plan.DefaultReaderRoleExists = types.BoolValue(defaultReaderExists)
	plan.Enabled = types.BoolValue(true)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the OneLake security status and DefaultReader presence from the Fabric API.
func (r *LakehouseOnelakeSecurityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LakehouseOnelakeSecurityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()

	// Refresh Lakehouse Name and verify item existence
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

	// Check OneLake Data Access Roles
	roles, _, err := r.client.ListDataAccessRoles(ctx, wsID, lhID)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		var notEnabledErr *client.OneLakeSecurityNotEnabledError
		if errors.As(err, &notEnabledErr) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read Lakehouse OneLake Security", err.Error())
		return
	}

	defaultReaderExists := false
	for _, role := range roles {
		if strings.EqualFold(role.Name, "DefaultReader") {
			defaultReaderExists = true
			break
		}
	}

	state.DefaultReaderRoleExists = types.BoolValue(defaultReaderExists)
	state.Enabled = types.BoolValue(true)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update manages the DefaultReader purge lifecycle when configuration changes.
func (r *LakehouseOnelakeSecurityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LakehouseOnelakeSecurityResourceModel
	var state LakehouseOnelakeSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	wsID := state.WorkspaceID.ValueString()
	lhID := state.LakehouseID.ValueString()

	roles, _, err := r.client.ListDataAccessRoles(ctx, wsID, lhID)
	if err != nil {
		var notEnabledErr *client.OneLakeSecurityNotEnabledError
		if errors.As(err, &notEnabledErr) {
			resp.Diagnostics.AddError("OneLake Data Access Security Not Enabled", notEnabledErr.Error())
			return
		}
		resp.Diagnostics.AddError("Unable to Verify OneLake Data Access Security", err.Error())
		return
	}

	var defaultReaderRoleName string
	for _, role := range roles {
		if strings.EqualFold(role.Name, "DefaultReader") {
			defaultReaderRoleName = role.Name
			break
		}
	}
	defaultReaderExists := defaultReaderRoleName != ""

	if plan.RemoveDefaultReaderRole.ValueBool() && defaultReaderExists {
		if err := r.client.DeleteDataAccessRole(ctx, wsID, lhID, defaultReaderRoleName); err != nil {
			resp.Diagnostics.AddError("Failed to Remove DefaultReader Role", err.Error())
			return
		}
		defaultReaderExists = false
	}

	plan.ID = state.ID
	plan.WorkspaceID = state.WorkspaceID
	plan.LakehouseID = state.LakehouseID
	plan.LakehouseName = state.LakehouseName
	plan.DefaultReaderRoleExists = types.BoolValue(defaultReaderExists)
	plan.Enabled = types.BoolValue(true)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete emits an informational warning since Microsoft Fabric does not support disabling OneLake security via API.
func (r *LakehouseOnelakeSecurityResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"OneLake Security Cannot Be Disabled via REST API",
		"Microsoft Fabric does not support programmatically disabling OneLake Data Access Security via REST API. The resource has been removed from Terraform state, but the Lakehouse remains security-enabled in Microsoft Fabric. To disable OneLake security, use the Fabric web portal.",
	)
}

// ImportState imports an existing Lakehouse OneLake security state using {workspace_id}/{lakehouse_id}.
func (r *LakehouseOnelakeSecurityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before resource operation.")
		return
	}

	parts := strings.Split(strings.TrimSpace(req.ID), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import Identifier",
			fmt.Sprintf("Expected composite identifier in format {workspace_id}/{lakehouse_id}, got: %q", req.ID),
		)
		return
	}

	wsID := parts[0]
	lhID := parts[1]
	if !uuidRegex.MatchString(wsID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("workspace_id %q must be a valid UUID", wsID))
		return
	}
	if !uuidRegex.MatchString(lhID) {
		resp.Diagnostics.AddError("Invalid Import Identifier", fmt.Sprintf("lakehouse_id %q must be a valid UUID", lhID))
		return
	}

	item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Imported Lakehouse", err.Error())
		return
	}

	roles, _, err := r.client.ListDataAccessRoles(ctx, wsID, lhID)
	if err != nil {
		var notEnabledErr *client.OneLakeSecurityNotEnabledError
		if errors.As(err, &notEnabledErr) {
			resp.Diagnostics.AddError("OneLake Data Access Security Not Enabled on Imported Lakehouse", notEnabledErr.Error())
			return
		}
		resp.Diagnostics.AddError("Unable to Verify OneLake Data Access Security on Imported Lakehouse", err.Error())
		return
	}

	defaultReaderExists := false
	for _, role := range roles {
		if strings.EqualFold(role.Name, "DefaultReader") {
			defaultReaderExists = true
			break
		}
	}

	removeDefaultReader := !defaultReaderExists

	state := LakehouseOnelakeSecurityResourceModel{
		ID:                      types.StringValue(fmt.Sprintf("%s/%s", wsID, lhID)),
		WorkspaceID:             types.StringValue(wsID),
		LakehouseID:             types.StringValue(lhID),
		LakehouseName:           types.StringValue(item.DisplayName),
		RemoveDefaultReaderRole: types.BoolValue(removeDefaultReader),
		DefaultReaderRoleExists: types.BoolValue(defaultReaderExists),
		Enabled:                 types.BoolValue(true),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
