import type { TelemetrySample } from '../sample';

export interface FlightPathFixture {
  name: string;
  input: Partial<TelemetrySample>;
  expected: {
    climbMps: number;
    groundSpeedMps: number;
    source: 'VFR_HUD' | 'GLOBAL_POSITION_INT';
  } | null;
}

export const flightPathFixtures: FlightPathFixture[] = [
  {
    // VFR_HUD climb is already positive-up. It passes through unflipped.
    name: 'vfr-hud-primary-climb-no-flip',
    input: { climbMps: 3.5, groundspeedMps: 15, headingDeg: 90 },
    expected: { climbMps: 3.5, groundSpeedMps: 15, source: 'VFR_HUD' },
  },
  {
    name: 'vfr-hud-primary-descent-no-flip',
    input: { climbMps: -2.5, groundspeedMps: 15, headingDeg: 90 },
    expected: { climbMps: -2.5, groundSpeedMps: 15, source: 'VFR_HUD' },
  },
  {
    // NED vz is positive down, so descending at 5 m/s is climb -5. Note the
    // absence of any /100: the proto boundary already normalised cm/s to m/s.
    name: 'gpi-fallback-descending-ned-flip',
    input: { vzMs: 5, vxMs: 12, vyMs: 0, headingDeg: 0 },
    expected: { climbMps: -5, groundSpeedMps: 12, source: 'GLOBAL_POSITION_INT' },
  },
  {
    name: 'gpi-fallback-climbing-ned-flip',
    input: { vzMs: -4, vxMs: 9, vyMs: 0, headingDeg: 0 },
    expected: { climbMps: 4, groundSpeedMps: 9, source: 'GLOBAL_POSITION_INT' },
  },
  {
    // Both present: VFR_HUD wins and source names it. Sources are never mixed
    // — a result reporting VFR_HUD climb against GPI-derived speed would be
    // untraceable.
    name: 'source-names-the-winner',
    input: { climbMps: 1.5, vzMs: 5, groundspeedMps: 20, vxMs: 12, vyMs: 0, headingDeg: 45 },
    expected: { climbMps: 1.5, groundSpeedMps: 20, source: 'VFR_HUD' },
  },
  {
    name: 'missing-climb-source',
    input: { groundspeedMps: 15, headingDeg: 90 },
    expected: null,
  },
];
