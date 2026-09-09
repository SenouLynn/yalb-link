// @vitest-environment happy-dom

import { act, useEffect } from 'react';
import { create, toJson } from '@bufbuild/protobuf';
import { MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { initialFleetState, type FleetState, type VehicleView } from '@/fleet/state';

const maps = vi.hoisted(() => ({ mounts: 0 }));
vi.mock('@/map/MapPanel', () => ({
  MapPanel: () => { useEffect(() => { maps.mounts += 1; }, []); return <div aria-label="Vehicle position map" />; },
}));

import { FlightDisplay } from './FlightDisplay';

const view = (sysId: number): VehicleView => ({
  key: `${String(sysId)}:1`,
  sysId,
  compId: 1,
  lifecycle: undefined,
  heartbeat: undefined,
  lastFleetAtMs: undefined,
  sample: { sourceMessage: 'NONE', receivedAtMs: 0 },
  track: [],
  familySeenMs: {},
  lastSeenMs: 0,
});

function fleet(...views: VehicleView[]): FleetState {
  const vehicles = Object.fromEntries(views.map((item) => [item.key, item]));
  return {
    ...initialFleetState,
    vehicles,
    order: views.map((item) => item.key),
    selected: views[0]?.key ?? null,
  };
}

function display(state: FleetState) {
  return <FlightDisplay fleet={state} nowMs={0} source="live" onSelect={() => undefined} />;
}

afterEach(() => vi.unstubAllGlobals());

describe('FlightDisplay lifecycle', () => {
  it('rerenders from no vehicle to a selected vehicle and back', () => {
    const container = document.createElement('div');
    const root = createRoot(container);
    act(() => {
      root.render(display(initialFleetState));
    });
    expect(container.textContent).toContain('No vehicle');

    act(() => {
      root.render(display(fleet(view(1))));
    });
    expect(container.textContent).toContain('Onboard mission');

    act(() => {
      root.render(display(initialFleetState));
    });
    expect(container.textContent).toContain('No vehicle');

    act(() => {
      root.unmount();
    });
  });

  it('cancels the prior mission request when selection changes', () => {
    let signal: AbortSignal | undefined;
    vi.stubGlobal('fetch', vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      signal = init?.signal instanceof AbortSignal ? init.signal : undefined;
      return new Promise<Response>(() => undefined);
    }));

    const container = document.createElement('div');
    const root = createRoot(container);
    act(() => {
      root.render(display(fleet(view(1))));
    });
    const button = Array.from(container.querySelectorAll('button')).find(
      (candidate) => candidate.textContent === 'Download mission',
    );
    if (button === undefined) throw new Error('mission download button was not rendered');
    act(() => {
      button.click();
    });
    expect(signal?.aborted).toBe(false);

    act(() => {
      root.render(display(fleet(view(2))));
    });
    expect(signal?.aborted).toBe(true);
    expect(container.textContent).toContain('Not downloaded');

    act(() => {
      root.unmount();
    });
  });

});

describe('workspace ownership', () => {
  it('keeps a pending download and map DOM through all pane toggles, then isolates A → B → A and empty', () => {
    let signal: AbortSignal | undefined;
    vi.stubGlobal('fetch', vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      signal = init?.signal instanceof AbortSignal ? init.signal : undefined;
      return new Promise<Response>(() => undefined);
    }));
    const container = document.createElement('div');
    const root = createRoot(container);
    act(() => { root.render(display(fleet(view(1)))); });
    const map = container.querySelector('[aria-label="Vehicle position map"]');
    const mounts = maps.mounts;
    const download = Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Download mission');
    act(() => { download?.click(); });
    for (const id of ['instruments', 'map', 'mission']) {
      const toggle = container.querySelector<HTMLButtonElement>(`[aria-controls="pane-${id}"]`);
      act(() => { toggle?.click(); });
      expect(container.querySelector<HTMLElement>(`#pane-${id}`)?.hidden).toBe(true);
      expect(signal?.aborted).toBe(false);
      expect(maps.mounts).toBe(mounts);
      expect(container.querySelector('[aria-label="Vehicle position map"]')).toBe(map);
    }
    expect(container.textContent).toContain('All panels hidden');
    act(() => { container.querySelector<HTMLButtonElement>('[aria-controls="pane-mission"]')?.click(); });
    expect(container.querySelector('.mission-panel')?.textContent).toContain('Downloading');
    act(() => { root.render(display(fleet(view(2)))); });
    expect(signal?.aborted).toBe(true);
    act(() => { root.render(display(fleet(view(1)))); });
    expect(container.textContent).toContain('Not downloaded');
    act(() => { download?.click(); });
    act(() => { root.render(display(initialFleetState)); });
    expect(signal?.aborted).toBe(true);
    expect(container.querySelector('.workspace__topbar')).not.toBeNull();
    act(() => { root.unmount(); });
  });

  it('retains arm confirmation on pane toggles and clears it when switching vehicles', () => {
    const container = document.createElement('div');
    const root = createRoot(container);
    act(() => { root.render(display({ ...fleet(view(1)), connected: true })); });
    const arm = () => container.querySelector<HTMLButtonElement>('.command-control button');
    act(() => { arm()?.click(); });
    expect(arm()?.textContent).toBe('CONFIRM ARM');
    act(() => { container.querySelector<HTMLButtonElement>('[aria-controls="pane-map"]')?.click(); });
    expect(arm()?.textContent).toBe('CONFIRM ARM');
    act(() => { root.render(display({ ...fleet(view(2)), connected: true })); });
    expect(arm()?.textContent).toBe('ARM');
    act(() => { root.unmount(); });
  });
});

it('retains a completed mission when hidden and shown, and restores focus outside a hidden pane', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(toJson(MissionSnapshotSchema,
    create(MissionSnapshotSchema, { vehicleId: { systemId: 1, componentId: 1 } }))))));
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  act(() => { root.render(display(fleet(view(1)))); });
  const download = container.querySelector<HTMLButtonElement>('.mission-panel button');
  await act(async () => { download?.click(); await Promise.resolve(); });
  expect(container.textContent).toContain('Complete');
  download?.focus();
  const toggle = container.querySelector<HTMLButtonElement>('[aria-controls="pane-mission"]');
  act(() => { toggle?.click(); });
  expect(document.activeElement).toBe(toggle);
  act(() => { toggle?.click(); });
  expect(container.textContent).toContain('Complete');
  expect(fetch).toHaveBeenCalledTimes(1);
  act(() => { root.unmount(); });
  container.remove();
});

it('keeps command busy and unresolved state through pane toggles', async () => {
  let resolve: ((response: Response) => void) | undefined;
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(done => { resolve = done; })));
  const container = document.createElement('div');
  const root = createRoot(container);
  act(() => { root.render(display({ ...fleet(view(1)), connected: true })); });
  const arm = () => container.querySelector<HTMLButtonElement>('.command-control button');
  act(() => { arm()?.click(); });
  act(() => { arm()?.click(); });
  expect(arm()?.textContent).toBe('WAITING…');
  act(() => { container.querySelector<HTMLButtonElement>('[aria-controls="pane-instruments"]')?.click(); });
  expect(arm()?.disabled).toBe(true);
  await act(async () => {
    resolve?.(new Response(JSON.stringify({ code: 'command_unresolved', transaction: { id: 1, state: 'COMMAND_STATE_TIMED_OUT' } }), { status: 409 }));
    await Promise.resolve();
  });
  expect(container.textContent).toContain('What the vehicle did is unknown');
  act(() => { container.querySelector<HTMLButtonElement>('[aria-controls="pane-instruments"]')?.click(); });
  expect(container.textContent).toContain('OBSERVED DISARMED');
  act(() => { root.unmount(); });
});
