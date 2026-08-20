/** Battery resolution: BATTERY_STATUS → SYS_STATUS, with the sentinels honoured. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface BatteryResult {
  /** Pack voltage, volts. */
  voltageV: number | null;
  /** State of charge, percent. */
  remainingPct: number | null;
  /** Pack current, amps. Negative means charging. */
  currentA: number | null;
  source: 'BATTERY_STATUS' | 'SYS_STATUS';
}

/** UINT16_MAX. On SYS_STATUS.voltage_battery it means unknown; on a
 *  BATTERY_STATUS cell slot it means the cell is not populated. */
const UNKNOWN_MV = 65535;

/** -1 is "not provided" for current and remaining charge. */
const NOT_PROVIDED = -1;

const MV_TO_V = 1e-3;
const CA_TO_A = 1e-2;

/**
 * Resolves the battery reading.
 *
 * Returns `null` only when neither message has been seen. Within a result the
 * individual fields are independently nullable, because the two messages carry
 * different subsets: a pack can report per-cell voltages while declining to
 * estimate state of charge, and an operator is better served by a voltage with
 * an unknown percentage than by neither.
 */
export function resolveBattery(sample: TelemetrySample): BatteryResult | null {
  const cells = cellVoltageV(sample.batteryStatusCellVoltagesMv);
  const batteryRemaining = percentOf(sample.batteryStatusRemainingPct);
  const batteryCurrent = currentOf(sample.batteryStatusCurrentCa);

  if (cells !== null || batteryRemaining !== null || batteryCurrent !== null) {
    return {
      voltageV: cells,
      remainingPct: batteryRemaining,
      currentA: batteryCurrent,
      source: 'BATTERY_STATUS',
    };
  }

  if (isNum(sample.systemStatusVoltageMv) && sample.systemStatusVoltageMv !== UNKNOWN_MV) {
    return {
      voltageV: sample.systemStatusVoltageMv * MV_TO_V,
      remainingPct: percentOf(sample.systemStatusRemainingPct),
      currentA: currentOf(sample.systemStatusCurrentCa),
      source: 'SYS_STATUS',
    };
  }

  // SYS_STATUS charge without voltage is still a reading worth showing.
  const remaining = percentOf(sample.systemStatusRemainingPct);

  if (remaining !== null) {
    return {
      voltageV: null,
      remainingPct: remaining,
      currentA: currentOf(sample.systemStatusCurrentCa),
      source: 'SYS_STATUS',
    };
  }

  return null;
}

/**
 * Sums the populated cells.
 *
 * Unpopulated slots carry 65535 and must be dropped rather than added: a
 * 4-cell pack reported in a 14-slot array would otherwise read as 670 volts.
 */
function cellVoltageV(cells: number[] | undefined): number | null {
  if (cells === undefined || cells.length === 0) {
    return null;
  }

  const populated = cells.filter((mv) => isNum(mv) && mv !== UNKNOWN_MV);

  if (populated.length === 0) {
    return null;
  }

  return populated.reduce((total, mv) => total + mv, 0) * MV_TO_V;
}

function percentOf(value: number | undefined): number | null {
  return isNum(value) && value !== NOT_PROVIDED ? value : null;
}

function currentOf(value: number | undefined): number | null {
  return isNum(value) && value !== NOT_PROVIDED ? value * CA_TO_A : null;
}
