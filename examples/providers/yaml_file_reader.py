#!/usr/bin/env python3
"""Reference qql external provider: reads YAML files and emits each document
as a JSONL "tree" envelope, leaving qql to apply its prefix-glob unfolding.

Wire protocol (see README): qql sends a single JSON object on stdin and reads
JSONL envelopes on stdout. Each line must be of the form
{"type": "row"|"tree", "value": ...}. We always emit "tree" so the WITH prefix
controls how each document is turned into rows.

Multi-document YAML (`---` separators) emits one envelope per document.
"""

import json
import os
import sys

import yaml


def main() -> int:
    ctx = json.load(sys.stdin)
    sources = ctx.get("files") or ([ctx["source"]] if ctx.get("source") else [])
    if not sources:
        print("yaml.py: no source files provided", file=sys.stderr)
        return 1

    for path in sources:
        emit_file(path)
    return 0


def emit_file(path: str) -> None:
    if not os.path.isfile(path):
        print(f"yaml.py: not a file: {path}", file=sys.stderr)
        return
    try:
        with open(path) as f:
            for doc in yaml.safe_load_all(f):
                if doc is None:
                    continue
                print(json.dumps({"type": "tree", "value": doc}))
    except (OSError, yaml.YAMLError) as e:
        print(f"yaml.py: {path}: {e}", file=sys.stderr)


if __name__ == "__main__":
    sys.exit(main())
