
Review [docs/fabric-itemshare-provider.docx](./fabric-itemshare-provider.docx) and create an initial plan for the delivery. Then spin up a small team of subagents for adversarial review before publishing the plan. `mise` is already installed on this system so all tooling should be defined via `mise.toml`.

The scope of the plan should include the creation of GHA workflows for publishing to the Terraform registry and suitable unit test cases. The plan should also account for how to document the provider.

The provider page should show an initial notice warning that this is pre-alpha and not stable. We should also have dependabot appropriately configured with a `7-day` lag window.

Part of the plan should be writing a fresh `DESIGN.md` document with the full refined spec and architecture etc.

You should also port the existing `Taskfile` tasks to `mise.toml` then remove the Taskfile surface.