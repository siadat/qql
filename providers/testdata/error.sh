#!/usr/bin/env bash
# Test fixture: writes a row to stdout but exits non-zero. qql must surface
# the error and discard the row.
echo '{"key": "should_be_dropped"}'
echo "error.sh: kaboom" >&2
exit 1
