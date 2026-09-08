---
id: T-015
title: Show the decoded flight state the display discards
status: ready
priority: 1
owner: unassigned
depends_on: none
---

## Motivation and evidence

The codec decodes twelve telemetry families; `projectTelemetry`
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

- [ ] Each of the three families is either projected and rendered, or declined
      in `sample.ts` with a comment giving the reason.
- [ ] `NAV_CONTROLLER_OUTPUT` is requested by the rate policy, with a test that
      fails if it is dropped — following `TestDefaultRatesRequestsMissionCurrent`.
- [ ] New readouts use the existing vocabulary: palette tokens, `Readout`,
      `Provenance`, `--data` for numerics. No hardcoded colours.
- [ ] Stale posture is preserved — a stale value renders as `- - -` and drives
      no instrument.
- [ ] `RADIO_STATUS` acceptance is fixture-based and the card records that it
      cannot be demonstrated on the current stack.

## Verification

```sh
go test -race ./internal/bridge/...
cd frontend && pnpm typecheck && pnpm lint && pnpm vitest run
cd frontend && pnpm dev   # /?source=mock
docker compose --profile ui up --build   # NAV_CONTROLLER_OUTPUT against live SITL
```

## Open questions

- `HOME_POSITION` send behaviour. It is believed to be sent when home is set
  and on request rather than streamed, which decides whether a rate request
  applies at all. Confirm against ArduPilot before adding one — an unnecessary
  `SET_MESSAGE_INTERVAL` for a non-streamed family is noise on the link.
- Where guidance state belongs on screen. Placement is T-013's subject; this
  card should not invent a layout that T-013 then has to undo.

## Notes

`RADIO_STATUS` originates from a SiK radio. Compose SITL carries MAVLink over
UDP with no radio in the path, so it is expected never to arrive on the current
stack. It is worth projecting because the decode is already paid for, but do not
treat its absence in SITL as a defect, and do not spend a live session chasing it.
