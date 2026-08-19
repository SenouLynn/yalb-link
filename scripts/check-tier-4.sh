#!/usr/bin/env bash
# Tier 4 gate: the properties of the container scaffolding that no compiler and
# no `docker compose config` will check.
#
# Both are version-drift checks. Neither failure mode is loud: a Dockerfile
# building with a different Go than Bazel produces a working image that behaves
# differently from CI, and a compose file naming a different ArduPilot tag than
# the image defaults to produces a vehicle nobody chose.
set -euo pipefail

fail=0

check() { # check <description> <status>
  if [[ "$2" == "0" ]]; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi
}

# 1. One Go version across go.mod, MODULE.bazel and the backend image.
#    ADR-0006: a version written in two places is a version that will disagree.
#    It is written in three, so it is checked.
gomod_go=$(awk '/^go /{print $2}' go.mod)
bazel_go=$(sed -n 's/.*go_sdk.download(version = "\([^"]*\)").*/\1/p' MODULE.bazel)
docker_go=$(sed -n 's/^ARG GO_VERSION=\(.*\)$/\1/p' Dockerfile)

[[ -n "$gomod_go" && "$gomod_go" == "$bazel_go" && "$gomod_go" == "$docker_go" ]] && r=0 || r=1
check "Go version agrees: go.mod=$gomod_go MODULE.bazel=$bazel_go Dockerfile=$docker_go" "$r"

# 2. Every ArduPilot pin is a well-formed upstream release tag.
#
# This check used to compare the compose pin against the Dockerfile default and
# assert they were equal. They were — both said `ArduCopter-4.6.0`, a tag that
# does not exist upstream, so the gate passed for three months while every
# image build failed. Agreement between two copies of the same wrong value is
# not evidence. The check is now against the upstream naming rule instead.
#
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

# We support Copter and Plane (ADR-0009 §4). A pin for a vehicle we do not
# vendor parameter metadata for would run, but its parameters would render
# unannotated with no set to select.
for t in $tags; do
  case "$t" in
    Copter-*|Plane-*) r=0 ;;
    *) r=1 ;;
  esac
  check "pinned vehicle is in the supported set, Copter or Plane ($t)" "$r"
done

# Opt-in because it needs the network; CI sets it. Offline runs still get the
# shape and supported-set checks above, which is what catches the class of bug
# that got through before.
if [[ "${CHECK_TIER_4_ONLINE:-0}" == "1" ]]; then
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
