import { expect, it } from 'vitest';
import { framePoints, nextFollowMode, shouldFollow } from './camera';
it('handles empty, invalid and single-point extents', () => {
  expect(framePoints([])).toBeNull();
  expect(framePoints([{ latDeg: NaN, lonDeg: 0 }])).toBeNull();
  expect(framePoints([{ latDeg: 37, lonDeg: -122 }])).toEqual({ center: [-122, 37], zoom: 16 });
});
it('frames all positional points and bounds identical points', () => {
  expect(framePoints([{latDeg: 37, lonDeg: -122}, {latDeg: 38, lonDeg: -120}]))
    .toEqual({bounds: [[-122, 37], [-120, 38]], maxZoom: 16});
  expect(framePoints([{latDeg: 37, lonDeg: -122}, {latDeg: 37, lonDeg: -122}]))
    .toEqual({center: [-122, 37], zoom: 16});
});
it('uses the short arc across the date line', () => {
  expect(framePoints([{latDeg: 0, lonDeg: 179}, {latDeg: 1, lonDeg: -179}]))
    .toEqual({bounds: [[179, 0], [181, 1]], maxZoom: 16});
});
it('does not undo mission framing or a pan until following is explicitly restored', () => {
  const mission = nextFollowMode('follow', 'mission');
  expect(shouldFollow(mission, false)).toBe(false);
  expect(shouldFollow(nextFollowMode(mission, 'pan'), false)).toBe(false);
  expect(shouldFollow(nextFollowMode('manual', 'follow'), false)).toBe(true);
  expect(shouldFollow('follow', true)).toBe(false);
  expect(nextFollowMode('manual', 'mission', false)).toBe('manual');
});
