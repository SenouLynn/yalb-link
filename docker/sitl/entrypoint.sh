#!/bin/sh
# Launch one SITL instance directly from environment variables.
set -eu

: "${SYSID:=1}"
: "${VEHICLE_MODEL:=+}"
: "${SPEEDUP:=1}"
: "${HOME_LOCATION:=37.7749,-122.4194,10,0}"
: "${GCS_OUT:=udpclient:gcs-backend:14550}"

# ArduPilot's udpclient parser expects a numeric IPv4 address. Passing a
# Compose service name silently turns the destination into 255.255.255.255,
# which lets the backend hear SITL broadcasts but prevents a reliable return
# path for commands. Resolve the service name while Docker DNS is available.
case "${GCS_OUT}" in
  udpclient:*:*)
    endpoint=${GCS_OUT#udpclient:}
    host=${endpoint%:*}
    port=${endpoint##*:}
    resolved_host=$(getent ahostsv4 "${host}" | sed -n '1{s/[[:space:]].*//;p;}')

    if [ -z "${resolved_host}" ]; then
      echo "could not resolve SITL GCS host: ${host}" >&2
      exit 1
    fi

    GCS_OUT="udpclient:${resolved_host}:${port}"
    ;;
esac

echo "starting SITL sysid=${SYSID} model=${VEHICLE_MODEL} out=${GCS_OUT}"

# serial0 streams to the GCS; serial1 remains available to external tools.
exec ardupilot-sitl \
  --model "${VEHICLE_MODEL}" \
  --sysid "${SYSID}" \
  --speedup "${SPEEDUP}" \
  --home "${HOME_LOCATION}" \
  --defaults "/sitl/defaults.parm" \
  --serial0 "${GCS_OUT}" \
  --serial1 "tcp:0" \
  "$@"
