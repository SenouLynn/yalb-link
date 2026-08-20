#!/usr/bin/env bash
# Every implementation row in the capability matrix must be complete.
set -euo pipefail

MATRIX="${1:-docs/reference/codec-capability-matrix.md}"

if [[ ! -f "$MATRIX" ]]; then
  echo "FAIL: capability matrix not found at $MATRIX"
  exit 1
fi

# Match status cells rather than prose.
if grep -nE '\|[[:space:]]*(unstarted|pending|blocked)[[:space:]]*\|' "$MATRIX"; then
  echo "FAIL: capability matrix has incomplete rows (listed above)"
  exit 1
fi

echo "OK: all implementation rows are complete in $MATRIX"
