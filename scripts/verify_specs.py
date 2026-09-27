#!/usr/bin/env python3
"""Verify YAML frontmatter, targets, and [@test] links across specs/*.spec.md."""

from __future__ import annotations

import glob
import pathlib
import re
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SPECS_DIR = REPO_ROOT / "specs"
TEST_LINK_RE = re.compile(r"`\[@test\]\s+([^`]+)`")


def parse_frontmatter(text: str, spec_path: pathlib.Path) -> tuple[dict[str, object], str]:
    if not text.startswith("---\n"):
        raise ValueError(f"{spec_path}: missing opening YAML frontmatter delimiter '---'")
    end_idx = text.find("\n---\n", 4)
    if end_idx == -1:
        raise ValueError(f"{spec_path}: missing closing YAML frontmatter delimiter '---'")
    fm_block = text[4:end_idx]
    body = text[end_idx + 5 :]

    data: dict[str, object] = {}
    current_list_key: str | None = None
    for raw_line in fm_block.splitlines():
        line = raw_line.rstrip()
        if not line or line.lstrip().startswith("#"):
            continue
        if line.startswith("  - ") or line.startswith("- "):
            if current_list_key is None:
                raise ValueError(f"{spec_path}: unexpected list item '{line}' in frontmatter")
            val = line.split("- ", 1)[1].strip()
            lst = data.setdefault(current_list_key, [])
            if isinstance(lst, list):
                lst.append(val)
            continue
        if ":" in line:
            key, val = line.split(":", 1)
            key = key.strip()
            val = val.strip()
            if val == "":
                current_list_key = key
                data[key] = []
            else:
                current_list_key = None
                data[key] = val
    return data, body


def main() -> int:
    spec_files = sorted(SPECS_DIR.glob("*.spec.md"))
    if not spec_files:
        print("ERROR: no specs/*.spec.md files found", file=sys.stderr)
        return 1

    errors: list[str] = []
    for spec_path in spec_files:
        text = spec_path.read_text(encoding="utf-8")
        try:
            fm, body = parse_frontmatter(text, spec_path)
        except ValueError as exc:
            errors.append(str(exc))
            continue

        for req_key in ("name", "description", "targets"):
            if not fm.get(req_key):
                errors.append(f"{spec_path.name}: missing required frontmatter key '{req_key}'")

        targets = fm.get("targets", [])
        if isinstance(targets, list):
            for pattern in targets:
                resolved_glob = str((spec_path.parent / pattern).resolve())
                matches = glob.glob(resolved_glob, recursive=True)
                if not matches and not pathlib.Path(resolved_glob).exists():
                    errors.append(f"{spec_path.name}: target '{pattern}' does not match any existing file")

        test_links = TEST_LINK_RE.findall(body)
        if not test_links:
            errors.append(f"{spec_path.name}: contains no `[@test] <path>` links")
        for link in test_links:
            raw_link = link.strip()
            func_name: str | None = None
            if "::" in raw_link:
                raw_link, func_name = raw_link.split("::", 1)
            test_path = (spec_path.parent / raw_link).resolve()
            if not test_path.is_file():
                errors.append(f"{spec_path.name}: [@test] target '{link}' does not exist at {test_path}")
                continue
            test_content = test_path.read_text(encoding="utf-8")
            if func_name:
                if f"func {func_name}(" not in test_content:
                    errors.append(f"{spec_path.name}: [@test] function '{func_name}' not found in {test_path}")
            elif "func Test" not in test_content:
                errors.append(f"{spec_path.name}: [@test] file {test_path} contains no Test functions")

    if errors:
        for err in errors:
            print(f"ERROR: {err}", file=sys.stderr)
        return 1

    print(f"Verified {len(spec_files)} spec files in specs/*.spec.md (all frontmatter, targets, and [@test] links valid).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
