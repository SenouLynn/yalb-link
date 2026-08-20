# One pinned ArduPilot SITL vehicle per multi-stage image.

ARG UBUNTU_VERSION=22.04

FROM ubuntu:${UBUNTU_VERSION} AS build

# Release tag, waf target, and binary names use different conventions.
ARG ARDUPILOT_TAG=Copter-4.7.0

# waf target: `copter` or `plane`. The binary is always `ardu<target>`.
ARG WAF_TARGET=copter

# Stock parameter paths differ between vehicle trees and are explicit.
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

# Normalize build outputs for a vehicle-independent runtime stage.
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

# Runtime libraries for the dynamically linked SITL binary.
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

# MAVLink TCP port for external tooling.
EXPOSE 5760

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
