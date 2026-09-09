import { describe, expect, it } from 'vitest';

import { resolveLinkQuality } from './link';
import { baseSample } from './testing';

const radioSample = {
  radioRssi: 190,
  radioRemrssi: 185,
  radioNoise: 40,
  radioRemnoise: 38,
  radioTxbufPct: 92,
  radioRxerrors: 7,
};

describe('resolveLinkQuality', () => {
  it('preserves valid 254 signal and noise values', () => {
    const result = resolveLinkQuality(baseSample({ ...radioSample,
      radioRssi: 254, radioRemrssi: 254, radioNoise: 254, radioRemnoise: 254,
    }));
    expect(result).toMatchObject({ rssi: 254, remrssi: 254, noise: 254, remnoise: 254 });
  });

  it('resolves every field from RADIO_STATUS', () => {
    expect(resolveLinkQuality(baseSample(radioSample))).toEqual({
      rssi: 190,
      remrssi: 185,
      noise: 40,
      remnoise: 38,
      txbufPct: 92,
      rxerrors: 7,
      source: 'RADIO_STATUS',
    });
  });

  it('is unresolved before the family has been heard', () => {
    // Expected on this stack: Compose SITL carries MAVLink over UDP with no SiK
    // radio in the path, so RADIO_STATUS never arrives there.
    expect(resolveLinkQuality(baseSample())).toBeNull();
  });

  it('treats the 255 sentinel as unknown signal, not as a strong signal', () => {
    // 255 is RADIO_STATUS's "unknown". Rendering it as a number would show the
    // strongest reading on the scale at the moment the radio admits it has none.
    const actual = resolveLinkQuality(
      baseSample({ ...radioSample, radioRssi: 255, radioRemrssi: 255, radioNoise: 255, radioRemnoise: 255 }),
    );

    expect(actual?.rssi).toBeNull();
    expect(actual?.remrssi).toBeNull();
    expect(actual?.noise).toBeNull();
    expect(actual?.remnoise).toBeNull();
  });

  it('still reports free buffer space when signal strength is unknown', () => {
    // txbuf is the most actionable field and does not share the sentinel.
    const actual = resolveLinkQuality(baseSample({ ...radioSample, radioRssi: 255 }));

    expect(actual?.txbufPct).toBe(92);
    expect(actual?.remrssi).toBe(185);
  });

  it('keeps a zero error count, which is a healthy link rather than no data', () => {
    expect(resolveLinkQuality(baseSample({ ...radioSample, radioRxerrors: 0 }))?.rxerrors).toBe(0);
  });
});
