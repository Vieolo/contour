#!/usr/bin/env python3
"""Verify contour the same way every time.

Runs every check a change has to pass, under both build tags — the dev build
uses its own store, config and usage directory, so a test that hardcodes a path
can pass one and fail the other. Every check runs even after a failure, so one
run reports everything that is wrong rather than the first thing.

Exit status is 0 only when every check passes.

    python3 agent.py
"""

import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.abspath(__file__))

CHECKS = [
    ("build", ["go", "build", "./..."]),
    ("vet", ["go", "vet", "./..."]),
    ("build (dev)", ["go", "build", "-tags", "dev", "./..."]),
    ("vet (dev)", ["go", "vet", "-tags", "dev", "./..."]),
    # -count=1 bypasses the test cache, so a pass reflects the code as it is now.
    ("test", ["go", "test", "-count=1", "./..."]),
    ("test (dev)", ["go", "test", "-count=1", "-tags", "dev", "./..."]),
]


def run(name, cmd):
    result = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
    ok = result.returncode == 0
    print(f"{'PASS' if ok else 'FAIL'}  {name}")
    if not ok:
        print(indent(result.stdout + result.stderr))
    return ok


def gofmt():
    # gofmt -l exits 0 even when files need formatting; the list is the signal.
    result = subprocess.run(
        ["gofmt", "-l", "cmd", "internal", "main.go"],
        cwd=ROOT, capture_output=True, text=True,
    )
    unformatted = result.stdout.strip()
    ok = result.returncode == 0 and not unformatted
    print(f"{'PASS' if ok else 'FAIL'}  gofmt")
    if not ok:
        print(indent(unformatted or result.stderr))
    return ok


def indent(text):
    return "\n".join("      " + line for line in text.strip().splitlines())


def main():
    results = [run(name, cmd) for name, cmd in CHECKS]
    results.append(gofmt())

    failed = results.count(False)
    print()
    print("all checks passed" if not failed else f"{failed} check(s) failed")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
