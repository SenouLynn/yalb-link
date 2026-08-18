# ArduPilot SITL, built from a pinned release tag.
#
# Two stages because the builder is enormous: the ArduPilot tree plus its
# submodules and the waf toolchain is several GB, and none of it is needed to
# run the resulting binary. Only the executable and the default parameters come
# forward.
#
# This build is slow (10-20 minutes cold) and is a CI-only gate. The clone and
# compile are one layer keyed on ARDUPILOT_TAG so it is rebuilt only when the
# pin moves; developers pull the built image rather than making it.

ARG UBUNTU_VERSION=22.04

FROM ubuntu:${UBUNTU_VERSION} AS build

# A release tag, never a branch. A floating branch makes "it worked yesterday"
# unfalsifiable: the vehicle firmware under test would change without a commit
# in this repo.
ARG ARDUPILOT_TAG=ArduCopter-4.6.0

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
# needs them. This is the expensive layer; ARDUPILOT_TAG is the only thing
# above it that changes.
RUN git clone --depth=1 --branch=${ARDUPILOT_TAG} \
      https://github.com/ArduPilot/ardupilot.git /ardupilot \
    && cd /ardupilot \
    && git submodule update --init --recursive --depth=1 \
    && ./waf configure --board sitl \
    && ./waf copter

FROM ubuntu:${UBUNTU_VERSION}

ENV DEBIAN_FRONTEND=noninteractive

# The SITL binary is dynamically linked against libstdc++ and libm; nothing
# else from the builder is needed.
RUN apt-get update && apt-get install -y --no-install-recommends \
      libatomic1 \
      libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 sitl

# waf emits `arducopter`, lowercase. The Tier 4 plan wrote `ArduCopter`, which
# is the vehicle name and the tag prefix, not the artefact.
COPY --from=build /ardupilot/build/sitl/bin/arducopter /usr/local/bin/arducopter
COPY --from=build /ardupilot/Tools/autotest/default_params/copter.parm /sitl/copter.parm

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER sitl
WORKDIR /home/sitl

# 5760 is SITL's own MAVLink TCP port for external tooling. The feed to the GCS
# is an outbound UDP client and needs no published port.
EXPOSE 5760

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
