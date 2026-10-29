# Power BI Metadata Engineering Protocol

This document provides a sanitized engineering reference for the Power BI service `/metadata/*` endpoints that back declarative item-level permissions for Microsoft Fabric Lakehouses, Warehouses, and SQL Databases in `terraform-provider-fabricext`.

---

## 1. Authentication & Token Acquisition

Power BI service metadata calls require Microsoft Entra ID bearer tokens scoped to the Power BI resource (`https://analysis.windows.net/powerbi/api/.default`, App ID `00000009-0000-0000-c000-000000000000`):

### Azure CLI (Delegated User)

```bash
ACCESS_TOKEN=$(az account get-access-token \
  --resource https://analysis.windows.net/powerbi/api \
  --query accessToken -o tsv)
```

### Service Principal (Client Credentials)

```bash
TENANT_ID="00000000-0000-0000-0000-000000000001"
CLIENT_ID="55555555-5555-5555-5555-555555555555"
CLIENT_SECRET="<secret>"

ACCESS_TOKEN=$(curl -s -X POST \
  -d "grant_type=client_credentials&client_id=${CLIENT_ID}&client_secret=${CLIENT_SECRET}&scope=https://analysis.windows.net/powerbi/api/.default" \
  "https://login.microsoftonline.com/${TENANT_ID}/oauth2/v2.0/token" | jq -r .access_token)
```

---

## 2. Cluster URI Discovery

Power BI web frontends route requests through regional backend clusters. The regional cluster host is discovered by querying the bootstrap portal with manual redirect following:

```bash
CLUSTER_URL=$(curl -s -i \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  "https://app.powerbi.com/" | \
  grep -i "Set-Cookie:.*ClusterUri=" | \
  sed -n 's/.*ClusterUri=[^=]*=\(https:\/\/[^;]*\).*/\1/p' | \
  tr -d '\r\n')

# Example resolved cluster URL:
# https://wabi-region-redirect.analysis.windows.net
```

---

## 3. Workload Artifact Definitions

Retrieve workload-specific bitmask definitions from the regional cluster:

```bash
curl -s -X GET \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  "${CLUSTER_URL}/metadata/artifacts/definitions" | jq .
```

### Bitmask Definitions Table

#### Core Permissions Bitmask (`permissions`)

| Flag | Decimal Bit | Description |
| :--- | :--- | :--- |
| `None` | `0` | No access (used in revocation) |
| `Read` | `1` | Core Read permission (mandatory for all item shares) |
| `Write` | `2` | Write permission (supported on SQL Database, not Warehouse) |
| `ReShare` | `4` | Core Reshare permission |
| `Explore` | `8` | Explore data |
| `CopyOnWrite` | `16` | Copy on write |
| `WriteCheckRelaxForReadOnlyGroup` | `32` | Group relaxed write check |
| `Execute` | `64` | Execute permission (SQL Database TDS connection / Lakehouse) |
| `Subscribe` | `128` | Subscribe to item events |

#### Workload Artifact Permissions (`artifactPermissions`)

| Workload Key | Fabric Item Type | Valid Permission Names & Bit Values |
| :--- | :--- | :--- |
| `Datawarehouse` | `Warehouse` | `Read: 1`, `ReadData: 2`, `ReadAll: 4`, `Reshare: 8`, `Monitor: 16`, `Audit: 64`, `Restore: 128` |
| `Lakehouse` | `Lakehouse` | `Read: 1`, `ReadData: 2`, `ReadAll: 4`, `Execute: 64`, `Reshare: 8` |
| `SQLDbNative` | `SQLDatabase` | `Read: 1`, `ReadData: 2`, `ReadAll: 4`, `Execute: 64`, `Reshare: 8` |

---

## 4. Item Permission Queries

Inspect current effective and direct permissions on an item:

```bash
ITEM_ID="22222222-2222-2222-2222-222222222222"

curl -s -X GET \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  "${CLUSTER_URL}/metadata/access/artifacts/${ITEM_ID}" | jq .
```

### Direct vs Inherited Distinction

- `accessSource == null`: Direct item-level share (managed by Terraform).
- `accessSource != null`: Inherited workspace-level role assignment (Admin, Member, Contributor, Viewer).

---

## 5. Item Permission Mutations

### Silent Direct Grant (`POST /metadata/access`)

Grant `Read` (1) + `ReadData` (2) to an Entra User:

```bash
curl -s -X POST \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "dashboards": [],
    "reports": [],
    "workbooks": [],
    "models": [],
    "datamarts": [],
    "artifacts": [
      {
        "id": 1,
        "artifactObjectId": "22222222-2222-2222-2222-222222222222",
        "permissions": 1,
        "artifactPermissions": 3,
        "userObjectId": "33333333-3333-3333-3333-333333333333",
        "isServicePrincipal": false
      }
    ]
  }' \
  "${CLUSTER_URL}/metadata/access"
```

### In-Place Update & Drift Remediation (`PUT /metadata/access`)

Update exact bitmask in-place (for example, stripping unwanted privileges):

```bash
curl -s -X PUT \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "dashboards": [],
    "reports": [],
    "workbooks": [],
    "models": [],
    "datamarts": [],
    "artifacts": [
      {
        "id": 1,
        "artifactObjectId": "22222222-2222-2222-2222-222222222222",
        "permissions": 1,
        "artifactPermissions": 1,
        "userId": 1234567
      }
    ]
  }' \
  "${CLUSTER_URL}/metadata/access"
```

### Permission Revocation (`PUT /metadata/access` with 0/0)

Setting both `permissions: 0` and `artifactPermissions: 0` revokes direct item access:

```bash
curl -s -X PUT \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "dashboards": [],
    "reports": [],
    "workbooks": [],
    "models": [],
    "datamarts": [],
    "artifacts": [
      {
        "id": 1,
        "artifactObjectId": "22222222-2222-2222-2222-222222222222",
        "permissions": 0,
        "artifactPermissions": 0,
        "userId": 1234567
      }
    ]
  }' \
  "${CLUSTER_URL}/metadata/access"
```

---

## 6. Maintenance & Bundle Inspection

If Microsoft updates the web client or enum bitmasks in future builds:

1. Open Developer Tools (F12) in the Fabric Portal on the Share dialog.
2. Locate the active Web Frontend chunk from `app.powerbi.com` (for example, chunk scripts matching `TridentArtifactPermissions` or `metadata/access`).
3. Search for `/metadata/access` and inspect the payload construction logic and bitmask lookup constants.
