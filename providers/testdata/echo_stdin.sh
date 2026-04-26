#!/usr/bin/env bash
# Test fixture: round-trips ctx fields back as a row so the test can assert
# that the request payload was delivered intact on stdin. Uses python3
# (already a project dependency for examples/providers/fs.py).
exec python3 -c '
import json, sys
ctx = json.load(sys.stdin)
print(json.dumps({
    "key": "stdin",
    "version": ctx.get("version"),
    "source": ctx.get("source", ""),
    "prefix": ctx.get("prefix", ""),
    "where": ctx.get("where", ""),
}))
'
