#!/usr/bin/env python3
"""Detect modified, missing and newly generated files, including untracked ones."""
from pathlib import Path
import hashlib
import subprocess
import sys

ROOTS = ("pkg/api", "internal/postgres/gen/db", "sdk/ts/src/gen")

def snapshot():
    return {str(path): hashlib.sha256(path.read_bytes()).hexdigest()
            for root in ROOTS for path in Path(root).rglob("*") if path.is_file()}

before = snapshot()
subprocess.run(["make", "generate"], check=True)
after = snapshot()
changed = sorted(path for path in before.keys() | after.keys() if before.get(path) != after.get(path))
if changed:
    sys.exit("Generated contracts differ; run make generate and commit the output:\n" + "\n".join(changed))
print("Generated Go, TypeScript and SQL contracts are reproducible.")
