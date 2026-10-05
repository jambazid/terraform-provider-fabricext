// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/jambazid/terraform-provider-fabricext/internal/testutil/fabricmock"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"fabricext": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccProviderConfig(srv *fabricmock.Server) string {
	return fmt.Sprintf(`
provider "fabricext" {
  endpoint     = %q
  access_token = "mock-static-access-token"
  tenant_id    = "77777777-7777-7777-7777-777777777777"
}
`, srv.URL())
}

func TestProvider_MetadataAndSchema(t *testing.T) {
	t.Parallel()

	p := New("0.1.0")()
	var metaResp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &metaResp)
	if metaResp.TypeName != "fabricext" {
		t.Fatalf("expected provider TypeName %q, got %q", "fabricext", metaResp.TypeName)
	}
	if metaResp.Version != "0.1.0" {
		t.Fatalf("expected provider Version %q, got %q", "0.1.0", metaResp.Version)
	}

	var schemaResp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", schemaResp.Diagnostics)
	}
	for _, attrName := range []string{"endpoint", "access_token", "tenant_id", "client_id", "client_secret", "use_msi", "use_oidc", "use_cli"} {
		if _, ok := schemaResp.Schema.Attributes[attrName]; !ok {
			t.Fatalf("expected provider schema attribute %q to exist", attrName)
		}
	}
}

func TestAccProvider_PartialClientSecretError(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "fabricext" {
  endpoint      = %q
  client_secret = "partial-secret-without-client-id"
}

data "fabricext_item" "test" {
  workspace_id = %q
  display_name = "any_wh"
  type         = "Warehouse"
}
`, srv.URL(), wsID),
				ExpectError: regexp.MustCompile(`partial service principal configuration`),
			},
		},
	})
}

func TestAccPermissionsModule_MatrixFlattening(t *testing.T) {
	t.Parallel()

	srv := fabricmock.NewServer(t)
	wsID := "11111111-1111-1111-1111-111111111111"
	whID := "22222222-2222-2222-2222-222222222222"
	dbID := "33333333-3333-3333-3333-333333333333"
	lhID := "44444444-4444-4444-4444-444444444444"

	srv.UpsertItem(fabricmock.Item{
		ID:          whID,
		WorkspaceID: wsID,
		DisplayName: "sales_analytics_wh",
		Type:        "Warehouse",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          dbID,
		WorkspaceID: wsID,
		DisplayName: "operational_orders_db",
		Type:        "SQLDatabase",
	})
	srv.UpsertItem(fabricmock.Item{
		ID:          lhID,
		WorkspaceID: wsID,
		DisplayName: "raw_bronze_lh",
		Type:        "Lakehouse",
	})

	modulePath, err := filepath.Abs(filepath.Join("..", "..", "modules", "permissions"))
	if err != nil {
		t.Fatalf("resolve modules/permissions path: %v", err)
	}

	varsBytes, err := os.ReadFile(filepath.Join(modulePath, "variables.tf"))
	if err != nil {
		t.Fatalf("read variables.tf: %v", err)
	}
	mainBytes, err := os.ReadFile(filepath.Join(modulePath, "main.tf"))
	if err != nil {
		t.Fatalf("read main.tf: %v", err)
	}
	outputsBytes, err := os.ReadFile(filepath.Join(modulePath, "outputs.tf"))
	if err != nil {
		t.Fatalf("read outputs.tf: %v", err)
	}

	varsWithDefault := strings.TrimSuffix(strings.TrimSpace(string(varsBytes)), "}") + fmt.Sprintf(`
  default = {
    workspace_id = %q
    warehouses = {
      sales_analytics_wh = {
        read  = [{ id = "55555555-5555-5555-5555-555555555551", type = "Group" }]
        write = [{ id = "55555555-5555-5555-5555-555555555552", type = "ServicePrincipal" }]
      }
    }
    sql_databases = {
      operational_orders_db = {
        read_data = [{ id = "55555555-5555-5555-5555-555555555551", type = "Group" }]
      }
    }
    lakehouses = {
      raw_bronze_lh = {
        BronzeReaders = {
          paths          = ["/Tables/customers"]
          actions        = ["Read"]
          principal_ids  = ["55555555-5555-5555-5555-555555555551"]
          principal_type = "Group"
        }
      }
    }
  }
}
`, wsID)

	partialVarsWithDefault := strings.TrimSuffix(strings.TrimSpace(string(varsBytes)), "}") + fmt.Sprintf(`
  default = {
    workspace_id = %q
    warehouses = {
      sales_analytics_wh = {
        write = [
          { id = "55555555-5555-5555-5555-555555555551", type = "Group" },
          { id = "55555555-5555-5555-5555-555555555552", type = "ServicePrincipal" }
        ]
      }
    }
  }
}
`, wsID)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: strings.Join([]string{
					testAccProviderConfig(srv),
					varsWithDefault,
					string(mainBytes),
					string(outputsBytes),
				}, "\n"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("warehouse_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{
						"sales_analytics_wh/Group/55555555-5555-5555-5555-555555555551":            knownvalue.StringExact(wsID + "/" + whID + "/Group/55555555-5555-5555-5555-555555555551"),
						"sales_analytics_wh/ServicePrincipal/55555555-5555-5555-5555-555555555552": knownvalue.StringExact(wsID + "/" + whID + "/ServicePrincipal/55555555-5555-5555-5555-555555555552"),
					})),
					statecheck.ExpectKnownOutputValue("sql_database_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{
						"operational_orders_db/Group/55555555-5555-5555-5555-555555555551": knownvalue.StringExact(wsID + "/" + dbID + "/Group/55555555-5555-5555-5555-555555555551"),
					})),
					statecheck.ExpectKnownOutputValue("lakehouse_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{
						"raw_bronze_lh/BronzeReaders": knownvalue.StringExact(wsID + "/" + lhID + "/BronzeReaders"),
					})),
				},
			},
			// Step 2: Upgrade Group from read -> write in-place (same key) and omit sql_databases & lakehouses
			{
				Config: strings.Join([]string{
					testAccProviderConfig(srv),
					partialVarsWithDefault,
					string(mainBytes),
					string(outputsBytes),
				}, "\n"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("warehouse_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{
						"sales_analytics_wh/Group/55555555-5555-5555-5555-555555555551":            knownvalue.StringExact(wsID + "/" + whID + "/Group/55555555-5555-5555-5555-555555555551"),
						"sales_analytics_wh/ServicePrincipal/55555555-5555-5555-5555-555555555552": knownvalue.StringExact(wsID + "/" + whID + "/ServicePrincipal/55555555-5555-5555-5555-555555555552"),
					})),
					statecheck.ExpectKnownOutputValue("sql_database_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownOutputValue("lakehouse_permission_ids", knownvalue.MapExact(map[string]knownvalue.Check{})),
				},
			},
			// Cleanup step so post-test destroy sees an empty state
			{
				Config: testAccProviderConfig(srv),
			},
		},
	})
}

func TestToolingAndCI_Invariants(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repoRoot: %v", err)
	}

	// 1. Taskfile.yaml and .taskfiles/ must not exist.
	for _, retired := range []string{"Taskfile.yaml", ".taskfiles"} {
		if _, err := os.Stat(filepath.Join(repoRoot, retired)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be retired and absent, got err=%v", retired, err)
		}
	}

	// 2. mise.toml must enforce lockfile = true, minimum_release_age = "7d", and all required tasks.
	miseBytes, err := os.ReadFile(filepath.Join(repoRoot, "mise.toml"))
	if err != nil {
		t.Fatalf("read mise.toml: %v", err)
	}
	miseText := string(miseBytes)
	for _, snippet := range []string{
		`lockfile = true`,
		`minimum_release_age = "7d"`,
		`[tasks.build]`,
		`[tasks.install-local]`,
		`[tasks."specs:check"]`,
		`[tasks."specs:sync"]`,
		`[tasks."test:unit"]`,
		`[tasks."test:acc"]`,
		`[tasks.test]`,
		`[tasks.docs]`,
		`[tasks."docs:check"]`,
		`[tasks.scan]`,
		`[tasks.lint]`,
		`[tasks.format]`,
		`[tasks.bump]`,
		`[tasks."spec:verify"]`,
		`[tasks.check]`,
	} {
		if !strings.Contains(miseText, snippet) {
			t.Errorf("mise.toml missing required snippet %q", snippet)
		}
	}

	// 3. .pre-commit-config.yaml and trivy.yaml invariants.
	prekBytes, err := os.ReadFile(filepath.Join(repoRoot, ".pre-commit-config.yaml"))
	if err != nil {
		t.Fatalf("read .pre-commit-config.yaml: %v", err)
	}
	prekText := string(prekBytes)
	for _, hookID := range []string{
		"id: commitizen",
		"id: yamllint",
		"id: rumdl",
		"id: zizmor",
		"id: trivy-fs",
		"id: trivy-config",
		"id: terraform-fmt",
		"id: openapi-specs-check",
		"id: spec-verify",
		"id: golangci-lint",
	} {
		if !strings.Contains(prekText, hookID) {
			t.Errorf(".pre-commit-config.yaml missing required hook %q", hookID)
		}
	}
	if !strings.Contains(prekText, "--message-length-limit 72") {
		t.Errorf(".pre-commit-config.yaml missing --message-length-limit 72 on commitizen hook")
	}

	czBytes, err := os.ReadFile(filepath.Join(repoRoot, ".cz.yaml"))
	if err != nil {
		t.Fatalf("read .cz.yaml: %v", err)
	}
	if !strings.Contains(string(czBytes), "message_length_limit: 72") {
		t.Errorf(".cz.yaml missing message_length_limit: 72")
	}

	trivyBytes, err := os.ReadFile(filepath.Join(repoRoot, "trivy.yaml"))
	if err != nil {
		t.Fatalf("read trivy.yaml: %v", err)
	}
	if !strings.Contains(string(trivyBytes), "exit-code: 1") {
		t.Errorf("trivy.yaml must set exit-code: 1")
	}

	// 4. Dependabot 7-day cooldown and groups.
	depBytes, err := os.ReadFile(filepath.Join(repoRoot, ".github", "dependabot.yaml"))
	if err != nil {
		t.Fatalf("read .github/dependabot.yaml: %v", err)
	}
	depText := string(depBytes)
	for _, snippet := range []string{"default-days: 7", "terraform-plugin:", "github-actions:"} {
		if !strings.Contains(depText, snippet) {
			t.Errorf(".github/dependabot.yaml missing required snippet %q", snippet)
		}
	}

	// 5. Separated CI & Release workflows with StepSecurity harden-runner and SLSA provenance.
	for _, wf := range []string{"ci.yaml", "release.yaml"} {
		wfBytes, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", wf))
		if err != nil {
			t.Fatalf("read .github/workflows/%s: %v", wf, err)
		}
		wfText := string(wfBytes)
		if !strings.Contains(wfText, "permissions: {}") {
			t.Errorf("%s must declare top-level permissions: {}", wf)
		}
		if !strings.Contains(wfText, "step-security/harden-runner@") {
			t.Errorf("%s must use step-security/harden-runner", wf)
		}
		if strings.Contains(wfText, "pull_request_target") {
			t.Errorf("%s must never use pull_request_target", wf)
		}
	}

	relBytes, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yaml"))
	if err != nil {
		t.Fatalf("read release.yaml: %v", err)
	}
	relText := string(relBytes)
	for _, snippet := range []string{
		"name: release | prepare",
		"name: provider | release",
		"environment: release",
		"actions/attest-build-provenance@",
		"step-security/goreleaser-action@",
		"release --clean",
	} {
		if !strings.Contains(relText, snippet) {
			t.Errorf("release.yaml missing required release pipeline invariant %q", snippet)
		}
	}
}
