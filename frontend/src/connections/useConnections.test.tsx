// @vitest-environment happy-dom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useConnections, type UseConnectionsResult } from './useConnections';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  if (root !== null) act(() => { root?.unmount(); });
  container?.remove();
  container = null;
  root = null;
  vi.restoreAllMocks();
});

function mount(live: boolean): { latest: () => UseConnectionsResult } {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);

  let latest: UseConnectionsResult | null = null;
  function Harness() {
    latest = useConnections(live);
    return null;
  }

  act(() => { root?.render(<Harness />); });
  return {
    latest: () => {
      if (latest === null) throw new Error('not mounted');
      return latest;
    },
  };
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('useConnections', () => {
  it('bootstraps the fold from GET /api/connections when live', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse([{ id: 'a', state: 'REPORTING' }]));
    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });
    expect(harness.latest().connections).toEqual([{ id: 'a', state: 'REPORTING', vehicleKeys: [] }]);
  });

  it('never calls the backend for mock/replay sources', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch');
    mount(false);
    await act(async () => { await Promise.resolve(); });
    expect(fetch).not.toHaveBeenCalled();
  });

  it('dispatchStream folds an acquisition event and ignores every other kind', () => {
    const harness = mount(false);
    act(() => {
      harness.latest().dispatchStream({ kind: 'acquisition', event: { id: 'a', state: 'IDLE', vehicleKeys: [] }, receivedAtMs: 1 });
    });
    expect(harness.latest().connections).toEqual([{ id: 'a', state: 'IDLE', vehicleKeys: [] }]);

    act(() => {
      harness.latest().dispatchStream({ kind: 'connection', connected: true, receivedAtMs: 2 });
    });
    expect(harness.latest().connections).toEqual([{ id: 'a', state: 'IDLE', vehicleKeys: [] }]);
  });

  it('connectDevice marks pending, applies the returned status, then clears pending', async () => {
    // The connect call is held open on a deferred promise rather than an
    // auto-resolving mock, so the "still in flight" assertion below is
    // deterministic instead of racing however many microtask turns the real
    // fetch/json/parse chain happens to take.
    let resolveConnect: ((response: Response) => void) | undefined;
    const held = new Promise<Response>((resolve) => { resolveConnect = resolve; });

    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([])) // bootstrap
      .mockReturnValueOnce(held);

    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });

    act(() => { harness.latest().connectDevice('/dev/x', { baudRate: 57600 }); });
    expect(harness.latest().pending.has('/dev/x')).toBe(true);

    await act(async () => {
      resolveConnect?.(jsonResponse({ id: '/dev/x', state: 'OPENING' }));
      await held;
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(harness.latest().pending.has('/dev/x')).toBe(false);
    expect(harness.latest().connections).toEqual([{ id: '/dev/x', state: 'OPENING', vehicleKeys: [] }]);
    expect(harness.latest().actionError).toBeNull();
  });

  it('surfaces a failed connect as actionError without inventing a cause, and clears pending', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([]))
      .mockResolvedValueOnce(new Response('permission denied', { status: 403 }));

    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      harness.latest().connectDevice('/dev/x', { baudRate: 57600 });
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(harness.latest().actionError).toBe('permission denied');
    expect(harness.latest().pending.size).toBe(0);
  });

  it('disconnect clears pending on the backend\'s bare {disconnected:true} reply without inventing a status', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([{ id: 'a', state: 'REPORTING' }])) // bootstrap
      .mockResolvedValueOnce(jsonResponse({ disconnected: true })); // disconnect

    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      harness.latest().disconnect('a');
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(harness.latest().pending.has('a')).toBe(false);
    expect(harness.latest().actionError).toBeNull();
    // Still REPORTING: the disconnect response carries no Status, so only the
    // acquisition event the backend publishes moves the fold to RELEASED.
    expect(harness.latest().connections).toEqual([{ id: 'a', state: 'REPORTING', vehicleKeys: [] }]);

    act(() => {
      harness.latest().dispatchStream({
        kind: 'acquisition', event: { id: 'a', state: 'RELEASED', vehicleKeys: [] }, receivedAtMs: 1,
      });
    });
    expect(harness.latest().connections).toEqual([{ id: 'a', state: 'RELEASED', vehicleKeys: [] }]);
  });

  it('saveProfile chains into connectProfile once the profile is saved', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([])) // bootstrap
      .mockResolvedValueOnce(jsonResponse({ id: 'p1', name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 57600 } })) // save
      .mockResolvedValueOnce(jsonResponse({ id: 'p1', state: 'OPENING' })); // connect

    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      harness.latest().saveProfile({ name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 57600 } });
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(harness.latest().profiles).toEqual([{ id: 'p1', name: 'Bench', deviceId: '/dev/x', settings: { baudRate: 57600 } }]);
    expect(harness.latest().connections).toEqual([{ id: 'p1', state: 'OPENING', vehicleKeys: [] }]);
  });

  it('deleteProfile removes it from the profile list on success', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse([])) // bootstrap
      .mockResolvedValueOnce(jsonResponse([{ id: 'p1', name: 'Bench', device: { id: '/dev/x' }, settings: { baud_rate: 57600 } }])) // refreshProfiles
      .mockResolvedValueOnce(new Response(null, { status: 204 })); // delete

    const harness = mount(true);
    await act(async () => { await Promise.resolve(); });
    await act(async () => { harness.latest().refreshProfiles(); await Promise.resolve(); });
    expect(harness.latest().profiles).toHaveLength(1);

    await act(async () => {
      harness.latest().deleteProfile('p1');
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(harness.latest().profiles).toEqual([]);
  });
});
