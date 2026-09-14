// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';

const { state, MockMap } = vi.hoisted(() => {
const state = { maps: [] as MockMap[] };
class MockMap {
  events: Record<string, () => void> = {};
  sources = new Map<string, unknown>();
  layers: unknown[] = [];
  zoomTo = vi.fn();
  easeTo = vi.fn();
  jumpTo = vi.fn();
  constructor() { state.maps.push(this); }
  on(name: string, fn: () => void) { this.events[name] = fn; }
  setStyle() { this.sources.clear(); this.layers = []; }
  // A real GeoJSONSource stays live and mutable via `setData` after
  // `addSource` — mirror that so an effect racing a style reload (loaded
  // still true, source still the old style's) behaves like MapLibre rather
  // than throwing on a plain data object.
  addSource(id: string, value: { data: unknown }) {
    const entry: { data: unknown; setData: (data: unknown) => void } = {
      data: value.data,
      setData(data: unknown) { entry.data = data; },
    };
    this.sources.set(id, entry);
  }
  addLayer(layer: unknown) { this.layers.push(layer); }
  getSource(id: string) { return this.sources.get(id); }
  getContainer() { return document.createElement('div'); }
  getZoom() { return 10; }
  getPitch() { return 0; }
  resize = vi.fn();
  remove = vi.fn();
}
return { state, MockMap };
});
vi.mock('maplibre-gl', () => ({ default: { Map: MockMap } }));
import { MapPanel } from './MapPanel';
import { BASEMAPS } from './tileSource';

const satellite = BASEMAPS.find((source) => source.id === 'satellite');
const streets = BASEMAPS.find((source) => source.id === 'streets');
if (!satellite || !streets) throw new Error('Expected basemap catalogue entries are missing');

afterEach(() => { vi.unstubAllGlobals(); state.maps = []; });
it('restores the latest flight geometry after rapid basemap changes and exposes zoom', () => {
  // Basemap switching is no longer operator-facing (the selector was
  // removed), but the underlying config-driven restyle — `tileSource` prop
  // changing under a mounted map — still has to survive with overlays intact.
  vi.stubGlobal('ResizeObserver', class { observe = vi.fn(); disconnect = vi.fn(); });
  const container = document.createElement('div');
  const root = createRoot(container);
  const empty: { latDeg: number; lonDeg: number; atMs: number }[] = [];
  const track = [{ latDeg: 47, lonDeg: -122, atMs: 1 }];
  act(() => { root.render(<MapPanel position={null} track={empty} />); });
  const map = state.maps[0];
  if (!map) throw new Error('Map was not mounted');
  act(() => { map.events['style.load']?.(); });
  // `track` stays the same reference across the loop — only `tileSource`
  // changes — so this exercises rapid restyles, not a spurious track update.
  for (const source of [satellite, streets]) {
    act(() => { root.render(<MapPanel position={null} track={empty} tileSource={source} />); });
  }
  act(() => { root.render(<MapPanel position={null} track={track} tileSource={streets} />); });
  act(() => { map.events['style.load']?.(); });
  expect(map.sources.get('track')).toMatchObject({ data: { geometry: { coordinates: [[-122, 47]] } } });
  expect(map.sources.has('trajectory')).toBe(true);
  expect(map.sources.has('mission')).toBe(true);
  expect(map.layers).toHaveLength(4);
  act(() => { container.querySelector<HTMLButtonElement>('[aria-label="Zoom in"]')?.click(); });
  expect(map.zoomTo).toHaveBeenCalledWith(11, { duration: 200 });
  act(() => { container.querySelector<HTMLButtonElement>('[aria-label="Zoom out"]')?.click(); });
  expect(map.zoomTo).toHaveBeenCalledWith(9, { duration: 200 });
  act(() => { root.unmount(); });
});
