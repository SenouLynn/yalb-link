import type { BatteryResult } from '../battery';
import type { TelemetrySample } from '../sample';

export interface BatteryFixture {
  name: string;
  input: Partial<TelemetrySample>;
  expected: BatteryResult | null;
}

/** A 4S pack at 3.7 V per cell, in the 14-slot array BATTERY_STATUS uses. */
const FOUR_S = [3700, 3700, 3700, 3700, 65535, 65535, 65535, 65535, 65535, 65535];

export const batteryFixtures: BatteryFixture[] = [
  {
    name: 'primary-battery-status-sums-populated-cells',
    input: {
      batteryStatusCellVoltagesMv: FOUR_S,
      batteryStatusRemainingPct: 87,
      batteryStatusCurrentCa: 850,
    },
    expected: { voltageV: 14.8, remainingPct: 87, currentA: 8.5, source: 'BATTERY_STATUS' },
  },
  {
    // The whole reason the sentinel is filtered: summing the 65535 slots would
    // report a 4S pack as several hundred volts.
    name: 'primary-battery-status-ignores-unpopulated-slots',
    input: { batteryStatusCellVoltagesMv: [4200, 4200, 65535, 65535] },
    expected: { voltageV: 8.4, remainingPct: null, currentA: null, source: 'BATTERY_STATUS' },
  },
  {
    name: 'fallback-sys-status-voltage',
    input: {
      systemStatusVoltageMv: 12400,
      systemStatusRemainingPct: 64,
      systemStatusCurrentCa: 300,
    },
    expected: { voltageV: 12.4, remainingPct: 64, currentA: 3, source: 'SYS_STATUS' },
  },
  {
    name: 'fallback-sys-status-when-no-cell-is-populated',
    input: {
      batteryStatusCellVoltagesMv: [65535, 65535],
      systemStatusVoltageMv: 11000,
    },
    expected: { voltageV: 11, remainingPct: null, currentA: null, source: 'SYS_STATUS' },
  },
  {
    name: 'sentinel-unknown-sys-status-voltage-is-not-65-volts',
    input: { systemStatusVoltageMv: 65535, systemStatusRemainingPct: 40 },
    expected: { voltageV: null, remainingPct: 40, currentA: null, source: 'SYS_STATUS' },
  },
  {
    name: 'sentinel-not-provided-remaining-is-unknown-not-minus-one-percent',
    input: { systemStatusVoltageMv: 12000, systemStatusRemainingPct: -1 },
    expected: { voltageV: 12, remainingPct: null, currentA: null, source: 'SYS_STATUS' },
  },
  {
    name: 'sentinel-not-provided-current-is-unknown-not-minus-ten-milliamps',
    input: { systemStatusVoltageMv: 12000, systemStatusCurrentCa: -1 },
    expected: { voltageV: 12, remainingPct: null, currentA: null, source: 'SYS_STATUS' },
  },
  {
    // Charging draws negative current; that is a real reading, not a sentinel.
    name: 'negative-current-is-a-charging-reading',
    input: { systemStatusVoltageMv: 12000, systemStatusCurrentCa: -250 },
    expected: { voltageV: 12, remainingPct: null, currentA: -2.5, source: 'SYS_STATUS' },
  },
  {
    name: 'remaining-without-voltage-is-still-a-reading',
    input: { systemStatusRemainingPct: 15 },
    expected: { voltageV: null, remainingPct: 15, currentA: null, source: 'SYS_STATUS' },
  },
  {
    name: 'battery-status-remaining-without-cells-keeps-its-source',
    input: { batteryStatusCellVoltagesMv: [], batteryStatusRemainingPct: 80 },
    expected: { voltageV: null, remainingPct: 80, currentA: null, source: 'BATTERY_STATUS' },
  },
  {
    name: 'zero-percent-is-a-reading-not-an-absence',
    input: { systemStatusVoltageMv: 9600, systemStatusRemainingPct: 0 },
    expected: { voltageV: 9.6, remainingPct: 0, currentA: null, source: 'SYS_STATUS' },
  },
  {
    name: 'no-battery-message-yet',
    input: {},
    expected: null,
  },
  {
    name: 'empty-cell-array-is-no-reading',
    input: { batteryStatusCellVoltagesMv: [] },
    expected: null,
  },
];
