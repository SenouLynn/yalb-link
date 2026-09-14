// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';

const { state, MockMap, MockMarker } = vi.hoisted(() => {
  const state = { maps: [] as MockMap[] };
  class MockMap {
    events: Record<string, () => void> = {};
    container: HTMLElement;
    constructor(options: { container: HTMLElement }) { state.maps.push(this); this.container = options.container; }
    on(name: string, fn: () => void) { this.events[name] = fn; }
    getCenter() { return { lng: 0, lat: 0 }; }
    getZoom() { return 1; }
    getBearing() { return 0; }
    getPitch() { return 0; }
    zoomTo = vi.fn();
    easeTo = vi.fn();
    jumpTo = vi.fn();
    fitBounds = vi.fn();
    resize = vi.fn();
    remove = vi.fn();
  }
  // Real maplibregl.Marker appends its element into the map container, which
  // is how the fleet marker button becomes queryable at all.
  class MockMarker {
    element: HTMLElement;
    constructor(options: { element: HTMLElement }) { this.element = options.element; }
    setLngLat() { return this; }
    addTo(map: MockMap) { map.container.appendChild(this.element); return this; }
    remove() { this.element.remove(); }
  }
  return { state, MockMap, MockMarker };
});
vi.mock('maplibre-gl', () => ({ default: { Map: MockMap, Marker: MockMarker } }));
import { FleetMap } from './FleetMap';

afterEach(() => { vi.unstubAllGlobals(); state.maps = []; });

it('shows an explicit imagery-unavailable chip when the fleet basemap fails, without hiding vehicles', () => {
  vi.stubGlobal('ResizeObserver', class { observe = vi.fn(); disconnect = vi.fn(); });
  const container = document.createElement('div');
  const root = createRoot(container);
  const positions = [{ key: '1', latDeg: 47, lonDeg: -122, altM: 0, altRef: 'RELATIVE' as const, source: 'GLOBAL_POSITION_INT' as const }];
  act(() => {
    root.render(
      <FleetMap positions={positions} onOpen={vi.fn()} camera={{ current: null }} request={null} />,
    );
  });
  const map = state.maps[0];
  if (!map) throw new Error('Map was not mounted');
  expect(container.textContent).not.toContain('Imagery unavailable');

  act(() => { (map.events['error'] as ((e: { sourceId: string }) => void) | undefined)?.({ sourceId: 'basemap' }); });
  expect(container.textContent).toContain('Imagery unavailable');
  // The vehicle marker button is unaffected — it is a DOM overlay, not a basemap tile.
  expect(container.querySelector('[aria-label="Open vehicle 1"]')).not.toBeNull();

  act(() => { map.events['style.load']?.(); });
  expect(container.textContent).not.toContain('Imagery unavailable');
  act(() => { root.unmount(); });
});
