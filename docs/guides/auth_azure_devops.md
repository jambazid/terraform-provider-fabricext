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
