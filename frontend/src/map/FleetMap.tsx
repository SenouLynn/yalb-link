/** One shared map with keyed markers, explicit camera actions and no follow loop. */
import maplibregl from 'maplibre-gl';
import { useEffect, useRef, type RefObject } from 'react';
import type { FleetPosition } from '@/fleet/overview';
import { buildStyle } from './MapPanel';
import { DEFAULT_BASEMAP } from './tileSource';
import { framePoints, type Frame } from './camera';

export interface SavedCamera { center: [number, number]; zoom: number; bearing: number; pitch: number }
export function FleetMap({ positions, onOpen, camera, request }: {
  positions: FleetPosition[];
  onOpen: (key: string) => void;
  camera: RefObject<SavedCamera | null>;
  request: { frame: Frame } | null;
}) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const markers = useRef(new Map<string, maplibregl.Marker>());
  const open = useRef(onOpen);
  open.current = onOpen;
  const fitted = useRef(camera.current !== null);
  const apply = (frame: Frame) => {
    if ('bounds' in frame) map.current?.fitBounds(frame.bounds, { padding: 60, maxZoom: frame.maxZoom, duration: 0 });
    else map.current?.jumpTo(frame);
  };
  useEffect(() => {
    if (!container.current) return;
    const instance = new maplibregl.Map({
      container: container.current, style: buildStyle(DEFAULT_BASEMAP),
      ...(camera.current ?? { center: [0, 0] as [number, number], zoom: 1 }),
      attributionControl: { compact: true },
    });
    map.current = instance;
    instance.on('movestart', event => { if (event.originalEvent) fitted.current = true; });
    fitted.current = camera.current !== null;
    const observer = new ResizeObserver(() => instance.resize());
    observer.observe(container.current);
    return () => {
      const center = instance.getCenter();
      camera.current = fitted.current ? { center: [center.lng, center.lat], zoom: instance.getZoom(), bearing: instance.getBearing(), pitch: instance.getPitch() } : null;
      observer.disconnect();
      for (const marker of markers.current.values()) marker.remove();
      markers.current.clear();
      instance.remove();
      map.current = null;
    };
  }, []);
  useEffect(() => {
    const instance = map.current;
    if (!instance) return;
    const keys = new Set(positions.map(p => p.key));
    for (const [key, marker] of markers.current) {
      if (!keys.has(key)) { marker.remove(); markers.current.delete(key); }
    }
    for (const position of positions) {
      let marker = markers.current.get(position.key);
      if (!marker) {
        // design-exemption: imperative MapLibre marker uses the lever class.
        const button = document.createElement('button');
        button.type = 'button'; button.className = 'fleet-marker lever';
        button.textContent = position.key;
        button.setAttribute('aria-label', `Open vehicle ${position.key}`);
        button.onclick = () => { open.current(position.key); };
        marker = new maplibregl.Marker({ element: button }).setLngLat([position.lonDeg, position.latDeg]).addTo(instance);
        markers.current.set(position.key, marker);
      }
      marker.setLngLat([position.lonDeg, position.latDeg]);
    }
    if (!fitted.current) {
      const frame = framePoints(positions);
      if (frame) { apply(frame); fitted.current = true; }
    }
  }, [positions]);
  useEffect(() => { if (request) { apply(request.frame); fitted.current = true; } }, [request]);
  return <div className="map-shell fleet-map">
    <div ref={container} className="map-panel" aria-label="Fleet position map" />
    {positions.length === 0 && <p className="fleet-map__empty">No fresh vehicle positions</p>}
  </div>;
}
