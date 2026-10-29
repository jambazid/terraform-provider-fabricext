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
		"../../templates/resources/lakehouse_data_access_role.md.tmpl",
		"../../templates/resources/lakehouse_onelake_security.md.tmpl",
		"../../templates/resources/warehouse_permission.md.tmpl",
		"../../templates/resources/sql_database_permission.md.tmpl",
		"../../templates/resources/lakehouse_permission.md.tmpl",
		"../../templates/resources/item_share_gate.md.tmpl",
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

// TestRegistryDocs_ActivePermissionResourcesPresent verifies that warehouse, sql database,
// lakehouse permission, and item share gate templates are present in templates/resources/.
func TestRegistryDocs_ActivePermissionResourcesPresent(t *testing.T) {
	requiredFiles := []string{
		"../../templates/resources/warehouse_permission.md.tmpl",
		"../../templates/resources/sql_database_permission.md.tmpl",
		"../../templates/resources/lakehouse_permission.md.tmpl",
		"../../templates/resources/item_share_gate.md.tmpl",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("active resource template %s missing from templates/resources/: %v", file, err)
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

// TestRegistryDocs_GuidesReorganizationAndPhrasing verifies that all guides are organized under
// subcategory "Guides", carry no "Use Cases:" prefix in titles, use open-ended connectivity phrasing,
// include Atlas alongside SQL migration tools, and contain zero legacy fabricext_lakehouse_permission references.
func TestRegistryDocs_GuidesReorganizationAndPhrasing(t *testing.T) {
	guidesDir := "../../templates/guides"
	entries, err := os.ReadDir(guidesDir)
	if err != nil {
		t.Fatalf("failed to read guides dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md.tmpl") {
			continue
		}

		filePath := filepath.Join(guidesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", filePath, err)
		}
		str := string(content)

		// 1. Verify no file has "Use Cases" subcategory
		if strings.Contains(str, `subcategory: "Use Cases"`) {
			t.Errorf("guide %s still has subcategory: %q", entry.Name(), "Use Cases")
		}

		// Former use case files and playbook must be in subcategory "Guides"
		if strings.HasPrefix(entry.Name(), "use_case_") || entry.Name() == "access_control_playbook.md.tmpl" {
			if !strings.Contains(str, `subcategory: "Guides"`) {
				t.Errorf("guide %s should have subcategory: %q", entry.Name(), "Guides")
			}
		}

		// 2. Verify no "Use Cases:" prefix in page_title
		if strings.Contains(str, `page_title: "Use Cases:`) {
			t.Errorf("guide %s still has 'Use Cases:' prefix in page_title", entry.Name())
		}

		// 3. Verify legacy fabricext_lakehouse_permission is not declared as a role resource (with role_name)
		legacyRoleRegex := regexp.MustCompile(`(?s)resource\s+"fabricext_lakehouse_permission"[^{]*\{[^}]*role_name`)
		if legacyRoleRegex.MatchString(str) {
			t.Errorf("guide %s declares fabricext_lakehouse_permission with legacy OneLake role_name attribute", entry.Name())
		}
	}

	// 4. Verify open-ended connectivity phrasing and Atlas in key guides
	keyGuides := []string{
		"../../templates/guides/access_control_playbook.md.tmpl",
		"../../templates/guides/use_case_warehouse_schema_isolation.md.tmpl",
	}

	citationPattern := regexp.MustCompile(`\(C[0-9]`)

	for _, path := range keyGuides {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		str := string(content)

		if !strings.Contains(str, "any ODBC or JDBC client") && !strings.Contains(str, "Any ODBC or JDBC client") {
			t.Errorf("guide %s missing open-ended connectivity phrasing ('any ODBC or JDBC client')", path)
		}

		if !strings.Contains(str, "Atlas") {
			t.Errorf("guide %s missing 'Atlas' tool reference", path)
		}

		// 5. Verify zero raw citation codes leak into user-facing guides
		if citationPattern.MatchString(str) {
			t.Errorf("guide %s contains unpurged raw citation code (e.g., (Cxx))", path)
		}
	}

	// 6. Verify companion guide granular_access_control.md
	compPath := "../../guides/granular_access_control.md"
	compContent, err := os.ReadFile(compPath)
	if err == nil {
		compStr := string(compContent)
		if strings.Contains(compStr, `subcategory: "Use Cases"`) {
			t.Errorf("companion guide %s still has subcategory: %q", compPath, "Use Cases")
		}
		if citationPattern.MatchString(compStr) {
			t.Errorf("companion guide %s contains unpurged raw citation code", compPath)
		}
	}
}
