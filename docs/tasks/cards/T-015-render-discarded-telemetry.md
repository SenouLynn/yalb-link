---
id: T-015
title: Show the decoded flight state the display discards
status: done
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The codec decodes twelve telemetry families; `sampleFromEvent`
(`frontend/src/logic/sample.ts`) projects eight. Three of the remainder reach
the browser fully decoded and typed, then lose their payload to the `default`
branch, which keeps only `sourceMessage` and `receivedAtMs` for freshness.

This is a last mile, not a feature. For each family below the proto contract,
Go decoder, golden `.bin`/`.json` fixture, per-field decoder assertion, matrix
row, and generated TypeScript type all exist already, and `TestMatrixCoverage`
fails if a decoder and its matrix row disagree.

| Family | ID | Carries | Missing |
|---|---|---|---|
| `NAV_CONTROLLER_OUTPUT` | 62 | nav/target bearing, wp distance, alt/aspd/xtrack error | projection, render, rate request |
| `HOME_POSITION` | 242 | home lat/lon/alt and its frame | projection, render |
| `RADIO_STATUS` | 109 | rssi, remrssi, noise, txbuf, errors | projection, render |

Fields inside already-projected families are also unrendered — battery cell
voltages and current, EKF variances, GPS `eph`/`epv`, and airspeed all sit in
`TelemetrySample` with no readout.

`NAV_CONTROLLER_OUTPUT` is a streamed family and is absent from
`bridge.DefaultRates`, which is the same shape as the `MISSION_CURRENT` defect
T-010 fixed: decoded, consumed, never requested.

## Outcome

Guidance state, home position, and link quality are visible in the display, and
no telemetry family is decoded by the backend only to be dropped by the browser
without a stated reason.

## Scope

- In: projection into `TelemetrySample`, rate policy for streamed families,
  readouts for the three families, and rendering already-projected fields that
  have no readout.
- Out: the `STATUSTEXT` log (T-016), layout and density rework (T-013), the
  mission panel's restyle (T-014).

## Acceptance criteria

- [x] Each of the three families is either projected and rendered, or declined
      in `sample.ts` with a comment giving the reason. All three are projected
      and rendered; the fields left behind carry a stated reason in the case arm.
- [x] `NAV_CONTROLLER_OUTPUT` is requested by the rate policy, with a test that
      fails if it is dropped — `TestDefaultRatesRequestsNavControllerOutput`,
      confirmed failing with the entry removed.
- [x] New readouts use the existing vocabulary: palette tokens, `Readout`,
      `Provenance`, `--data` for numerics. No hardcoded colours.
- [x] Stale posture is preserved — a stale value renders as `- - -` and drives
      no instrument. Demonstrated on live SITL at 56 s
      (`live-stale.png`).
- [x] `RADIO_STATUS` acceptance is fixture-based and the card records that it
      cannot be demonstrated on the current stack.

## Verification

```sh
go test -race ./internal/bridge/...
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run
cd frontend && pnpm dev   # /?source=mock
docker compose --profile ui up --build   # NAV_CONTROLLER_OUTPUT against live SITL
```

## Open questions

Both settled during execution; kept here with their answers.

- **`HOME_POSITION` send behaviour — answered by measurement.** It is an event,
  not a stream: a 45 s Copter capture contained exactly one, and a 40 s capture
  after a backend restart contained none. No rate request was added, and
  `DefaultRates` records the reason. Because `TELEMETRY_TTL_MS` is 5 s, home
  would otherwise dash five seconds into every flight while the altitude tape
  beside it still read "above home"; it is aged against `HOME_TTL_MS` instead.
- **Placement — as proposed.** The tier renders below the primary readings in
  the instruments pane, inside the independently scrolling `.pane__body`.
  Reachability measured at both widths
  (`1440`,
  `768`).

## Notes

`RADIO_STATUS` originates from a SiK radio. Compose SITL carries MAVLink over
UDP with no radio in the path, so it is expected never to arrive on the current
stack. It is worth projecting because the decode is already paid for, but do not
treat its absence in SITL as a defect, and do not spend a live session chasing it.
Measured: zero arrivals in every capture. Its acceptance is fixture-based.

Full evidence chain: [T-015 evidence](../../runbooks/validation/t015.md).

### Field audit

The card's scope included rendering already-projected fields that had no
readout. Every field in `TelemetrySample` was swept; the decisions are:

| Field | Decision |
|---|---|
| `airspeedMps` | **Rendered.** Projected from VFR_HUD and consumed by nothing — a primary Plane reading that was being dropped. Given its own `resolveAirspeed` rather than a field on `FlightPathResult`, whose `source` names where the *climb rate* came from and can be `GLOBAL_POSITION_INT`. |
| `rollspeedRadS`, `pitchspeedRadS`, `yawspeedRadS` | Declined. Consumed by the trajectory prediction. Rates of change are read from the moving horizon, not from a number. |
| `vxMs`, `vyMs`, `vzMs` | Declined. Consumed by `resolveFlightPath2d`; the derived track and climb are what an operator reads, not the NED components. |
| `hdgCdeg`, `cogCdeg`, `velCmS` | Declined. Fallback inputs to the heading and flight-path resolvers, already surfaced through those readings with their own provenance. |
| `batteryStatusCurrentCa`, `systemStatusCurrentCa` | Declined **for now.** Current draw is a real reading, but pack current belongs with a power tier that does not exist yet; adding one number for it would be the density decision T-013 owns. |
| `batteryStatusCellVoltagesMv` | Declined. Per-cell voltages are a diagnostic array, not a readout, and dumping them is the shape T-014 exists to remove. |
| `gpsFixType`, `satellitesVisible`, `ekfFlags` | Already rendered, in `StatusBar`. |
| `missionCurrentSeq` | Already rendered, as the mission panel's active-item highlight. |
| `vehicleType`, `customMode`, `systemStatus`, `armed` | Already rendered, in `StatusBar` and `ArmControl`. |

GPS `eph`/`epv` and the EKF variances are named in the motivation above as
sitting in `TelemetrySample`. They do not: they are decoded and reach the
browser, but `sample.ts` never projected them, so they are the same defect as
the three families, one level down. They are **not** absorbed here — dilution
and variance are a GPS/EKF quality tier, and inventing one would take this card
into T-013. Worth a card of its own.

### Defect found during acceptance

After a SITL restart the vehicle returns on a new UDP source port; the backend
logs `SOURCE_CONFLICT`, never marks it lost, and so never re-issues the rate
requests. All eight previously requested families go silent too, so this
predates this card. Raised as
[T-023](T-023-rate-requests-after-source-change.md) and not fixed here.

### Regression found and fixed here

The mock's health families were sent every tenth cycle — exactly
`TELEMETRY_TTL_MS` before this card, and over it once guidance added a sixth
frame per cycle. Battery, GPS, EKF and radio all dashed at `?source=mock` while
being actively sent. The cadence is now every fifth cycle, and `mock.test.ts`
fails if any family's worst gap reaches the TTL.

### Gates not run locally

`bazel test //...` (the checkout pins 8.7.0; Homebrew provides 8.3.1) and
`golangci-lint` (not installed) were **not** run on this machine and are left to
CI. `go test -race ./...`, `pnpm typecheck`, `pnpm lint`, `pnpm vitest run` and
`pnpm build` all ran and passed.
