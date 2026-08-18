# GCS Reference Architecture Analysis: Cockpit (Blue Robotics / BlueOS)

## Purpose

`qgc-missionplanner-analysis.md` surveyed the two mature desktop GCS implementations
before ADR-0002 fixed our stack. Both are desktop-native — Qt/C++ and WinForms/C# — and
neither answers the questions in front of us now, because we are building a *web* GCS
and they are not.

Cockpit is the missing third reference: a Vue 3 web GCS speaking MAVLink to ArduPilot,
shipping WebRTC video, a centralized "data lake" telemetry store, and a settings system
that syncs to the vehicle. It is the closest existing thing to what we are building, and
the only one of the three that has already met — or failed to meet — the browser-specific
problems waiting in tiers 5–10.

Three questions prompted this survey: what a real configuration layer looks like, whether
we need a telemetry store beyond what the roadmap already commits to, and where the port
boundary belongs for video.

> **Name collision.** This is `bluerobotics/cockpit`. It is unrelated to
> `cockpit-project/cockpit`, Red Hat's Linux server admin console. Same name, different
> universe. Search accordingly.

> **License.** Cockpit is **AGPL-3.0** (dual-licensed; a commercial license exists).
> Patterns may be derived; code may not be lifted. This is the same posture the QGC doc
> takes, but AGPL's network-use clause makes it sharper for software we intend to run as
> a server. We currently ship no LICENSE file of our own — noted as a finding, out of
> scope here.

---

## Cockpit

**Stack:** Vue 3 + TypeScript (Composition API), Vite, Pinia, Tailwind + Vuetify 3,
Electron, WebSocket + WebRTC, MAVLink via `mavlink2rest-wasm`. AGPL-3.0.

**Shape:** a browser application with no backend of its own. It talks to BlueOS services
(`mavlink-camera-manager`, vehicle storage) over the network and keeps everything else —
domain model, telemetry history, settings — in the browser. Almost every design decision
below follows from that one constraint, and that is the lens to read them through: we
have a Go backend, so the constraint does not transfer even where the pattern does.

### Configuration — the pattern worth lifting

`src/libs/settings-management.ts`. The model:

- Keys are a three-tier hierarchy: `Record<userId, Record<vehicleId, SettingsPackage>>`.
  **Settings are scoped per-user *and* per-vehicle.** This is the insight worth taking.
- Every value carries `epochLastChangedLocally` beside the value itself. Settings are not
  bare values; they are values with provenance.
- `StorageAdapter` and `VehicleAdapter` interfaces abstract local persistence
  (localStorage) from remote (vehicle storage at `settings/{userId}/{key}`). Ports and
  adapters, consistent with ADR-0003.
- Sync pipeline when a vehicle comes online: backup → migrate → import → merge-by-epoch →
  push, with a 100 ms debounce on outbound updates.
- Conflict resolution: newer epoch wins, and **on a tie the vehicle wins**. That tiebreak
  is what makes the merge deterministic when two clients are connected at once.
- Migration: old flat-format settings are detected and wrapped at epoch zero, behind a
  fallback chain that degrades through known-good user/vehicle combinations before
  reconstructing from backup.

**Verdict.** The shape is right and directly applicable to us — per-user/per-vehicle
scoping, last-write-wins with a deterministic tiebreak, adapters over storage backends.
The mechanism is not: this is browser localStorage doing a distributed-systems job,
untestable without a browser. We have Go and Redis; the same model belongs server-side.

For contrast, our configuration today is `os.LookupEnv` inline in `cmd/gcs/main.go`
(`codec.EnvUDPBind`, `codec.EnvSigningKey`). There is no config layer at all. Cockpit is
a useful picture of what one eventually has to handle — but note that *none* of the hard
parts above are about reading config. They are about reconciling it across nodes.

### Profile / View / Widget — layout as versioned data

`src/stores/widgetManager.ts`. A `Profile` contains `View`s; a `View` contains `Widget`s
with `Point2D` position, `SizeRect2D` size, a `WidgetType`, and a `persistentInternalState`
bag. Storage keys are explicitly versioned (`cockpit-views-group-v1`,
`cockpit-mini-widgets-profile-v4`), legacy layouts are migrated on boot, and both profiles
and individual views import/export as JSON files validated by `validateProfile()` /
`validateView()` before being accepted.

This is the same problem as tier-10 UI composition, already solved once. The parts worth
copying: layout is serializable data rather than code, the storage key carries a schema
version, and import validates before it trusts.

### Standardized builds

One `package.json` drives four distribution channels: Vite for web,
`electron-builder` for desktop (`deploy:electron:mac:{x64,arm64}`, `:windows` via NSIS,
`deploy:flatpak`), a Docker image (`docker run -p 8080:8080 bluerobotics/cockpit:latest`),
and a BlueOS-extension packaging of the same artifact. The packaging matrix is genuinely
more distribution surface than we have, and the discipline of one build graph feeding all
of it is correct.

**But it is less hermetic than what ADR-0001 already buys us.** Cockpit's `postinstall`
downloads ffmpeg, go2rtc, and Piper binaries from the network at install time — an
unpinned network fetch in the build path, which is the specific failure Bazel exists to
prevent. Credit the multi-target matrix; do not copy the postinstall. When we eventually
take binary dependencies, they belong in `MODULE.bazel` with hashes.

### Data lake — why they need it

`src/libs/actions/data-lake.ts`. Four parallel in-memory maps keyed by variable ID:
metadata, values, timestamps, listeners. Values are `string | number | boolean | undefined`.
Variables are registered with `createDataLakeVariable()`; subscribers call
`listenDataLakeVariable()` and get a UUID back for teardown, with a `notifyOnTimestampChange`
option so a value that repeats still ticks its listeners. Persistence is opt-in per
variable (`persistent`, `persistValue`) into localStorage.

**Why it exists.** Cockpit has no backend and its widgets are user-composed, so any widget
must bind to any telemetry field with no compile-time contract available. The data lake is
a late-bound, string-keyed pub/sub bus, and it is what makes drag-and-drop widget binding
possible at all. It buys real capability: expression inputs computed over variables
(`data-lake-expression-input.ts`), live plotting, and one uniform target for logging.

**What it costs.** It is a global mutable singleton with untyped values and string keys.
Our own QGC analysis already indicts exactly these two failure modes — "Singletons
everywhere" and "Mutable signal-driven state … no audit trail, no replay, hard to debug
transitions." Cockpit reproduces both, in a newer language. This is what happens when the
UI layer has to invent a domain model because there is not one underneath it.

**Our position.** We have the opposite structure already: `.proto` contracts are the
schema, `internal/vehicle/state.go` is an immutable snapshot replaced atomically by a pure
fold (`internal/vehicle/fold.go`), and Connect server streams fan out typed events. We get
the data lake's actual benefit — one consistent store that any component subscribes to
selectively — from the React Query cache fed by typed streams, per ADR-0002 and ADR-0003.

**We do not need a data lake. We need the one thing it has that we lack: late binding** —
so an operator can point a panel at a telemetry field chosen at runtime rather than at
compile time. That is schema reflection over protos we already generate, not a new storage
layer. Framing it as "we need a data lake" would import the singleton and the string keys
along with the feature.

### Storage — two answers, no verdict here

Cockpit's telemetry logging (`src/libs/data-lake-logging.ts`) is worth recording precisely,
because the concrete numbers are the useful part:

| Aspect | Cockpit's choice |
|---|---|
| Modes | **interval** (full snapshot, default 1000 ms) or **raw** (on every change, buffering only the changed variable) |
| Buffering | in memory, flushed to IndexedDB as one batched entry every **250 ms** |
| Stores | `cockpit-data-lake-logs-db` (points), `cockpit-data-lake-sessions-db` (session metadata) |
| Key format | `boot=<bootId>;epoch=<epoch>;seq=<seq>`, for lexicographic ordering |
| Sessions | split on a **5-minute** gap in data |
| Retention | **1 day** by default, via `deleteOldDataSessions()` |
| Export | JSON or CSV (CSV forward-fills sparse raw-mode data), zipped one file per session |

The key format is the tell: `boot;epoch;seq` for lexicographic ordering is Redis Streams'
ID scheme, reinvented on top of IndexedDB by a project that has no Redis to reach for.

Two postures are open to us. **This document does not choose between them** — that belongs
in an ADR.

**Option A — Redis Streams only.** What tier-4 and tier-5 already commit to. Server-side
durable history via `XADD`, multi-consumer replay, survives a browser refresh, queryable
with no client attached, and it composes with the hermetic replay apparatus in
`docs/roadmap/tier-2-parity-apparatus.md`. Under this reading, Cockpit's IndexedDB layer is
a workaround for the absence of a server, and we should not port a workaround for a
constraint we do not have.

**Option B — Redis plus a client-side ring buffer.** Adds instant local scrubback with no
round trip, and keeps the last N minutes viewable when the operator's link to the *backend*
degrades while the vehicle link stays up. Costs a second persistence path to keep correct
and a second retention policy to reason about — and any divergence between the two becomes
a debugging problem precisely when things are already going wrong.

**The decision criterion**, stated so the later ADR has something to test against: does an
operator ever run the UI across a link that can drop independently of the vehicle link? If
yes, B earns its complexity. If the UI is always co-located with the backend, A is
sufficient and B is two sources of truth for no gain.

### Video — where the port goes

Cockpit's WebRTC client is three files: `src/libs/webrtc/{session.ts, signaller.ts,
signalling_protocol.d.ts}`. The protocol is a question/answer envelope — `peerId`,
`availableStreams`, `startSession` carrying `BindOffer`/`BindAnswer`, `endSession` — plus
`MediaNegotiation` (`sdp: RTCSessionDescription`) and `IceNegotiation`
(`ice: RTCIceCandidateInit`), each tagged with `consumer_id`, `producer_id`, `session_id`.
A stream is `{id, name, encode, height, width, interval, source, created}`. It is the
`mavlink-camera-manager` signalling server's protocol, carried over a WebSocket.

**That maps onto what `proto/gcs/v1/video.proto` already declares.** Our `SdpOffer` /
`SdpAnswer` and `VideoService.ExchangeSdp` are the same negotiation expressed in protobuf
over Connect instead of ad-hoc JSON over a WebSocket — better typed, and consistent with
ADR-0002. `WatchStreams` covers their `availableStreams`. Whoever implements the video tier
is not designing from scratch; they are re-expressing a protocol that already has a working
reference client.

Both projects also agree on the split that matters: **control plane over the typed RPC,
media plane direct.** `video.proto`'s comment already says it — "actual video data does not
flow through Connect/protobuf."

#### The port, before any adapter

The sources ahead of us do not resemble each other. WebRTC and RTSP IP cameras. OpenIPC
over wfb-ng, arriving as H.264 in UDP. **Analog capture** off a card on the GCS host — a
local device, no network at all, no negotiation, no URL. **Hand-rolled digital streams**
whose framing we define ourselves. They share nothing at the transport layer, which is
exactly why the boundary belongs above it.

`video.proto` already draws that boundary, and it is worth saying so explicitly rather than
rediscovering it later. `VideoStreamType` enumerates transports, while `VideoStream` keeps
one uniform shape across all of them: `stream_url` is an opaque string interpreted per type
(the comment block already specifies the scheme for each variant), and `width_px`,
`framerate`, `bitrate_kbps`, `packet_loss_pct`, `latency_ms`, and `status` are
transport-agnostic. `WatchStreams`, `StartRecording`, and `ExchangeSdp` never branch on
source type. **That is the port.** Everything below it is an adapter — the same cut ADR-0003
makes for transport.

Consequences:

- Adding analog capture or a bespoke digital framing should be **one new adapter plus one
  enum value**, never a change to the service. If it ever requires touching `VideoService`,
  the port leaked.
- **Analog is the useful stress test** of the abstraction, precisely because it is the
  weirdest: no URL, no packet loss, no negotiation, no remote peer. If the port survives
  analog, it survives everything else.
- Two gaps this exposes in the current contract: `VIDEO_STREAM_TYPE_ANALOG` does not exist
  (`USB_WEBCAM` is adjacent but means something else), and `packet_loss_pct` / `latency_ms`
  are meaningless for a local capture device. Unset-versus-zero needs a stated convention
  before any UI reads those fields, or a capture card will render as a perfect link.
- Quality metadata is the leaky part to watch generally. Each transport reports different
  things; the port should carry the intersection and let adapters omit rather than
  fabricate.

#### go2rtc is a candidate adapter, not the design

Cockpit's postinstall pulls **go2rtc** binaries. go2rtc ingests RTSP/RTMP/UDP and
republishes as WebRTC at sub-second latency, has explicit OpenIPC support, and is written
in Go — so for us it is a library-or-sidecar option rather than a foreign dependency. It is
attractive because it could collapse several adapters onto one implementation.

The caveat: that is only worth doing *if* the latency cost is acceptable, and it is not a
reason to make WebRTC the universal internal representation. Standardizing every source
onto WebRTC would push a transport concern back through the port we just drew — the analog
capture card would be paying for SDP negotiation it has no use for.

#### Proposed bench — hypothesis, not result

Nothing below has been measured. The first rig available is OpenIPC camera → wfb-ng air
unit → ground receiver → `wfb_rx` emitting H.264 over UDP, which is already contracted as
`VIDEO_STREAM_TYPE_WFB_NG` with a `udp://host:port` stream URL.

The question worth spending hardware time on is not "does wfb-ng work" — it does — but
**what the adapter boundary costs**. Measure glass-to-glass latency two ways over the same
source: the UDP stream decoded directly, versus the same stream republished through go2rtc
as WebRTC. The delta is the price of adapter uniformity.

That number generalizes, which is why it is worth getting once and carefully: analog
capture and a hand-rolled digital stream will each ask the same question later. If the
penalty is small, one WebRTC-shaped adapter can serve most sources and the implementation
collapses. If it is large, low-latency sources need a direct path and the port must
tolerate both — which is a thing to learn before writing the video tier, not after.

### Frontend — Tailwind yes, Vue no

**Tailwind is the right call**, and for a reason specific to us: a TAK-style UI is many
small, dense, composable panels. That is where utility CSS pays for itself and where a
component library's theming works against you.

The wart to avoid: Cockpit runs Tailwind *and* Vuetify 3 *and* Flowbite *and* FontAwesome
simultaneously. Overlapping style systems are a cost, not a feature — every component
becomes a question of which system owns it. Take Tailwind; skip the component library.

**Vue does not get reopened here**, and the honest reason is more useful than restating
ADR-0002. That ADR rejected Vue partly because "magic proxy reactivity obscures which
streams drive which components." Cockpit is a live demonstration of the failure mode: the
data lake exists in part *because* Vue-reactive globals make late-bound subscription feel
free, so nobody was forced to define the domain model that would have prevented it.

The correction that matters, since the motivation for reconsidering Vue was latency:
**a framework swap would not fix a telemetry latency bottleneck, because framework render
cost is not where that bottleneck will be.** At MAVLink telemetry rates the cost lands in
transport, decode, and per-message state churn. All three are fixable inside React:

- batch stream updates into the React Query cache rather than calling `setQueryData()`
  per message;
- keep high-rate values out of the React tree entirely — imperative refs or canvas for
  attitude indicators, plots, and video overlay, which is where the frames actually go;
- subscribe components to the narrowest slice they need, which the typed streams already
  make possible.

Those are days of work against months for a rewrite. Revisit the framework only if those
three are done and measured and still insufficient — and if that happens, the measurement
will tell us something far more specific than "try Vue."

---

## What Cockpit gets wrong

| Problem | Consequence | Our mitigation |
|---|---|---|
| No backend → domain model lives in the browser | Every client re-derives state; nothing is authoritative | Go backend owns the fold; browser is a view |
| Data lake is a global mutable singleton | Untestable, un-mockable — the QGC failure mode again | Immutable snapshots, typed streams, DI |
| String-keyed untyped variables | No schema, no compile-time safety, silent drift | `.proto` contracts are the schema |
| localStorage as a distributed store | Epoch-merge conflict resolution in a browser tab | Server-side settings with Redis |
| `postinstall` fetches binaries from the network | Unpinned, non-hermetic builds | Bazel with pinned hashes (ADR-0001) |
| Tailwind + Vuetify + Flowbite together | Three systems competing for the same components | Tailwind only |

## What Cockpit validates

- **Control plane over typed RPC, media plane direct.** Both projects reached this split
  independently; `video.proto` already encodes it.
- **Settings scoped per-user *and* per-vehicle**, with provenance on every value and a
  deterministic tiebreak. Our fleet-first design has the same shape and no such concept yet.
- **Layout as serializable, versioned, validated JSON** — profiles and views import/export
  as data. Directly relevant to tier-10.
- **Web first, desktop as a packaging step.** ADR-0002 said "Electron is a deployment
  wrapper, not an architecture." Cockpit ships exactly that way, and it works.
- **The QGC failure modes are not language-specific.** Singletons and mutable global state
  showed up again in a 2024-era TypeScript codebase. The mitigations in ADR-0002 and
  ADR-0003 are load-bearing, not ceremony.
