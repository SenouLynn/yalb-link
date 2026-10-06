/** Deterministic connection scenarios — one per contract §8 state, for tests
 *  and Storybook. No network or backend involved, matching `stream/fixtures.ts`. */

import type { ConnectionStatus, DeviceInfo, Profile } from './types';

const FIELD_RADIO_DEVICE: DeviceInfo = {
  id: '/dev/cu.usbserial-A5069RR4',
  description: 'FTDI USB Serial',
  serialNumber: 'A5069RR4',
  manufacturer: 'FTDI',
  vid: '0403',
  pid: '6001',
};

const BENCH_CONTROLLER_DEVICE: DeviceInfo = {
  id: '/dev/cu.usbmodem14201',
  description: 'CubeOrange',
  serialNumber: '2900379533353532333939',
  manufacturer: 'ArduPilot',
  vid: '2DAE',
  pid: '1016',
};

// No serial number: an adapter the contract expects to require explicit
// selection rather than auto-matching (T-047's identity rules).
const UNIDENTIFIED_DEVICE: DeviceInfo = { id: '/dev/cu.usbserial-1420', description: 'CP2102 USB to UART' };

export const FIXTURE_DEVICES: readonly DeviceInfo[] = [
  FIELD_RADIO_DEVICE,
  BENCH_CONTROLLER_DEVICE,
  UNIDENTIFIED_DEVICE,
];

export const FIXTURE_PROFILES: readonly Profile[] = [
  { id: 'bench-controller', name: 'Bench controller', deviceId: BENCH_CONTROLLER_DEVICE.id, settings: { baudRate: 57600 } },
  { id: 'field-radio', name: 'Field radio', deviceId: FIELD_RADIO_DEVICE.id, settings: { baudRate: 57600 } },
];

const BASE_MS = Date.parse('2026-09-23T12:00:00Z');

function status(partial: Partial<ConnectionStatus> & Pick<ConnectionStatus, 'id' | 'state'>): ConnectionStatus {
  return { vehicleKeys: [], ...partial };
}

/** One representative status per `ConnectionState`, keyed the same. */
export const FIXTURE_STATUSES: Readonly<Record<ConnectionStatus['state'], ConnectionStatus>> = {
  DEVICE_MISSING: status({ id: 'bench-controller', state: 'DEVICE_MISSING' }),
  AMBIGUOUS: status({
    id: 'field-radio',
    state: 'AMBIGUOUS',
    detailedError: '2 devices match the saved identity.',
  }),
  IDLE: status({ id: 'bench-controller', state: 'IDLE', device: BENCH_CONTROLLER_DEVICE, settings: { baudRate: 57600 } }),
  OPENING: status({ id: 'bench-controller', state: 'OPENING', device: BENCH_CONTROLLER_DEVICE, settings: { baudRate: 57600 } }),
  ACCESS_FAILED: status({
    id: 'field-radio',
    state: 'ACCESS_FAILED',
    device: FIELD_RADIO_DEVICE,
    settings: { baudRate: 57600 },
    detailedError: 'Permission denied',
  }),
  OPEN_AWAITING_TRAFFIC: status({
    id: 'field-radio',
    state: 'OPEN_AWAITING_TRAFFIC',
    device: FIELD_RADIO_DEVICE,
    settings: { baudRate: 57600 },
    openedAtMs: BASE_MS - 4000,
  }),
  REPORTING: status({
    id: 'bench-controller',
    state: 'REPORTING',
    device: BENCH_CONTROLLER_DEVICE,
    settings: { baudRate: 57600 },
    openedAtMs: BASE_MS - 60_000,
    lastFrameAtMs: BASE_MS - 200,
    vehicleKeys: [{ systemId: 1, componentId: 1 }],
  }),
  INTERRUPTED: status({
    id: 'field-radio',
    state: 'INTERRUPTED',
    device: FIELD_RADIO_DEVICE,
    settings: { baudRate: 57600 },
    openedAtMs: BASE_MS - 300_000,
    lastFrameAtMs: BASE_MS - 90_000,
    vehicleKeys: [{ systemId: 1, componentId: 1 }],
  }),
  DEVICE_LOST: status({
    id: 'field-radio',
    state: 'DEVICE_LOST',
    settings: { baudRate: 57600 },
    openedAtMs: BASE_MS - 500_000,
  }),
  TRANSPORT_FAILED: status({
    id: 'bench-controller',
    state: 'TRANSPORT_FAILED',
    device: BENCH_CONTROLLER_DEVICE,
    settings: { baudRate: 57600 },
    detailedError: 'read /dev/cu.usbmodem14201: input/output error',
  }),
  RELEASED: status({
    id: 'field-radio',
    state: 'RELEASED',
    device: FIELD_RADIO_DEVICE,
    settings: { baudRate: 57600 },
    openedAtMs: BASE_MS - 900_000,
    lastFrameAtMs: BASE_MS - 850_000,
  }),
};

/** A plausible `GET /api/connections` bootstrap: one bench profile reporting,
 *  one field profile waiting on the aircraft. */
export const FIXTURE_CONNECTION_LIST: readonly ConnectionStatus[] = [
  FIXTURE_STATUSES.REPORTING,
  FIXTURE_STATUSES.OPEN_AWAITING_TRAFFIC,
];
