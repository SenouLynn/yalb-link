import { describe, expect, it } from 'vitest';

import { LiveEventSource } from './live';
import { MockEventSource } from './mock';
import { ReplayEventSource } from './replay';
import { parseRecordingId, replayUrl, selectStream } from './select';

describe('selectStream', () => {
  it('defaults to live', () => {
    for (const search of ['', '?', '?other=1', '?source=', '?source=LIVE', '?source=bogus']) {
      const selection = selectStream(search);

      // Live is the default in every ambiguous case. Anything else risks
      // showing an operator an aircraft that is not flying.
      expect(selection.source, search).toBe('live');
      expect(selection.stream, search).toBeInstanceOf(LiveEventSource);
      expect(selection.replay, search).toBeNull();
    }
  });

  it('selects fixtures only for an exact ?source=mock', () => {
    const selection = selectStream('?source=mock');

    expect(selection.source).toBe('mock');
    expect(selection.stream).toBeInstanceOf(MockEventSource);
  });

  it('selects a recording for ?source=replay&recording=N', () => {
    const selection = selectStream('?source=replay&recording=12');

    expect(selection.source).toBe('replay');
    expect(selection.stream).toBeInstanceOf(ReplayEventSource);
    expect(selection.replay).toBe(selection.stream);
  });

  it('stays in replay mode with no recording rather than falling back to live', () => {
    for (const search of ['?source=replay', '?source=replay&recording=abc', '?source=replay&recording=0']) {
      const selection = selectStream(search);

      expect(selection.source, search).toBe('replay');
      expect(selection.replay, search).toBeNull();
      expect(selection.stream, search).not.toBeInstanceOf(LiveEventSource);
    }
  });

  it('delivers nothing from the idle stream and stops cleanly', () => {
    const { stream } = selectStream('?source=replay');
    let delivered = 0;
    const cancel = stream.start(() => {
      delivered += 1;
    });

    expect(delivered).toBe(0);
    expect(() => {
      cancel();
    }).not.toThrow();
  });
});

describe('parseRecordingId', () => {
  it('accepts positive integers only', () => {
    expect(parseRecordingId('1')).toBe(1);
    expect(parseRecordingId('4096')).toBe(4096);
  });

  it('rejects anything else', () => {
    for (const raw of [null, '', '0', '-1', '1.5', 'abc', '1e3', ' 1', '01x']) {
      expect(parseRecordingId(raw), String(raw)).toBeNull();
    }
  });
});

describe('replayUrl', () => {
  it('round-trips through selectStream', () => {
    const selection = selectStream(replayUrl(7));

    expect(selection.source).toBe('replay');
    expect(selection.replay).not.toBeNull();
  });
});
