# ArduPilot SITL, built from a pinned release tag, for one vehicle type.
#
# Two stages because the builder is enormous: the ArduPilot tree plus its
# submodules and the waf toolchain is several GB, and none of it is needed to
# run the resulting binary. Only the executable and the default parameters come
# forward.
#
# One image per vehicle type, not one image with both. Copter and Plane are
# pinned to *different* firmware lines on purpose (see ADR-0009 §4), so they
# cannot share a clone, and building both in one image would double the
# expensive layer to produce a container that only ever runs one of them.
#
# This build is slow (10-20 minutes cold) and is a CI-only gate. The clone and
# compile are one layer keyed on ARDUPILOT_TAG so it is rebuilt only when the
# pin moves; developers pull the built image rather than making it.

ARG UBUNTU_VERSION=22.04

FROM ubuntu:${UBUNTU_VERSION} AS build

# A release tag, never a branch. A floating branch makes "it worked yesterday"
# unfalsifiable: the vehicle firmware under test would change without a commit
# in this repo.
#
# The tag prefix is the *vehicle* name and does not match the waf target or the
# binary: the Copter line is tagged `Copter-4.7.0` (not `ArduCopter-4.7.0`,
# which does not exist upstream and is what this file pinned until 2026-08-19),
# built with `./waf copter`, emitting `arducopter`.
ARG ARDUPILOT_TAG=Copter-4.7.0

# waf target: `copter` or `plane`. The binary is always `ardu<target>`.
ARG WAF_TARGET=copter

# In-tree path to the frame's stock parameters. NOT derivable from WAF_TARGET —
# Copter's live in `Tools/autotest/default_params/copter.parm` while Plane's
# live in `Tools/autotest/models/plane.parm`, and there is no
# `default_params/plane.parm` at any tag. Passed explicitly so the asymmetry is
# visible in docker-compose.yml rather than hidden in string interpolation.
ARG DEFAULT_PARAMS_PATH=Tools/autotest/default_params/copter.parm

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      g++ \
      gcc \
      git \
      libtool \
      make \
      pkg-config \
      python3 \
      python3-dev \
      python3-pip \
      python3-setuptools \
      rsync \
    && rm -rf /var/lib/apt/lists/*

RUN pip3 install --no-cache-dir empy==3.3.4 pexpect future

# --depth=1 on the tag, and --recursive on submodules because the SITL build
# needs them. This is the expensive layer; the ARGs above are the only things
# that change it.
#
# The artefacts are normalised into /out under fixed names so the runtime stage
# and the entrypoint carry no vehicle-specific branching at all.
RUN git clone --depth=1 --branch=${ARDUPILOT_TAG} \
      https://github.com/ArduPilot/ardupilot.git /ardupilot \
    && cd /ardupilot \
    && git submodule update --init --recursive --depth=1 \
    && ./waf configure --board sitl \
    && ./waf ${WAF_TARGET} \
    && mkdir -p /out \
    && cp /ardupilot/build/sitl/bin/ardu${WAF_TARGET} /out/ardupilot-sitl \
    && cp /ardupilot/${DEFAULT_PARAMS_PATH} /out/defaults.parm

FROM ubuntu:${UBUNTU_VERSION}

ENV DEBIAN_FRONTEND=noninteractive

# The SITL binary is dynamically linked against libstdc++ and libm; nothing
# else from the builder is needed.
RUN apt-get update && apt-get install -y --no-install-recommends \
      libatomic1 \
      libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 sitl

COPY --from=build /out/ardupilot-sitl /usr/local/bin/ardupilot-sitl
COPY --from=build /out/defaults.parm /sitl/defaults.parm

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER sitl
WORKDIR /home/sitl

# 5760 is SITL's own MAVLink TCP port for external tooling. The feed to the GCS
# is an outbound UDP client and needs no published port.
EXPOSE 5760

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
