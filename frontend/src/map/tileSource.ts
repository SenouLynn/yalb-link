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

export const DEFAULT_BASEMAP: TileSource = {
  id: 'streets',
  label: 'Streets (OSM)',
  tiles: expandSubdomains('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', ['a', 'b', 'c']),
  attribution: OSM_ATTRIBUTION,
  maxZoom: 19,
  tileSize: 256,
};

export const BASEMAPS: TileSource[] = [
  DEFAULT_BASEMAP,
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
    id: 'dark',
    label: 'Dark',
    tiles: expandSubdomains('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png', [
      'a',
      'b',
      'c',
      'd',
    ]),
    attribution: `${OSM_ATTRIBUTION}, &copy; CARTO`,
    maxZoom: 20,
    tileSize: 256,
    dark: true,
  },
  {
    id: 'light',
    label: 'Light',
    tiles: expandSubdomains('https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png', [
      'a',
      'b',
      'c',
      'd',
    ]),
    attribution: `${OSM_ATTRIBUTION}, &copy; CARTO`,
    maxZoom: 20,
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
