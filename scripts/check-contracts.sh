#!/usr/bin/env bash
# Contract properties that buf lint cannot express.
set -euo pipefail

PROTO_DIR="${1:-proto/gcs/v1}"
fail=0

check() { # check <description> <condition-already-evaluated>
  if [[ "$2" == "0" ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}

# Telemetry payload messages carry no vehicle_id — identity lives on the envelope.
count=$(grep -c 'VehicleId vehicle_id' "$PROTO_DIR/telemetry.proto")
[[ "$count" == "1" ]] && r=0 || r=1
check "telemetry.proto declares vehicle_id once, on TelemetryEvent (found $count)" "$r"

# A mission snapshot is addressed by full identity. Components sharing a system
# ID hold separate missions, so a bare system_id here would let one component's
# download be attributed to another.
count=$(grep -c 'VehicleId vehicle_id' "$PROTO_DIR/missions.proto")
[[ "$count" == "1" ]] && r=0 || r=1
check "missions.proto identifies a snapshot by VehicleId, not system_id (found $count)" "$r"

# go_package matches the Go module path so generated imports resolve.
module=$(awk '/^module /{print $2}' go.mod)
packages=$(grep -h 'go_package' "$PROTO_DIR"/*.proto | sort -u)
[[ "$packages" == "option go_package = \"$module/internal/gen/gcs/v1;gcsv1\";" ]] && r=0 || r=1
check "go_package matches go.mod module ($module)" "$r"

exit "$fail"
