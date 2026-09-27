#!/usr/bin/env python3
# Copyright jambazid 2026
# SPDX-License-Identifier: MPL-2.0

"""
Parses Go coverage profiles (coverage.out) and generates GitHub Flavored Markdown
coverage summary tables for $GITHUB_STEP_SUMMARY and PR comments, with HTML export
and optional threshold validation.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
import os
from pathlib import Path
import subprocess
import sys


def parse_coverage_profile(filepath: Path) -> tuple[dict[str, dict[str, int]], int, int]:
    """Parse Go coverage profile into per-package statement counts."""
    packages: dict[str, dict[str, int]] = defaultdict(lambda: {"total": 0, "covered": 0})

    if not filepath.exists():
        raise FileNotFoundError(f"Coverage profile not found at {filepath}")

    with open(filepath, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            parts = line.split()
            if len(parts) != 3:
                continue
            loc, num_stmts_str, count_str = parts[0], parts[1], parts[2]
            try:
                num_stmts = int(num_stmts_str)
                count = int(count_str)
            except ValueError:
                continue

            file_identifier = loc.split(":")[0]
            # Strip module import prefix (e.g. github.com/jambazid/terraform-provider-fabricext/)
            path_segments = file_identifier.split("/")
            if len(path_segments) > 3:
                pkg_name = "/".join(path_segments[3:-1])
            else:
                pkg_name = "/".join(path_segments[:-1])

            if not pkg_name:
                pkg_name = "(root)"

            packages[pkg_name]["total"] += num_stmts
            if count > 0:
                packages[pkg_name]["covered"] += num_stmts

    total_stmts = sum(p["total"] for p in packages.values())
    total_covered = sum(p["covered"] for p in packages.values())
    return packages, total_covered, total_stmts


def status_badge(percentage: float) -> str:
    """Return status icon based on coverage percentage."""
    if percentage >= 80.0:
        return "🟢 PASS"
    if percentage >= 60.0:
        return "🟡 WARN"
    return "🔴 FAIL"


def render_markdown_summary(
    packages: dict[str, dict[str, int]],
    total_covered: int,
    total_stmts: int,
    threshold: float | None = None,
) -> str:
    """Render a GitHub Flavored Markdown summary table."""
    total_pct = (total_covered / total_stmts * 100.0) if total_stmts > 0 else 0.0

    lines = [
        "## 🧪 Go Test Coverage Summary",
        "",
        "<!-- test-coverage-summary -->",
        f"**Overall Project Statement Coverage**: `{total_pct:.1f}%` ({total_covered:,} / {total_stmts:,} statements) {status_badge(total_pct)}",
        "",
        "| Package | Covered | Total | Coverage | Status |",
        "| :--- | :---: | :---: | :---: | :---: |",
    ]

    for pkg_name in sorted(packages.keys()):
        stats = packages[pkg_name]
        cov = stats["covered"]
        tot = stats["total"]
        pct = (cov / tot * 100.0) if tot > 0 else 0.0
        lines.append(f"| `{pkg_name}` | {cov:,} | {tot:,} | `{pct:.1f}%` | {status_badge(pct)} |")

    lines.append(f"| **Total** | **{total_covered:,}** | **{total_stmts:,}** | **`{total_pct:.1f}%`** | {status_badge(total_pct)} |")
    lines.append("")

    if threshold is not None:
        if total_pct >= threshold:
            lines.append(f"> [!NOTE]\n> Coverage threshold of `{threshold:.1f}%` met.")
        else:
            lines.append(f"> [!WARNING]\n> Coverage threshold of `{threshold:.1f}%` not met (`{total_pct:.1f}%` < `{threshold:.1f}%`).")
        lines.append("")

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="Parse Go test coverage profiles and generate summary reports.")
    parser.add_argument("profile", nargs="?", default="coverage.out", help="Path to Go coverage profile (default: coverage.out)")
    parser.add_argument("--html", metavar="HTML_PATH", help="Export interactive HTML report via go tool cover")
    parser.add_argument("--output", "-o", metavar="MD_PATH", help="Write Markdown table to file")
    parser.add_argument("--threshold", type=float, help="Minimum required total coverage percentage (exits with code 1 if unmet)")

    args = parser.parse_args()
    profile_path = Path(args.profile)

    try:
        packages, total_covered, total_stmts = parse_coverage_profile(profile_path)
    except FileNotFoundError as err:
        print(f"Error: {err}", file=sys.stderr)
        return 1

    md_report = render_markdown_summary(packages, total_covered, total_stmts, args.threshold)
    print(md_report)

    if args.output:
        out_path = Path(args.output)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(md_report, encoding="utf-8")

    if args.html:
        html_path = Path(args.html)
        html_path.parent.mkdir(parents=True, exist_ok=True)
        cmd = ["go", "tool", "cover", f"-html={profile_path}", f"-o={html_path}"]
        res = subprocess.run(cmd, capture_output=True, text=True)
        if res.returncode != 0:
            print(f"Failed to generate HTML coverage report: {res.stderr}", file=sys.stderr)
            return res.returncode

    total_pct = (total_covered / total_stmts * 100.0) if total_stmts > 0 else 0.0
    if args.threshold is not None and total_pct < args.threshold:
        print(f"Error: Total coverage {total_pct:.1f}% is below required threshold {args.threshold:.1f}%", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
