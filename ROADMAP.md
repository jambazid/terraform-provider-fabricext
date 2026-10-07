# Roadmap

> **Stopgap Mission Statement:** This provider (`registry.terraform.io/jambazid/fabricext`) exists strictly as an **interim stopgap** to unblock declarative Terraform management of Microsoft Fabric item-level sharing (`fabricext_*`) until Microsoft's official [`microsoft/fabric` Terraform provider](https://github.com/microsoft/terraform-provider-fabric) ships equivalent native resources.
>
> **Pre-Alpha Status (`v0.x`):** All resources in this repository are pre-alpha. Pin exact versions in `required_providers`.

---

## Current Scope

| Artifact | Type | Fabric Item Type | Underlying Fabric API Surface | Status |
| :--- | :--- | :--- | :--- | :--- |
| `fabricext_warehouse_permission` | Resource | `Warehouse` | Item Permissions (`GET /permissions`, `POST /grantPermissions`, `POST /revokePermissions`) | Implemented (`v0.1.x`) |
| `fabricext_sql_database_permission` | Resource | `SQLDatabase` | Item Permissions (`GET /permissions`, `POST /grantPermissions`, `POST /revokePermissions` supporting `Read`, `ReadData`, `ReadAll` + `SubscribeOneLakeEvents`, `Write`, `Reshare`) | Implemented (`v0.1.x`) |
| `fabricext_lakehouse_permission` | Resource | `Lakehouse` | OneLake Data Access Security (`GET` & `PUT /dataAccessRoles` with per-item mutex & `If-Match` ETag RMW) | Implemented (`v0.1.x`) |
| `fabricext_item` | Data Source | `Warehouse`, `Lakehouse`, `SQLDatabase`, `SQLEndpoint`, `SemanticModel` | Workspace Items (`GET /v1/workspaces/{workspaceId}/items?type={type}`) | Implemented (`v0.1.x`) |
| `modules/permissions` | HCL Module | `Warehouse`, `Lakehouse`, `SQLDatabase` | Flattens declarative workspace matrix into `for_each` resource bindings | Implemented (`v0.1.x`) |

---

## Candidate Future Resources

If upstream parity in `microsoft/fabric` has not yet landed for the following item types, contributors may add them by following the playbook in [`CONTRIBUTING.md`](CONTRIBUTING.md):

1. **`fabricext_semantic_model_permission`**:
   - Declarative item-level sharing (`Read`, `Build`, `Write`, `Reshare`) for Fabric / Power BI Semantic Models (`SemanticModel`).
2. **`fabricext_eventhouse_permission` & `fabricext_kql_database_permission`**:
   - Declarative item-level sharing for Real-Time Intelligence Eventhouses and KQL Databases.
3. **`fabricext_mirrored_database_permission`**:
   - Declarative item-level sharing and OneLake read permissions for Mirrored Databases (`MirroredDatabase`).
4. **Preview Single-Role OneLake Data Access Endpoints**:
   - `microsoft/fabric-rest-api-specs/platform/swagger.json` includes preview single-role endpoints (`CreateOrUpdateSingleDataAccessRole`, `GetDataAccessRole`, `DeleteDataAccessRole`). Once promoted from `preview=true` to GA by Microsoft, `FabricClient` can cut over from full-document `PUT` RMW to single-role CRUD without breaking the `fabricext_lakehouse_permission` HCL schema.

---

## Deprecation and Migration Policy

Each `fabricext_*` resource is tracked against upstream support in [`microsoft/terraform-provider-fabric`](https://github.com/microsoft/terraform-provider-fabric):

1. **Trigger**: When `microsoft/fabric` releases a GA resource covering the same Fabric API surface (for example, an official item-permission or OneLake data-access-role resource).
2. **Deprecation Notice**: The corresponding `fabricext_*` resource will mark `DeprecationMessage` in its Terraform Plugin Framework schema (surfacing a non-breaking warning during `terraform plan`) and document a `removed` + `import` cross-provider migration guide (Terraform 1.7+) in `docs/`.
3. **Archive**: Once all `fabricext_*` resources in this repository have GA equivalents in `microsoft/fabric`, a final release with migration instructions will be published and the repository will be archived.
4. **Comparative Analysis & State Migration Playbook**: See the published Terraform Registry guide [`docs/guides/official_provider_comparison.md`](docs/guides/official_provider_comparison.md) (generated from [`templates/guides/official_provider_comparison.md.tmpl`](templates/guides/official_provider_comparison.md.tmpl)) for the architectural comparison with `microsoft/terraform-provider-fabric` and Terraform 1.7+ `removed` + `import` migration playbooks covering Lakehouses, Warehouses, and SQL Databases.
