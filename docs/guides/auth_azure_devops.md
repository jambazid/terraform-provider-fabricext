---
page_title: "Authenticating with Azure DevOps Workload Identity Federation"
subcategory: "Authentication Guides"
description: |-
  How to configure the fabricext provider using Azure DevOps Pipeline Workload Identity Federation with a service connection.
---

# Azure DevOps OIDC Authentication

This guide explains how to authenticate the `fabricext` provider using Azure DevOps Workload Identity Federation with Azure Pipelines.

## Azure Pipelines YAML Example

In your Azure Pipelines pipeline, make sure `$(System.AccessToken)` is accessible to the task:

```yaml
steps:
  - task: AzureCLI@2
    inputs:
      azureSubscription: 'my-fabric-service-connection'
      scriptType: 'bash'
      scriptLocation: 'inlineScript'
      inlineScript: |
        terraform apply
    env:
      SYSTEM_ACCESSTOKEN: $(System.AccessToken)
```

## Provider Configuration

```terraform
# Authenticate using Azure DevOps Workload Identity Federation (Service Connection OIDC).
provider "fabricext" {
  client_id                          = "00000000-0000-0000-0000-000000000001"
  tenant_id                          = "00000000-0000-0000-0000-000000000000"
  azure_devops_service_connection_id = "00000000-0000-0000-0000-000000000002"
}
```

## Tenant Permissions & Scopes

To manage item-level permissions (`fabricext_*_permission`) and OneLake Data Access Roles (`fabricext_lakehouse_data_access_role`), the Service Principal behind the Azure DevOps service connection requires:

1. **Workspace Access**: **Admin** or **Member** role on the target workspace.
2. **Tenant Settings**: The tenant setting *"Service principals can use Fabric APIs"* enabled in Fabric Admin Portal under Developer settings.

If using `fabricext_item_share_gate` or `modules/permissions` with `verify_item_shares = true`, the provider queries the Microsoft Fabric Admin API (`GET /v1/admin/workspaces/{workspaceId}/items/{itemId}/users`). This additionally requires:

1. Fabric tenant setting *"Service principals can use Fabric admin APIs"* (or *"read-only admin APIs"*).
2. Microsoft Entra ID Application permission `Tenant.Read.All` or `Tenant.ReadWrite.All`.

~> **Note:** If `verify_item_shares = false`, Admin APIs are bypassed completely, avoiding any requirement for tenant admin roles.

## Regional Cluster Discovery

When authenticating via an Azure DevOps service connection (`app-only` token posture), the provider automatically queries the Power BI REST gateway (`GET /v1.0/myorg/groups`) to discover regional backend cluster URIs via the `home-cluster-uri` response header. For interactive user identities (`delegated-user` posture), the provider uses the portal bootstrap endpoint.
