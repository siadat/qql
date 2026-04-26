#!/usr/bin/env bash
# Test fixture: mixes valid JSON rows with garbage so we can verify malformed
# lines are skipped (with a stderr warning) rather than killing the query.
echo '{"key": "first"}'
echo 'not json at all'
echo '{"key": "second"}'
