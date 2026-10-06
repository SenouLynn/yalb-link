import { describe, expect, it } from 'vitest';

import {
  parseConnectionList,
  parseConnectionStatus,
  parseDeviceInfo,
  parseDeviceList,
  parseProfile,
  parseProfileList,
} from './types';

describe('parseDeviceInfo', () => {
  it('reads every optional metadata field', () => {
    expect(parseDeviceInfo({
      id: '/dev/cu.usbserial-1', description: 'FTDI', serial_number: 'A1', manufacturer: 'FTDI', vid: '0403', pid: '6001',
    })).toEqual({
      id: '/dev/cu.usbserial-1', description: 'FTDI', serialNumber: 'A1', manufacturer: 'FTDI', vid: '0403', pid: '6001',
    });
  });

  it('omits absent metadata rather than inventing it', () => {
    expect(parseDeviceInfo({ id: '/dev/cu.usbserial-1' })).toEqual({ id: '/dev/cu.usbserial-1' });
  });

  it('rejects anything without an id', () => {
    expect(parseDeviceInfo({ description: 'no id' })).toBeNull();
    expect(parseDeviceInfo(null)).toBeNull();
    expect(parseDeviceInfo('nonsense')).toBeNull();
    expect(parseDeviceInfo(undefined)).toBeNull();
  });

  it('treats an empty id as no device, not a device named ""', () => {
    // `internal/connection/types.go`'s `Status.Device` carries no `omitempty`
    // and is always serialized, even as the zero `Device{}` for a connection
    // with nothing resolved (DEVICE_MISSING/AMBIGUOUS) — id `""`, not absent.
    expect(parseDeviceInfo({ id: '', kind: '', path: '' })).toBeNull();
  });
});

describe('parseConnectionStatus', () => {
  it('reads a full Status object', () => {
    const status = parseConnectionStatus({
      id: 'bench-controller',
      state: 'REPORTING',
      device: { id: '/dev/x', description: 'Cube' },
      settings: { baud_rate: 57600 },
      opened_at_ms: 1000,
      last_frame_at_ms: 2000,
      vehicle_keys: [{ system_id: 1, component_id: 1 }],
    });

    expect(status).toEqual({
      id: 'bench-controller',
      state: 'REPORTING',
      device: { id: '/dev/x', description: 'Cube' },
      settings: { baudRate: 57600 },
      openedAtMs: 1000,
      lastFrameAtMs: 2000,
      vehicleKeys: [{ systemId: 1, componentId: 1 }],
    });
  });

  it('defaults vehicleKeys to empty and omits absent optional fields', () => {
    expect(parseConnectionStatus({ id: 'x', state: 'IDLE' })).toEqual({
      id: 'x', state: 'IDLE', vehicleKeys: [],
    });
  });

  it('reads a real DEVICE_MISSING wire payload — the zero-value device/settings the backend always sends, not omits', () => {
    // The actual shape `GET /api/connections` sends for an unresolved profile:
    // `Status.Device`/`Status.Settings` have no `omitempty`, so both arrive as
    // their zero values rather than being left out of the JSON.
    const status = parseConnectionStatus({
      id: 'bench-controller',
      device: { id: '', kind: '', path: '' },
      settings: { baud_rate: 0 },
      state: 'DEVICE_MISSING',
      opened_at_ms: 0,
      last_frame_at_ms: 0,
      vehicle_keys: null,
    });

    expect(status).toEqual({ id: 'bench-controller', state: 'DEVICE_MISSING', vehicleKeys: [] });
  });

  it('carries the OS/library error text verbatim, never inventing one', () => {
    const status = parseConnectionStatus({ id: 'x', state: 'ACCESS_FAILED', detailed_error: 'Permission denied' });
    expect(status?.detailedError).toBe('Permission denied');
  });

  it('rejects an unrecognised state rather than guessing one', () => {
    expect(parseConnectionStatus({ id: 'x', state: 'SOMETHING_NEW' })).toBeNull();
  });

  it('rejects a missing id or a non-object payload', () => {
    expect(parseConnectionStatus({ state: 'IDLE' })).toBeNull();
    expect(parseConnectionStatus(null)).toBeNull();
    expect(parseConnectionStatus('IDLE')).toBeNull();
    expect(parseConnectionStatus([1, 2, 3])).toBeNull();
  });

  it('drops malformed vehicle key entries rather than crashing', () => {
    const status = parseConnectionStatus({
      id: 'x', state: 'REPORTING', vehicle_keys: [{ system_id: 1, component_id: 1 }, { system_id: 'nope' }, null],
    });
    expect(status?.vehicleKeys).toEqual([{ systemId: 1, componentId: 1 }]);
  });
});

describe('parseConnectionList / parseDeviceList / parseProfileList', () => {
  it('drops one malformed entry without discarding the rest', () => {
    expect(parseConnectionList([{ id: 'a', state: 'IDLE' }, { state: 'IDLE' }, { id: 'b', state: 'RELEASED' }]))
      .toHaveLength(2);
    expect(parseDeviceList([{ id: 'a' }, {}, { id: 'b' }])).toHaveLength(2);
    expect(parseProfileList([
      { id: 'a', name: 'A', device: { id: 'd' }, settings: { baud_rate: 57600 } },
      { id: 'b', name: 'B' },
    ])).toHaveLength(1);
  });

  it('returns an empty array for anything that is not an array', () => {
    expect(parseConnectionList(null)).toEqual([]);
    expect(parseDeviceList({})).toEqual([]);
    expect(parseProfileList('nope')).toEqual([]);
  });
});

describe('parseProfile', () => {
  it('round-trips a full profile, reading the nested device the response actually carries', () => {
    expect(parseProfile({ id: 'p1', name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 115200 } })).toEqual({
      id: 'p1', name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 115200 },
    });
  });

  it('rejects a profile missing any required field', () => {
    expect(parseProfile({ id: 'p1', name: 'Bench', device: { id: '/dev/x' } })).toBeNull();
    expect(parseProfile({ name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 1 } })).toBeNull();
  });
});
