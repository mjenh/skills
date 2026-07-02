#!/usr/bin/env python3
"""Update root README module table and go get examples for a release."""

from __future__ import annotations

import re
import sys
from pathlib import Path


def update_readme(path: Path, module: str, version: str, anchor: str) -> None:
    text = path.read_text(encoding="utf-8")
    import_path = f"github.com/mjenh/skills/{module}"
    latest_link = f"[v{version}]({module}/CHANGELOG.md#{anchor})"

    row_pattern = re.compile(
        rf"(\|\s*\[{re.escape(module)}\]\({re.escape(module)}/\)\s*\|\s*`{re.escape(import_path)}`\s*\|\s*)\[[^\]]+\]\([^)]+\)(\s*\|)",
        re.MULTILINE,
    )
    updated, count = row_pattern.subn(rf"\1{latest_link}\2", text, count=1)
    if count == 0:
        raise SystemExit(f"Could not find README table row for module {module}")

    get_pattern = re.compile(
        rf"(go get {re.escape(import_path)}@)v[0-9]+\.[0-9]+\.[0-9]+"
    )
    updated = get_pattern.sub(rf"\1v{version}", updated)

    path.write_text(updated, encoding="utf-8")


def main() -> None:
    if len(sys.argv) != 5:
        raise SystemExit(
            "usage: update-readme.py <readme-path> <module> <version> <anchor>"
        )

    update_readme(
        Path(sys.argv[1]),
        sys.argv[2],
        sys.argv[3],
        sys.argv[4],
    )


if __name__ == "__main__":
    main()
