import { describe, expect, it, vi } from 'vitest';

import { missionGeometry } from './model';
import { mockMissionSnapshot, MOCK_MISSION_LENGTH } from './fixtures';
import { missionLoaderFor } from './source';

describe('mission loader selection', () => {
  it('serves the fixture for the mock display with no network call', async () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);

    const snapshot = await missionLoaderFor('mock')(1, 1);

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(snapshot.items).toHaveLength(MOCK_MISSION_LENGTH);
    vi.unstubAllGlobals();
  });

  // A fixture rendered beside live telemetry is a commanded route an operator
  // may act on. It must only ever come from the vehicle.
  it('never serves the fixture to the live or replay display', async () => {
    const fetchSpy = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ code: 'mission_no_route', message: 'no route to vehicle' }),
    });
    vi.stubGlobal('fetch', fetchSpy);

    for (const source of ['live', 'replay'] as const) {
      await expect(missionLoaderFor(source)(1, 1)).rejects.toThrow('no route to vehicle');
    }

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    vi.unstubAllGlobals();
  });

  it('honours an already-aborted request like the network path', async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(missionLoaderFor('mock')(1, 1, controller.signal)).rejects.toThrow();
  });
});

describe('mock mission fixture', () => {
  it('carries the vehicle it was asked for', () => {
    const snapshot = mockMissionSnapshot(7, 42, 1_000);

    expect(snapshot.vehicleId).toMatchObject({ systemId: 7, componentId: 42 });
    expect(snapshot.observedAt).toBeDefined();
  });

  // Without an unmappable item the fixture could never demonstrate the
  // "Not mapped" posture, which is a state the display is required to have.
  it('demonstrates both mapped geometry and an explained omission', () => {
    const geometry = missionGeometry(mockMissionSnapshot(1, 1, 1_000));

    expect(geometry.points.length).toBeGreaterThan(1);
    expect(Object.keys(geometry.omitted).length).toBeGreaterThan(0);
    expect(geometry.points.length + Object.keys(geometry.omitted).length).toBe(MOCK_MISSION_LENGTH);
  });
});
