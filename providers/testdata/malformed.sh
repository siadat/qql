#!/usr/bin/env bash
# Test fixture: mixes valid envelope-shaped rows with garbage so we can verify
# malformed lines are skipped (with a stderr warning) rather than killing the
# query.
echo '{"type": "row", "value": {"key": "first"}}'
echo 'not json at all'
echo '{"type": "row", "value": {"key": "second"}}'
