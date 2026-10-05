// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package provider implements the Terraform Plugin Framework v6 provider for Microsoft Fabric item sharing.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jambazid/terraform-provider-fabricext/internal/client"
	"github.com/jambazid/terraform-provider-fabricext/internal/credentials"
)

const (
	defaultFallbackTenantID = "00000000-0000-0000-0000-000000000000"
	defaultRequestTimeout   = "60s"
)

var (
	_ provider.Provider = &FabricProvider{}

	uuidRegex     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	roleNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
)

func uuidValidator() validator.String {
	return stringvalidator.RegexMatches(uuidRegex, "must be a valid UUID (e.g. 00000000-0000-0000-0000-000000000000)")
}

// Data holds the configured Fabric API client and tenant context shared with resources and data sources.
type Data struct {
	Client   *client.FabricClient
	TenantID string
}

// FabricProvider defines the Microsoft Fabric item-sharing Terraform provider.
type FabricProvider struct {
	version string
}

// FabricProviderModel describes the HCL configuration schema for `provider "fabric"`.
type FabricProviderModel struct {
	Endpoint                  types.String `tfsdk:"endpoint"`
	AccessToken               types.String `tfsdk:"access_token"`
	TenantID                  types.String `tfsdk:"tenant_id"`
	ClientID                  types.String `tfsdk:"client_id"`
	ClientSecret              types.String `tfsdk:"client_secret"`
	UseMSI                    types.Bool   `tfsdk:"use_msi"`
	UseOIDC                   types.Bool   `tfsdk:"use_oidc"`
	UseCLI                    types.Bool   `tfsdk:"use_cli"`
	RequestTimeout            types.String `tfsdk:"request_timeout"`
	SkipCredentialsValidation types.Bool   `tfsdk:"skip_credentials_validation"`
}

// New creates a new provider factory for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FabricProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name and version.
func (p *FabricProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fabricext"
	resp.Version = p.version
}

// Schema defines the provider-level configuration schema.
func (p *FabricProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The **Fabric Extensions (fabricext)** provider (`registry.terraform.io/jambazid/fabricext`) is a purpose-built provider for declaratively managing Microsoft Fabric item-level sharing and data-access permissions (`fabricext_*`) across Warehouses, SQL Databases, and Lakehouses until equivalent resources are available in Microsoft's official `microsoft/fabric` provider.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL for the Microsoft Fabric REST API. Defaults to `https://api.fabric.microsoft.com`. Can also be sourced from `FABRIC_ENDPOINT`.",
			},
			"access_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Static Microsoft Entra ID bearer token for the Microsoft Fabric API (`https://api.fabric.microsoft.com/.default`). Can also be sourced from `FABRIC_ACCESS_TOKEN`.",
			},
			"tenant_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Microsoft Entra ID tenant UUID. Can also be sourced from `FABRIC_TENANT_ID`, `AZURE_TENANT_ID`, or `ARM_TENANT_ID`.",
			},
			"client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Microsoft Entra ID application (client) UUID. Can also be sourced from `FABRIC_CLIENT_ID`, `AZURE_CLIENT_ID`, or `ARM_CLIENT_ID`.",
			},
			"client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Microsoft Entra ID Service Principal client secret. Can also be sourced from `FABRIC_CLIENT_SECRET`, `AZURE_CLIENT_SECRET`, or `ARM_CLIENT_SECRET`.",
			},
			"use_msi": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable Azure Managed Identity authentication. Can also be sourced from `FABRIC_USE_MSI`, `AZURE_USE_MSI`, or `ARM_USE_MSI`.",
			},
			"use_oidc": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable Microsoft Entra Workload Identity (OIDC) authentication. Can also be sourced from `FABRIC_USE_OIDC`, `AZURE_USE_OIDC`, or `ARM_USE_OIDC`.",
			},
			"use_cli": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow fallback to the local Azure CLI (`az login`) session. Can also be sourced from `FABRIC_USE_CLI`, `AZURE_USE_CLI`, or `ARM_USE_CLI`.",
			},
			"request_timeout": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Per-request HTTP timeout as a Go duration string (for example, `60s`). Defaults to `60s`. Can also be sourced from `FABRIC_REQUEST_TIMEOUT`.",
			},
			"skip_credentials_validation": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip eager credential validation during `Configure()`. Defaults to `false`. Can also be sourced from `FABRIC_SKIP_CREDENTIALS_VALIDATION`.",
			},
		},
	}
}

// Configure initializes the Microsoft Fabric API client and credential chain.
func (p *FabricProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config FabricProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"), "Unknown Fabric API Endpoint", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for endpoint.")
	}
	if config.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("access_token"), "Unknown Fabric Access Token", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for access_token.")
	}
	if config.TenantID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("tenant_id"), "Unknown Tenant ID", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for tenant_id.")
	}
	if config.ClientID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_id"), "Unknown Client ID", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_id.")
	}
	if config.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "Unknown Client Secret", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_secret.")
	}
	if config.UseMSI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_msi"), "Unknown UseMSI Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_msi.")
	}
	if config.UseOIDC.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_oidc"), "Unknown UseOIDC Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_oidc.")
	}
	if config.UseCLI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_cli"), "Unknown UseCLI Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_cli.")
	}
	if config.RequestTimeout.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("request_timeout"), "Unknown Request Timeout", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for request_timeout.")
	}
	if config.SkipCredentialsValidation.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("skip_credentials_validation"), "Unknown Skip Credentials Validation Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for skip_credentials_validation.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := strings.TrimSpace(os.Getenv("FABRIC_ENDPOINT"))
	if !config.Endpoint.IsNull() && strings.TrimSpace(config.Endpoint.ValueString()) != "" {
		endpoint = strings.TrimSpace(config.Endpoint.ValueString())
	}

	timeoutStr := strings.TrimSpace(os.Getenv("FABRIC_REQUEST_TIMEOUT"))
	if !config.RequestTimeout.IsNull() && strings.TrimSpace(config.RequestTimeout.ValueString()) != "" {
		timeoutStr = strings.TrimSpace(config.RequestTimeout.ValueString())
	}
	if timeoutStr == "" {
		timeoutStr = defaultRequestTimeout
	}
	requestTimeout, err := time.ParseDuration(timeoutStr)
	if err != nil || requestTimeout <= 0 {
		resp.Diagnostics.AddAttributeError(path.Root("request_timeout"), "Invalid Request Timeout", fmt.Sprintf("request_timeout %q must be a positive Go duration string (for example, \"60s\")", timeoutStr))
		return
	}

	var useOIDCPtr *bool
	if !config.UseOIDC.IsNull() {
		v := config.UseOIDC.ValueBool()
		useOIDCPtr = &v
	}
	var useCLIPtr *bool
	if !config.UseCLI.IsNull() {
		v := config.UseCLI.ValueBool()
		useCLIPtr = &v
	}

	credCfg := credentials.Config{
		AccessToken:  strings.TrimSpace(config.AccessToken.ValueString()),
		TenantID:     strings.TrimSpace(config.TenantID.ValueString()),
		ClientID:     strings.TrimSpace(config.ClientID.ValueString()),
		ClientSecret: strings.TrimSpace(config.ClientSecret.ValueString()),
		UseMSI:       config.UseMSI.ValueBool(),
		UseOIDC:      useOIDCPtr,
		UseCLI:       useCLIPtr,
	}

	if err := credentials.ValidateConfig(credCfg); err != nil {
		resp.Diagnostics.AddError("Invalid Provider Credential Configuration", err.Error())
		return
	}

	chain := credentials.NewChain(credCfg)

	fabricClient, err := client.NewFabricClient(client.Config{
		Endpoint:        endpoint,
		CredentialChain: chain,
		HTTPClient:      &http.Client{Timeout: requestTimeout},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Microsoft Fabric Client", err.Error())
		return
	}

	tenantID := credCfg.TenantID
	for _, envKey := range []string{"FABRIC_TENANT_ID", "AZURE_TENANT_ID", "ARM_TENANT_ID"} {
		if tenantID == "" {
			tenantID = strings.TrimSpace(os.Getenv(envKey))
		}
	}
	if tenantID == "" {
		tenantID = defaultFallbackTenantID
	}

	providerData := &Data{
		Client:   fabricClient,
		TenantID: tenantID,
	}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

// Resources returns the managed resource constructors registered with the provider.
func (p *FabricProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewWarehousePermissionResource,
		NewSQLDatabasePermissionResource,
		NewLakehousePermissionResource,
	}
}

// DataSources returns the data source constructors registered with the provider.
func (p *FabricProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewItemDataSource,
	}
}
