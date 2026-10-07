// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package provider implements the Terraform Plugin Framework provider for fabricext.
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

	uuidRegex       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	roleNameRegex   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	sourcePathRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func uuidValidator() validator.String {
	return stringvalidator.RegexMatches(uuidRegex, "must be a valid UUID (e.g. 00000000-0000-0000-0000-000000000000)")
}

func sourcePathValidator() validator.String {
	return stringvalidator.RegexMatches(sourcePathRegex, "must be in the format {workspace_id}/{item_id} where both are valid UUIDs")
}

func normalizePrincipalType(itemType, pt string) (string, error) {
	norm := ""
	switch strings.ToLower(strings.TrimSpace(pt)) {
	case "user":
		norm = "User"
	case "group":
		norm = "Group"
	case "serviceprincipal":
		norm = "ServicePrincipal"
	case "managedidentity":
		norm = "ManagedIdentity"
	case "serviceprincipalprofile":
		norm = "ServicePrincipalProfile"
	default:
		return "", fmt.Errorf("invalid principal_type %q: must be one of User, Group, ServicePrincipal, ManagedIdentity, ServicePrincipalProfile", pt)
	}

	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "warehouse", "sqldatabase":
		if norm == "ManagedIdentity" {
			return "", fmt.Errorf("invalid principal_type %q for %s: must be one of User, Group, ServicePrincipal, ServicePrincipalProfile", pt, itemType)
		}
	case "lakehouse":
		if norm == "ServicePrincipalProfile" {
			return "", fmt.Errorf("invalid principal_type %q for %s: must be one of User, Group, ServicePrincipal, ManagedIdentity", pt, itemType)
		}
	}
	return norm, nil
}

// Data holds the configured Fabric API client and tenant context shared with resources and data sources.
type Data struct {
	Client   *client.FabricClient
	TenantID string
}

// Environment represents the target Microsoft Fabric cloud environment.
type Environment string

// Supported target Microsoft Fabric cloud environments.
const (
	// EnvironmentPublic represents the global Azure public cloud.
	EnvironmentPublic Environment = "public"
	// EnvironmentUSGovernment represents Azure Government cloud (Fairfax).
	EnvironmentUSGovernment Environment = "usgovernment"
	// EnvironmentChina represents Azure China cloud (Mooncake).
	EnvironmentChina Environment = "china"
)

// FabricProvider defines the Microsoft Fabric item-sharing Terraform provider.
type FabricProvider struct {
	version string
}

// FabricProviderModel describes the HCL configuration schema for `provider "fabricext"`.
type FabricProviderModel struct {
	Endpoint                       types.String `tfsdk:"endpoint"`
	AccessToken                    types.String `tfsdk:"access_token"`
	TenantID                       types.String `tfsdk:"tenant_id"`
	TenantIDFilePath               types.String `tfsdk:"tenant_id_file_path"`
	ClientID                       types.String `tfsdk:"client_id"`
	ClientIDFilePath               types.String `tfsdk:"client_id_file_path"`
	ClientSecret                   types.String `tfsdk:"client_secret"`
	ClientSecretFilePath           types.String `tfsdk:"client_secret_file_path"`
	ClientCertificate              types.String `tfsdk:"client_certificate"`
	ClientCertificateFilePath      types.String `tfsdk:"client_certificate_file_path"`
	ClientCertificatePassword      types.String `tfsdk:"client_certificate_password"`
	UseMSI                         types.Bool   `tfsdk:"use_msi"`
	UseOIDC                        types.Bool   `tfsdk:"use_oidc"`
	OIDCToken                      types.String `tfsdk:"oidc_token"`
	OIDCTokenFilePath              types.String `tfsdk:"oidc_token_file_path"`
	OIDCRequestToken               types.String `tfsdk:"oidc_request_token"`
	OIDCRequestURL                 types.String `tfsdk:"oidc_request_url"`
	AzureDevOpsServiceConnectionID types.String `tfsdk:"azure_devops_service_connection_id"`
	UseCLI                         types.Bool   `tfsdk:"use_cli"`
	UseDevCLI                      types.Bool   `tfsdk:"use_dev_cli"`
	Environment                    types.String `tfsdk:"environment"`
	AuxiliaryTenantIDs             types.List   `tfsdk:"auxiliary_tenant_ids"`
	RequestTimeout                 types.String `tfsdk:"request_timeout"`
	SkipCredentialsValidation      types.Bool   `tfsdk:"skip_credentials_validation"`
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

// Schema defines the provider-level configuration schema matching official Fabric provider parity.
func (p *FabricProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The **Fabric Extensions (fabricext)** provider (`registry.terraform.io/jambazid/fabricext`) is a purpose-built provider for declaratively managing Microsoft Fabric item-level sharing and data-access permissions (`fabricext_*`) across Warehouses, SQL Databases, and Lakehouses until equivalent resources are available in Microsoft's official `microsoft/fabric` provider.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL for the Microsoft Fabric REST API. Defaults to `https://api.fabric.microsoft.com` (or sovereign cloud equivalent). Can also be sourced from `FABRIC_ENDPOINT`.",
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
			"tenant_id_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to a file containing the Entra ID tenant UUID.",
			},
			"client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Microsoft Entra ID application (client) UUID. Can also be sourced from `FABRIC_CLIENT_ID`, `AZURE_CLIENT_ID`, or `ARM_CLIENT_ID`.",
			},
			"client_id_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to a file containing the Entra ID application (client) UUID.",
			},
			"client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Microsoft Entra ID Service Principal client secret. Can also be sourced from `FABRIC_CLIENT_SECRET`, `AZURE_CLIENT_SECRET`, or `ARM_CLIENT_SECRET`.",
			},
			"client_secret_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to a file containing the Service Principal client secret.",
			},
			"client_certificate": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded PKCS#12 certificate bundle (.pfx / .p12). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE`, `AZURE_CLIENT_CERTIFICATE`, or `ARM_CLIENT_CERTIFICATE`.",
			},
			"client_certificate_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to a PKCS#12 certificate file (.pfx / .p12). Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_PATH`, `AZURE_CLIENT_CERTIFICATE_PATH`, or `ARM_CLIENT_CERTIFICATE_PATH`.",
			},
			"client_certificate_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password associated with the PKCS#12 client certificate. Can also be sourced from `FABRIC_CLIENT_CERTIFICATE_PASSWORD`, `AZURE_CLIENT_CERTIFICATE_PASSWORD`, or `ARM_CLIENT_CERTIFICATE_PASSWORD`.",
			},
			"use_msi": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable Azure Managed Identity authentication. Can also be sourced from `FABRIC_USE_MSI`, `AZURE_USE_MSI`, or `ARM_USE_MSI`.",
			},
			"use_oidc": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable Microsoft Entra Workload Identity (OIDC) authentication. Can also be sourced from `FABRIC_USE_OIDC`, `AZURE_USE_OIDC`, or `ARM_USE_OIDC`.",
			},
			"oidc_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN` or `ARM_OIDC_TOKEN`.",
			},
			"oidc_token_file_path": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to a file containing the OIDC JWT assertion token. Can also be sourced from `FABRIC_OIDC_TOKEN_FILE_PATH` or `ARM_OIDC_TOKEN_FILE_PATH`.",
			},
			"oidc_request_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Bearer token for requesting an ID token from an OIDC provider. Can also be sourced from `SYSTEM_ACCESSTOKEN`, `FABRIC_OIDC_REQUEST_TOKEN`, or `ARM_OIDC_REQUEST_TOKEN`.",
			},
			"oidc_request_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URL for requesting an ID token from an OIDC provider. Can also be sourced from `SYSTEM_OIDCREQUESTURI`, `FABRIC_OIDC_REQUEST_URL`, or `ARM_OIDC_REQUEST_URL`.",
			},
			"azure_devops_service_connection_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Azure DevOps Service Connection ID that uses Workload Identity Federation. Can also be sourced from `FABRIC_AZURE_DEVOPS_SERVICE_CONNECTION_ID`.",
			},
			"use_cli": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow fallback to the local Azure CLI (`az login`) session. Defaults to `true`. Can also be sourced from `FABRIC_USE_CLI`, `AZURE_USE_CLI`, or `ARM_USE_CLI`.",
			},
			"use_dev_cli": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow fallback to the Azure Developer CLI (`azd auth login`) session. Can also be sourced from `FABRIC_USE_DEV_CLI`.",
			},
			"environment": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cloud environment to target (`public`, `usgovernment`, `china`). Defaults to `public`. Can also be sourced from `FABRIC_ENVIRONMENT` or `ARM_ENVIRONMENT`.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(EnvironmentPublic), string(EnvironmentUSGovernment), string(EnvironmentChina)),
				},
			},
			"auxiliary_tenant_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Auxiliary Tenant IDs for multi-tenant token acquisition. Can also be sourced from comma-separated `FABRIC_AUXILIARY_TENANT_IDS`.",
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

	// Validate unknown values
	if config.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"), "Unknown Fabric API Endpoint", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for endpoint.")
	}
	if config.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("access_token"), "Unknown Fabric Access Token", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for access_token.")
	}
	if config.TenantID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("tenant_id"), "Unknown Tenant ID", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for tenant_id.")
	}
	if config.TenantIDFilePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("tenant_id_file_path"), "Unknown Tenant ID File Path", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for tenant_id_file_path.")
	}
	if config.ClientID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_id"), "Unknown Client ID", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_id.")
	}
	if config.ClientIDFilePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_id_file_path"), "Unknown Client ID File Path", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_id_file_path.")
	}
	if config.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "Unknown Client Secret", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_secret.")
	}
	if config.ClientSecretFilePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret_file_path"), "Unknown Client Secret File Path", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_secret_file_path.")
	}
	if config.ClientCertificate.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_certificate"), "Unknown Client Certificate", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_certificate.")
	}
	if config.ClientCertificateFilePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_certificate_file_path"), "Unknown Client Certificate File Path", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_certificate_file_path.")
	}
	if config.ClientCertificatePassword.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("client_certificate_password"), "Unknown Client Certificate Password", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for client_certificate_password.")
	}
	if config.UseMSI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_msi"), "Unknown UseMSI Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_msi.")
	}
	if config.UseOIDC.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_oidc"), "Unknown UseOIDC Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_oidc.")
	}
	if config.OIDCToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("oidc_token"), "Unknown OIDC Token", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for oidc_token.")
	}
	if config.OIDCTokenFilePath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("oidc_token_file_path"), "Unknown OIDC Token File Path", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for oidc_token_file_path.")
	}
	if config.OIDCRequestToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("oidc_request_token"), "Unknown OIDC Request Token", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for oidc_request_token.")
	}
	if config.OIDCRequestURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("oidc_request_url"), "Unknown OIDC Request URL", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for oidc_request_url.")
	}
	if config.AzureDevOpsServiceConnectionID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("azure_devops_service_connection_id"), "Unknown Azure DevOps Service Connection ID", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for azure_devops_service_connection_id.")
	}
	if config.UseCLI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_cli"), "Unknown UseCLI Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_cli.")
	}
	if config.UseDevCLI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("use_dev_cli"), "Unknown UseDevCLI Flag", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for use_dev_cli.")
	}
	if config.Environment.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("environment"), "Unknown Environment", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for environment.")
	}
	if config.AuxiliaryTenantIDs.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("auxiliary_tenant_ids"), "Unknown Auxiliary Tenant IDs", "The provider cannot create the Microsoft Fabric API client as there is an unknown configuration value for auxiliary_tenant_ids.")
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

	envStr := strings.TrimSpace(os.Getenv("FABRIC_ENVIRONMENT"))
	if envStr == "" {
		envStr = strings.TrimSpace(os.Getenv("ARM_ENVIRONMENT"))
	}
	if !config.Environment.IsNull() && strings.TrimSpace(config.Environment.ValueString()) != "" {
		envStr = strings.TrimSpace(config.Environment.ValueString())
	}
	if envStr == "" {
		envStr = "public"
	}

	var defaultEndpoint string
	switch strings.ToLower(envStr) {
	case "public":
		defaultEndpoint = "https://api.fabric.microsoft.com"
	case "usgovernment":
		defaultEndpoint = "https://api.fabric.microsoft.us"
	case "china":
		defaultEndpoint = "https://api.fabric.microsoft.cn"
	default:
		resp.Diagnostics.AddAttributeError(path.Root("environment"), "Invalid Environment", fmt.Sprintf("environment %q must be one of public, usgovernment, china", envStr))
		return
	}

	endpoint := strings.TrimSpace(os.Getenv("FABRIC_ENDPOINT"))
	if !config.Endpoint.IsNull() && strings.TrimSpace(config.Endpoint.ValueString()) != "" {
		endpoint = strings.TrimSpace(config.Endpoint.ValueString())
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
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
	var useDevCLIPtr *bool
	if !config.UseDevCLI.IsNull() {
		v := config.UseDevCLI.ValueBool()
		useDevCLIPtr = &v
	}

	var auxTenants []string
	if !config.AuxiliaryTenantIDs.IsNull() {
		diag := config.AuxiliaryTenantIDs.ElementsAs(ctx, &auxTenants, false)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	credCfg := credentials.Config{
		AccessToken:                    strings.TrimSpace(config.AccessToken.ValueString()),
		TenantID:                       strings.TrimSpace(config.TenantID.ValueString()),
		TenantIDFilePath:               strings.TrimSpace(config.TenantIDFilePath.ValueString()),
		ClientID:                       strings.TrimSpace(config.ClientID.ValueString()),
		ClientIDFilePath:               strings.TrimSpace(config.ClientIDFilePath.ValueString()),
		ClientSecret:                   strings.TrimSpace(config.ClientSecret.ValueString()),
		ClientSecretFilePath:           strings.TrimSpace(config.ClientSecretFilePath.ValueString()),
		ClientCertificate:              strings.TrimSpace(config.ClientCertificate.ValueString()),
		ClientCertificateFilePath:      strings.TrimSpace(config.ClientCertificateFilePath.ValueString()),
		ClientCertificatePassword:      strings.TrimSpace(config.ClientCertificatePassword.ValueString()),
		UseMSI:                         config.UseMSI.ValueBool(),
		UseOIDC:                        useOIDCPtr,
		OIDCToken:                      strings.TrimSpace(config.OIDCToken.ValueString()),
		OIDCTokenFilePath:              strings.TrimSpace(config.OIDCTokenFilePath.ValueString()),
		OIDCRequestToken:               strings.TrimSpace(config.OIDCRequestToken.ValueString()),
		OIDCRequestURL:                 strings.TrimSpace(config.OIDCRequestURL.ValueString()),
		AzureDevOpsServiceConnectionID: strings.TrimSpace(config.AzureDevOpsServiceConnectionID.ValueString()),
		UseCLI:                         useCLIPtr,
		UseDevCLI:                      useDevCLIPtr,
		Environment:                    envStr,
		AuxiliaryTenantIDs:             auxTenants,
	}

	if err := credentials.ValidateConfig(credCfg); err != nil {
		resp.Diagnostics.AddError("Invalid Provider Credential Configuration", err.Error())
		return
	}

	chain := credentials.NewChain(credCfg)

	skipValidation := config.SkipCredentialsValidation.ValueBool() ||
		strings.EqualFold(os.Getenv("FABRIC_SKIP_CREDENTIALS_VALIDATION"), "true")
	if !skipValidation {
		if _, err := chain.Resolve(ctx); err != nil {
			resp.Diagnostics.AddError("Eager Credential Validation Failed", err.Error())
			return
		}
	}

	fabricClient, err := client.NewFabricClient(client.Config{
		Endpoint:        endpoint,
		CredentialChain: chain,
		HTTPClient:      &http.Client{Timeout: requestTimeout},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Microsoft Fabric Client", err.Error())
		return
	}

	tenantID, err := credCfg.ResolveTenantID()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Tenant ID", err.Error())
		return
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

// reconcileItemIdentifiersPlan adjusts item name and item ID plan values when resources
// are replaced. If an identifier changes, the unconfigured counterpart is set to unknown
// so stale values preserved by UseStateForUnknown do not cause replacement failures.
func reconcileItemIdentifiersPlan(
	nameConfig, idConfig types.String,
	nameState, idState types.String,
	namePlan, idPlan *types.String,
) {
	// If name was configured and changed relative to state, clear unconfigured ID
	if !nameConfig.IsNull() && !nameConfig.IsUnknown() {
		if idConfig.IsNull() && !nameState.IsNull() && nameConfig.ValueString() != nameState.ValueString() {
			*idPlan = types.StringUnknown()
		}
	}

	// If ID was configured and changed relative to state, clear unconfigured Name
	if !idConfig.IsNull() && !idConfig.IsUnknown() {
		if nameConfig.IsNull() && !idState.IsNull() && idConfig.ValueString() != idState.ValueString() {
			*namePlan = types.StringUnknown()
		}
	}
}
