// @vitest-environment happy-dom
import { StrictMode, act } from 'react';
import { createRoot } from 'react-dom/client';
import { create } from '@bufbuild/protobuf';
import { afterEach, expect, it, vi } from 'vitest';
import { FleetEventSchema, FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { StreamEvent } from '@/stream/events';

const harness = vi.hoisted(() => ({ start: vi.fn(), stop: vi.fn() }));
vi.mock('@/stream/select', () => ({ selectStream: () => ({ stream: { start: harness.start }, source: 'live' }) }));
vi.mock('@/map/FleetMap', () => ({ FleetMap: () => <div aria-label="Fleet position map" /> }));
vi.mock('@/map/MapPanel', () => ({ MapPanel: () => <div /> }));
import App from './App';
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); });

it('keeps one active stream and selection across pane toggles and rerenders in StrictMode; cleans up its clock', () => {
  vi.useFakeTimers();
  let deliver: ((event: StreamEvent) => void) | undefined;
  harness.start.mockImplementation((sink: (event: StreamEvent) => void) => { deliver = sink; return harness.stop; });
  const container = document.createElement('div');
  const root = createRoot(container);
  act(() => { root.render(<StrictMode><App /></StrictMode>); });
  const starts = harness.start.mock.calls.length;
  expect(starts - harness.stop.mock.calls.length).toBe(1);
  act(() => {
    for (const systemId of [1, 2]) deliver?.({ kind: 'fleet', receivedAtMs: Date.now(), event: create(FleetEventSchema, {
      type: FleetEventType.VEHICLE_DISCOVERED, vehicleId: { systemId, componentId: 1 },
    }) });
  });
  expect(container.querySelector('[aria-label="Fleet roster"]')).not.toBeNull();
  act(() => { container.querySelector<HTMLButtonElement>('[aria-label="Open vehicle 2:1"]')?.click(); });
  const selector = Array.from(container.querySelectorAll<HTMLButtonElement>('.selector button'));
  act(() => { selector[1]?.click(); });
  const selectedText = container.querySelector('.selector [aria-pressed="true"]')?.textContent;
  expect(selectedText).toContain('2:1');
  for (const button of container.querySelectorAll<HTMLButtonElement>('[aria-controls]')) {
    act(() => { button.click(); });
  }
  act(() => { vi.advanceTimersByTime(1000); root.render(<StrictMode><App /></StrictMode>); });
  act(() => { Array.from(container.querySelectorAll('button')).find(b => b.textContent === 'Back to fleet')?.click(); });
  expect(container.querySelector('[aria-label="Fleet roster"]')).not.toBeNull();
  act(() => { container.querySelector<HTMLButtonElement>('[aria-label="Open vehicle 2:1"]')?.click(); });
  expect(harness.start).toHaveBeenCalledTimes(starts);
  expect(container.querySelector('.selector [aria-pressed="true"]')?.textContent).toBe(selectedText);
  act(() => { root.unmount(); });
  expect(harness.stop).toHaveBeenCalledTimes(starts);
  expect(vi.getTimerCount()).toBe(0);
});
