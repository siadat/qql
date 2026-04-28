#!/usr/bin/env bash
# Test fixture: round-trips ctx fields back as a row so the test can assert
# that the request payload was delivered intact on stdin. Uses python3
# (already a project dependency for examples/providers/fs.py).
exec python3 -c '
import json, sys
ctx = json.load(sys.stdin)
print(json.dumps({"type": "row", "value": {
    "key": "stdin",
    "version": ctx.get("version"),
    "source": ctx.get("source", ""),
    "has_prefix": "prefix" in ctx,
    "where": ctx.get("where", ""),
}}))
'
