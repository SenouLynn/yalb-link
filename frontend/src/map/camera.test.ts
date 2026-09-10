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
it('keeps one position at a bounded zoom when its longitude wrap does not round-trip', () => {
  // (lon + 360) % 360 rarely returns the input exactly, so a single vehicle used to
  // frame as a whole-globe span at its antipode instead of a bounded zoom.
  for (const lonDeg of [-122.4194001, -122.42, -64.11632079318275]) {
    const frame = framePoints([{ latDeg: 37.7748999, lonDeg }]);
    expect(frame).toMatchObject({ zoom: 16 });
    expect((frame as { center: [number, number] }).center[0]).toBeCloseTo(lonDeg, 9);
  }
});
it('never places a frame west edge east of its east edge', () => {
  for (let step = 0; step < 2000; step += 1) {
    const lonDeg = -180 + step * 0.1801;
    for (const points of [[{ latDeg: 10, lonDeg }], [{ latDeg: 10, lonDeg }, { latDeg: 11, lonDeg }]]) {
      const frame = framePoints(points);
      if (frame && 'bounds' in frame) expect(frame.bounds[0][0]).toBeLessThanOrEqual(frame.bounds[1][0]);
    }
  }
});
