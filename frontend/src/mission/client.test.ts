import { afterEach, describe, expect, it, vi } from 'vitest';
import { downloadMission } from './client';

afterEach(() => vi.unstubAllGlobals());

describe('downloadMission', () => {
  it('addresses the full vehicle identity and decodes protobuf JSON', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      vehicleId: { systemId: 7, componentId: 42 }, observedAt: '2023-11-14T22:13:20Z', items: [],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetch);
    const snapshot = await downloadMission(7, 42);
    expect(fetch).toHaveBeenCalledWith('/api/vehicles/7/42/mission', {});
    expect(snapshot.vehicleId).toMatchObject({ systemId: 7, componentId: 42 });
    expect(snapshot.observedAt).toBeDefined();
  });

  it('reports the backend error and never returns an old snapshot', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ code: 'mission_download_timeout', message: 'vehicle stopped responding' }),
      { status: 504 },
    )));
    await expect(downloadMission(1, 1)).rejects.toThrow('vehicle stopped responding');
  });
});
