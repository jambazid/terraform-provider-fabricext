// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

package provider_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestRegistryDocs_ExperimentalBannerPresent verifies that the experimental provider warning
// banner is prominently present in root docs, modules, and resource templates.
func TestRegistryDocs_ExperimentalBannerPresent(t *testing.T) {
	requiredFiles := []string{
		"../../templates/index.md.tmpl",
		"../../README.md",
		"../../templates/resources/lakehouse_permission.md.tmpl",
		"../../templates/resources/lakehouse_onelake_security.md.tmpl",
		"../../templates/resources/warehouse_permission.md.tmpl",
		"../../templates/resources/sql_database_permission.md.tmpl",
	}

	for _, file := range requiredFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read required file %s: %v", file, err)
		}

		str := string(content)
		if !strings.Contains(str, "Experimental Provider") && !strings.Contains(str, "experimental") {
			t.Errorf("file %s is missing experimental provider warning banner", file)
		}
	}
}

// TestRegistryDocs_DeprecationNoticePresent verifies that warehouse and sql database templates
// contain the formal deprecation notice and explanation.
func TestRegistryDocs_DeprecationNoticePresent(t *testing.T) {
	deprecatedFiles := []string{
		"../../templates/resources/warehouse_permission.md.tmpl",
		"../../templates/resources/sql_database_permission.md.tmpl",
	}

	for _, file := range deprecatedFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read deprecated file %s: %v", file, err)
		}

		str := string(content)
		if !strings.Contains(str, "!> **Deprecated:**") {
			t.Errorf("file %s is missing '!> **Deprecated:**' callout", file)
		}
		if !strings.Contains(str, "v0.4.0") {
			t.Errorf("file %s is missing removal target version v0.4.0", file)
		}
	}
}

// TestRegistryDocs_AccessControlPlaybook verifies that the playbook template exists,
// contains the Start Here routing table, and provides structured cards with verify commands.
func TestRegistryDocs_AccessControlPlaybook(t *testing.T) {
	playbookPath := "../../templates/guides/access_control_playbook.md.tmpl"
	content, err := os.ReadFile(playbookPath)
	if err != nil {
		t.Fatalf("failed to read playbook file %s: %v", playbookPath, err)
	}

	str := string(content)
	if !strings.Contains(str, "## Start Here Routing Table") {
		t.Errorf("playbook missing 'Start Here Routing Table' section")
	}

	scenarios := []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11"}
	for _, sc := range scenarios {
		if !strings.Contains(str, "## "+sc+":") {
			t.Errorf("playbook missing scenario card for %s", sc)
		}
	}

	if !strings.Contains(str, "sqlcmd") {
		t.Errorf("playbook missing sqlcmd verification queries")
	}
	if !strings.Contains(str, "spark.read.table") {
		t.Errorf("playbook missing Spark verification code")
	}
}

// TestRegistryDocs_HeadingLengthBound verifies that all markdown headings in templates/**
// do not exceed 5 words in length.
func TestRegistryDocs_HeadingLengthBound(t *testing.T) {
	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	err := filepath.Walk("../../templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md.tmpl") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")
		inCodeBlock := false
		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}

			matches := headingRegex.FindStringSubmatch(trimmed)
			if len(matches) == 3 {
				title := strings.TrimSpace(matches[2])
				// Ignore template placeholders
				if strings.HasPrefix(title, "{{") {
					continue
				}
				words := strings.Fields(title)
				if len(words) > 5 {
					t.Errorf("%s:%d heading %q has %d words (> 5 limit)", path, lineNum+1, title, len(words))
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk templates directory: %v", err)
	}
}

// TestRegistryDocs_NoActiveItemSharingClaims verifies that templates and documentation
// contain zero claims of nonexistent REST item sharing or fake overlays.
func TestRegistryDocs_NoActiveItemSharingClaims(t *testing.T) {
	filesToCheck := []string{
		"../../templates/guides/official_provider_comparison.md.tmpl",
		"../../guides/official_provider_comparison.md",
		"../../README.md",
		"../../templates/index.md.tmpl",
		"../../templates/guides/use_case_overview.md.tmpl",
		"../../templates/guides/use_case_controls_and_interactions.md.tmpl",
		"../../templates/guides/use_case_powerbi_identity_propagation.md.tmpl",
		"../../templates/guides/matrix_composition.md.tmpl",
	}

	bannedSubstrings := []string{
		"specs/openapi/overlays/item-permissions.json",
		"POST /grantPermissions",
		"POST /revokePermissions",
		"grantPermissions` endpoint",
		"revokePermissions` endpoint",
	}

	for _, file := range filesToCheck {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", file, err)
		}
		str := string(content)

		for _, banned := range bannedSubstrings {
			if strings.Contains(str, banned) {
				t.Errorf("file %s contains banned claim: %q", file, banned)
			}
		}
	}

	// Verify README Quick Start does not configure deprecated resources
	readmeContent, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	readmeStr := string(readmeContent)
	if strings.Contains(readmeStr, `resource "fabricext_warehouse_permission" "analytics_readers"`) {
		t.Errorf("README.md Quick Start still declares active warehouse_permission resource")
	}
	if strings.Contains(readmeStr, `resource "fabricext_sql_database_permission" "orders_readers"`) {
		t.Errorf("README.md Quick Start still declares active sql_database_permission resource")
	}
}

// TestRegistryDocs_HonestComparisonNarrative verifies that the comparison guides contain
// genuine technical differentiators and no nonexistent REST sequence diagrams.
func TestRegistryDocs_HonestComparisonNarrative(t *testing.T) {
	guides := []string{
		"../../templates/guides/official_provider_comparison.md.tmpl",
		"../../guides/official_provider_comparison.md",
	}

	requiredTerms := []string{
		"DefaultReader",
		"If-Match",
		"kin-openapi",
		"fabricmock",
	}

	for _, guide := range guides {
		content, err := os.ReadFile(guide)
		if err != nil {
			t.Fatalf("failed to read guide %s: %v", guide, err)
		}
		str := string(content)

		for _, term := range requiredTerms {
			if !strings.Contains(str, term) {
				t.Errorf("guide %s missing required honest comparison term: %q", guide, term)
			}
		}

		if strings.Contains(str, "revokePermissions (permissions:") {
			t.Errorf("guide %s contains nonexistent REST sequence diagram", guide)
		}
	}
}
