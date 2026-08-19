#!/usr/bin/env bash
# Tier 2 gate: no capability-matrix row may be left unstarted.
#
# The obvious one-liner is wrong twice, so it is written out here instead:
#   grep -q unstarted FILE && (echo ERROR && exit 1) || true
# The trailing `|| true` swallows the subshell's exit 1, so make always sees 0 —
# the gate reports failure and passes anyway. And a bare `unstarted` also matches
# the document's own status legend, so it fires whether or not any row is
# unstarted. Match table cells, and let the exit status be the result.
set -euo pipefail

MATRIX="${1:-docs/reference/codec-capability-matrix.md}"

if [[ ! -f "$MATRIX" ]]; then
  echo "FAIL: capability matrix not found at $MATRIX"
  exit 1
fi

# Status is a table cell: "| unstarted |". The legend line is prose and uses
# backticks, so it does not match.
if grep -nE '\|[[:space:]]*unstarted[[:space:]]*\|' "$MATRIX"; then
  echo "FAIL: capability matrix has unstarted rows (listed above)"
  exit 1
fi

echo "OK: no unstarted rows in $MATRIX"
