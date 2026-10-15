// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ datasource.DataSource              = &ItemAccessDetailsDataSource{}
	_ datasource.DataSourceWithConfigure = &ItemAccessDetailsDataSource{}
)

// ItemAccessDetailsDataSource queries the Microsoft Fabric Admin API to read effective permissions on an item.
type ItemAccessDetailsDataSource struct {
	client *client.FabricClient
}

// ItemAccessDetailsDataSourceModel describes the Terraform state model for data.fabricext_item_access_details.
type ItemAccessDetailsDataSourceModel struct {
	WorkspaceID          types.String `tfsdk:"workspace_id"`
	ItemID               types.String `tfsdk:"item_id"`
	Type                 types.String `tfsdk:"type"`
	AccessDetails        types.List   `tfsdk:"access_details"`
	PrincipalPermissions types.Map    `tfsdk:"principal_permissions"`
}

var principalAttrTypes = map[string]attr.Type{
	"id":                  types.StringType,
	"display_name":        types.StringType,
	"type":                types.StringType,
	"user_principal_name": types.StringType,
	"group_type":          types.StringType,
	"aad_app_id":          types.StringType,
}

var itemAccessDetailAttrTypes = map[string]attr.Type{
	"type":                   types.StringType,
	"permissions":            types.SetType{ElemType: types.StringType},
	"additional_permissions": types.SetType{ElemType: types.StringType},
}

var accessDetailElemAttrTypes = map[string]attr.Type{
	"principal":           types.ObjectType{AttrTypes: principalAttrTypes},
	"item_access_details": types.ObjectType{AttrTypes: itemAccessDetailAttrTypes},
}

// NewItemAccessDetailsDataSource constructs a new fabricext_item_access_details data source.
func NewItemAccessDetailsDataSource() datasource.DataSource {
	return &ItemAccessDetailsDataSource{}
}

// Metadata sets the data source type name.
func (d *ItemAccessDetailsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item_access_details"
}

// Schema defines the schema for data.fabricext_item_access_details.
func (d *ItemAccessDetailsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Queries effective permissions and user access details for a Microsoft Fabric workspace item via the Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`). Requires Fabric Administrator privileges or a service principal enrolled in read-only admin APIs.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The UUID of the Microsoft Fabric workspace.",
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"item_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The UUID of the Microsoft Fabric item.",
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional item type for route disambiguation (e.g., `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `Report`).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"access_details": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of all user, group, and service principal access details returned for the item.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"principal": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Information regarding the principal who has access to the item.",
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The principal's Microsoft Entra Object ID (UUID).",
								},
								"display_name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The principal's display name.",
								},
								"type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "The principal type (`User`, `Group`, `ServicePrincipal`, `ServicePrincipalProfile`, `EntireTenant`).",
								},
								"user_principal_name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "User Principal Name (UPN), if the principal type is `User`.",
								},
								"group_type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Group type (e.g., `SecurityGroup`), if the principal type is `Group`.",
								},
								"aad_app_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Microsoft Entra Application ID (UUID), if the principal type is `ServicePrincipal`.",
								},
							},
						},
						"item_access_details": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Permissions granted to the principal on the item.",
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Item type returned by the Admin API.",
								},
								"permissions": schema.SetAttribute{
									Computed:            true,
									ElementType:         types.StringType,
									MarkdownDescription: "Standard Fabric item permissions (e.g. `Read`, `Write`, `Reshare`, `Explore`, `Execute`).",
								},
								"additional_permissions": schema.SetAttribute{
									Computed:            true,
									ElementType:         types.StringType,
									MarkdownDescription: "Workload-specific additional permissions (e.g. `ReadData`, `ReadAll`, `viewOutput`).",
								},
							},
						},
					},
				},
			},
			"principal_permissions": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.SetType{ElemType: types.StringType},
				MarkdownDescription: "Convenience map lookup keyed by principal ID (UUID), mapping to the union set of item permissions and additional permissions.",
			},
		},
	}
}

// Configure wires the provider's FabricClient into the data source.
func (d *ItemAccessDetailsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *Data, got: %T", req.ProviderData))
		return
	}
	d.client = pd.Client
}

// Read queries the Microsoft Fabric Admin API and projects the item access details.
func (d *ItemAccessDetailsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ItemAccessDetailsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError("Unconfigured Fabric Client", "The provider was not properly configured before data source operation.")
		return
	}

	var itemTypePtr *string
	if !data.Type.IsNull() && !data.Type.IsUnknown() && data.Type.ValueString() != "" {
		v := data.Type.ValueString()
		itemTypePtr = &v
	}

	wsID := data.WorkspaceID.ValueString()
	itemID := data.ItemID.ValueString()

	apiResp, err := d.client.ListItemAccessDetails(ctx, wsID, itemID, itemTypePtr)
	if err != nil {
		var adminErr *client.AdminAPIAccessDeniedError
		if errors.As(err, &adminErr) {
			resp.Diagnostics.AddError("Fabric Admin API Access Denied", adminErr.Error())
			return
		}
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Microsoft Fabric Item Not Found", fmt.Sprintf("Item %q was not found in workspace %q", itemID, wsID))
			return
		}
		resp.Diagnostics.AddError("Unable to Read Microsoft Fabric Item Access Details", err.Error())
		return
	}

	elemObjs := make([]attr.Value, 0, len(apiResp.AccessDetails))
	permMap := make(map[string]types.Set)

	for _, item := range apiResp.AccessDetails {
		var upnVal, grpTypeVal, appIDVal types.String
		if item.Principal.UserDetails != nil && item.Principal.UserDetails.UserPrincipalName != "" {
			upnVal = types.StringValue(item.Principal.UserDetails.UserPrincipalName)
		} else {
			upnVal = types.StringNull()
		}

		if item.Principal.GroupDetails != nil && item.Principal.GroupDetails.GroupType != "" {
			grpTypeVal = types.StringValue(item.Principal.GroupDetails.GroupType)
		} else {
			grpTypeVal = types.StringNull()
		}

		if item.Principal.ServicePrincipalDetails != nil && item.Principal.ServicePrincipalDetails.AadAppID != "" {
			appIDVal = types.StringValue(item.Principal.ServicePrincipalDetails.AadAppID)
		} else {
			appIDVal = types.StringNull()
		}

		principalAttrs := map[string]attr.Value{
			"id":                  types.StringValue(item.Principal.ID),
			"display_name":        stringOrNull(item.Principal.DisplayName),
			"type":                stringOrNull(item.Principal.Type),
			"user_principal_name": upnVal,
			"group_type":          grpTypeVal,
			"aad_app_id":          appIDVal,
		}
		pObj, pDiags := types.ObjectValue(principalAttrTypes, principalAttrs)
		resp.Diagnostics.Append(pDiags...)

		perms := item.ItemAccessDetails.Permissions
		if perms == nil {
			perms = []string{}
		}
		permsSet, permsDiags := types.SetValueFrom(ctx, types.StringType, perms)
		resp.Diagnostics.Append(permsDiags...)

		addPerms := item.ItemAccessDetails.AdditionalPermissions
		if addPerms == nil {
			addPerms = []string{}
		}
		addPermsSet, addPermsDiags := types.SetValueFrom(ctx, types.StringType, addPerms)
		resp.Diagnostics.Append(addPermsDiags...)

		itemAttrs := map[string]attr.Value{
			"type":                   stringOrNull(item.ItemAccessDetails.Type),
			"permissions":            permsSet,
			"additional_permissions": addPermsSet,
		}
		iObj, iDiags := types.ObjectValue(itemAccessDetailAttrTypes, itemAttrs)
		resp.Diagnostics.Append(iDiags...)

		detailAttrs := map[string]attr.Value{
			"principal":           pObj,
			"item_access_details": iObj,
		}
		dObj, dDiags := types.ObjectValue(accessDetailElemAttrTypes, detailAttrs)
		resp.Diagnostics.Append(dDiags...)
		elemObjs = append(elemObjs, dObj)

		// Populate convenience map keyed by principal.id
		pid := item.Principal.ID
		if pid != "" {
			var unionPerms []string
			if existingSet, ok := permMap[pid]; ok {
				_ = existingSet.ElementsAs(ctx, &unionPerms, false)
			}
			for _, p := range item.ItemAccessDetails.Permissions {
				if !slices.Contains(unionPerms, p) {
					unionPerms = append(unionPerms, p)
				}
			}
			for _, ap := range item.ItemAccessDetails.AdditionalPermissions {
				if !slices.Contains(unionPerms, ap) {
					unionPerms = append(unionPerms, ap)
				}
			}
			s, sDiags := types.SetValueFrom(ctx, types.StringType, unionPerms)
			resp.Diagnostics.Append(sDiags...)
			permMap[pid] = s
		}
	}

	listVal, lDiags := types.ListValue(types.ObjectType{AttrTypes: accessDetailElemAttrTypes}, elemObjs)
	resp.Diagnostics.Append(lDiags...)
	data.AccessDetails = listVal

	mapVal, mDiags := types.MapValueFrom(ctx, types.SetType{ElemType: types.StringType}, permMap)
	resp.Diagnostics.Append(mDiags...)
	data.PrincipalPermissions = mapVal

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
