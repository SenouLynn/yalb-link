/** The `docs/tasks/connection-contract.md` §7 HTTP surface. */

import {
  parseConnectionList,
  parseConnectionStatus,
  parseDeviceList,
  parseProfile,
  parseProfileList,
  type ConnectionSettings,
  type ConnectionStatus,
  type DeviceInfo,
  type Profile,
} from './types';

const CONNECTIONS_PATH = '/api/connections';
const DEVICES_PATH = '/api/connections/devices';
const CONNECT_PATH = '/api/connections/connect';
const DISCONNECT_PATH = '/api/connections/disconnect';
const PROFILES_PATH = '/api/connections/profiles';

/**
 * Unlike `CommandHTTPError`, the contract defines no typed conflict body for
 * this surface (§11) — every failure names only what the OS/library actually
 * reported, so this carries that text verbatim rather than a decoded shape.
 */
export class ConnectionHTTPError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ConnectionHTTPError';
  }
}

async function failureMessage(response: Response): Promise<string> {
  const raw = (await response.text()).trim();

  if (raw === '') {
    return `Connection request returned HTTP ${String(response.status)}.`;
  }

  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed === 'object' && parsed !== null) {
      const body = parsed as { error?: unknown; message?: unknown; detail?: unknown };
      const named = body.error ?? body.message ?? body.detail;
      if (typeof named === 'string' && named !== '') {
        return named;
      }
    }
  } catch {
    // Not JSON — the raw text itself is what the OS/library reported.
  }

  return raw;
}

async function request(path: string, init?: { method: string; body?: string }): Promise<unknown> {
  const response = await globalThis.fetch(path, {
    method: init?.method ?? 'GET',
    ...(init?.body === undefined ? {} : { body: init.body }),
    headers: { 'Content-Type': 'application/json' },
  });

  if (!response.ok) {
    throw new ConnectionHTTPError(await failureMessage(response), response.status);
  }

  const text = await response.text();
  return text === '' ? null : (JSON.parse(text) as unknown);
}

export async function listConnections(): Promise<ConnectionStatus[]> {
  return parseConnectionList(await request(CONNECTIONS_PATH));
}

export async function listDevices(): Promise<DeviceInfo[]> {
  return parseDeviceList(await request(DEVICES_PATH));
}

export interface ConnectRequest {
  deviceId?: string | undefined;
  profileId?: string | undefined;
  settings?: ConnectionSettings | undefined;
}

/**
 * Opens a device or resolves a saved profile (contract §7/§9).
 *
 * `profileId` alone lets the backend resolve identity automatically;
 * `profileId` with `deviceId` is the explicit-selection path for
 * `AMBIGUOUS`/`DEVICE_MISSING`; `deviceId` alone is an unsaved, ad hoc open.
 */
export async function connect(body: ConnectRequest): Promise<ConnectionStatus> {
  const payload: Record<string, unknown> = {};
  if (body.deviceId !== undefined) payload['device_id'] = body.deviceId;
  if (body.profileId !== undefined) payload['profile_id'] = body.profileId;
  if (body.settings !== undefined) payload['settings'] = { baud_rate: body.settings.baudRate };

  const status = parseConnectionStatus(
    await request(CONNECT_PATH, { method: 'POST', body: JSON.stringify(payload) }),
  );

  if (status === null) {
    throw new ConnectionHTTPError('The backend accepted the connection but returned no status.', 200);
  }

  return status;
}

/**
 * Explicit release — the MissionPlanner hand-off path (ADR 0006).
 *
 * The backend's response is `{"disconnected": true}` (`http.go`), not a
 * `Status`: this surface's own `acquisition` event is what carries `RELEASED`
 * back to the fold, the same as any other state transition (contract §6).
 */
export async function disconnect(id: string): Promise<void> {
  await request(DISCONNECT_PATH, { method: 'POST', body: JSON.stringify({ id }) });
}

export async function listProfiles(): Promise<Profile[]> {
  return parseProfileList(await request(PROFILES_PATH));
}

/** Saves a new profile, or updates one when `id` names an existing profile.
 *  Device identity is always re-resolved by the backend from `deviceId`,
 *  never trusted back from a previous read (§7). */
export async function saveProfile(profile: {
  id?: string;
  name: string;
  deviceId: string;
  settings: ConnectionSettings;
}): Promise<Profile> {
  const payload: Record<string, unknown> = {
    name: profile.name,
    device_id: profile.deviceId,
    settings: { baud_rate: profile.settings.baudRate },
  };
  if (profile.id !== undefined) payload['id'] = profile.id;

  const saved = parseProfile(await request(PROFILES_PATH, { method: 'POST', body: JSON.stringify(payload) }));

  if (saved === null) {
    throw new ConnectionHTTPError('The backend accepted the profile but returned none.', 200);
  }

  return saved;
}

export async function deleteProfile(id: string): Promise<void> {
  await request(`${PROFILES_PATH}/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
