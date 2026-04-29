#!/usr/bin/env python3
"""Reference qql external provider: walks directories and emits one row per file.

Wire protocol (see README): qql sends a single JSON object on stdin and reads
JSONL envelopes on stdout, each shaped {"type": "row"|"tree", "value": ...}.
Stderr passes through to the user. Hints (where, order_by, select) are
optional, qql re-applies them to whatever rows we emit.

The `lines` column is computed only when the query references it. Counting
lines reads the whole file, so we use the hints to skip the work otherwise.
"""

import json
import os
import sys


def main() -> int:
    ctx = json.load(sys.stdin)
    roots = ctx.get("files") or [ctx.get("source") or "."]
    with_lines = column_referenced("lines", ctx)

    for root in roots:
        if os.path.isfile(root):
            emit_file(root, with_lines)
            continue
        for dirpath, _, filenames in os.walk(root):
            for name in filenames:
                emit_file(os.path.join(dirpath, name), with_lines)
    return 0


def column_referenced(name: str, ctx: dict) -> bool:
    """True if any of select/where/order_by mentions name.

    `where` is the raw SQL substring, so a plain `in` check is enough. False
    positives (a string literal that happens to contain the word) only cost
    us extra work, never wrong rows: qql re-applies the parsed predicate.
    """
    if name in (ctx.get("select") or []):
        return True
    if any(t.get("col") == name for t in (ctx.get("order_by") or [])):
        return True
    if name in (ctx.get("where") or ""):
        return True
    return False


def emit_file(path: str, with_lines: bool) -> None:
    try:
        st = os.stat(path)
    except OSError as e:
        print(f"fs.py: stat {path}: {e}", file=sys.stderr)
        return
    row = {
        "key": path,
        "name": os.path.basename(path),
        "path": os.path.dirname(path),
        "size": st.st_size,
        "mtime": int(st.st_mtime),
        "is_dir": False,
    }
    if with_lines:
        row["lines"] = count_lines(path)
    print(json.dumps({"type": "row", "value": row}))


def count_lines(path):
    try:
        with open(path, "rb") as f:
            return sum(1 for _ in f)
    except OSError as e:
        print(f"fs.py: read {path}: {e}", file=sys.stderr)
        return None


if __name__ == "__main__":
    sys.exit(main())
