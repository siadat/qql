#!/usr/bin/env bash
# Test fixture: drains stdin, then emits a fixed set of rows interleaved with
# blank lines, comments, and stderr noise so the test exercises the whole
# parsing path.
cat >/dev/null
echo "echo.sh: starting" >&2
echo '{"key": "alpha", "n": 1, "ok": true}'
echo ''
echo '# this is a comment, must be skipped'
echo '{"key": "beta", "n": 2.5, "ok": false}'
echo "echo.sh: done" >&2
