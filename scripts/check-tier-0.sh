#!/usr/bin/env bash
# Tier 0 gate: contract properties that buf lint cannot express.
set -euo pipefail

PROTO_DIR="${1:-proto/gcs/v1}"
fail=0

check() { # check <description> <condition-already-evaluated>
  if [[ "$2" == "0" ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}

# 1. SetArmedRequest carries no force field.
grep -A5 'message SetArmedRequest' "$PROTO_DIR/commands.proto" | grep -q 'force' && r=1 || r=0
check "SetArmedRequest has no force field" "$r"

# 2. No client-streaming or bidi RPCs — connect-web cannot call them from a browser.
grep -nE 'rpc [A-Za-z]+\(stream ' "$PROTO_DIR/services.proto" && r=1 || r=0
check "no client-streaming or bidi RPCs" "$r"

# 3. Every RPC declares a required_role. Absent means deny, so an unannotated
#    RPC is a dead endpoint rather than an open one — catch it here.
rpcs=$(grep -cE '^\s*rpc [A-Za-z]+\(' "$PROTO_DIR/services.proto")
roles=$(grep -c 'option (gcs.v1.required_role)' "$PROTO_DIR/services.proto")
[[ "$rpcs" == "$roles" ]] && r=0 || r=1
check "all $rpcs RPCs declare required_role (found $roles)" "$r"

# 4. Telemetry payload messages carry no vehicle_id — identity lives on the envelope.
count=$(grep -c 'VehicleId vehicle_id' "$PROTO_DIR/telemetry.proto")
[[ "$count" == "1" ]] && r=0 || r=1
check "telemetry.proto declares vehicle_id once, on TelemetryEvent (found $count)" "$r"

# 5. go_package matches the Go module path so generated imports resolve.
module=$(awk '/^module /{print $2}' go.mod)
grep -h 'go_package' "$PROTO_DIR"/*.proto | sort -u | grep -q "\"$module/internal/gen/gcs/v1;gcsv1\"" && r=0 || r=1
check "go_package matches go.mod module ($module)" "$r"

exit "$fail"
