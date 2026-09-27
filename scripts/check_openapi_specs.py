#!/usr/bin/env python3
"""Verify or sync vendored Microsoft Fabric OpenAPI specifications."""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import sys
import urllib.request

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
OPENAPI_DIR = REPO_ROOT / "specs" / "openapi"
LOCK_FILE = OPENAPI_DIR / "lock.json"

UPSTREAM_COMMITS_URL = "https://api.github.com/repos/microsoft/fabric-rest-api-specs/commits/main"
SPEC_REL_PATHS = [
    "common/definitions.json",
    "platform/swagger.json",
    "platform/definitions/platform.json",
    "warehouse/swagger.json",
    "warehouse/definitions.json",
    "lakehouse/swagger.json",
    "lakehouse/definitions.json",
    "sqlDatabase/swagger.json",
    "sqlDatabase/definitions.json",
]


def sync_specs() -> int:
    req = urllib.request.Request(
        UPSTREAM_COMMITS_URL,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "terraform-fabric-provider",
        },
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        commit_info = json.loads(resp.read().decode("utf-8"))

    commit_sha = commit_info["sha"]
    commit_date = commit_info["commit"]["committer"]["date"]

    files_lock: dict[str, dict[str, str]] = {}
    for rel_path in SPEC_REL_PATHS:
        url = f"https://raw.githubusercontent.com/microsoft/fabric-rest-api-specs/{commit_sha}/{rel_path}"
        with urllib.request.urlopen(url, timeout=30) as resp:
            raw_bytes = resp.read()
        json.loads(raw_bytes.decode("utf-8"))
        out_path = OPENAPI_DIR / rel_path
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_bytes(raw_bytes)
        files_lock[rel_path] = {
            "sha256": hashlib.sha256(raw_bytes).hexdigest(),
            "source_url": url,
        }

    overlay_rel = "overlays/item-permissions.json"
    overlay_path = OPENAPI_DIR / overlay_rel
    if overlay_path.is_file():
        files_lock[overlay_rel] = {
            "sha256": hashlib.sha256(overlay_path.read_bytes()).hexdigest(),
            "source_url": f"local://specs/openapi/{overlay_rel}",
        }

    lock_data = {
        "repository": "https://github.com/microsoft/fabric-rest-api-specs",
        "branch": "main",
        "commit_sha": commit_sha,
        "commit_date": commit_date,
        "files": files_lock,
    }
    LOCK_FILE.write_text(json.dumps(lock_data, indent=2) + "\n", encoding="utf-8")
    print(f"Synced {len(files_lock)} OpenAPI specs at commit {commit_sha}")
    return 0


def check_specs() -> int:
    if not LOCK_FILE.is_file():
        print(f"ERROR: missing lockfile {LOCK_FILE}", file=sys.stderr)
        return 1

    lock_data = json.loads(LOCK_FILE.read_text(encoding="utf-8"))
    files_lock = lock_data.get("files", {})
    if not files_lock or "overlays/item-permissions.json" not in files_lock:
        print("ERROR: lock.json is missing required tracked files or overlay", file=sys.stderr)
        return 1

    for rel_path, meta in sorted(files_lock.items()):
        file_path = OPENAPI_DIR / rel_path
        if not file_path.is_file():
            print(f"ERROR: missing vendored OpenAPI file: {file_path}", file=sys.stderr)
            return 1
        raw_bytes = file_path.read_bytes()
        actual_sha = hashlib.sha256(raw_bytes).hexdigest()
        expected_sha = meta.get("sha256")
        if actual_sha != expected_sha:
            print(
                f"ERROR: SHA-256 mismatch for {rel_path}: expected {expected_sha}, got {actual_sha}",
                file=sys.stderr,
            )
            return 1
        try:
            doc = json.loads(raw_bytes.decode("utf-8"))
        except json.JSONDecodeError as exc:
            print(f"ERROR: invalid JSON in {rel_path}: {exc}", file=sys.stderr)
            return 1
        if doc.get("swagger") != "2.0":
            print(f"ERROR: expected swagger 2.0 in {rel_path}", file=sys.stderr)
            return 1

    print(
        f"Verified {len(files_lock)} vendored OpenAPI specs (commit {lock_data.get('commit_sha', 'unknown')[:12]}) + item-permissions overlay."
    )
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sync", action="store_true", help="Sync specs from upstream before checking")
    args = parser.parse_args()
    if args.sync:
        rc = sync_specs()
        if rc != 0:
            return rc
    return check_specs()


if __name__ == "__main__":
    sys.exit(main())
