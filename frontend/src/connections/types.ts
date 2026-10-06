/**
 * The browser's view of `docs/tasks/connection-contract.md`.
 *
 * Plain JSON, not protobuf: §6/§7 of the contract deliberately keep the
 * connection surface off the generated schemas so it can evolve independently
 * of the MAVLink-shaped `gen/gcs` messages. Every parser here follows
 * `stream/parse.ts`'s convention of returning `null` for anything
 * unrecognised rather than throwing — one malformed device or status must not
 * take the picker down.
 */

/** `Status.State`, contract §8. Kept as the exact wire strings: a label table
 *  keyed by these is the only place that ever has to spell them out. */
export type ConnectionState =
  | 'DEVICE_MISSING'
  | 'AMBIGUOUS'
  | 'IDLE'
  | 'OPENING'
  | 'ACCESS_FAILED'
  | 'OPEN_AWAITING_TRAFFIC'
  | 'REPORTING'
  | 'INTERRUPTED'
  | 'DEVICE_LOST'
  | 'TRANSPORT_FAILED'
  | 'RELEASED';

const CONNECTION_STATES: readonly ConnectionState[] = [
  'DEVICE_MISSING', 'AMBIGUOUS', 'IDLE', 'OPENING', 'ACCESS_FAILED',
  'OPEN_AWAITING_TRAFFIC', 'REPORTING', 'INTERRUPTED', 'DEVICE_LOST',
  'TRANSPORT_FAILED', 'RELEASED',
];

function isConnectionState(value: unknown): value is ConnectionState {
  return typeof value === 'string' && (CONNECTION_STATES as readonly string[]).includes(value);
}

/** OS inventory metadata, contract's T-046 wire notes. Never a guessed radio
 *  identity: any of these may be absent, and absence is not itself an error. */
export interface DeviceInfo {
  id: string;
  description?: string | undefined;
  serialNumber?: string | undefined;
  manufacturer?: string | undefined;
  vid?: string | undefined;
  pid?: string | undefined;
}

export interface ConnectionSettings {
  baudRate: number;
}

export interface VehicleKeyRef {
  systemId: number;
  componentId: number;
}

/** One connection's current evidence. `id` is a profile id for a saved
 *  connection or a device-snapshot id for an ad hoc one — the contract's §7
 *  keys `GET /api/connections` entries either way. */
export interface ConnectionStatus {
  id: string;
  device?: DeviceInfo | undefined;
  settings?: ConnectionSettings | undefined;
  state: ConnectionState;
  openedAtMs?: number | undefined;
  lastFrameAtMs?: number | undefined;
  vehicleKeys: VehicleKeyRef[];
  /** The OS/library's own words (§11). Never synthesised. */
  detailedError?: string | undefined;
  errorCode?: string | undefined;
}

/** A saved connection profile (T-047, ADR 0010). */
export interface Profile {
  id: string;
  name: string;
  deviceId: string;
  settings: ConnectionSettings;
}

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : null;
}

function str(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function num(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

/** Builds an object with only the keys whose value is defined — the
 *  `exactOptionalPropertyTypes`-safe way to skip an absent optional field
 *  instead of assigning it `undefined` explicitly. */
function defined<T extends object>(fields: { [K in keyof T]: T[K] | undefined }): Partial<T> {
  const result: Partial<T> = {};
  for (const key in fields) {
    const value = fields[key];
    if (value !== undefined) {
      result[key] = value;
    }
  }
  return result;
}

export function parseDeviceInfo(value: unknown): DeviceInfo | null {
  const body = record(value);
  const id = body === null ? undefined : str(body['id']);

  // The backend's `Status.Device` field carries no `omitempty` (types.go) and
  // is always present, even as the zero `Device{}` for a connection with no
  // resolved device (DEVICE_MISSING/AMBIGUOUS) — its id is `""`, not absent.
  // Treated the same as a missing field: an empty id names no real device.
  if (body === null || id === undefined || id === '') {
    return null;
  }

  return {
    id,
    ...defined<Omit<DeviceInfo, 'id'>>({
      description: str(body['description']),
      serialNumber: str(body['serial_number']),
      manufacturer: str(body['manufacturer']),
      vid: str(body['vid']),
      pid: str(body['pid']),
    }),
  };
}

function parseSettings(value: unknown): ConnectionSettings | undefined {
  const body = record(value);
  const baudRate = body === null ? undefined : num(body['baud_rate']);

  // Same always-present-zero-value shape as `device` (§ above): a baud of 0
  // is never a real setting (the contract requires a positive integer), so a
  // connection with no settings configured yet reads as having none, not
  // as one bauded at zero.
  return baudRate === undefined || baudRate <= 0 ? undefined : { baudRate };
}

function parseVehicleKeys(value: unknown): VehicleKeyRef[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const keys: VehicleKeyRef[] = [];
  for (const entry of value) {
    const body = record(entry);
    const systemId = body === null ? undefined : num(body['system_id']);
    const componentId = body === null ? undefined : num(body['component_id']);
    if (systemId !== undefined && componentId !== undefined) {
      keys.push({ systemId, componentId });
    }
  }
  return keys;
}

/** Parses one `Status` object — a REST list entry or an `acquisition` frame. */
export function parseConnectionStatus(value: unknown): ConnectionStatus | null {
  const body = record(value);

  if (body === null) {
    return null;
  }

  const id = str(body['id']);
  const state = body['state'];

  if (id === undefined || !isConnectionState(state)) {
    return null;
  }

  return {
    id,
    state,
    vehicleKeys: parseVehicleKeys(body['vehicle_keys']),
    ...defined<Omit<ConnectionStatus, 'id' | 'state' | 'vehicleKeys'>>({
      device: parseDeviceInfo(body['device']) ?? undefined,
      settings: parseSettings(body['settings']),
      // Also no `omitempty` (types.go): a connection that has never opened
      // sends 0, not an absent field. Never displayed as an epoch timestamp.
      openedAtMs: zeroAsAbsent(num(body['opened_at_ms'])),
      lastFrameAtMs: zeroAsAbsent(num(body['last_frame_at_ms'])),
      detailedError: str(body['detailed_error']),
      errorCode: str(body['error_code']),
    }),
  };
}

function zeroAsAbsent(value: number | undefined): number | undefined {
  return value === undefined || value === 0 ? undefined : value;
}

export function parseConnectionList(value: unknown): ConnectionStatus[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.reduce<ConnectionStatus[]>((acc, entry) => {
    const status = parseConnectionStatus(entry);
    if (status !== null) acc.push(status);
    return acc;
  }, []);
}

export function parseDeviceList(value: unknown): DeviceInfo[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.reduce<DeviceInfo[]>((acc, entry) => {
    const device = parseDeviceInfo(entry);
    if (device !== null) acc.push(device);
    return acc;
  }, []);
}

export function parseProfile(value: unknown): Profile | null {
  const body = record(value);

  if (body === null) {
    return null;
  }

  const id = str(body['id']);
  const name = str(body['name']);
  // The response shape nests the device (`internal/connection/profile.go`'s
  // `Profile.Device Device`), unlike the flat `device_id` the POST *request*
  // body takes (`http.go`'s decode struct) — the two are genuinely different
  // shapes on the same field name, not a typo either way.
  const deviceId = parseDeviceInfo(body['device'])?.id;
  const settings = parseSettings(body['settings']);

  if (id === undefined || name === undefined || deviceId === undefined || settings === undefined) {
    return null;
  }

  return { id, name, deviceId, settings };
}

export function parseProfileList(value: unknown): Profile[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.reduce<Profile[]>((acc, entry) => {
    const profile = parseProfile(entry);
    if (profile !== null) acc.push(profile);
    return acc;
  }, []);
}
