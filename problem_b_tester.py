#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shlex
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


def load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def find_first_difference(expected: Any, actual: Any, path: str = "$") -> str | None:
    if type(expected) is not type(actual):
        return (
            f"{path}: type mismatch "
            f"(expected {type(expected).__name__}, got {type(actual).__name__})"
        )

    if isinstance(expected, dict):
        expected_keys = set(expected)
        actual_keys = set(actual)

        missing = sorted(expected_keys - actual_keys)
        if missing:
            return f"{path}: missing key {missing[0]!r}"

        extra = sorted(actual_keys - expected_keys)
        if extra:
            return f"{path}: unexpected key {extra[0]!r}"

        for key in sorted(expected):
            diff = find_first_difference(expected[key], actual[key], f"{path}.{key}")
            if diff is not None:
                return diff
        return None

    if isinstance(expected, list):
        if len(expected) != len(actual):
            return (
                f"{path}: length mismatch (expected {len(expected)}, got {len(actual)})"
            )

        for idx, (expected_item, actual_item) in enumerate(zip(expected, actual)):
            diff = find_first_difference(expected_item, actual_item, f"{path}[{idx}]")
            if diff is not None:
                return diff
        return None

    if expected != actual:
        return f"{path}: value mismatch (expected {expected!r}, got {actual!r})"

    return None


def build_command(args: argparse.Namespace) -> list[str]:
    if args.command:
        return shlex.split(args.command)
    return [sys.executable, "problem_b.py", str(args.input)]


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--input",
        type=Path,
        default=Path("problem_b_generate.jsonl"),
        help="Input NDJSON file passed to the CLI.",
    )
    ap.add_argument(
        "--expected",
        type=Path,
        default=Path("problem_b_generate.jsonl.expected.json"),
        help="Expected JSON output file.",
    )
    ap.add_argument(
        "--timeout",
        type=float,
        default=300.0,
        help="Subprocess timeout in seconds.",
    )
    ap.add_argument("--command", type=str, help="Optional command override.")
    args = ap.parse_args()

    expected = load_json(args.expected)
    command = build_command(args)
    started_at = time.perf_counter()
    try:
        result = subprocess.run(
            command,
            capture_output=True,
            text=True,
            timeout=args.timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        elapsed_ms = (time.perf_counter() - started_at) * 1000.0
        print("Result: FAIL")
        print(f"Reason: command timed out after {args.timeout:.3f}s")
        print(f"Elapsed: {elapsed_ms:.3f} ms")
        raise SystemExit(1)

    elapsed_ms = (time.perf_counter() - started_at) * 1000.0

    print("Command:", " ".join(command))
    print(f"Exit code: {result.returncode}")
    print(f"Elapsed: {elapsed_ms:.3f} ms")

    if result.stderr.strip():
        print("Stderr:")
        print(result.stderr.strip())

    if result.returncode != 0:
        print("Result: FAIL")
        print("Reason: command exited non-zero")
        raise SystemExit(1)

    stdout = result.stdout.strip()
    if not stdout:
        print("Result: FAIL")
        print("Reason: command produced empty stdout")
        raise SystemExit(1)

    try:
        actual = json.loads(stdout)
    except json.JSONDecodeError as exc:
        print("Result: FAIL")
        print(f"Reason: stdout was not valid JSON ({exc})")
        raise SystemExit(1)

    diff = find_first_difference(expected, actual)
    if diff is None:
        print("Result: PASS")
        raise SystemExit(0)

    print("Result: FAIL")
    print(f"Reason: {diff}")
    raise SystemExit(1)


if __name__ == "__main__":
    main()
