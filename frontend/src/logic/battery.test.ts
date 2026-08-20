import { describe, expect, it } from 'vitest';

import { batteryFixtures } from './__fixtures__/battery.fixtures';
import { resolveBattery } from './battery';
import { baseSample } from './testing';

describe('resolveBattery', () => {
  it.each(batteryFixtures)('$name', ({ input, expected }) => {
    const actual = resolveBattery(baseSample(input));

    if (expected === null) {
      expect(actual).toBeNull();

      return;
    }

    expect(actual).not.toBeNull();
    expect(actual?.source).toBe(expected.source);

    for (const field of ['voltageV', 'remainingPct', 'currentA'] as const) {
      const want = expected[field];

      if (want === null) {
        expect(actual?.[field]).toBeNull();
      } else {
        expect(actual?.[field]).toBeCloseTo(want, 6);
      }
    }
  });

  it('never reports a pack voltage from unpopulated cell slots alone', () => {
    const actual = resolveBattery(
      baseSample({ batteryStatusCellVoltagesMv: [65535, 65535, 65535] }),
    );

    expect(actual).toBeNull();
  });
});
