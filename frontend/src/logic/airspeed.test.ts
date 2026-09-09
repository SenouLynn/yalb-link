import { describe, expect, it } from 'vitest';

import { resolveAirspeed } from './airspeed';
import { baseSample } from './testing';

describe('resolveAirspeed', () => {
  it('resolves airspeed from VFR_HUD', () => {
    expect(resolveAirspeed(baseSample({ airspeedMps: 18.4 }))).toEqual({
      airspeedMps: 18.4,
      source: 'VFR_HUD',
    });
  });

  it('is unresolved when VFR_HUD has not been heard', () => {
    expect(resolveAirspeed(baseSample())).toBeNull();
  });

  it('keeps a stationary zero rather than reporting no sensor', () => {
    expect(resolveAirspeed(baseSample({ airspeedMps: 0 }))?.airspeedMps).toBe(0);
  });

  it('names VFR_HUD, not the flight path source', () => {
    // Ground speed and climb can resolve from GLOBAL_POSITION_INT. Airspeed
    // cannot: only VFR_HUD carries it, and provenance must say so.
    const actual = resolveAirspeed(
      baseSample({ airspeedMps: 12, vxMs: 3, vyMs: 4, vzMs: -1 }),
    );

    expect(actual?.source).toBe('VFR_HUD');
  });
});
