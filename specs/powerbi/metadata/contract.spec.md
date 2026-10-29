---
name: powerbi-metadata-contract
description: Behavioral contract specification for Power BI metadata /metadata/* service endpoints and mock server schema validation.
targets:
  - openapi.json
  - README.md
  - ../../../internal/testutil/pbimock/server.go
  - ../../../internal/testutil/pbimock/server_test.go
---

# Power BI Metadata Contract Specification

## Schema Validation & Endpoints

- The Power BI metadata specification conforms to OpenAPI 3.0.3 and defines all core paths: `/metadata/access`, `/metadata/access/notify`, `/metadata/access/artifacts/{itemId}`, and `/metadata/artifacts/definitions`
  `[@test] ../../../internal/testutil/pbimock/server_test.go::TestPBIMockServer_ContractValidation`
- `pbimock.Server` enforces runtime request and response schema validation using `kin-openapi` against `specs/powerbi/metadata/openapi.json`
  `[@test] ../../../internal/testutil/pbimock/server_test.go::TestPBIMockServer_ContractValidation`
- Direct grants, bitmask updates, and revocations validate request payloads with proper principal XOR constraints and response schemas
  `[@test] ../../../internal/testutil/pbimock/server_test.go::TestPBIMockServer_ContractValidation`
