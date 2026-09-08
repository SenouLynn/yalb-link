/** Imperative MapLibre adapter for one vehicle and its breadcrumb track. */

import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import { useEffect, useRef } from 'react';

import type { GeoPoint } from '@/logic/geoTrack';
import type { PositionResult } from '@/logic/position';
import type { GeoCoordinate } from '@/logic/trajectory';
import type { MissionGeometry } from '@/mission/model';

import { DEFAULT_BASEMAP, type TileSource } from './tileSource';

export interface MapPanelProps {
  position: PositionResult | null;
  track: GeoPoint[];
  /** Five-second prediction; empty means the inputs are unavailable or stale. */
  trajectory?: GeoCoordinate[];
  mission?: MissionGeometry;
  tileSource?: TileSource;
}

const TRACK_SOURCE = 'track';
const TRACK_LAYER = 'track-line';
const TRAJECTORY_SOURCE = 'trajectory';
const TRAJECTORY_LAYER = 'trajectory-line';
const MISSION_SOURCE = 'mission';
const MISSION_LINE_LAYER = 'mission-line';
const MISSION_POINT_LAYER = 'mission-points';

/** The single coordinate-order flip: this codebase is lat-first, MapLibre is lng-first. */
export function toLngLat(latDeg: number, lonDeg: number): [number, number] {
  return [lonDeg, latDeg];
}

export function buildStyle(tileSource: TileSource): maplibregl.StyleSpecification {
  return {
    version: 8,
    sources: {
      basemap: {
        type: 'raster',
        tiles: tileSource.tiles,
        tileSize: tileSource.tileSize,
        maxzoom: tileSource.maxZoom,
        attribution: tileSource.attribution,
      },
    },
    layers: [
      { id: 'background', type: 'background', paint: { 'background-color': '#0d1013' } },
      { id: 'basemap', type: 'raster', source: 'basemap' },
    ],
  };
}

function lineFeature(points: GeoCoordinate[]) {
  return {
    type: 'Feature' as const,
    properties: {},
    geometry: {
      type: 'LineString' as const,
      coordinates: points.map((point) => toLngLat(point.latDeg, point.lonDeg)),
    },
  };
}

/**
 * The overlay layers, separated from the map so their capabilities can be
 * asserted without a WebGL context.
 *
 * Every layer here draws geometry only. Mission sequence numbers are text, and
 * `text-field` needs a symbol layer, which needs a glyph server the style
 * deliberately does not have — so they are DOM markers instead. See
 * `missionLabelMarkers`.
 */
export function flightLayers(): maplibregl.LayerSpecification[] {
  return [
    {
      id: TRACK_LAYER,
      type: 'line',
      source: TRACK_SOURCE,
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: { 'line-color': '#74d7ff', 'line-width': 2.5, 'line-opacity': 0.9 },
    },
    {
      id: TRAJECTORY_LAYER,
      type: 'line',
      source: TRAJECTORY_SOURCE,
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: {
        'line-color': '#d9a441',
        'line-width': 3,
        'line-opacity': 0.95,
        'line-dasharray': [2, 2],
      },
    },
    {
      id: MISSION_LINE_LAYER,
      type: 'line',
      source: MISSION_SOURCE,
      filter: ['==', '$type', 'LineString'],
      paint: { 'line-color': '#c7f0ff', 'line-width': 3 },
    },
    {
      id: MISSION_POINT_LAYER,
      type: 'circle',
      source: MISSION_SOURCE,
      filter: ['==', '$type', 'Point'],
      paint: {
        'circle-radius': 10,
        'circle-color': '#14171c',
        'circle-stroke-color': '#c7f0ff',
        'circle-stroke-width': 2,
      },
    },
  ];
}

function addFlightLayers(
  map: maplibregl.Map,
  track: GeoPoint[],
  trajectory: GeoCoordinate[],
  mission: MissionGeometry,
): void {
  map.addSource(TRACK_SOURCE, { type: 'geojson', data: lineFeature(track) });
  map.addSource(TRAJECTORY_SOURCE, { type: 'geojson', data: lineFeature(trajectory) });
  map.addSource(MISSION_SOURCE, { type: 'geojson', data: missionFeatures(mission) });

  for (const layer of flightLayers()) {
    map.addLayer(layer);
  }
}

export function missionFeatures(mission: MissionGeometry) {
  const coordinates = mission.points.map((point) => toLngLat(point.latDeg, point.lonDeg));
  const line = coordinates.length < 2 ? [] : [
    { type: 'Feature' as const, properties: {}, geometry: { type: 'LineString' as const, coordinates } },
  ];
  return { type: 'FeatureCollection' as const, features: [
    ...line,
    ...mission.points.map((point) => ({ type: 'Feature' as const, properties: { label: String(point.seq) }, geometry: { type: 'Point' as const, coordinates: toLngLat(point.latDeg, point.lonDeg) } })),
  ] };
}

/**
 * Builds the sequence-number badge that sits inside a mission point.
 *
 * This is a DOM overlay rather than a `symbol` layer because MapLibre cannot
 * rasterise `text-field` without a `glyphs` endpoint, and adding one would put
 * a second public network dependency next to the prototyping basemap. The
 * badge is centred on the same coordinate as its circle.
 */
function createMissionLabelElement(label: string): HTMLElement {
  const element = document.createElement('div');
  element.classList.add('mission-marker');
  element.textContent = label;
  return element;
}

function createVehicleMarkerElement(): HTMLElement {
  const element = document.createElement('div');
  element.classList.add('vehicle-marker');
  element.innerHTML = `<svg width="28" height="28" viewBox="0 0 28 28" aria-hidden="true">
    <polygon points="14,3 21,24 14,19 7,24" fill="#d9a441" stroke="#14171c" stroke-width="1.5" stroke-linejoin="round" />
  </svg>`;
  return element;
}

export function MapPanel({
  position,
  track,
  trajectory = [],
  mission = { points: [], omitted: {} },
  tileSource = DEFAULT_BASEMAP,
}: MapPanelProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markerRef = useRef<maplibregl.Marker | null>(null);
  const loadedRef = useRef(false);
  const centredRef = useRef(false);
  const trackRef = useRef(track);
  const trajectoryRef = useRef(trajectory);
  const missionRef = useRef(mission);
  const missionMarkersRef = useRef<maplibregl.Marker[]>([]);

  trackRef.current = track;
  trajectoryRef.current = trajectory;
  missionRef.current = mission;

  useEffect(() => {
    if (containerRef.current === null) {
      return;
    }

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: buildStyle(tileSource),
      center: [0, 0],
      zoom: 1,
      attributionControl: { compact: true },
    });
    mapRef.current = map;

    map.on('load', () => {
      loadedRef.current = true;
      addFlightLayers(map, trackRef.current, trajectoryRef.current, missionRef.current);
    });

    const observer = new ResizeObserver(() => map.resize());
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      map.remove();
      mapRef.current = null;
      markerRef.current = null;
      missionMarkersRef.current = [];
      loadedRef.current = false;
      centredRef.current = false;
    };
    // Creation is one-shot; the next effect owns any later style changes.
  }, []);

  useEffect(() => {
    const map = mapRef.current;

    if (map === null || !loadedRef.current) {
      return;
    }

    void map.setStyle(buildStyle(tileSource));
    void map.once('styledata', () => {
      if (map.getSource(TRACK_SOURCE) === undefined) {
        addFlightLayers(map, trackRef.current, trajectoryRef.current, missionRef.current);
      }
    });
  }, [tileSource]);

  useEffect(() => {
    const map = mapRef.current;

    if (map === null) {
      return;
    }

    if (position === null) {
      markerRef.current?.remove();
      markerRef.current = null;
      return;
    }

    const lngLat = toLngLat(position.latDeg, position.lonDeg);

    if (markerRef.current === null) {
      markerRef.current = new maplibregl.Marker({ element: createVehicleMarkerElement() })
        .setLngLat(lngLat)
        .addTo(map);
    } else {
      markerRef.current.setLngLat(lngLat);
    }

    if (!centredRef.current) {
      map.jumpTo({ center: lngLat, zoom: Math.min(16, tileSource.maxZoom) });
      centredRef.current = true;
    } else if (!map.isMoving()) {
      map.jumpTo({ center: lngLat });
    }
  }, [position, tileSource.maxZoom]);

  useEffect(() => {
    const source = mapRef.current?.getSource<maplibregl.GeoJSONSource>(TRACK_SOURCE);

    if (loadedRef.current && source !== undefined) {
      source.setData(lineFeature(track));
    }
  }, [track]);

  useEffect(() => {
    const source = mapRef.current?.getSource<maplibregl.GeoJSONSource>(TRAJECTORY_SOURCE);

    if (loadedRef.current && source !== undefined) {
      source.setData(lineFeature(trajectory));
    }
  }, [trajectory]);

  useEffect(() => {
    const source = mapRef.current?.getSource<maplibregl.GeoJSONSource>(MISSION_SOURCE);
    if (loadedRef.current && source !== undefined) source.setData(missionFeatures(mission));
  }, [mission]);

  // Sequence numbers are markers, not a layer, so they are rebuilt here rather
  // than following the source data. Missions are tens of items, so replacing
  // the whole set costs less than reconciling it.
  useEffect(() => {
    for (const marker of missionMarkersRef.current) {
      marker.remove();
    }

    const map = mapRef.current;

    if (map === null) {
      missionMarkersRef.current = [];
      return;
    }

    missionMarkersRef.current = mission.points.map((point) =>
      new maplibregl.Marker({ element: createMissionLabelElement(String(point.seq)) })
        .setLngLat(toLngLat(point.latDeg, point.lonDeg))
        .addTo(map),
    );
  }, [mission]);

  return (
    <div className="map-shell">
      <div ref={containerRef} className="map-panel" aria-label="Vehicle position map" />
      {trajectory.length > 1 ? <div className="trajectory-key">5 s prediction</div> : null}
      {mission.points.length > 0 ? <div className="mission-key">Commanded mission</div> : null}
    </div>
  );
}
