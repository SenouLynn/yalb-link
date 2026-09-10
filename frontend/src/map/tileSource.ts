/** Plain-data basemap catalogue. MapLibre details stay in MapPanel. */

export interface TileSource {
  id: string;
  label: string;
  /** Fully expanded URLs: MapLibre does not expand Leaflet's `{s}` token. */
  tiles: string[];
  attribution: string;
  maxZoom: number;
  tileSize: number;
  dark?: boolean;
}

const OSM_ATTRIBUTION = '&copy; OpenStreetMap contributors';
const ESRI_ATTRIBUTION = 'Tiles &copy; Esri';

function expandSubdomains(template: string, subdomains: string[]): string[] {
  return subdomains.map((subdomain) => template.replace('{s}', subdomain));
}

const STREETS: TileSource = {
  id: 'streets',
  label: 'Streets (OSM)',
  tiles: expandSubdomains('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', ['a', 'b', 'c']),
  attribution: OSM_ATTRIBUTION,
  maxZoom: 19,
  tileSize: 256,
};

/*
 * Dark by default.
 *
 * The map is the majority of the screen, so whichever basemap is default
 * decides what the display looks like. Full-saturation OSM Streets puts
 * hundreds of multicolour POI glyphs on an instrument panel whose whole palette
 * is graphite and two signal hues — the tile vendor's design language becomes
 * the application's. A desaturated dark ground keeps the flown track, the
 * prediction and the commanded route as the only saturated things on the map,
 * which is what makes them readable. Streets remains in the catalogue.
 *
 * Esri's Dark Gray Canvas rather than CARTO's `dark_all`: CARTO's basemap CDN
 * now requires an API key and serves "API KEY REQUIRED" watermark tiles without
 * one, so the two CARTO entries this catalogue used to carry were dead. Esri's
 * canvas services are keyless and are designed as a ground for data overlay,
 * which is exactly this use.
 */
export const DEFAULT_BASEMAP: TileSource = {
  id: 'dark',
  label: 'Dark canvas',
  tiles: [
    'https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}',
  ],
  attribution: ESRI_ATTRIBUTION,
  maxZoom: 16,
  tileSize: 256,
  dark: true,
};

export const BASEMAPS: TileSource[] = [
  DEFAULT_BASEMAP,
  STREETS,
  {
    id: 'satellite',
    label: 'Satellite',
    tiles: [
      'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    ],
    attribution: ESRI_ATTRIBUTION,
    maxZoom: 19,
    tileSize: 256,
    dark: true,
  },
  {
    id: 'topo',
    label: 'Topographic',
    tiles: expandSubdomains('https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png', ['a', 'b', 'c']),
    attribution: `&copy; OpenTopoMap (CC-BY-SA), ${OSM_ATTRIBUTION}`,
    maxZoom: 17,
    tileSize: 256,
  },
  {
    id: 'relief',
    label: 'Terrain relief',
    tiles: [
      'https://server.arcgisonline.com/ArcGIS/rest/services/Elevation/World_Hillshade/MapServer/tile/{z}/{y}/{x}',
    ],
    attribution: ESRI_ATTRIBUTION,
    maxZoom: 16,
    tileSize: 256,
  },
  {
    id: 'light',
    label: 'Light canvas',
    tiles: [
      'https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}',
    ],
    attribution: ESRI_ATTRIBUTION,
    maxZoom: 16,
    tileSize: 256,
  },
];

export function findBasemap(id: string): TileSource {
  return BASEMAPS.find((basemap) => basemap.id === id) ?? DEFAULT_BASEMAP;
}

/** Creates a configuration for a locally served `{z}/{x}/{y}.png` tree. */
export function localTileSource(basePath = '/tiles', maxZoom = 16): TileSource {
  return {
    id: 'local',
    label: 'Local tiles',
    tiles: [`${basePath}/{z}/{x}/{y}.png`],
    attribution: 'Local tiles',
    maxZoom,
    tileSize: 256,
  };
}
