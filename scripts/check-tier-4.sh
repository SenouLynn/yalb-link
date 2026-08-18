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

# 2. ArduPilot is pinned to a release tag in both places, and to the same one.
compose_tag=$(sed -n 's/^ *ARDUPILOT_TAG: *\(.*\)$/\1/p' docker-compose.yml | tr -d '"')
image_tag=$(sed -n 's/^ARG ARDUPILOT_TAG=\(.*\)$/\1/p' docker/sitl/Dockerfile)

[[ -n "$image_tag" && "$compose_tag" == "$image_tag" ]] && r=0 || r=1
check "ArduPilot tag agrees: compose=$compose_tag image=$image_tag" "$r"

# A branch name here would make "it worked yesterday" unfalsifiable: the
# firmware under test could change with no commit in this repo.
case "$image_tag" in
  master|main|latest|stable*|*-dev) r=1 ;;
  *) r=0 ;;
esac
check "ArduPilot pin is a release tag, not a moving ref ($image_tag)" "$r"

exit "$fail"
