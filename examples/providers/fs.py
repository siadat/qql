#!/usr/bin/env python3
"""Reference qql external provider: walks directories and emits one row per file.

Wire protocol (see README): qql sends a single JSON object on stdin and reads
JSONL envelopes on stdout, each shaped {"type": "row"|"tree", "value": ...}.
Stderr passes through to the user. Hints (where, order_by, select, prefix) are
optional — qql re-applies them to whatever rows we emit.
"""

import json
import os
import sys


def main() -> int:
    ctx = json.load(sys.stdin)
    roots = ctx.get("files") or [ctx.get("source") or "."]

    for root in roots:
        if os.path.isfile(root):
            emit_file(root)
            continue
        for dirpath, _, filenames in os.walk(root):
            for name in filenames:
                emit_file(os.path.join(dirpath, name))
    return 0


def emit_file(path: str) -> None:
    try:
        st = os.stat(path)
    except OSError as e:
        print(f"fs.py: stat {path}: {e}", file=sys.stderr)
        return
    print(json.dumps({"type": "row", "value": {
        "key": path,
        "name": os.path.basename(path),
        "path": os.path.dirname(path),
        "size": st.st_size,
        "mtime": int(st.st_mtime),
        "is_dir": False,
    }}))


if __name__ == "__main__":
    sys.exit(main())
