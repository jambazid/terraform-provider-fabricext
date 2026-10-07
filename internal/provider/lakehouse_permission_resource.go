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
	_ resource.Resource                   = &LakehousePermissionResource{}
	_ resource.ResourceWithConfigure      = &LakehousePermissionResource{}
	_ resource.ResourceWithImportState    = &LakehousePermissionResource{}
	_ resource.ResourceWithValidateConfig = &LakehousePermissionResource{}
	_ resource.ResourceWithModifyPlan     = &LakehousePermissionResource{}
)

var (
	rowConstraintAttrTypes = map[string]attr.Type{
		"table_path": types.StringType,
		"predicate":  types.StringType,
	}
	rowConstraintElemType = types.ObjectType{AttrTypes: rowConstraintAttrTypes}

	columnConstraintAttrTypes = map[string]attr.Type{
		"table_path": types.StringType,
		"columns":    types.SetType{ElemType: types.StringType},
		"action":     types.StringType,
		"effect":     types.StringType,
	}
	columnConstraintElemType = types.ObjectType{AttrTypes: columnConstraintAttrTypes}

	decisionRuleAttrTypes = map[string]attr.Type{
		"paths":             types.SetType{ElemType: types.StringType},
		"actions":           types.SetType{ElemType: types.StringType},
		"effect":            types.StringType,
		"row_constraint":    types.ListType{ElemType: rowConstraintElemType},
		"column_constraint": types.ListType{ElemType: columnConstraintElemType},
	}
	decisionRuleElemType = types.ObjectType{AttrTypes: decisionRuleAttrTypes}

	entraMemberAttrTypes = map[string]attr.Type{
		"object_id":   types.StringType,
		"object_type": types.StringType,
		"tenant_id":   types.StringType,
	}
	entraMemberElemType = types.ObjectType{AttrTypes: entraMemberAttrTypes}

	fabricItemMemberAttrTypes = map[string]attr.Type{
		"source_path": types.StringType,
		"item_access": types.SetType{ElemType: types.StringType},
	}
	fabricItemMemberElemType = types.ObjectType{AttrTypes: fabricItemMemberAttrTypes}
)

// LakehousePermissionResource manages a OneLake Data Access Role on a Microsoft Fabric Lakehouse.
type LakehousePermissionResource struct {
	client   *client.FabricClient
	tenantID string
}

// RowConstraintModel describes a row-level security predicate constraint.
type RowConstraintModel struct {
	TablePath types.String `tfsdk:"table_path"`
	Predicate types.String `tfsdk:"predicate"`
}

// ColumnConstraintModel describes a column-level security constraint.
type ColumnConstraintModel struct {
	TablePath types.String `tfsdk:"table_path"`
	Columns   types.Set    `tfsdk:"columns"`
	Action    types.String `tfsdk:"action"`
	Effect    types.String `tfsdk:"effect"`
}

// DecisionRuleModel describes an advanced decision rule with optional constraints.
type DecisionRuleModel struct {
	Paths            types.Set    `tfsdk:"paths"`
	Actions          types.Set    `tfsdk:"actions"`
	Effect           types.String `tfsdk:"effect"`
	RowConstraint    types.List   `tfsdk:"row_constraint"`
	ColumnConstraint types.List   `tfsdk:"column_constraint"`
}

// EntraMemberModel describes an explicit Microsoft Entra member in advanced mode.
type EntraMemberModel struct {
	ObjectID   types.String `tfsdk:"object_id"`
	ObjectType types.String `tfsdk:"object_type"`
	TenantID   types.String `tfsdk:"tenant_id"`
}

// FabricItemMemberModel describes a workspace item member for cross-item inheritance.
type FabricItemMemberModel struct {
	SourcePath types.String `tfsdk:"source_path"`
	ItemAccess types.Set    `tfsdk:"item_access"`
}

// LakehousePermissionResourceModel describes the Terraform state model for fabricext_lakehouse_permission.
type LakehousePermissionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	LakehouseName types.String `tfsdk:"lakehouse_name"`
	LakehouseID   types.String `tfsdk:"lakehouse_id"`
	RoleName      types.String `tfsdk:"role_name"`
	Kind          types.String `tfsdk:"kind"`

	// Simple flat mode attributes
	Paths         types.Set    `tfsdk:"paths"`
	Actions       types.Set    `tfsdk:"actions"`
	PrincipalIDs  types.Set    `tfsdk:"principal_ids"`
	PrincipalType types.String `tfsdk:"principal_type"`

	// Advanced structured mode blocks
	DecisionRule     types.List `tfsdk:"decision_rule"`
	EntraMember      types.List `tfsdk:"entra_member"`
	FabricItemMember types.List `tfsdk:"fabric_item_member"`
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
		Description: "Manages a OneLake Data Access Role on a Microsoft Fabric Lakehouse to grant fine-grained folder, table, row-level (RLS), and column-level (CLS) access control to Microsoft Entra users, groups, service principals, or managed identities.",
		MarkdownDescription: "Manages a OneLake Data Access Role on a Microsoft Fabric **Lakehouse** to grant fine-grained folder, table, row-level (RLS), and column-level (CLS) access control to Microsoft Entra users, groups, service principals, or managed identities.\n\n" +
			"~> **Recommendation:** Microsoft's official provider (`registry.terraform.io/microsoft/fabric`) provides the [`fabric_onelake_data_access_security`](https://registry.terraform.io/providers/microsoft/fabric/latest/docs/resources/onelake_data_access_security) resource (preview) for managing OneLake Data Access Roles. If consolidating on the official provider with `preview = true`, consider using upstream.\n\n" +
			"-> **Note:** This resource manages **OneLake Data Access Roles** (storage layer access control via the OneLake Data Access Security API). It does **not** manage Fabric workspace item-level sharing (`ReadAll` workspace share grants), as Microsoft does not currently provide a public REST API for Lakehouse item sharing (see [microsoft/terraform-provider-fabric#425](https://github.com/microsoft/terraform-provider-fabric/issues/425)).\n\n" +
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
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the target Microsoft Fabric Lakehouse. At least one of `lakehouse_name` or `lakehouse_id` must be specified.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					uuidValidator(),
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
			"kind": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Policy"),
				MarkdownDescription: "Kind of the OneLake Data Access Role. The only supported value is `Policy`. Defaults to `Policy`.",
				Validators: []validator.String{
					stringvalidator.OneOf("Policy"),
				},
			},
			"paths": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of OneLake paths (`/Tables/...`, `/Files/...`, or `*`) granted by this role in simple mode.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"actions": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of OneLake actions permitted on `paths` in simple mode. Valid values: `Read`. Defaults to `[\"Read\"]` in simple mode.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf("Read")),
				},
			},
			"principal_ids": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of Microsoft Entra Object IDs (UUIDs) assigned as members of this role in simple mode.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(uuidValidator()),
				},
			},
			"principal_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Microsoft Entra object type for `principal_ids` in simple mode. Valid values: `User`, `Group`, `ServicePrincipal`, `ManagedIdentity`. Defaults to `Group` in simple mode.",
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ManagedIdentity"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"decision_rule": schema.ListNestedBlock{
				MarkdownDescription: "Advanced decision rules defining fine-grained path permissions, row-level security (RLS), and column-level security (CLS).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"paths": schema.SetAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Set of OneLake paths (`/Tables/...`, `/Files/...`, or `*`) granted by this decision rule.",
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
						"effect": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("Permit"),
							MarkdownDescription: "Effect of the rule. The only supported value is `Permit`. Defaults to `Permit`.",
							Validators: []validator.String{
								stringvalidator.OneOf("Permit"),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"row_constraint": schema.ListNestedBlock{
							MarkdownDescription: "Row-Level Security (RLS) predicates applied to tables.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"table_path": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Relative OneLake path to the table (e.g., `/Tables/sales`).",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
									"predicate": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "T-SQL predicate expression used to filter visible rows (e.g., `Region = 'EMEA'`).",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
								},
							},
						},
						"column_constraint": schema.ListNestedBlock{
							MarkdownDescription: "Column-Level Security (CLS) constraints applied to tables.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"table_path": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Relative OneLake path to the table (e.g., `/Tables/sales`).",
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
									"columns": schema.SetAttribute{
										Required:            true,
										ElementType:         types.StringType,
										MarkdownDescription: "Set of column names visible to the role members, or `*` for all columns.",
										Validators: []validator.Set{
											setvalidator.SizeAtLeast(1),
										},
									},
									"action": schema.StringAttribute{
										Optional:            true,
										Computed:            true,
										Default:             stringdefault.StaticString("Read"),
										MarkdownDescription: "Action applied to columns. Valid values: `Read`. Defaults to `Read`.",
										Validators: []validator.String{
											stringvalidator.OneOf("Read"),
										},
									},
									"effect": schema.StringAttribute{
										Optional:            true,
										Computed:            true,
										Default:             stringdefault.StaticString("Permit"),
										MarkdownDescription: "Effect given to columns. Valid values: `Permit`. Defaults to `Permit`.",
										Validators: []validator.String{
											stringvalidator.OneOf("Permit"),
										},
									},
								},
							},
						},
					},
				},
			},
			"entra_member": schema.ListNestedBlock{
				MarkdownDescription: "Explicit Microsoft Entra ID members (users, groups, service principals, managed identities) assigned to this role in advanced mode.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"object_id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Microsoft Entra Object ID (UUID) of the member.",
							Validators: []validator.String{
								uuidValidator(),
							},
						},
						"object_type": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("Group"),
							MarkdownDescription: "Microsoft Entra object type. Valid values: `User`, `Group`, `ServicePrincipal`, `ManagedIdentity`. Defaults to `Group`.",
							Validators: []validator.String{
								stringvalidator.OneOf("User", "Group", "ServicePrincipal", "ManagedIdentity"),
							},
						},
						"tenant_id": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Microsoft Entra Tenant ID (UUID). If omitted, defaults to the provider tenant ID.",
							Validators: []validator.String{
								uuidValidator(),
							},
						},
					},
				},
			},
			"fabric_item_member": schema.ListNestedBlock{
				MarkdownDescription: "Workspace item members granted access through Fabric item inheritance or shortcuts in advanced mode.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"source_path": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Workspace item source path in the format `{workspace_id}/{item_id}`.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"item_access": schema.SetAttribute{
							Optional:            true,
							Computed:            true,
							ElementType:         types.StringType,
							Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{types.StringValue("ReadAll")})),
							MarkdownDescription: "Set of Fabric item access permissions. Valid values: `Read`, `Write`, `Reshare`, `Explore`, `Execute`, `ReadAll`. Defaults to `[\"ReadAll\"]`.",
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueStringsAre(stringvalidator.OneOf("Read", "Write", "Reshare", "Explore", "Execute", "ReadAll")),
							},
						},
					},
				},
			},
		},
	}
}

func isListConfigured(l types.List) bool {
	if l.IsNull() || l.IsUnknown() {
		return false
	}
	return len(l.Elements()) > 0
}

// ValidateConfig validates cross-attribute requirements, mutually exclusive modes, and required identifiers.
func (r *LakehousePermissionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// 1. Lakehouse identifier check
	if !config.LakehouseName.IsUnknown() && !config.LakehouseID.IsUnknown() {
		if (config.LakehouseName.IsNull() || config.LakehouseName.ValueString() == "") &&
			(config.LakehouseID.IsNull() || config.LakehouseID.ValueString() == "") {
			resp.Diagnostics.AddError(
				"Missing Lakehouse Identifier",
				"At least one of lakehouse_name or lakehouse_id must be specified.",
			)
		}
	}

	// If any mode-determining attribute or block is unknown during ValidateConfig,
	// defer cross-validation until planning / apply when expressions are resolved.
	if config.Paths.IsUnknown() || config.PrincipalIDs.IsUnknown() ||
		config.DecisionRule.IsUnknown() || config.EntraMember.IsUnknown() || config.FabricItemMember.IsUnknown() {
		return
	}

	hasSimplePaths := !config.Paths.IsNull() && len(config.Paths.Elements()) > 0
	hasSimplePrincipals := !config.PrincipalIDs.IsNull() && len(config.PrincipalIDs.Elements()) > 0
	hasSimple := hasSimplePaths || hasSimplePrincipals

	hasRules := isListConfigured(config.DecisionRule)
	hasEntra := isListConfigured(config.EntraMember)
	hasFabric := isListConfigured(config.FabricItemMember)
	hasAdvanced := hasRules || hasEntra || hasFabric

	// 2. Mutual exclusivity between simple mode and advanced mode
	if hasSimple && hasAdvanced {
		resp.Diagnostics.AddError(
			"Conflicting Role Definition",
			"Cannot configure both simple mode attributes (paths, principal_ids) and advanced mode blocks (decision_rule, entra_member, fabric_item_member). Use either simple mode or advanced mode.",
		)
		return
	}

	// 3. Completeness of simple mode
	if hasSimple {
		if !hasSimplePaths {
			resp.Diagnostics.AddError(
				"Missing Required Attribute in Simple Mode",
				"Attribute paths is required when configuring a role in simple mode.",
			)
		}
		if !hasSimplePrincipals {
			resp.Diagnostics.AddError(
				"Missing Required Attribute in Simple Mode",
				"Attribute principal_ids is required when configuring a role in simple mode.",
			)
		}
		return
	}

	// 4. Completeness of advanced mode
	if hasAdvanced {
		if !hasRules {
			resp.Diagnostics.AddError(
				"Missing Required Block in Advanced Mode",
				"At least one decision_rule block must be defined when configuring a role in advanced mode.",
			)
		}
		if !hasEntra && !hasFabric {
			resp.Diagnostics.AddError(
				"Missing Required Members in Advanced Mode",
				"At least one entra_member or fabric_item_member block must be defined when configuring a role in advanced mode.",
			)
		}
		return
	}

	// 5. Missing both modes
	resp.Diagnostics.AddError(
		"Missing Role Definition",
		"Must configure either simple mode attributes (paths, principal_ids) or advanced mode blocks (decision_rule, entra_member, fabric_item_member).",
	)
}

// ModifyPlan sets defaults and cleans up mutually exclusive mode attributes in plan.
func (r *LakehousePermissionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan LakehousePermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.isAdvancedMode(&plan) {
		plan.Paths = types.SetNull(types.StringType)
		plan.Actions = types.SetNull(types.StringType)
		plan.PrincipalIDs = types.SetNull(types.StringType)
		plan.PrincipalType = types.StringNull()
	} else {
		plan.DecisionRule = types.ListNull(decisionRuleElemType)
		plan.EntraMember = types.ListNull(entraMemberElemType)
		plan.FabricItemMember = types.ListNull(fabricItemMemberElemType)

		if plan.Actions.IsUnknown() || plan.Actions.IsNull() {
			plan.Actions = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("Read")})
		}
		if plan.PrincipalType.IsUnknown() || plan.PrincipalType.IsNull() {
			plan.PrincipalType = types.StringValue("Group")
		}
	}

	if plan.Kind.IsUnknown() || plan.Kind.IsNull() {
		plan.Kind = types.StringValue("Policy")
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
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

func (r *LakehousePermissionResource) isAdvancedMode(m *LakehousePermissionResourceModel) bool {
	return isListConfigured(m.DecisionRule) || isListConfigured(m.EntraMember) || isListConfigured(m.FabricItemMember)
}

func (r *LakehousePermissionResource) buildRolePayload(ctx context.Context, m *LakehousePermissionResourceModel) (client.DataAccessRole, diag.Diagnostics) {
	var diags diag.Diagnostics

	kind := "Policy"
	if !m.Kind.IsNull() && !m.Kind.IsUnknown() && m.Kind.ValueString() != "" {
		kind = m.Kind.ValueString()
	}

	if r.isAdvancedMode(m) {
		var rules []client.DecisionRule
		var decisionRules []DecisionRuleModel
		if isListConfigured(m.DecisionRule) && !m.DecisionRule.IsUnknown() {
			diags.Append(m.DecisionRule.ElementsAs(ctx, &decisionRules, false)...)
		}

		for _, dr := range decisionRules {
			var rulePaths []string
			diags.Append(dr.Paths.ElementsAs(ctx, &rulePaths, false)...)
			var ruleActions []string
			if !dr.Actions.IsNull() && !dr.Actions.IsUnknown() {
				diags.Append(dr.Actions.ElementsAs(ctx, &ruleActions, false)...)
			}
			if len(ruleActions) == 0 {
				ruleActions = []string{"Read"}
			}

			effect := "Permit"
			if !dr.Effect.IsNull() && !dr.Effect.IsUnknown() && dr.Effect.ValueString() != "" {
				effect = dr.Effect.ValueString()
			}

			var rowConstraints []client.RowConstraint
			var rowModels []RowConstraintModel
			if isListConfigured(dr.RowConstraint) && !dr.RowConstraint.IsUnknown() {
				diags.Append(dr.RowConstraint.ElementsAs(ctx, &rowModels, false)...)
				for _, rc := range rowModels {
					rowConstraints = append(rowConstraints, client.RowConstraint{
						TablePath: rc.TablePath.ValueString(),
						Value:     rc.Predicate.ValueString(),
					})
				}
			}

			var colConstraints []client.ColumnConstraint
			var colModels []ColumnConstraintModel
			if isListConfigured(dr.ColumnConstraint) && !dr.ColumnConstraint.IsUnknown() {
				diags.Append(dr.ColumnConstraint.ElementsAs(ctx, &colModels, false)...)
				for _, cc := range colModels {
					var cols []string
					diags.Append(cc.Columns.ElementsAs(ctx, &cols, false)...)
					action := "Read"
					if !cc.Action.IsNull() && !cc.Action.IsUnknown() && cc.Action.ValueString() != "" {
						action = cc.Action.ValueString()
					}
					colEffect := "Permit"
					if !cc.Effect.IsNull() && !cc.Effect.IsUnknown() && cc.Effect.ValueString() != "" {
						colEffect = cc.Effect.ValueString()
					}
					colConstraints = append(colConstraints, client.ColumnConstraint{
						TablePath:    cc.TablePath.ValueString(),
						ColumnNames:  cols,
						ColumnAction: []string{action},
						ColumnEffect: colEffect,
					})
				}
			}

			var constraints *client.Constraints
			if len(rowConstraints) > 0 || len(colConstraints) > 0 {
				constraints = &client.Constraints{
					Rows:    rowConstraints,
					Columns: colConstraints,
				}
			}

			rules = append(rules, client.DecisionRule{
				Effect: effect,
				Permission: []client.PermissionScope{
					{
						AttributeName:            "Path",
						AttributeValueIncludedIn: rulePaths,
					},
					{
						AttributeName:            "Action",
						AttributeValueIncludedIn: ruleActions,
					},
				},
				Constraints: constraints,
			})
		}

		var entraMembers []client.MicrosoftEntraMember
		var entraModels []EntraMemberModel
		if isListConfigured(m.EntraMember) && !m.EntraMember.IsUnknown() {
			diags.Append(m.EntraMember.ElementsAs(ctx, &entraModels, false)...)
			for _, em := range entraModels {
				tID := r.tenantID
				if !em.TenantID.IsNull() && !em.TenantID.IsUnknown() && em.TenantID.ValueString() != "" {
					tID = em.TenantID.ValueString()
				}
				objType := "Group"
				if !em.ObjectType.IsNull() && !em.ObjectType.IsUnknown() && em.ObjectType.ValueString() != "" {
					objType = em.ObjectType.ValueString()
				}
				entraMembers = append(entraMembers, client.MicrosoftEntraMember{
					TenantID:   tID,
					ObjectID:   em.ObjectID.ValueString(),
					ObjectType: objType,
				})
			}
		}

		var fabricMembers []client.FabricItemMember
		var fabricModels []FabricItemMemberModel
		if isListConfigured(m.FabricItemMember) && !m.FabricItemMember.IsUnknown() {
			diags.Append(m.FabricItemMember.ElementsAs(ctx, &fabricModels, false)...)
			for _, fm := range fabricModels {
				var access []string
				if !fm.ItemAccess.IsNull() && !fm.ItemAccess.IsUnknown() {
					diags.Append(fm.ItemAccess.ElementsAs(ctx, &access, false)...)
				}
				if len(access) == 0 {
					access = []string{"ReadAll"}
				}
				fabricMembers = append(fabricMembers, client.FabricItemMember{
					SourcePath: fm.SourcePath.ValueString(),
					ItemAccess: access,
				})
			}
		}

		members := &client.Members{}
		if len(entraMembers) > 0 {
			members.MicrosoftEntraMembers = entraMembers
		}
		if len(fabricMembers) > 0 {
			members.FabricItemMembers = fabricMembers
		}

		return client.DataAccessRole{
			Name:          m.RoleName.ValueString(),
			Kind:          kind,
			DecisionRules: rules,
			Members:       members,
		}, diags
	}

	// Simple flat mode
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

	principalType := "Group"
	if !m.PrincipalType.IsNull() && !m.PrincipalType.IsUnknown() && m.PrincipalType.ValueString() != "" {
		principalType = m.PrincipalType.ValueString()
	}

	entraMembers := make([]client.MicrosoftEntraMember, 0, len(principalIDs))
	for _, pid := range principalIDs {
		entraMembers = append(entraMembers, client.MicrosoftEntraMember{
			TenantID:   r.tenantID,
			ObjectID:   pid,
			ObjectType: principalType,
		})
	}

	return client.DataAccessRole{
		Name: m.RoleName.ValueString(),
		Kind: kind,
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
	var lhID string
	var lhName string

	if !plan.LakehouseID.IsNull() && !plan.LakehouseID.IsUnknown() && plan.LakehouseID.ValueString() != "" {
		lhID = plan.LakehouseID.ValueString()
		item, err := r.client.GetItemByID(ctx, wsID, lhID, "Lakehouse")
		if err != nil {
			resp.Diagnostics.AddError("Unable to Resolve Lakehouse by ID", err.Error())
			return
		}
		lhName = item.DisplayName
		plan.LakehouseName = types.StringValue(lhName)
	} else {
		lhName = plan.LakehouseName.ValueString()
		var err error
		lhID, err = r.client.GetItemIDByName(ctx, wsID, lhName, "Lakehouse")
		if err != nil {
			resp.Diagnostics.AddError("Unable to Resolve Lakehouse by Name", err.Error())
			return
		}
		plan.LakehouseID = types.StringValue(lhID)
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
	plan.LakehouseName = types.StringValue(lhName)
	plan.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", wsID, lhID, plan.RoleName.ValueString()))
	if plan.Kind.IsNull() || plan.Kind.IsUnknown() {
		plan.Kind = types.StringValue("Policy")
	}

	diags = r.populateLakehouseStateFromRole(ctx, &rolePayload, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	diags := r.populateLakehouseStateFromRole(ctx, role, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func isRoleAdvanced(role *client.DataAccessRole) bool {
	if len(role.DecisionRules) != 1 {
		return true
	}
	rule := role.DecisionRules[0]
	if rule.Constraints != nil && (len(rule.Constraints.Rows) > 0 || len(rule.Constraints.Columns) > 0) {
		return true
	}
	if role.Members != nil && len(role.Members.FabricItemMembers) > 0 {
		return true
	}
	if role.Members != nil && len(role.Members.MicrosoftEntraMembers) > 1 {
		firstType := role.Members.MicrosoftEntraMembers[0].ObjectType
		for _, m := range role.Members.MicrosoftEntraMembers[1:] {
			if m.ObjectType != firstType {
				return true
			}
		}
	}
	return false
}

func (r *LakehousePermissionResource) populateLakehouseStateFromRole(ctx context.Context, role *client.DataAccessRole, state *LakehousePermissionResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	advanced := r.isAdvancedMode(state) || isRoleAdvanced(role)

	kind := role.Kind
	if kind == "" {
		kind = "Policy"
	}
	state.Kind = types.StringValue(kind)

	if advanced {
		state.Paths = types.SetNull(types.StringType)
		state.Actions = types.SetNull(types.StringType)
		state.PrincipalIDs = types.SetNull(types.StringType)
		state.PrincipalType = types.StringNull()

		var decisionRuleModels []DecisionRuleModel
		for _, rule := range role.DecisionRules {
			var rulePaths []string
			var ruleActions []string
			for _, scope := range rule.Permission {
				switch scope.AttributeName {
				case "Path":
					rulePaths = append(rulePaths, scope.AttributeValueIncludedIn...)
				case "Action":
					ruleActions = append(ruleActions, scope.AttributeValueIncludedIn...)
				}
			}
			slices.Sort(rulePaths)
			slices.Sort(ruleActions)

			pathsSet, d := types.SetValueFrom(ctx, types.StringType, rulePaths)
			diags.Append(d...)
			if len(ruleActions) == 0 {
				ruleActions = []string{"Read"}
			}
			actionsSet, d := types.SetValueFrom(ctx, types.StringType, ruleActions)
			diags.Append(d...)

			effect := rule.Effect
			if effect == "" {
				effect = "Permit"
			}

			var rowModels []RowConstraintModel
			var colModels []ColumnConstraintModel

			if rule.Constraints != nil {
				for _, rc := range rule.Constraints.Rows {
					rowModels = append(rowModels, RowConstraintModel{
						TablePath: types.StringValue(rc.TablePath),
						Predicate: types.StringValue(rc.Value),
					})
				}
				for _, cc := range rule.Constraints.Columns {
					cols := slices.Clone(cc.ColumnNames)
					slices.Sort(cols)
					colsSet, d := types.SetValueFrom(ctx, types.StringType, cols)
					diags.Append(d...)
					colAction := "Read"
					if len(cc.ColumnAction) > 0 {
						colAction = cc.ColumnAction[0]
					}
					colEffect := cc.ColumnEffect
					if colEffect == "" {
						colEffect = "Permit"
					}
					colModels = append(colModels, ColumnConstraintModel{
						TablePath: types.StringValue(cc.TablePath),
						Columns:   colsSet,
						Action:    types.StringValue(colAction),
						Effect:    types.StringValue(colEffect),
					})
				}
			}

			var rowList types.List
			if len(rowModels) > 0 {
				var d diag.Diagnostics
				rowList, d = types.ListValueFrom(ctx, rowConstraintElemType, rowModels)
				diags.Append(d...)
			} else {
				rowList = types.ListNull(rowConstraintElemType)
			}

			var colList types.List
			if len(colModels) > 0 {
				var d diag.Diagnostics
				colList, d = types.ListValueFrom(ctx, columnConstraintElemType, colModels)
				diags.Append(d...)
			} else {
				colList = types.ListNull(columnConstraintElemType)
			}

			decisionRuleModels = append(decisionRuleModels, DecisionRuleModel{
				Paths:            pathsSet,
				Actions:          actionsSet,
				Effect:           types.StringValue(effect),
				RowConstraint:    rowList,
				ColumnConstraint: colList,
			})
		}

		if len(decisionRuleModels) > 0 {
			var d diag.Diagnostics
			state.DecisionRule, d = types.ListValueFrom(ctx, decisionRuleElemType, decisionRuleModels)
			diags.Append(d...)
		} else {
			state.DecisionRule = types.ListNull(decisionRuleElemType)
		}

		var entraModels []EntraMemberModel
		if role.Members != nil {
			for _, m := range role.Members.MicrosoftEntraMembers {
				objType := m.ObjectType
				if objType == "" {
					objType = "Group"
				} else if norm, err := normalizePrincipalType("Lakehouse", objType); err == nil {
					objType = norm
				}
				tenantIDVal := m.TenantID
				if tenantIDVal == "" {
					tenantIDVal = r.tenantID
				}
				entraModels = append(entraModels, EntraMemberModel{
					ObjectID:   types.StringValue(m.ObjectID),
					ObjectType: types.StringValue(objType),
					TenantID:   types.StringValue(tenantIDVal),
				})
			}
		}

		if len(entraModels) > 0 {
			var d diag.Diagnostics
			state.EntraMember, d = types.ListValueFrom(ctx, entraMemberElemType, entraModels)
			diags.Append(d...)
		} else {
			state.EntraMember = types.ListNull(entraMemberElemType)
		}

		var fabricModels []FabricItemMemberModel
		if role.Members != nil {
			for _, fm := range role.Members.FabricItemMembers {
				access := slices.Clone(fm.ItemAccess)
				if len(access) == 0 {
					access = []string{"ReadAll"}
				}
				slices.Sort(access)
				itemAccessSet, d := types.SetValueFrom(ctx, types.StringType, access)
				diags.Append(d...)
				fabricModels = append(fabricModels, FabricItemMemberModel{
					SourcePath: types.StringValue(fm.SourcePath),
					ItemAccess: itemAccessSet,
				})
			}
		}

		if len(fabricModels) > 0 {
			var d diag.Diagnostics
			state.FabricItemMember, d = types.ListValueFrom(ctx, fabricItemMemberElemType, fabricModels)
			diags.Append(d...)
		} else {
			state.FabricItemMember = types.ListNull(fabricItemMemberElemType)
		}

		return diags
	}

	// Simple flat mode
	state.DecisionRule = types.ListNull(decisionRuleElemType)
	state.EntraMember = types.ListNull(entraMemberElemType)
	state.FabricItemMember = types.ListNull(fabricItemMemberElemType)

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
	principalType := "Group"
	if !state.PrincipalType.IsNull() && !state.PrincipalType.IsUnknown() && state.PrincipalType.ValueString() != "" {
		principalType = state.PrincipalType.ValueString()
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

	slices.Sort(paths)
	slices.Sort(actions)
	slices.Sort(principalIDs)

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
	plan.LakehouseName = state.LakehouseName
	plan.ID = state.ID
	if plan.Kind.IsNull() || plan.Kind.IsUnknown() {
		plan.Kind = types.StringValue("Policy")
	}

	diags = r.populateLakehouseStateFromRole(ctx, &rolePayload, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
		Kind:          types.StringValue("Policy"),
	}
	resp.Diagnostics.Append(r.populateLakehouseStateFromRole(ctx, role, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
