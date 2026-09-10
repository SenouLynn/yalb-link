/**
 * Inspection tier rendering.
 *
 * Rendered to static markup, like the other component suites: what matters is
 * the text that reaches the operator, and above all that a value which stopped
 * arriving never reaches them as a number.
 */

import { create } from '@bufbuild/protobuf';
import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import { MavAutopilot, MavType } from '@/gen/gcs/v1/types_pb';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import {
  HomePositionSchema,
  NavControllerOutputSchema,
  RadioStatusSchema,
  TelemetryEventSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { HeartbeatStateSchema, VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import {
  fleetReducer,
  initialFleetState,
  TELEMETRY_TTL_MS,
  vehicleKey,
  type VehicleView,
} from '@/fleet/state';
import type { StreamEvent } from '@/stream/events';

import { InspectionPanel } from './InspectionPanel';
import { NO_VALUE } from './format';
import { HOME_TTL_MS, readFlight } from './readings';

const T0 = 1_700_000_000_000;

type TelemetryPayload = ReturnType<typeof create<typeof TelemetryEventSchema>>['payload'];

function telemetry(payload: TelemetryPayload, atMs: number): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, {
      vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
      payload,
    }),
  };
}

function navAt(atMs: number): StreamEvent {
  return telemetry(
    {
      case: 'navControllerOutput',
      value: create(NavControllerOutputSchema, {
        navBearingDeg: 91,
        targetBearingDeg: 94,
        wpDistM: 137,
        altErrorM: -2.4,
        aspdErrorMS: 0.6,
        xtrackErrorM: 3.1,
      }),
    },
    atMs,
  );
}

function homeAt(atMs: number): StreamEvent {
  return telemetry(
    {
      case: 'homePosition',
      value: create(HomePositionSchema, {
        latDeg: 37.7749,
        lonDeg: -122.4194,
        altMslM: 10.1,
      }),
    },
    atMs,
  );
}

function radioAt(atMs: number): StreamEvent {
  return telemetry(
    {
      case: 'radioStatus',
      value: create(RadioStatusSchema, {
        rssi: 190,
        remrssi: 185,
        noise: 40,
        remnoise: 38,
        txbufPct: 92,
        rxerrors: 7,
      }),
    },
    atMs,
  );
}

function viewOf(...events: StreamEvent[]): VehicleView {
  const state = events.reduce(
    (acc, event) => fleetReducer(acc, { type: 'stream', event }),
    initialFleetState,
  );

  const view = state.vehicles[vehicleKey(1, 1)];
  if (view === undefined) throw new Error('no vehicle in state');

  return view;
}

function render(view: VehicleView, nowMs: number): string {
  return renderToStaticMarkup(<InspectionPanel readings={readFlight(view, nowMs)} />);
}

describe('InspectionPanel', () => {
  it('normalizes retained telemetry when a Plane heartbeat arrives, including replay order', () => {
    const nav = telemetry({ case: 'navControllerOutput', value: create(NavControllerOutputSchema, {
      altErrorM: 12.5, aspdErrorMS: 60,
    }) }, T0);
    const heartbeat: StreamEvent = {
      kind: 'fleet', receivedAtMs: T0,
      event: create(FleetEventSchema, {
        type: FleetEventType.HEARTBEAT_UPDATED,
        vehicleId: create(VehicleIdSchema, { systemId: 1, componentId: 1 }),
        heartbeat: create(HeartbeatStateSchema, {
          autopilot: MavAutopilot.ARDUPILOTMEGA, type: MavType.FIXED_WING,
        }),
      }),
    };
    expect(readFlight(viewOf(nav), T0).guidance.value?.aspdErrorMps).toBeNull();
    for (const events of [[nav, heartbeat], [heartbeat, nav]]) {
      const view = viewOf(...events);
      expect(readFlight(view, T0).guidance.value?.aspdErrorMps).toBeCloseTo(0.6);
      expect(view.sample.aspdErrorRaw).toBe(60);
      const markup = render(view, T0);
      expect(markup).toContain('+0.6');
      expect(markup).toContain('+12.5');
      expect(markup).toContain('positive is below target');
    }
  });

  it('renders guidance from NAV_CONTROLLER_OUTPUT', () => {
    const markup = render(viewOf(navAt(T0)), T0);

    expect(markup).toContain('094'); // target bearing, zero-padded
    expect(markup).toContain('091'); // nav bearing
    expect(markup).toContain('137'); // waypoint distance
    expect(markup).toContain('NAV_CONTROLLER_OUTPUT');
  });

  it('keeps the sign on the errors an operator reads directionally', () => {
    const markup = render(viewOf(navAt(T0)), T0);

    expect(markup).toContain('-2.4'); // altitude error, above target
    expect(markup).toContain('+3.1'); // crosstrack, right of the leg
    expect(markup).toContain('positive is below target');
    expect(markup).not.toContain('positive is above target');
  });

  it('renders home and its elevation', () => {
    const markup = render(viewOf(homeAt(T0)), T0);

    expect(markup).toContain('37.774900, -122.419400');
    expect(markup).toContain('10.1');
    expect(markup).toContain('HOME_POSITION');
  });

  it('renders link quality from RADIO_STATUS', () => {
    const markup = render(viewOf(radioAt(T0)), T0);

    expect(markup).toContain('190');
    expect(markup).toContain('92'); // free buffer space
    expect(markup).toContain('Buffer free');
    expect(markup).toContain('back-pressure at 0');
    expect(markup).toContain('RADIO_STATUS');
  });

  it('withholds every guidance number once the family goes stale', () => {
    // The posture the whole display takes: a value that stopped arriving is not
    // shown as a number, however recently it was true.
    const markup = render(viewOf(navAt(T0)), T0 + TELEMETRY_TTL_MS + 1);

    expect(markup).toContain(NO_VALUE);
    expect(markup).not.toContain('137');
    expect(markup).not.toContain('094');
  });

  it('explains an absent family instead of showing a column of dashes', () => {
    // RADIO_STATUS never arrives over UDP. Six dashes would read as six broken
    // instruments rather than as a radio that is not part of this link.
    const markup = render(viewOf(navAt(T0)), T0);

    expect(markup).toContain('Radio telemetry may be unavailable');
    // Home has no group of its own to collapse, so it says it on its row.
    expect(markup).toContain('NOT SET');
  });

  it('keeps home showable long after the telemetry TTL has passed', () => {
    // Home is set once and does not perish. Under the telemetry TTL it would
    // dash five seconds into every flight while the altitude tape beside it
    // still read "above home".
    const markup = render(viewOf(homeAt(T0)), T0 + TELEMETRY_TTL_MS * 10);

    expect(markup).toContain('37.774900, -122.419400');
  });

  it('does eventually withhold a home position older than its own TTL', () => {
    const markup = render(viewOf(homeAt(T0)), T0 + HOME_TTL_MS + 1);

    expect(markup).not.toContain('37.774900');
    expect(markup).toContain(NO_VALUE);
  });

  it('declines the unset-home sentinel rather than pointing at Null Island', () => {
    const unset = telemetry(
      { case: 'homePosition', value: create(HomePositionSchema, {}) },
      T0,
    );
    const markup = render(viewOf(unset), T0);

    expect(markup).not.toContain('0.000000, 0.000000');
    expect(markup).toContain('NOT SET');
  });
});
