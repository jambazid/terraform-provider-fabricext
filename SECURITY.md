# Security Policy

## Reporting a Vulnerability

Please **do not** report security vulnerabilities through public GitHub issues, discussions, or pull requests.

Instead, use **[GitHub Private Vulnerability Reporting](https://github.com/jambazid/terraform-provider-fabricext/security/advisories/new)** on this repository to submit a confidential advisory. Include:

- Affected component (`internal/credentials`, `internal/client`, `internal/provider`, `modules/permissions`, or `.github/workflows/ci.yaml`)
- Reproduction steps or proof-of-concept configuration
- Potential impact (e.g., credential exposure, privilege escalation, or supply-chain risk)

We aim to acknowledge reports within 48 hours and publish a patched release once verified.

---

## Authentication and Secrets

1. **Native Microsoft Identity SDK (`azidentity`) — Shell Injection Protection**:
   - Credential resolution uses Microsoft's official Go SDK (`github.com/Azure/azure-sdk-for-go/sdk/azidentity` and `sdk/azcore`) exclusively. Service Principal (client secret, client certificate), Workload Identity (OIDC), and Managed Identity flows execute purely in-process over HTTPS with zero subprocesses. Local development CLI credentials (`azidentity.NewAzureCLICredential` and `NewAzureDeveloperCLICredential`) invoke official `az`/`azd` binaries with structured argument vectors rather than shell interpolation, and no temporary token files are written to disk.
2. **Redaction in Logs, Errors & Formatting**:
   - Schema attributes `access_token` and `client_secret` are marked `Sensitive: true` in the Terraform Plugin Framework schema so Terraform masks them in CLI output.
   - `credentials.Credentials` implements `String()` and `GoString()` to unconditionally mask `AccessToken` as `"[REDACTED]"`. Raw bearer tokens and client secrets are never logged via `tflog` or included in `resp.Diagnostics`.
3. **Workload Identity / OIDC First**:
   - For CI/CD pipelines (such as GitHub Actions or Azure DevOps), we strongly recommend passwordless OpenID Connect (`WorkloadIdentityCredential` / federated credentials) over static `FABRIC_ACCESS_TOKEN` or `AZURE_CLIENT_SECRET` values.

---

## Supply Chain and CI/CD Security

This repository enforces defense-in-depth against CI/CD and dependency supply-chain attacks (such as `s1ngularity`, `hackerbot-claw`, and `TeamPCP`):

1. **Runtime Egress Monitoring (`step-security/harden-runner`)**:
   - Every job in `.github/workflows/ci.yaml` runs `step-security/harden-runner` as its first step to monitor outbound network connections and detect anomalous egress.
2. **StepSecurity Maintained Actions & 40-Character SHA Pinning**:
   - Third-party community actions are replaced with [StepSecurity Maintained Actions](https://www.stepsecurity.io/blog/stepsecurity-maintained-actions-are-now-free-for-public-repos) (`step-security/mise-action`, `step-security/ghaction-import-gpg`) alongside official `actions/checkout`.
   - Every `uses:` reference is pinned to a full 40-character immutable commit SHA with a trailing version comment (`# vX.Y.Z`), verified on every commit by `zizmor`.
3. **7-Day Supply-Chain Release Cooldown**:
   - `mise.toml` sets `lockfile = true` and `minimum_release_age = "7d"`.
   - `.github/dependabot.yaml` sets `cooldown: { default-days: 7 }` for both `github-actions` and `gomod`.
   - `mise run bump` runs `actions-up --min-age 7`.
4. **Least-Privilege Workflows & Protected Release Environment**:
   - `.github/workflows/ci.yaml` sets top-level `permissions: {}` and `persist-credentials: false` on every `actions/checkout` step.
   - `pull_request_target` is strictly prohibited.
   - Untrusted `${{ ... }}` expressions are never interpolated inside `run:` shell blocks.
   - GPG release signing keys (`GPG_PRIVATE_KEY`, `PASSPHRASE`) are isolated inside the protected `release` GitHub Environment and accessed only on `push` to `refs/heads/main`.
5. **Continuous Static Analysis & Vulnerability Scanning**:
   - `trivy fs` (dependencies, secrets, misconfigurations), `trivy config` (Terraform HCL), `zizmor` (GitHub Actions), and `golangci-lint` run both locally via `prek` pre-commit hooks and in GitHub Actions CI.
