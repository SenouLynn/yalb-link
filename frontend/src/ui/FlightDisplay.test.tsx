/**
 * Component rendering tests.
 *
 * Rendered to static markup rather than into a DOM: these components are pure
 * projections of state, and the assertions that matter are about what text
 * reaches the operator — above all, that an absent or stale reading never
 * arrives as a number.
 */

import { create } from '@bufbuild/protobuf';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import {
  AttitudeSchema,
  EkfStatusReportSchema,
  GlobalPositionSchema,
  GpsRawSchema,
  SystemStatusSchema,
  TelemetryEventSchema,
  VfrHudSchema,
} from '@/gen/gcs/v1/telemetry_pb';
import { GpsFixType, MavType } from '@/gen/gcs/v1/types_pb';
import { HeartbeatStateSchema, VehicleIdSchema } from '@/gen/gcs/v1/vehicle_pb';
import {
  fleetReducer,
  initialFleetState,
  TELEMETRY_TTL_MS,
  type FleetState,
} from '@/fleet/state';
import type { StreamEvent } from '@/stream/events';

import { FlightDisplay } from './FlightDisplay';
import { NO_VALUE } from './format';

const T0 = 1_700_000_000_000;

type TelemetryPayload = ReturnType<typeof create<typeof TelemetryEventSchema>>['payload'];

function id(systemId: number, componentId: number) {
  return create(VehicleIdSchema, { systemId, componentId });
}

function telemetry(
  payload: TelemetryPayload,
  atMs = T0,
  sysId = 1,
  compId = 1,
): StreamEvent {
  return {
    kind: 'telemetry',
    receivedAtMs: atMs,
    event: create(TelemetryEventSchema, { vehicleId: id(sysId, compId), payload }),
  };
}

function discovered(sysId = 1, compId = 1, atMs = T0, armed = true): StreamEvent {
  return {
    kind: 'fleet',
    receivedAtMs: atMs,
    event: create(FleetEventSchema, {
      type: FleetEventType.VEHICLE_DISCOVERED,
      vehicleId: id(sysId, compId),
      heartbeat: create(HeartbeatStateSchema, {
        type: MavType.QUADROTOR,
        armed,
        customMode: 4,
      }),
    }),
  };
}

function lost(sysId: number, compId: number, atMs: number): StreamEvent {
  return {
    kind: 'fleet',
    receivedAtMs: atMs,
    event: create(FleetEventSchema, {
      type: FleetEventType.VEHICLE_LOST,
      vehicleId: id(sysId, compId),
    }),
  };
}

function build(...events: StreamEvent[]): FleetState {
  return events.reduce(
    (state, event) => fleetReducer(state, { type: 'stream', event }),
    fleetReducer(initialFleetState, {
      type: 'stream',
      event: { kind: 'connection', connected: true, receivedAtMs: T0 },
    }),
  );
}

function render(fleet: FleetState, nowMs = T0, mock = false): string {
  return renderToStaticMarkup(
    <FlightDisplay
      fleet={fleet}
      nowMs={nowMs}
      mock={mock}
      onSelect={() => {
        /* selection is exercised through the reducer's own tests */
      }}
    />,
  );
}

/** A vehicle reporting the full instrument set. */
function fullFlight(atMs = T0): StreamEvent[] {
  return [
    discovered(1, 1, atMs),
    telemetry(
      { case: 'attitude', value: create(AttitudeSchema, { rollRad: 0.3, pitchRad: -0.1 }) },
      atMs,
    ),
    telemetry(
      {
        case: 'vfrHud',
        value: create(VfrHudSchema, { groundspeedMS: 5.8, headingDeg: 92, climbMS: 1.4 }),
      },
      atMs,
    ),
    telemetry(
      {
        case: 'globalPosition',
        value: create(GlobalPositionSchema, {
          latDeg: 37.7749,
          lonDeg: -122.4194,
          altRelativeM: 25.3,
          altMslM: 132.5,
        }),
      },
      atMs,
    ),
    telemetry(
      {
        case: 'systemStatus',
        value: create(SystemStatusSchema, { voltageBatteryMv: 12_400, batteryRemainingPct: 87 }),
      },
      atMs,
    ),
    telemetry(
      {
        case: 'gpsRaw',
        value: create(GpsRawSchema, { fixType: GpsFixType.GPS_FIX_TYPE_3D_FIX, satellitesVisible: 14 }),
      },
      atMs,
    ),
    telemetry({ case: 'ekfStatusReport', value: create(EkfStatusReportSchema, { flags: 0xff }) }, atMs),
  ];
}

describe('FlightDisplay with a fully reporting vehicle', () => {
  const html = render(build(...fullFlight()));

  it('shows roll and pitch in degrees', () => {
    expect(html).toContain('17.2'); // 0.3 rad
    expect(html).toContain('-5.7'); // -0.1 rad
  });

  it('shows the altitude with its datum named', () => {
    expect(html).toContain('25.3');
    expect(html).toContain('above home');
  });

  it('shows ground speed and a signed climb rate', () => {
    expect(html).toContain('5.8');
    expect(html).toContain('+1.4');
  });

  it('shows battery voltage and remaining charge', () => {
    expect(html).toContain('12.40');
    expect(html).toContain('87');
  });

  it('shows the heading as a padded bearing', () => {
    expect(html).toContain('092');
  });

  it('names the source message under each reading', () => {
    expect(html).toContain('ATTITUDE');
    expect(html).toContain('VFR_HUD');
    expect(html).toContain('GLOBAL_POSITION_INT');
    expect(html).toContain('SYS_STATUS');
  });

  it('shows armed state, numeric mode, GPS fix, and EKF health', () => {
    expect(html).toContain('ARMED');
    expect(html).toContain('>4<'); // custom mode, undecoded
    expect(html).toContain('3D');
    expect(html).toContain('OK');
  });

  it('reports the source as live', () => {
    expect(html).toContain('LIVE');
  });

  it('renders no unavailable markers when everything is reporting', () => {
    expect(html).not.toContain(NO_VALUE);
  });
});

describe('FlightDisplay with missing readings', () => {
  it('renders an unavailable marker rather than a fabricated zero', () => {
    // Heartbeat only: nothing has reported attitude, altitude, or battery.
    const html = render(build(discovered()));

    expect(html).toContain(NO_VALUE);
    // A zero-valued readout would render as "0.0"; none may appear.
    expect(html).not.toContain('>0.0<');
  });

  it('leaves the horizon face empty rather than drawing a level aircraft', () => {
    const html = render(build(discovered()));

    expect(html).toContain('Attitude unavailable');
    expect(html).not.toContain('var(--sky)');
  });

  it('draws the horizon once attitude arrives, including a level one', () => {
    const level = render(
      build(
        discovered(),
        telemetry({ case: 'attitude', value: create(AttitudeSchema, { rollRad: 0, pitchRad: 0 }) }),
      ),
    );

    // A genuinely level aircraft is a reading and must be drawn.
    expect(level).toContain('var(--sky)');
    expect(level).toContain('var(--ground)');
    expect(level).toContain('Roll 0 degrees');
  });

  it('marks GPS and EKF as unknown when they have not reported', () => {
    const html = render(build(discovered()));

    expect(html).toContain('chip--dead');
  });
});

describe('FlightDisplay staleness', () => {
  it('withholds stale numbers while retaining stale provenance', () => {
    const html = render(build(...fullFlight()), T0 + TELEMETRY_TTL_MS + 1);

    expect(html).toContain('provenance--stale');
    expect(html).not.toContain('25.3');
    expect(html).toContain(NO_VALUE);
    expect(html).toContain('Attitude unavailable');
    expect(html).not.toContain('3D');
    expect(html).not.toContain('>OK<');
  });

  it('leaves fresh readings unmarked', () => {
    const html = render(build(...fullFlight()), T0);

    expect(html).not.toContain('provenance--stale');
  });

  it('stales only the families that stopped, not the whole display', () => {
    const html = render(
      build(
        ...fullFlight(T0),
        telemetry(
          {
            case: 'vfrHud',
            value: create(VfrHudSchema, { groundspeedMS: 6.1, headingDeg: 95, climbMS: 0.2 }),
          },
          T0 + 5_000,
        ),
      ),
      T0 + 5_100,
    );

    // ATTITUDE has aged out; VFR_HUD has not.
    expect(html).toContain('provenance--stale');
    expect(html).toContain('6.1');
    expect(html).toContain('095');
  });
});

describe('FlightDisplay vehicle selection', () => {
  it('hides the selector for a single vehicle', () => {
    const html = render(build(...fullFlight()));

    expect(html).not.toContain('Select vehicle');
  });

  it('shows the selector once a second vehicle appears', () => {
    const html = render(build(...fullFlight(), discovered(2, 1)));

    expect(html).toContain('Select vehicle');
    expect(html).toContain('1:1');
    expect(html).toContain('2:1');
  });

  it('marks the selected vehicle as pressed', () => {
    const html = render(build(...fullFlight(), discovered(2, 1)));

    expect(html).toContain('aria-pressed="true"');
  });

  it('labels a lost vehicle in the selector', () => {
    const html = render(build(...fullFlight(), discovered(2, 1), lost(2, 1, T0 + 1000)));

    expect(html).toContain('2:1 · LOST');
  });

  it('shows one vehicle\'s data without leaking another\'s', () => {
    const html = render(
      build(
        ...fullFlight(),
        discovered(2, 1),
        telemetry(
          { case: 'vfrHud', value: create(VfrHudSchema, { groundspeedMS: 99.9 }) },
          T0,
          2,
          1,
        ),
      ),
    );

    // Vehicle 1:1 is selected; 2:1's speed must not appear on the instrument.
    expect(html).toContain('5.8');
    expect(html).not.toContain('99.9');
  });
});

describe('FlightDisplay without a vehicle', () => {
  it('says it is waiting when connected to the backend', () => {
    const state = fleetReducer(initialFleetState, {
      type: 'stream',
      event: { kind: 'connection', connected: true, receivedAtMs: T0 },
    });

    expect(render(state)).toContain('Waiting for a heartbeat');
  });

  it('points at the mock URL when the backend is unreachable', () => {
    expect(render(initialFleetState)).toContain('?source=mock');
  });

  it('says so when replaying fixtures', () => {
    expect(render(initialFleetState, T0, true)).toContain('Replaying fixtures');
  });
});

describe('FlightDisplay link and vehicle state are separate', () => {
  it('reports a heard vehicle on a dropped connection without calling it lost', () => {
    const disconnected = fleetReducer(build(...fullFlight()), {
      type: 'stream',
      event: { kind: 'connection', connected: false, receivedAtMs: T0 },
    });

    const html = render(disconnected);

    expect(html).toContain('DISCONNECTED');
    expect(html).toContain('HEARD');
  });

  it('reports a lost vehicle on a healthy connection without calling it disconnected', () => {
    const html = render(build(...fullFlight(), lost(1, 1, T0 + 1000)));

    expect(html).toContain('LIVE');
    expect(html).toContain('LOST');
  });

  it('says MOCK rather than LIVE when replaying fixtures', () => {
    const html = render(build(...fullFlight()), T0, true);

    expect(html).toContain('MOCK');
  });
});
