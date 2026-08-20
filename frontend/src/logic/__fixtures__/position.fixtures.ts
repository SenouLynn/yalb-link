import type { PositionResult } from '../position';
import type { TelemetrySample } from '../sample';

export interface PositionFixture {
  name: string;
  input: Partial<TelemetrySample>;
  expected: PositionResult | null;
}

// Seattle. Values are physically plausible on purpose: a reviewer spots a 1e7
// scaling error instantly in 47.6 and never in 476000000.
export const positionFixtures: PositionFixture[] = [
  {
    name: 'primary-global-position-relative-datum',
    input: {
      globalLatDeg: 47.6062,
      globalLonDeg: -122.3321,
      globalAltMslM: 120.5,
      globalAltRelativeM: 50.25,
    },
    expected: {
      latDeg: 47.6062,
      lonDeg: -122.3321,
      altM: 50.25,
      altRef: 'RELATIVE',
      source: 'GLOBAL_POSITION_INT',
    },
  },
  {
    // The datum is the assertion here, not the number. A fallback that reports
    // MSL as relative altitude is the failure altRef exists to catch, and at
    // this field elevation it would read ~120 m too high.
    name: 'fallback-gps-raw-msl-datum',
    input: { gpsLatDeg: 47.6062, gpsLonDeg: -122.3321, gpsAltMslM: 120.5 },
    expected: {
      latDeg: 47.6062,
      lonDeg: -122.3321,
      altM: 120.5,
      altRef: 'MSL',
      source: 'GPS_RAW_INT',
    },
  },
  {
    name: 'missing-altitude-entirely',
    input: { globalLatDeg: 47.6062, globalLonDeg: -122.3321 },
    expected: null,
  },
  {
    name: 'missing-fix',
    input: {},
    expected: null,
  },
];
