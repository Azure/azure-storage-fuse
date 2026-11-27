# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

import subprocess
import sys
from pathlib import Path


GO_HEADER = [
    "// Copyright (c) Microsoft Corporation.",
    "// Licensed under the MIT License.",
]
C_HEADER = [
    "/*",
    " * Copyright (c) Microsoft Corporation.",
    " * Licensed under the MIT License",
    " */",
]


def has_header(text: str, suffix: str) -> bool:
    lines = text.splitlines()
    while lines and (
        not lines[0].strip()
        or (
            suffix in (".go", ".template")
            and lines[0].startswith(("//go:build ", "// +build "))
        )
    ):
        lines.pop(0)
    expected = C_HEADER if suffix in (".c", ".h") else GO_HEADER
    return lines[:len(expected)] == expected


def check_repository(root: Path) -> int:
    result = subprocess.run(
        [
            "git", "-C", str(root), "ls-files", "-z", "--",
            "*.go", "*.c", "*.h", "internal/component.template",
        ],
        check=True,
        stdout=subprocess.PIPE,
    )
    paths = [Path(name) for name in result.stdout.decode().split("\0") if name]
    if not paths:
        print("Copyright check failed: no tracked source files found.", file=sys.stderr)
        return 1

    invalid = []
    for path in paths:
        if not has_header((root / path).read_text(encoding="utf-8"), path.suffix):
            invalid.append(path)

    if invalid:
        for path in invalid:
            print(f"{path}: missing or incorrect copyright/license header", file=sys.stderr)
        print("Expected Go/template header:\n" + "\n".join(GO_HEADER), file=sys.stderr)
        print("Expected C/header notice:\n" + "\n".join(C_HEADER), file=sys.stderr)
        return 1

    print(f"Copyright check passed for {len(paths)} tracked source files/templates.")
    return 0


if __name__ == "__main__":
    sys.exit(check_repository(Path(__file__).resolve().parents[2]))
