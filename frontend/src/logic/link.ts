/** Radio link quality, as reported by a SiK-compatible modem. */

import { isNum } from './finite';
import type { TelemetrySample } from './sample';

export interface LinkQualityResult {
  /** Local signal strength, 0..254. Null when the radio reports unknown. */
  rssi: number | null;
  /** Remote (vehicle) signal strength, same scale and same unknown. */
  remrssi: number | null;
  noise: number | null;
  remnoise: number | null;
  /** Free transmit buffer space, per cent. Low means back-pressure. */
  txbufPct: number;
  /** Accumulated receive errors. Rising is the actionable signal, not the value. */
  rxerrors: number;
  source: 'RADIO_STATUS';
}

/** RADIO_STATUS reports 255 where it has no signal measurement. */
const UNKNOWN_RSSI = 255;

/** Maps the unknown sentinel to an absent reading. */
function signal(value: number): number | null {
  return value === UNKNOWN_RSSI ? null : value;
}

/**
 * Resolves link quality from RADIO_STATUS.
 *
 * This family originates in the radio, not the autopilot, so it is absent on
 * any link without a SiK modem in the path — including Compose SITL, which
 * carries MAVLink over UDP. An unresolved result there is expected, not a
 * defect.
 */
export function resolveLinkQuality(sample: TelemetrySample): LinkQualityResult | null {
  const {
    radioRssi,
    radioRemrssi,
    radioNoise,
    radioRemnoise,
    radioTxbufPct,
    radioRxerrors,
  } = sample;

  if (
    !isNum(radioRssi) ||
    !isNum(radioRemrssi) ||
    !isNum(radioNoise) ||
    !isNum(radioRemnoise) ||
    !isNum(radioTxbufPct) ||
    !isNum(radioRxerrors)
  ) {
    return null;
  }

  return {
    rssi: signal(radioRssi),
    remrssi: signal(radioRemrssi),
    noise: signal(radioNoise),
    remnoise: signal(radioRemnoise),
    txbufPct: radioTxbufPct,
    rxerrors: radioRxerrors,
    source: 'RADIO_STATUS',
  };
}
