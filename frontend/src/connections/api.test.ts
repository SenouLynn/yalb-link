import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  connect,
  ConnectionHTTPError,
  deleteProfile,
  disconnect,
  listConnections,
  listDevices,
  listProfiles,
  saveProfile,
} from './api';

afterEach(() => { vi.restoreAllMocks(); });

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('listConnections / listDevices / listProfiles', () => {
  it('reads and parses each list endpoint', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([{ id: 'a', state: 'REPORTING' }]))
      .mockResolvedValueOnce(jsonResponse([{ id: '/dev/x', description: 'Cube' }]))
      .mockResolvedValueOnce(jsonResponse([{ id: 'a', name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 57600 } }]));

    expect(await listConnections()).toEqual([{ id: 'a', state: 'REPORTING', vehicleKeys: [] }]);
    expect(await listDevices()).toEqual([{ id: '/dev/x', description: 'Cube' }]);
    expect(await listProfiles()).toEqual([{ id: 'a', name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 57600 } }]);

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/connections', expect.objectContaining({ method: 'GET' }));
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/connections/devices', expect.objectContaining({ method: 'GET' }));
    expect(fetch).toHaveBeenNthCalledWith(3, '/api/connections/profiles', expect.objectContaining({ method: 'GET' }));
  });
});

describe('connect', () => {
  it('sends device_id and settings as strict snake-case JSON', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({ id: '/dev/x', state: 'OPENING' }));

    const status = await connect({ deviceId: '/dev/x', settings: { baudRate: 57600 } });

    expect(status.state).toBe('OPENING');
    expect(fetch).toHaveBeenCalledWith('/api/connections/connect', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ device_id: '/dev/x', settings: { baud_rate: 57600 } }),
    }));
  });

  it('sends profile_id and device_id together for explicit AMBIGUOUS/DEVICE_MISSING selection', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({ id: 'p1', state: 'OPENING' }));

    await connect({ profileId: 'p1', deviceId: '/dev/x' });

    expect(fetch).toHaveBeenCalledWith('/api/connections/connect', expect.objectContaining({
      body: JSON.stringify({ device_id: '/dev/x', profile_id: 'p1' }),
    }));
  });

  it('carries the backend error text verbatim on 403/404/409', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('permission denied', { status: 403 }));
    const error = await connect({ deviceId: '/dev/x' }).catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(ConnectionHTTPError);
    expect((error as ConnectionHTTPError).status).toBe(403);
    expect((error as ConnectionHTTPError).message).toBe('permission denied');
  });

  it('reads a JSON error body\'s message field when present', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({ error: 'device busy' }, 409));
    const error = await connect({ deviceId: '/dev/x' }).catch((cause: unknown) => cause);
    expect((error as ConnectionHTTPError).status).toBe(409);
    expect((error as ConnectionHTTPError).message).toBe('device busy');
  });

  it('falls back to a generic message for an empty failure body', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('', { status: 500 }));
    const error = await connect({ deviceId: '/dev/x' }).catch((cause: unknown) => cause);
    expect((error as ConnectionHTTPError).message).toBe('Connection request returned HTTP 500.');
  });
});

describe('disconnect', () => {
  it('posts the connection id; the backend names no Status here (§6\'s acquisition event carries RELEASED instead)', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({ disconnected: true }));
    await expect(disconnect('a')).resolves.toBeUndefined();
    expect(fetch).toHaveBeenCalledWith('/api/connections/disconnect', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ id: 'a' }),
    }));
  });
});

describe('saveProfile / deleteProfile', () => {
  it('never trusts a caller-supplied device identity back into the request', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      // The response nests `device` (profile.go's `Profile.Device Device`);
      // only the POST *request* body below is the flat `device_id` shape.
      jsonResponse({ id: 'p1', name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 57600 } }),
    );

    const saved = await saveProfile({ name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 57600 } });

    expect(saved).toEqual({ id: 'p1', name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 57600 } });
    expect(fetch).toHaveBeenCalledWith('/api/connections/profiles', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'Bench', device_id: '/dev/x', settings: { baud_rate: 57600 } }),
    }));
  });

  it('deletes by id, URL-encoded', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
    await deleteProfile('needs escaping/slash');
    expect(fetch).toHaveBeenCalledWith(
      '/api/connections/profiles/needs%20escaping%2Fslash',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });
});
