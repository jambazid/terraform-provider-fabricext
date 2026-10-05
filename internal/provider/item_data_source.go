// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
)

var (
	_ datasource.DataSource              = &ItemDataSource{}
	_ datasource.DataSourceWithConfigure = &ItemDataSource{}
)

// ItemDataSource resolves a Microsoft Fabric item's UUID by (workspace_id, display_name, type).
type ItemDataSource struct {
	client *client.FabricClient
}

// ItemDataSourceModel describes the Terraform state model for data.fabricext_item.
type ItemDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	DisplayName types.String `tfsdk:"display_name"`
	Type        types.String `tfsdk:"type"`
}

// NewItemDataSource constructs a new fabricext_item data source.
func NewItemDataSource() datasource.DataSource {
	return &ItemDataSource{}
}

// Metadata sets the data source type name.
func (d *ItemDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

// Schema defines the schema for data.fabricext_item.
func (d *ItemDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resolves a Microsoft Fabric item's canonical UUID (`id`) from its `(workspace_id, display_name, type)` tuple.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved UUID of the Microsoft Fabric item.",
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the Microsoft Fabric workspace containing the item.",
				Validators: []validator.String{
					uuidValidator(),
				},
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the Microsoft Fabric item.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Microsoft Fabric item type. Valid values: `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `SemanticModel`.",
				Validators: []validator.String{
					stringvalidator.OneOf("Warehouse", "Lakehouse", "SQLDatabase", "SQLEndpoint", "SemanticModel"),
				},
			},
		},
	}
}

// Configure wires the provider's FabricClient into the data source.
func (d *ItemDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read resolves the Microsoft Fabric item's UUID and updates Terraform state.
func (d *ItemDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ItemDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := d.client.GetItemIDByName(
		ctx,
		data.WorkspaceID.ValueString(),
		data.DisplayName.ValueString(),
		data.Type.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Microsoft Fabric Item", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
