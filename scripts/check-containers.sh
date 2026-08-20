#!/usr/bin/env bash
# Container version checks not covered by compilers or Compose validation.
set -euo pipefail

fail=0

check() { # check <description> <status>
  if [[ "$2" == "0" ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}

# One Go version across go.mod, MODULE.bazel and the backend image.
gomod_go=$(awk '/^go /{print $2}' go.mod)
bazel_go=$(sed -n 's/.*go_sdk.download(version = "\([^"]*\)").*/\1/p' MODULE.bazel)
docker_go=$(sed -n 's/^ARG GO_VERSION=\(.*\)$/\1/p' Dockerfile)

[[ -n "$gomod_go" && "$gomod_go" == "$bazel_go" && "$gomod_go" == "$docker_go" ]] && r=0 || r=1
check "Go version agrees: go.mod=$gomod_go MODULE.bazel=$bazel_go Dockerfile=$docker_go" "$r"

# Every ArduPilot pin is a well-formed upstream release tag.
# ArduPilot tags the vehicle line, not the binary: `Copter-4.7.0`, not
# `ArduCopter-4.7.0`. The `Ardu` prefix belongs to the built artefact
# (`arducopter`) and to nothing else.
tags=$(sed -n 's/^ *ARDUPILOT_TAG: *\(.*\)$/\1/p' docker-compose.yml | tr -d '"' | sort -u)
image_tag=$(sed -n 's/^ARG ARDUPILOT_TAG=\(.*\)$/\1/p' docker/sitl/Dockerfile)

[[ -n "$tags" ]] && r=0 || r=1
check "docker-compose.yml pins at least one ARDUPILOT_TAG" "$r"

for t in $tags $image_tag; do
  if [[ "$t" =~ ^(Copter|Plane|Rover|Sub|Tracker|Blimp)-[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    r=0
  else
    r=1
  fi
  check "ArduPilot pin is a well-formed release tag ($t)" "$r"
done

# The Compose topology currently supports Copter and Plane only.
for t in $tags; do
  case "$t" in
    Copter-*|Plane-*) r=0 ;;
    *) r=1 ;;
  esac
  check "pinned vehicle is in the supported set, Copter or Plane ($t)" "$r"
done

# Upstream tag resolution is opt-in because it requires network access.
if [[ "${CHECK_CONTAINERS_ONLINE:-0}" == "1" ]]; then
  for t in $tags $image_tag; do
    if git ls-remote --tags --exit-code \
         https://github.com/ArduPilot/ardupilot.git "refs/tags/$t" >/dev/null 2>&1; then
      r=0
    else
      r=1
    fi
    check "ArduPilot tag resolves upstream ($t)" "$r"
  done
fi

exit "$fail"
