import type { HeadingResult } from '../heading';
import type { TelemetrySample } from '../sample';

export interface HeadingFixture {
  name: string;
  input: Partial<TelemetrySample>;
  expected: HeadingResult | null;
}

export const headingFixtures: HeadingFixture[] = [
  {
    name: 'primary-vfr-hud',
    input: { headingDeg: 270 },
    expected: { headingDeg: 270, source: 'VFR_HUD', isFallback: false },
  },
  {
    // VFR_HUD.heading is int16 on the wire; ArduPilot may report it negative.
    // This is the real gap on this path — not a 65535 sentinel, which an
    // int16 cannot hold.
    name: 'primary-vfr-hud-negative-normalised',
    input: { headingDeg: -90 },
    expected: { headingDeg: 270, source: 'VFR_HUD', isFallback: false },
  },
  {
    name: 'fallback-attitude-yaw',
    input: { yawRad: Math.PI / 2 },
    expected: { headingDeg: 90, source: 'ATTITUDE', isFallback: true },
  },
  {
    name: 'fallback-attitude-yaw-negative-normalised',
    input: { yawRad: -Math.PI / 2 },
    expected: { headingDeg: 270, source: 'ATTITUDE', isFallback: true },
  },
  {
    name: 'fallback-global-position-hdg',
    input: { hdgCdeg: 18000 },
    expected: { headingDeg: 180, source: 'GLOBAL_POSITION_INT', isFallback: true },
  },
  {
    // UINT16_MAX is the unknown sentinel on GLOBAL_POSITION_INT.hdg. This is
    // the only source where it can legitimately appear.
    name: 'gpi-unknown-sentinel-rejected',
    input: { hdgCdeg: 65535 },
    expected: null,
  },
  {
    name: 'vfr-hud-wins-over-attitude',
    input: { headingDeg: 45, yawRad: Math.PI },
    expected: { headingDeg: 45, source: 'VFR_HUD', isFallback: false },
  },
  {
    name: 'missing-all-sources',
    input: {},
    expected: null,
  },
];
