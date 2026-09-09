import { describe, expect, it } from 'vitest';

import { resolveHome } from './home';
import { baseSample } from './testing';

const homeSample = { homeLatDeg: 37.7749, homeLonDeg: -122.4194, homeAltMslM: 10.1 };

describe('resolveHome', () => {
  it('resolves home from HOME_POSITION', () => {
    expect(resolveHome(baseSample(homeSample))).toEqual({
      latDeg: 37.7749,
      lonDeg: -122.4194,
      altMslM: 10.1,
      source: 'HOME_POSITION',
    });
  });

  it('is unresolved before the family has been heard', () => {
    expect(resolveHome(baseSample())).toBeNull();
  });

  it('rejects the unset-home origin rather than pointing at Null Island', () => {
    // ArduPilot reports lat/lon 0 with time_usec 0 until home is set. Rendering
    // that as a position would put home in the Gulf of Guinea and, worse, make
    // "above home" altitude look like it had a datum when it does not.
    expect(resolveHome(baseSample({ homeLatDeg: 0, homeLonDeg: 0, homeAltMslM: 0 }))).toBeNull();
  });

  it('accepts a real position that has one zero component', () => {
    // Only the both-zero pair is the sentinel. Greenwich is a real longitude.
    const actual = resolveHome(
      baseSample({ homeLatDeg: 51.4779, homeLonDeg: 0, homeAltMslM: 47 }),
    );

    expect(actual?.latDeg).toBe(51.4779);
    expect(actual?.lonDeg).toBe(0);
  });

  it('is unresolved when the altitude is missing', () => {
    expect(resolveHome(baseSample({ homeLatDeg: 37.7749, homeLonDeg: -122.4194 }))).toBeNull();
  });
});
