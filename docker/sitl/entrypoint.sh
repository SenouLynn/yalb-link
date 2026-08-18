#!/bin/sh
# Launch one SITL instance from environment variables.
#
# The Tier 4 plan wrote `--out=udp:gcs-backend:14550`, which is sim_vehicle.py's
# syntax — that wrapper is a Python launcher that also starts mavproxy. This
# image runs the autopilot binary directly (no Python at runtime, no mavproxy in
# the path between the vehicle and the thing under test), so the equivalent is a
# serial port configured as a UDP client.
set -eu

: "${SYSID:=1}"
: "${VEHICLE_MODEL:=+}"
: "${SPEEDUP:=1}"
: "${HOME_LOCATION:=37.7749,-122.4194,10,0}"
: "${GCS_OUT:=udpclient:gcs-backend:14550}"

# SYSID_THISMAV cannot be a command-line flag on every release, and a parameter
# overlay works on all of them. Written per container, layered after the stock
# defaults so it wins.
printf 'SYSID_THISMAV %s\n' "${SYSID}" > /tmp/sysid.parm

echo "starting SITL sysid=${SYSID} model=${VEHICLE_MODEL} out=${GCS_OUT}"

# serial0 is the GCS link: a UDP client starts streaming immediately, where a
# TCP listener would sit waiting for someone to connect. serial1 keeps the
# conventional TCP port free for mavproxy or Mission Planner.
exec arducopter \
  --model "${VEHICLE_MODEL}" \
  --speedup "${SPEEDUP}" \
  --home "${HOME_LOCATION}" \
  --defaults "/sitl/copter.parm,/tmp/sysid.parm" \
  --serial0 "${GCS_OUT}" \
  --serial1 "tcp:0" \
  "$@"
