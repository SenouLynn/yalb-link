/** Camera policy shared by mission framing and explicit fleet actions. */
import type { GeoCoordinate } from '@/logic/trajectory';

export type Frame = { center: [number, number]; zoom: number }
  | { bounds: [[number, number], [number, number]]; maxZoom: number };

/** Smallest longitude arc, including missions/fleets crossing the date line. */
export function framePoints(points: readonly GeoCoordinate[], maxZoom = 16): Frame | null {
  const valid = points.filter(p => Number.isFinite(p.latDeg) && Math.abs(p.latDeg) <= 90
    && Number.isFinite(p.lonDeg) && Math.abs(p.lonDeg) <= 180);
  if (valid.length === 0) return null;
  const longitudes = valid.map(p => (p.lonDeg + 360) % 360).sort((a, b) => a - b);
  let gap = -1;
  let start = 0;
  for (let i = 0; i < longitudes.length; i += 1) {
    const current = (longitudes[i] ?? 0);
    const next = (longitudes[(i + 1) % longitudes.length] ?? 0) + (i === longitudes.length - 1 ? 360 : 0);
    if (next - current > gap) { gap = next - current; start = next % 360; }
  }
  let west = start;
  let east = start + 360 - gap;
  if (west > 180) { west -= 360; east -= 360; }
  const south = Math.max(-85, Math.min(85, Math.min(...valid.map(p => p.latDeg))));
  const north = Math.max(-85, Math.min(85, Math.max(...valid.map(p => p.latDeg))));
  if (west === east && south === north) return { center: [west, south], zoom: maxZoom };
  return { bounds: [[west, south], [east, north]], maxZoom };
}

export type FollowMode = 'follow' | 'mission' | 'manual';
export function nextFollowMode(mode: FollowMode, action: 'pan' | 'follow' | 'mission', hasPoints = true): FollowMode {
  if (action === 'pan') return 'manual';
  if (action === 'follow') return 'follow';
  return hasPoints ? 'mission' : mode;
}
export function shouldFollow(mode: FollowMode, moving: boolean): boolean {
  return mode === 'follow' && !moving;
}
