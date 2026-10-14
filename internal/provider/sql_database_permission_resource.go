// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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
	_ resource.Resource                     = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithConfigure        = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithModifyPlan       = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithImportState      = &SQLDatabasePermissionResource{}
	_ resource.ResourceWithConfigValidators = &SQLDatabasePermissionResource{}
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
		DeprecationMessage:  deprecatedItemSharingMessage,
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
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric SQL Database. At least one of `sql_database_name` or `sql_database_id` must be specified.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					uuidValidator(),
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

// ConfigValidators validates cross-attribute requirements on the resource configuration.
func (r *SQLDatabasePermissionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("sql_database_name"),
			path.MatchRoot("sql_database_id"),
		),
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

// ModifyPlan clears unconfigured counterpart identifiers on replacement so stale
// values from UseStateForUnknown do not cause replacement failures.
func (r *SQLDatabasePermissionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() || req.Config.Raw.IsNull() {
		return
	}

	var state SQLDatabasePermissionResourceModel
	var config SQLDatabasePermissionResourceModel
	var plan SQLDatabasePermissionResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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

// Create returns an error diagnostic indicating that Fabric does not support item-level sharing via REST API.
func (r *SQLDatabasePermissionResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		deprecatedItemSharingCreateErrorSummary,
		deprecatedItemSharingCreateErrorDetail,
	)
}

// Read preserves existing state without making network requests.
func (r *SQLDatabasePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SQLDatabasePermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update returns an error diagnostic indicating that Fabric does not support item-level sharing via REST API.
func (r *SQLDatabasePermissionResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		deprecatedItemSharingCreateErrorSummary,
		deprecatedItemSharingCreateErrorDetail,
	)
}

// Delete removes the resource from state with an informational warning diagnostic.
func (r *SQLDatabasePermissionResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Deprecated Resource Removed",
		"fabricext_sql_database_permission has been removed from Terraform state. No Microsoft Fabric API calls were made.",
	)
	resp.State.RemoveResource(ctx)
}

// ImportState returns an error diagnostic directing users away from importing deprecated resources.
func (r *SQLDatabasePermissionResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"Cannot Import Deprecated Resource",
		"fabricext_sql_database_permission cannot be imported because item-level sharing endpoints do not exist in Microsoft Fabric. Use portal sharing and T-SQL roles.",
	)
}
