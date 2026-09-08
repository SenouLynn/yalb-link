// @vitest-environment happy-dom

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { initialFleetState, type FleetState, type VehicleView } from '@/fleet/state';

vi.mock('@/map/MapPanel', () => ({
  MapPanel: () => <div aria-label="Vehicle position map" />,
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
