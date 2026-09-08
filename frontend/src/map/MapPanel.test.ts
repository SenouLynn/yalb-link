import { describe, expect, it } from 'vitest';

import { missionFeatures } from './MapPanel';

describe('missionFeatures', () => {
  it('emits no invalid line for an empty mission', () => {
    expect(missionFeatures({ points: [], omitted: {} }).features).toEqual([]);
  });

  it('retains a single waypoint marker without inventing a line', () => {
    const features = missionFeatures({
      points: [{ seq: 3, latDeg: 47.6, lonDeg: -122.3 }],
      omitted: {},
    }).features;

    expect(features).toHaveLength(1);
    expect(features[0]).toMatchObject({
      properties: { label: '3' },
      geometry: { type: 'Point', coordinates: [-122.3, 47.6] },
    });
  });

  it('emits the ordered route once two positions exist', () => {
    const features = missionFeatures({
      points: [
        { seq: 1, latDeg: 47.6, lonDeg: -122.3 },
        { seq: 4, latDeg: 47.7, lonDeg: -122.2 },
      ],
      omitted: {},
    }).features;

    expect(features[0]).toMatchObject({
      geometry: {
        type: 'LineString',
        coordinates: [[-122.3, 47.6], [-122.2, 47.7]],
      },
    });
    expect(features).toHaveLength(3);
  });
});
