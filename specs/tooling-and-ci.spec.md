---
name: Tooling, Pre-Commit Hooks, and Hardened CI/CD
description: Consolidated mise tasks, prek hooks, Trivy/Zizmor scanning, Dependabot cooldown, and StepSecurity-hardened GitHub Actions release pipeline
targets:
  - ../mise.toml
  - ../.pre-commit-config.yaml
  - ../.cz.yaml
  - ../trivy.yaml
  - ../.golangci.yaml
  - ../.rumdl.toml
  - ../.yamllint.yaml
  - ../.goreleaser.yaml
  - ../terraform-registry-manifest.json
  - ../.github/dependabot.yaml
  - ../.github/workflows/ci.yaml
  - ../.github/workflows/release.yaml
---

# Tooling, Pre-Commit Hooks, and Hardened CI/CD

## Consolidated `mise.toml` Task Surface

```toml
[settings]
lockfile = true
minimum_release_age = "7d"
```

- `Taskfile.yaml` and `.taskfiles/` are retired; `mise.toml` is the single source of truth for all developer tools and tasks (`build`, `install-local`, `specs:check`, `specs:sync`, `test:unit`, `test:acc`, `test`, `docs`, `docs:check`, `scan`, `lint`, `format`, `bump`, `spec:verify`, `check`)
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`
- `mise.toml` enforces `lockfile = true` and `minimum_release_age = "7d"` to mitigate supply-chain compromises on newly published tool versions
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`

## Pre-Commit (`prek`) & Security Scanning

- `.pre-commit-config.yaml` and `.cz.yaml` configure `pre-commit` and `commit-msg` hooks including `commitizen` (Conventional Commits v1.0.0 with 72-character header limit), `yamllint`, `rumdl`, `zizmor`, `trivy` (`fs` and `config`), `terraform fmt`, `openapi-specs-check`, `spec-verify`, and `golangci-lint`
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`
- `trivy.yaml` scans filesystem dependencies, secrets, and Terraform HCL configurations (`modules/`, `examples/`) with exit code `1` on high/critical findings
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`

## Dependabot & StepSecurity-Hardened GitHub Actions (`ci.yaml` & `release.yaml`)

- `.github/dependabot.yaml` configures weekly updates with a 7-day `cooldown` (`default-days: 7`) and dependency grouping (`terraform-plugin` and `github-actions`) for both `github-actions` and `gomod`
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`
- `.github/workflows/ci.yaml` and `.github/workflows/release.yaml` use ` | ` pipe-delimited job and step names (`jambazid/gha-actions` style), start every job with `step-security/harden-runner`, use StepSecurity Maintained Actions (`step-security/mise-action`, `step-security/ghaction-import-gpg`, `step-security/goreleaser-action`) and official GitHub Actions (`actions/checkout`, `actions/attest-build-provenance`) pinned to 40-character commit SHAs, set `permissions: {}` at top level, and never use `pull_request_target` or inline `${{ ... }}` script interpolation
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`
- `.github/workflows/release.yaml` isolates unprivileged version calculation (`release | prepare` with `contents: read` and zero secrets) from privileged publication (`provider | release` inside the protected `release` environment, gated on `should_release == 'true'`), publishing GPG-signed Terraform Registry release artifacts via `step-security/goreleaser-action` (`release --clean` with changie release notes) and SLSA build provenance attestations
  `[@test] ../internal/provider/provider_test.go::TestToolingAndCI_Invariants`
