# Podman vs Docker: Infrastructure Decision for yalb-gcs

**Date:** 2026-08-16  
**Status:** Research spike — feeds potential ADR revision  
**Author:** Research agent, commissioned for yalb-gcs infrastructure decision

---

## Executive Summary

Docker is the correct choice for yalb-gcs at this stage. Podman has genuine architectural advantages in security and systemd integration, but it introduces meaningful friction on macOS (the primary dev platform), has compose compatibility gaps that will bite a complex multi-container SITL stack, and its most compelling advantages (Quadlet on edge Linux) are deferred to a later tier of the roadmap. The right pattern is: **Docker Compose for dev/CI now, with a deliberate migration path to Podman Quadlets on edge Linux when that tier is reached.**

This document details the full technical basis for that recommendation.

---

## Table of Contents

1. [Architecture Differences](#1-architecture-differences)
2. [Rootless Containers on Linux](#2-rootless-containers-on-linux)
3. [Serial Port and USB Passthrough](#3-serial-port-and-usb-passthrough)
4. [Networking](#4-networking)
5. [Compose Compatibility](#5-compose-compatibility)
6. [Pod Semantics](#6-pod-semantics)
7. [Systemd Integration](#7-systemd-integration)
8. [ARM and Embedded Linux Support](#8-arm-and-embedded-linux-support)
9. [CI/CD](#9-cicd)
10. [Image Compatibility](#10-image-compatibility)
11. [macOS Developer Experience](#11-macos-developer-experience)
12. [Security Posture](#12-security-posture)
13. [Migration Path](#13-migration-path)
14. [Community and Ecosystem](#14-community-and-ecosystem)
15. [Recommendation](#15-recommendation)

---

## 1. Architecture Differences

### Docker's Daemon Model

Docker uses a client-server architecture where `dockerd` is a long-lived root daemon. The call chain is:

```
docker CLI  →  dockerd (root, persistent)  →  containerd  →  containerd-shim  →  runc/crun  →  container
```

`dockerd` owns all container lifecycle, image storage, networking, and volume management. It exposes a REST API on `/var/run/docker.sock`. The CLI is a thin HTTP client.

**Operational consequence:** the daemon is always running even when no containers are active. It consumes ~140–180 MB RAM at idle and 0.5–1.2% CPU. If the daemon crashes, all containers it owns stop receiving management signals (though running containers continue running since containerd holds them; the issue is loss of control plane).

### Podman's Daemonless Fork/Exec Model

Podman uses a fork/exec model. Each `podman run` directly invokes `conmon` (container monitor) and the OCI runtime (`crun` by default):

```
podman CLI  →  libpod  →  conmon  →  crun/runc  →  container
```

There is no central supervisor process. Each container is an independent process tree, parented to `conmon`, which in turn is reparented to systemd (PID 1) or the calling shell.

**Operational consequence:** idle Podman consumes ~45–60 MB RAM and 0.1–0.3% CPU because there is nothing running when containers are not running. However, container start time is ~15% slower (180–220 ms vs 150–180 ms) because each invocation re-initializes the library.

### What "Daemonless" Actually Means Operationally

| Property | Docker | Podman |
|---|---|---|
| Idle RAM | 140–180 MB | 45–60 MB |
| Idle CPU | 0.5–1.2% | ~0% |
| Single point of failure | Yes (dockerd crash) | No (containers outlive CLI) |
| Container start latency | 150–180 ms | 180–220 ms |
| Root requirement (default) | Yes (daemon runs as root) | No (user-space by default) |
| Socket location | `/var/run/docker.sock` | `$XDG_RUNTIME_DIR/podman/podman.sock` |
| Background container persistence | Daemon handles it | Requires systemd or `--restart` flag |

On constrained edge hardware (Raspberry Pi), the idle memory difference is real. A Pi 4 with 4 GB RAM dedicating 180 MB to a container daemon before any workload lands is non-trivial if that box is also running a GCS backend, Redis, and MAVLink relay. Podman's near-zero idle overhead is a genuine advantage at that tier.

---

## 2. Rootless Containers on Linux

### How Podman Rootless Works

Rootless Podman uses Linux **user namespaces** to map container UIDs into unprivileged UID ranges on the host. The mapping is configured in `/etc/subuid` and `/etc/subgid`:

```
# /etc/subuid
ubuntu:100000:65536
```

Inside the container, UID 0 (root) maps to UID 100000 on the host. If the container escapes, the process only has host UID 100000 privileges — an unprivileged user account.

**Kernel requirement:** user namespace support has been in mainline Linux since 4.9; practically requires 5.12+ for native overlayfs in rootless mode (below that, fuse-overlayfs fallback is used, which has ~10–15% performance overhead).

**cgroup requirement:** rootless resource limits require cgroups v2. On cgroups v1, Podman rootless can run containers but cannot enforce memory or CPU limits. All modern Debian/Ubuntu targets (22.04+, bookworm) default to cgroups v2.

### Networking for Rootless (Pasta vs Slirp4netns)

Rootless containers cannot modify iptables or create real kernel bridge interfaces. They need userspace networking.

**Slirp4netns** (legacy, removal planned in Podman 6.0):
- Translates every TCP/UDP packet into a host syscall
- Single-threaded; does not scale with parallel connections
- At 32 concurrent connections: ~28,930 req/s

**Pasta** (default since Podman 5.0):
- Reflects the host's network configuration (IP, routes, MTU) into the container namespace
- No NAT; uses L4 socket forwarding
- At low concurrency (≤8 connections): ~13,851 req/s (faster than slirp4netns)
- At high concurrency (32 connections): ~15,165 req/s (slirp4netns wins here; pasta is single-threaded pending multithread support)
- Default rootless networking backend in Podman 5.0+; RHEL 9.5+

**For MAVLink UDP specifically:** pasta's architecture maps better to low-latency single-stream UDP than slirp4netns since it avoids per-packet syscall translation. Neither is as fast as host networking. For latency-sensitive MAVLink at low parallelism (which is the yalb-gcs case — one SITL instance per compose stack), pasta's overhead should be acceptable. If hard latency budgets are needed, use `--network=host`.

### Rootless Capability Gaps vs Rootful

Rootless containers lack these Linux capabilities by default:

| Capability | Impact for yalb-gcs |
|---|---|
| `NET_ADMIN` | Cannot configure interfaces, set routes, or use `tc` for traffic shaping |
| `NET_RAW` | Cannot do raw packet capture; no `tcpdump` inside container without elevation |
| `CAP_MKNOD` | Cannot create device nodes inside container |
| `SYS_ADMIN` | Cannot mount filesystems inside container |

The `NET_RAW` gap matters for MAVLink debugging workflows where you might want to run Wireshark or tcpdump inside a SITL container. You'd need to do that on the host or switch to rootful.

### Comparison to Docker Rootless

Docker added rootless mode in v20.10 (2020), but it is not the default and requires explicit `dockerd-rootless-setuptool.sh` setup. Docker rootless has the same fundamental user namespace mechanics as Podman rootless, but with a key difference: **the daemon still runs**; it just runs as an unprivileged user. This means Docker rootless still has the single-point-of-failure daemon and the idle resource consumption, just without root privileges.

Podman's daemonless model means rootless is the natural default state — there was nothing to "bolt on." The architectural gap is real.

---

## 3. Serial Port and USB Passthrough

### The yalb-gcs Requirement

ArduPilot SITL in its hermetic container form communicates exclusively via UDP (MAVLink on port 14550 by default). The `--device` passthrough is needed for **production edge deployments** where the GCS backend talks to a real Pixhawk/CubeOrange over a serial link (`/dev/ttyUSB0`, `/dev/ttyACM0`).

### How `--device` Works

Both Docker and Podman support `--device /dev/ttyUSB0:/dev/ttyUSB0:rw`. Under the hood this adds a device node into the container's cgroup device whitelist and bind-mounts the device file.

**Rootful (both Docker and Podman):** works immediately as long as the host device exists. No special flags needed.

**Rootless Podman:** requires additional configuration:

```bash
# Step 1: Add user to the dialout group on the host
sudo usermod -aG dialout $USER

# Step 2: Run with group propagation flag (requires crun runtime, not runc)
podman run -it \
  --device /dev/ttyUSB0:/dev/ttyUSB0:rw \
  --group-add keep-groups \
  myimage

# Step 3 (SELinux systems — Fedora/RHEL only):
sudo setsebool -P container_use_devices on
# OR per-device:
sudo chcon -t container_file_t /dev/ttyUSB0
```

The `--group-add keep-groups` flag tells Podman to propagate the user's supplementary group memberships (including `dialout`) into the container. This flag requires `crun` as the OCI runtime (not the legacy `runc`). Crun is the default OCI runtime in Podman 4.0+, so this is not a problem on modern installs.

**On Debian bookworm (the primary edge target):** SELinux is not enabled by default (AppArmor is). The `chcon` step is not needed. The `--group-add keep-groups` + correct group membership is sufficient.

### Hotplug and udev

Neither Docker nor Podman handles USB hotplug within a container gracefully without additional setup. Stable device naming via udev rules is recommended for production:

```bash
# /etc/udev/rules.d/99-ardupilot.rules
SUBSYSTEM=="tty", ATTRS{idVendor}=="2341", ATTRS{idProduct}=="0042", SYMLINK+="ardulink"
```

Then use `--device /dev/ardulink` for stable references. Both runtimes handle symlinked device paths.

**Docker vs Podman on this axis:** identical. The device passthrough mechanism is the same OCI runtime mechanism (`--device` maps to `linux.devices` in the OCI spec). Rootless Podman requires the extra `--group-add keep-groups` flag; Docker rootful needs nothing extra.

---

## 4. Networking

### Rootful Networking (Both Runtimes)

In rootful mode, both Docker and Podman create a kernel bridge (`docker0` or `podman0`) and use iptables/nftables for NAT and port forwarding. Performance is equivalent — both use the kernel network stack with no userspace translation overhead.

**Docker's bridge:** `docker0` at `172.17.0.0/16` by default. Containers on the same bridge can reach each other by container name via Docker's embedded DNS.

**Podman's bridge:** `podman0` at `10.88.0.0/16` by default. Uses `netavark` (replacement for CNI since Podman 4.0) which is faster and has better nftables support. DNS is handled by `aardvark-dns`.

### Networking Modes Relevant to yalb-gcs

| Mode | MAVLink UDP latency | WebSocket latency | Use case |
|---|---|---|---|
| Bridge (rootful) | Low (kernel path) | Low (kernel path) | Default; good for SITL isolation |
| Host (`--network=host`) | Minimal (no NAT) | Minimal | Production edge; lowest latency |
| Pasta (rootless, Podman 5+) | Low-medium | Low-medium | Dev rootless; acceptable for SITL |
| Slirp4netns (rootless, legacy) | Medium | Medium | Avoid for production |
| MACVLAN | Minimal | Minimal | Production where container needs own MAC/IP |

**For SITL containers:** bridge networking is the correct choice. SITL speaks MAVLink UDP on port 14550; the bridge NAT adds negligible latency for a simulation workload. `--network=host` is simpler but leaks all ports and is inappropriate in multi-container dev stacks.

**For MAVLink UDP specifically:** UDP is connectionless, so it works correctly through bridge NAT. The concern is not connection setup but per-packet kernel forwarding overhead. For ArduPilot SITL at 50 Hz heartbeat + telemetry (well under 1 Mbit/s), bridge NAT overhead is unmeasurable against the simulation timestep.

**MACVLAN on embedded Linux:** useful when the edge GCS needs to appear on the LAN with its own IP (e.g., for TAK server discovery). Both Docker and Podman support MACVLAN rootful; rootless MACVLAN is not supported (requires `NET_ADMIN`).

### Port Below 1024 in Rootless

If any service needs to bind port 80 or 443 in a rootless container:

```bash
# Permanent sysctl fix (persists across reboot):
echo "net.ipv4.ip_unprivileged_port_start=80" | sudo tee /etc/sysctl.d/99-rootless-ports.conf
sudo sysctl -p /etc/sysctl.d/99-rootless-ports.conf
```

For yalb-gcs (ports 3000, 8080, 8081, 14550, 6379), this is not an issue — all ports are above 1024.

---

## 5. Compose Compatibility

### The Three Options

| Tool | Maintained by | Approach | Status |
|---|---|---|---|
| `docker compose` (v2) | Docker Inc | Plugin for `docker` CLI; Go rewrite of v1 | Production-ready, reference implementation |
| `podman-compose` | containers/ (community) | Translates compose spec → podman CLI calls; no API socket needed | Functional but gaps exist |
| `docker compose` + Podman socket | Docker Inc + Podman | Point `DOCKER_HOST` at Podman socket; Docker Compose v2 talks to Podman API | Best compatibility, some API gaps |

### Compatibility State (Podman 5.8, Docker Compose v2.x)

**What works identically:**
- Basic service definitions, `image:`, `ports:`, `environment:`, `volumes:`
- Named networks and volumes
- `depends_on:` (basic)
- Most health check definitions

**Known gaps in `podman-compose`:**
- `healthcheck.condition: service_healthy` — not fully implemented
- Compose profiles — partial support
- `--scale` — limited
- BuildKit-specific build arguments — not supported via API

**Known gaps when using `docker compose` against Podman socket:**
- BuildKit is not supported in Podman's Docker-compatible API; `docker buildx` builds must be done separately
- Some Compose v2 extensions have no Podman equivalent

**For the yalb-gcs compose stack** (ardupilot-sitl + redis + gcs-backend + gcs-frontend):

This is a straightforward four-service stack with no exotic Compose features. All four services use `image:`, `ports:`, `depends_on:`, and basic `healthcheck:`. **This stack will run correctly on both `docker compose` and `podman compose`** with minor adjustments (SELinux volume labels `:Z` on Fedora/RHEL; not needed on Debian).

However: **use `docker compose` (v2) as the canonical tool**. It is the reference implementation, it runs correctly against both the Docker daemon and the Podman socket, and the ecosystem (CI templates, developer tutorials, tooling) assumes it.

---

## 6. Pod Semantics

### What Podman Pods Are

Podman pods are Kubernetes-aligned groupings where containers share:
- Network namespace (same IP, can talk on `localhost`)
- IPC namespace (can use shared memory)
- Optionally, PID namespace

A pod starts with a hidden `infra` container that holds the namespaces, and user containers attach to it.

### Comparison to Docker Compose Service Groups

| Feature | Podman Pod | Docker Compose Services (same network) |
|---|---|---|
| Shared network namespace | Yes (same IP) | No (separate IPs, shared network) |
| Inter-container via `localhost` | Yes | No (use service names) |
| Kubernetes YAML import (`play kube`) | Yes | No |
| Export to K8s YAML (`generate kube`) | Yes | No (use Kompose separately) |
| Port published at pod level | Yes | No (per-container) |
| Compose file support | Via `podman-compose` | Native |

### Is the Pod Model Useful for SITL + MAVProxy + GCS Backend?

**Yes, with a caveat.** Grouping `ardupilot-sitl` + `mavproxy` into a pod means MAVProxy can reach SITL on `localhost:5760` instead of crossing the bridge network. This eliminates the NAT hop and is semantically cleaner (tight coupling between SITL and its relay is appropriate).

```
pod: sitl-pod
  ├── infra (holds namespace)
  ├── ardupilot-sitl (listens UDP 14550 on shared namespace)
  └── mavproxy (connects to localhost:14550, forwards to GCS bridge)
```

The `gcs-backend` sits outside the pod on the bridge network and connects to the MAVProxy output port.

**However:** pods are a Podman-native concept. Docker Compose achieves similar locality by putting containers on the same network — they can resolve each other by service name, which is good enough for the SITL use case. For a SITL integration test stack, Docker Compose's approach is simpler and more portable.

Pods become genuinely valuable when you want to run the same stack on Kubernetes (via `podman play kube`), which is a future concern, not a current one.

---

## 7. Systemd Integration

### Docker's Approach to Systemd on Edge Linux

Docker is managed as a system service (`systemctl start docker`). Individual containers are managed through Docker Compose or via hand-written systemd units that call `docker start`/`docker stop`. This is fragile because:

- The Docker daemon must be running for any container operation
- Restart semantics require Compose or Watchtower (third-party daemon)
- Boot ordering is achieved through `After=docker.service` in unit files, not native container ordering

A typical Docker container-as-service pattern:

```ini
[Unit]
Description=GCS Backend
Requires=docker.service
After=docker.service

[Service]
Restart=always
ExecStart=docker run --rm --name gcs-backend myimage
ExecStop=docker stop gcs-backend

[Install]
WantedBy=multi-user.target
```

This works but is a thin wrapper — it does not compose well when containers depend on each other.

### Podman Quadlet (Podman 4.4+, systemd ≥ 252)

Quadlet is the production systemd integration story for Podman. Instead of writing systemd units that call Podman commands, you write declarative `.container` files that Podman's systemd generator (`/usr/lib/systemd/system-generators/podman-system-generator`) converts to real `.service` units at boot.

**Rootful Quadlet:** place files in `/etc/containers/systemd/`  
**Rootless Quadlet:** place files in `~/.config/containers/systemd/`

Example for the GCS backend:

```ini
# ~/.config/containers/systemd/gcs-backend.container
[Unit]
Description=yalb-gcs Backend
After=redis.service
Requires=redis.service

[Container]
Image=ghcr.io/snoo/yalb-gcs-backend:latest
PublishPort=8080:8080
PublishPort=8081:8081
Environment=REDIS_URL=redis://localhost:6379
Volume=%h/.config/yalb-gcs:/config:Z
AutoUpdate=registry

[Service]
Restart=always
RestartSec=5s

[Install]
WantedBy=default.target
```

Enable and start:

```bash
systemctl --user daemon-reload
systemctl --user enable --now gcs-backend.service
sudo loginctl enable-linger $USER  # persist across logout
```

**What Quadlet gives you that Docker does not:**
- Containers are first-class systemd units with full dependency graph support
- `AutoUpdate=registry` + `podman auto-update` gives automatic image refresh (no Watchtower needed)
- Boot ordering between containers is expressed in standard `[Unit]` `After=`/`Requires=`
- Rootless containers persist across reboot without a daemon
- Pod Quadlet (`.pod` files) groups multiple container units under one pod unit

**Quadlet version requirements:**
- Quadlet landed in Podman 4.4 (September 2023)
- The full feature set (`.pod`, `.network`, `.volume`, `.kube` types) stabilized in Podman 4.6+
- Debian 13 (trixie, August 2025) ships Podman 5.4.2 which has full Quadlet support
- Debian bookworm (the current stable) ships Podman 4.3 — **Quadlet works but is an older version**; upgrade via backports or upstream repo for full features

### Docker vs Podman Systemd: Verdict

For edge device deployment (the Pi/NUC tier of yalb-gcs), Podman + Quadlet is a materially better solution than Docker + hand-written systemd units. It is native, composable, and does not require a daemon to be healthy for containers to start. The tradeoff: Quadlet has a learning curve and documentation is still maturing. This is a future concern — it becomes relevant when the edge deployment tier is implemented.

---

## 8. ARM and Embedded Linux Support

### Package Availability

| Distro | Podman version in repo | Notes |
|---|---|---|
| Debian bookworm (12) | 4.3.1 | Quadlet works; pasta networking; missing some Podman 5 features |
| Debian trixie (13) | 5.4.2 | Full Quadlet; pasta default; all Podman 5 features |
| Ubuntu 22.04 LTS | 3.4.4 | Old; use `apt.buildah.io` PPA or Debian backport |
| Ubuntu 24.04 LTS | 4.9.3 | Quadlet, pasta; Podman 5 not yet in main repo |
| Raspberry Pi OS (bookworm) | 4.3.1 | Follows Debian bookworm exactly |
| Arch Linux ARM | 5.x (rolling) | Most current |

**Recommendation for Pi 4/5 targets:** run Raspberry Pi OS Bookworm (Debian bookworm base). The shipped Podman 4.3.1 is usable but limited. For production Quadlet deployments, pin to a newer binary via the upstream Podman copr/OBS repo or compile from source. The Debian bookworm backports repository does not currently include a newer Podman build.

**Docker on ARM64:** Docker CE ARM64 packages are available via `apt.docker.com` for Debian bookworm. Version is typically Docker 27.x. Docker's ARM64 support is mature, well-tested, and the install experience is simpler than getting a current Podman on bookworm.

### Known ARM-Specific Issues

- **Podman machine on ARM64 macOS (M1/M2/M3):** the QEMU-based machine works but is slower than on x86. The Apple Virtualization Framework backend (introduced in Podman 4.5) is faster for M-series Macs.
- **fuse-overlayfs on Pi:** on kernels below 5.12, rootless Podman uses fuse-overlayfs instead of native overlayfs. Pi 4 with Raspberry Pi OS bookworm runs kernel 6.6+, so this is not an issue.
- **crun on ARM64:** crun (the preferred OCI runtime) has strong ARM64 support and is included in all distro packages listed above.

---

## 9. CI/CD

### GitHub Actions

**Docker:** GitHub-hosted Ubuntu runners come with Docker pre-installed (`docker/build-push-action`, `docker/login-action` are official actions). DIND (Docker-in-Docker) is available but requires privileged mode, which is disallowed on GitHub-hosted runners by default. The standard pattern is to use the host Docker daemon directly.

**Podman:** GitHub-hosted Ubuntu runners also come with Podman pre-installed (as of ubuntu-22.04 and ubuntu-24.04 images). The `redhat-actions/buildah-build` and `redhat-actions/push-to-registry` actions provide Podman-native CI steps. Alternatively, set `DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock` and use Docker Compose against the Podman socket.

**DIND alternative with Podman:** Podman can run inside a container without privileged mode because it does not require a daemon. This is a genuine security improvement for self-hosted runner scenarios:

```yaml
# Self-hosted runner: Podman rootless inside container (no --privileged needed)
jobs:
  build:
    runs-on: self-hosted
    container:
      image: quay.io/podman/stable
    steps:
      - run: podman build -t myimage .
```

**For yalb-gcs CI** (building Go backend + React frontend + SITL integration tests): the existing `docker compose up` pattern for integration testing works identically on GitHub-hosted runners. No change needed if Docker is chosen. If Podman is chosen, the Podman socket approach with `docker compose` (v2) against `DOCKER_HOST` is the lowest-friction path.

### GitLab CI

Podman's daemonless model shines in GitLab CI because it eliminates the DIND service requirement:

```yaml
# GitLab: Podman rootless, no privileged runner needed
build:
  image: quay.io/podman/stable
  script:
    - podman build -t $CI_REGISTRY_IMAGE .
    - podman push $CI_REGISTRY_IMAGE
```

Docker in GitLab CI either requires a privileged runner (security risk) or a DinD service container (complexity). Podman eliminates both. This is a meaningful advantage for teams running their own GitLab instance.

---

## 10. Image Compatibility

### OCI vs Docker Image Format

Both Docker and Podman produce OCI-compliant images by default since Podman 3.0 / Docker 20.10. The image format question is largely settled: **all modern images are OCI format and are interchangeable** between runtimes.

```bash
# Build with Docker, run with Podman — works
docker build -t myimage . && docker push registry.example.com/myimage
podman run registry.example.com/myimage

# Build with Podman/Buildah, run with Docker — works
podman build -t myimage . && podman push registry.example.com/myimage
docker run registry.example.com/myimage
```

### Multi-Arch Build Differences

**Docker buildx** (via BuildKit):
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --push \
  -t registry.example.com/yalb-gcs:latest .
```

Single command, creates a manifest list automatically.

**Podman** (native, Podman 3.4+):
```bash
podman build --manifest myimage:latest \
  --platform linux/amd64,linux/arm64 .
podman manifest push myimage:latest registry.example.com/yalb-gcs:latest
```

**Known Podman multi-arch issue (as of 2026):** Podman has a documented bug ([#27211](https://github.com/containers/podman/issues/27211)) where ambiguous multi-arch build output can silently create a single-arch image instead of a manifest list. Always verify with `podman manifest inspect` after build.

**For yalb-gcs:** building `linux/amd64` (cloud/CI) and `linux/arm64` (Pi/NUC edge) images. The `docker buildx` workflow is more mature and has no known manifest bugs. Use `docker buildx` in CI regardless of which runtime is used at deploy time — images are OCI format and runtime-agnostic.

---

## 11. macOS Developer Experience

### The Core Problem: Linux Containers on macOS

Both Docker and Podman require a Linux VM on macOS because containers use Linux kernel features. The VM is unavoidable. The difference is in how the VM is managed and how it performs.

### Docker Desktop

- **VM backend:** LinuxKit VM (originally) → Apple Virtualization Framework (HVF) on M-series Macs
- **Volume performance:** ~50–70% of native macOS filesystem speed via gRPC-FUSE
- **Idle RAM:** 1.5–3 GB (VM + daemon)
- **Startup time:** 20–60 seconds
- **Licensing:** Free for individuals; **paid for companies with >250 employees or >$10M revenue** (~$50K–$120K/year for large teams)
- **Compose support:** first-class (`docker compose` is bundled)
- **Extensions ecosystem:** mature (Portainer, Lens, etc.)
- **Dev experience:** polished GUI, automatic file sharing, volume bind mounts work transparently

### Podman Desktop / Podman Machine

- **VM backend:** QEMU (legacy) or Apple Virtualization Framework (Podman 4.5+ on M-series)
- **Volume performance:** comparable to Docker Desktop (~50–70% native)
- **Idle RAM:** ~500 MB
- **Startup time:** faster than Docker Desktop with AVF backend; QEMU backend is slower
- **Licensing:** free (Apache 2.0)
- **Compose support:** via `podman-compose` or `docker compose` + `DOCKER_HOST` env var
- **Dev experience:** improved through 2025 but trails Docker Desktop on UI polish; fewer extensions

### OrbStack (Honorable Mention)

Not Docker or Podman but worth knowing: OrbStack uses a custom virtiofs implementation and achieves 80–90% native filesystem speed with ~300 MB idle RAM and <1 second startup. It supports both Docker and Kubernetes workloads. **Not open source but free for personal use.** For macOS-heavy teams, OrbStack is often the best developer experience regardless of Docker vs Podman decision. It exposes a Docker-compatible socket so existing tooling works unchanged.

### Volume Mount Gotchas on macOS (Podman)

Podman Desktop on macOS has historically had volume mount edge cases in complex compose stacks — particularly around file watch (hot reload), networking health checks, and SELinux labels from Linux-targeted compose files being applied on macOS where they are irrelevant but can cause warnings. These are cosmetic issues, not blocking ones.

**The most common macOS + Podman friction for yalb-gcs:**

1. `DOCKER_HOST` must be set for `docker compose` to use Podman socket: `export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')`
2. Build images that touch `/dev/ttyUSB0` — the device does not exist in the macOS VM; SITL stack tests serial passthrough only on Linux targets
3. The default Podman machine may have a small disk quota; SITL images (Ubuntu-based, ~3 GB) can hit this

---

## 12. Security Posture

### Attack Surface Comparison

**Docker's threat model:** the `dockerd` root daemon is the crown jewel. If any process on the host can write to `/var/run/docker.sock`, it has effective root on the host. Docker's socket permissions (group `docker`, mode 0660) mean any user in the `docker` group has root-equivalent access. This is a well-known vector:

```bash
# Classic Docker socket escape:
docker run -v /:/host --rm -it alpine chroot /host sh
```

**Podman's threat model:** no daemon, no shared socket. A compromised container in rootless mode can only do what the host user can do (UID 100000, no special privileges). A compromised rootful Podman container has the same risk profile as a compromised rootful Docker container — full root on host.

The security gap is specifically **rootless Podman vs rootful Docker**. Rootful both is roughly equivalent.

### Default Capabilities

| Runtime | Default capabilities granted to container |
|---|---|
| Docker (rootful) | ~14 capabilities (includes `NET_BROADCAST`, `SYS_CHROOT`, others) |
| Podman (rootless) | ~11 capabilities (subset; no `NET_ADMIN`, `NET_RAW`, `SYS_ADMIN`) |
| Podman (rootful) | ~14 capabilities (same as Docker rootful) |

The difference of 3 capabilities reflects deliberate principle-of-least-privilege design in Podman.

### SELinux / AppArmor

**Podman on RHEL/Fedora:** automatic MCS (Multi-Category Security) labeling per container. Each container gets a unique `s0:c123,c456` label; without explicit policy allows, containers cannot access each other's resources or the host. Volume mounts require `:Z` (private) or `:z` (shared) SELinux relabeling.

**Podman on Debian/Ubuntu:** AppArmor is the LSM. Podman ships a default AppArmor profile (`containers-default`) that blocks mount, ptrace of non-children, and raw network sockets. Less mature than the SELinux integration but functional.

**Docker on Debian/Ubuntu:** also uses AppArmor (`docker-default` profile). Comparable to Podman's profile in practice.

### Notable CVEs

| CVE | Runtime affected | Severity | Description |
|---|---|---|---|
| CVE-2019-5736 | Docker (runc) | Critical | runc overwrite via `/proc/self/exe`; fixed runc 1.0-rc6 |
| CVE-2022-0492 | Both (kernel) | High | cgroups v1 escape via `release_agent`; mitigated by seccomp/AppArmor/rootless |
| CVE-2024-21626 | Both (runc) | High | "Leaky Vessels" — fd leak enabling host FS access; fixed runc 1.1.12, Docker 25.0.2 |
| CVE-2023-2602/3 | Podman (libcap) | Medium | User namespace privilege escalation in older libcap versions |

**Pattern:** high-severity container escape CVEs affect both runtimes at the OCI runtime layer (runc, which both use). Podman's rootless mode provides meaningful additional mitigation because even a successful runc escape lands in an unprivileged user namespace. Docker rootful's escapes land with full root.

### "Closer to Bare Metal" in Practice

Podman's daemonless model means containers are regular Linux processes in `ps` output. There is no abstraction layer to confuse audit tools. `strace`, `perf`, and `audit` work naturally against Podman-managed containers. This matters for security auditing on edge devices where you want direct visibility into what is running.

---

## 13. Migration Path

### Docker → Podman

**Effort estimate: 2–4 engineer-days for the yalb-gcs stack**

| Migration task | Effort | Notes |
|---|---|---|
| Install `podman-docker` shim (provides `docker` alias) | Trivial | Immediate CLI compatibility |
| Set `DOCKER_HOST` in shell profiles and CI | Trivial | Points tools at Podman socket |
| Audit compose file for incompatibilities | Low | Health check conditions, profiles |
| Add `:Z` labels to volume mounts (if on SELinux system) | Low | Not needed on Debian/Ubuntu |
| Update CI pipeline to use Podman socket or native Podman | Low–Medium | Runner image swap or env var |
| Replace `docker buildx` in CI with Podman equivalent | Medium | Manifest creation workflow differs |
| Convert key compose services to Quadlet for edge deployment | Medium | New file format; one-time effort |
| Test serial device passthrough with `--group-add keep-groups` | Low | One flag; straightforward |

**Compose file portability:** the yalb-gcs compose file as described in ADR-0002 will run under Podman with `podman compose` or `docker compose` + Podman socket with zero changes on Debian/Ubuntu. The only change needed for RHEL/Fedora targets is adding `:Z` to volume mounts.

**What will break initially:**
- `docker buildx` commands — need Podman-native manifest workflow or use Buildah
- Portainer, ctop, Lazydocker and other tools that open `/var/run/docker.sock` — need `DOCKER_HOST` or socket symlink
- `VS Code Dev Containers` extension — configure `DOCKER_HOST` in VS Code settings

### Podman → Docker (reverse migration)

**Effort estimate: 1–2 engineer-days**

Less painful because Docker is the superset in tooling terms. The primary losses:
- Quadlet unit files have no Docker equivalent — must be rewritten as Docker systemd wrappers
- Podman pods have no Docker equivalent — containers must be regrouped via Compose networks
- `podman generate kube` / `podman play kube` — no Docker equivalent; use Kompose

### "Can we run both side-by-side?"

Yes. Docker and Podman can coexist on the same Linux host. They use different socket paths, different storage roots (`/var/lib/docker` vs `~/.local/share/containers`), and different default bridge networks. There is no conflict.

---

## 14. Community and Ecosystem

### Adoption and Tooling

**Docker's ecosystem position (2026):**
- ~59% developer adoption (Stack Overflow 2024 survey)
- First-class support in: AWS ECS, Azure Container Instances, GCP Cloud Run, GitHub Actions, Jenkins, CircleCI, every cloud provider
- Native Docker socket assumed by: Portainer, Watchtower, Traefik, Lens, k9s, ctop, Lazydocker, VS Code Dev Containers, Testcontainers
- Docker Compose v2 is the reference implementation for the Compose Specification

**Podman's ecosystem position (2026):**
- Default container engine in RHEL 8+, CentOS Stream, Fedora, Rocky Linux, AlmaLinux
- Growing in government and defense (rootless security requirement)
- Native support in: OpenShift, Tekton, some GitLab CI templates
- Approximately 90–95% of Docker tooling works with Podman via socket compatibility, with minor tweaks
- `podman-docker` package provides full CLI shim

### Go SDK Compatibility

The yalb-gcs backend is written in Go and may need to programmatically manage containers (e.g., spawning SITL instances, health checks). The Go SDK options:

**Docker Go SDK** (`github.com/docker/docker/client`):
- Official, mature, large community
- Can point at Podman socket via `DOCKER_HOST`: `client.NewClientWithOpts(client.WithHost("unix:///run/user/1000/podman/podman.sock"), client.WithAPIVersionNegotiation())`
- Full API coverage for container lifecycle, image management, networking

**Podman Go bindings** (`github.com/containers/podman/v5/pkg/bindings`):
- Native Podman API; supports pod operations, Quadlet management
- Smaller community; fewer examples
- Required for pod-specific operations that have no Docker equivalent

**Recommendation:** use the Docker SDK pointed at the Podman socket if Podman is chosen. This keeps the Go code portable between runtimes. Only reach for the Podman bindings if pod-native operations are required.

### Kubernetes Parity

Podman's `podman play kube` and `podman generate kube` provide a path from Podman to Kubernetes:

```bash
# Export running stack to K8s YAML
podman generate kube sitl-pod > sitl-deployment.yaml

# Import K8s YAML locally
podman play kube sitl-deployment.yaml
```

This is genuinely useful for teams that want to prototype locally and deploy to K8s. Docker has no equivalent (Kompose is a separate tool with limited fidelity).

For yalb-gcs's current scope (single-host deployments), this is a future-value argument, not an immediate one.

---

## 15. Recommendation

### The Direct Answer

**Use Docker for yalb-gcs now.** Revisit Podman Quadlets when the edge deployment tier becomes active.

### Rationale

The yalb-gcs project is in **early infrastructure validation phase**, per the roadmap. The active developer platform is **macOS**. The immediate need is a hermetic SITL stack that runs identically in local dev (`docker compose up`) and CI (GitHub Actions). The edge Linux deployment tier (Quadlet, Systemd, boot-time start) is multiple tiers away.

**Why Docker wins for the current phase:**

1. **macOS is the dev platform.** Docker Desktop (or OrbStack with Docker socket) provides the most polished volume mount behavior, the most transparent compose experience, and zero socket-configuration overhead. Podman Machine on macOS has rough edges in complex compose stacks.

2. **The SITL compose stack is standard.** Four services, straightforward networking, no exotic Compose features. `docker compose up` is the hermetic test entry point from ADR-0002. This should be boring infrastructure that works first time.

3. **Ecosystem friction is real.** VS Code Dev Containers, the Docker SDK in Go, `docker buildx` for multi-arch, Testcontainers (if used in integration tests) — all assume Docker. The 90–95% Podman compatibility estimate is correct, but the remaining 5–10% is the kind of friction that derails early momentum.

4. **Docker CE is free on Linux edge devices.** The Docker Desktop license applies to the desktop GUI application, not Docker Engine. `apt install docker-ce` on the Raspberry Pi is free, perpetually, regardless of company size.

5. **Security advantage of Podman rootless is real but premature.** The yalb-gcs GCS is not yet a multi-tenant system with untrusted workloads. The daemon attack surface is a real concern, but it is manageable (restrict the `docker` group, use rootful only in controlled contexts) for a single-operator edge device.

**Why Podman becomes the right answer for edge Linux (future):**

1. **Quadlet is the correct model for a systemd-native edge device.** When the Pi/NUC deployment tier is built, running containers as first-class systemd units (with proper `After=`, `Restart=`, `WantedBy=`) is architecturally cleaner than Docker's wrapper approach.

2. **Idle overhead matters on a Pi.** The 140 MB daemon savings is meaningful at the edge tier where the GCS backend, Redis, and MAVLink relay are all competing for memory.

3. **Rootless is the right security posture for a shipped product.** When yalb-gcs is deployed to customer/operator edge boxes that are not exclusively controlled environments, a compromised container that lands in an unprivileged user namespace is categorically better than one that lands with root.

### Decision Tree

```
Are you working on the SITL integration or core pipeline tiers?
    YES → Docker Compose. Full stop.

Are you setting up a developer machine on macOS?
    YES → Docker Desktop (or OrbStack with Docker socket).

Are you deploying to a cloud Linux box (Debian/Ubuntu)?
    YES → Docker CE. Consider Docker Compose for service orchestration.

Are you implementing the edge device deployment tier (Pi/NUC)?
    YES → Podman + Quadlet on Debian trixie (Podman 5.4.2+).
           Use Docker for local dev; Quadlets for production systemd integration.
           Serial passthrough: add user to dialout group; use --group-add keep-groups.

Are you setting up CI/CD (GitHub Actions)?
    YES → Use host Docker daemon (pre-installed on ubuntu-24.04 runners).
           Docker Compose for integration test startup.
           docker buildx for multi-arch image builds (amd64 + arm64).

Are you being asked to switch from Docker to Podman wholesale?
    WAIT → Until the edge deployment tier is in scope.
           Migration is 2-4 days, not months. It can be done surgically.
```

### Concrete Starting Configuration (Docker)

For the yalb-gcs stack as described in ADR-0002:

```yaml
# docker-compose.yml (reference — already in ADR-0002)
services:
  ardupilot-sitl:
    image: ghcr.io/snoo/ardupilot-sitl:latest
    ports:
      - "14550:14550/udp"
      - "5760:5760/tcp"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  gcs-backend:
    image: ghcr.io/snoo/yalb-gcs-backend:latest
    ports:
      - "8080:8080"
      - "8081:8081"
    depends_on:
      redis:
        condition: service_healthy
      ardupilot-sitl:
        condition: service_started
    environment:
      - SITL_HOST=ardupilot-sitl
      - SITL_PORT=14550
      - REDIS_URL=redis://redis:6379

  gcs-frontend:
    image: ghcr.io/snoo/yalb-gcs-frontend:latest
    ports:
      - "3000:3000"
    depends_on:
      - gcs-backend
```

This works on Docker (macOS, Linux) and on Podman (via `docker compose` + `DOCKER_HOST` or `podman compose`) with zero changes on Debian/Ubuntu. When the edge tier is built, the same images are deployed via Quadlet unit files rather than this compose file.

### When to Migrate

**Trigger condition:** the tier-5 or tier-6 work begins (transport live / service connect) and the team starts testing on actual edge hardware.

**Migration scope:** write Quadlet `.container` files for the edge-deployed services (gcs-backend, redis, mavlink-relay). Keep `docker-compose.yml` for dev and CI — it continues to be the SITL hermetic test harness regardless of what runs in production.

The key insight: **these are not mutually exclusive**. Docker Compose for dev/CI and Podman Quadlets for edge production is a valid, coherent architecture. The OCI image format ensures the same artifacts work in both contexts.

---

## Appendix: Quick Reference Commands

### Docker

```bash
# Start SITL stack
docker compose up -d

# Build multi-arch image
docker buildx build --platform linux/amd64,linux/arm64 --push -t ghcr.io/snoo/yalb-gcs-backend:latest ./gcs

# Serial passthrough (rootful)
docker run --device /dev/ttyUSB0:/dev/ttyUSB0:rw ...
```

### Podman (when you get there)

```bash
# Use Docker Compose against Podman socket (macOS/Linux dev)
export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')
docker compose up -d

# Serial passthrough (rootless)
podman run --device /dev/ttyUSB0:/dev/ttyUSB0:rw --group-add keep-groups ...

# Quadlet: deploy at boot (edge Linux)
mkdir -p ~/.config/containers/systemd/
# write .container files
systemctl --user daemon-reload
sudo loginctl enable-linger $USER
systemctl --user enable --now gcs-backend.service

# Check port binding for unprivileged (rootless):
sudo sysctl -w net.ipv4.ip_unprivileged_port_start=1024

# Build multi-arch (verify carefully)
podman build --manifest yalb-gcs:latest --platform linux/amd64,linux/arm64 .
podman manifest inspect yalb-gcs:latest  # verify both arches present
podman manifest push yalb-gcs:latest ghcr.io/snoo/yalb-gcs-backend:latest
```

---

*Sources consulted: Podman project documentation (docs.podman.io), Red Hat engineering blogs, Podman GitHub issue tracker, Uptrace benchmark analysis, NVISO security blog, eriksjolund podman-networking-docs, oneuptime.com Podman guides, pkgpulse container tool comparison, Podman Desktop 2025 journey report.*
