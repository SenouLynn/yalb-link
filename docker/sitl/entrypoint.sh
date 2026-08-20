#!/bin/sh
# Launch one SITL instance directly from environment variables.
set -eu

: "${SYSID:=1}"
: "${VEHICLE_MODEL:=+}"
: "${SPEEDUP:=1}"
: "${HOME_LOCATION:=37.7749,-122.4194,10,0}"
: "${GCS_OUT:=udpclient:gcs-backend:14550}"

# Apply the per-container system ID after stock parameters.
printf 'SYSID_THISMAV %s\n' "${SYSID}" > /tmp/sysid.parm

echo "starting SITL sysid=${SYSID} model=${VEHICLE_MODEL} out=${GCS_OUT}"

# serial0 streams to the GCS; serial1 remains available to external tools.
exec ardupilot-sitl \
  --model "${VEHICLE_MODEL}" \
  --speedup "${SPEEDUP}" \
  --home "${HOME_LOCATION}" \
  --defaults "/sitl/defaults.parm,/tmp/sysid.parm" \
  --serial0 "${GCS_OUT}" \
  --serial1 "tcp:0" \
  "$@"
