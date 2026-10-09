#!/usr/bin/env python3
"""Targeted, deterministic mutation checks in disposable copies, not the checkout.

This is a finite regression set, not a general mutation score. Future policy and
authorization code must add its own mutations and tests.
"""
import pathlib
import shutil
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
MUTATIONS = [
    ("config validation removed", "internal/config/config.go",
     "err != nil || n < MinFrameBytes || n > DefaultMaxFrameBytes",
     "false", "./internal/config", "TestLoad"),
    ("maximum boundary shifted", "internal/config/config.go",
     "n > DefaultMaxFrameBytes", "n >= DefaultMaxFrameBytes",
     "./internal/config", "TestLoad"),
    ("transport limit increased by one", "internal/server/server.go",
     "MaxLineLength: cfg.MaxFrameBytes", "MaxLineLength: cfg.MaxFrameBytes + 1",
     "./internal/server", "TestFrameBoundary"),
]

def test(work, package, case):
    return subprocess.run(["go", "test", "-count=1", "-timeout=45s", package,
                           "-run", "^" + case + "$"], cwd=work,
                          capture_output=True, text=True, timeout=60)

for name, filename, before, after, package, case in MUTATIONS:
    with tempfile.TemporaryDirectory(prefix="pcloud-mutation-") as tmp:
        work = pathlib.Path(tmp) / "repo"
        shutil.copytree(ROOT, work, ignore=shutil.ignore_patterns(".git", "bin", "__pycache__"))
        baseline = test(work, package, case)
        if baseline.returncode:
            raise SystemExit("Baseline failed:\n" + baseline.stdout + baseline.stderr)
        path = work / filename
        source = path.read_text()
        if source.count(before) != 1:
            raise SystemExit("Mutation anchor is ambiguous: " + name)
        mutated = source.replace(before, after)
        # Removing the validation also makes err unused. Retain compilation.
        if name == "config validation removed":
            mutated = mutated.replace("if false {", "_ = err\n\t\tif false {")
        path.write_text(mutated)
        result = test(work, package, case)
        output = result.stdout + result.stderr
        if result.returncode == 0:
            raise SystemExit("SURVIVED: " + name)
        if "--- FAIL: " not in output or "[build failed]" in output:
            raise SystemExit("Invalid mutation result:\n" + output)
        print("KILLED: " + name)
