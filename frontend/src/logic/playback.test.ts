import { describe, expect, it } from 'vitest';

import {
  advance,
  createPlayback,
  dueCount,
  extendTo,
  pause,
  play,
  seekTo,
  setSpeed,
  type PlaybackState,
  type TimedItem,
} from './playback';

const START = 1_700_000_000_000;
const END = START + 10_000;

function playing(overrides: Partial<PlaybackState> = {}): PlaybackState {
  return { ...play(createPlayback(START, END)), ...overrides };
}

describe('createPlayback', () => {
  it('starts paused at the beginning at normal speed', () => {
    const state = createPlayback(START, END);

    expect(state).toEqual({ positionMs: START, startedAtMs: START, endedAtMs: END, speed: 1, playing: false });
  });

  it('never ends before it starts', () => {
    expect(createPlayback(START, START - 5_000).endedAtMs).toBe(START);
  });
});

describe('advance', () => {
  it('does nothing while paused', () => {
    const paused = createPlayback(START, END);

    // Identity, not just equality: App re-renders on every tick and an
    // unchanged clock must not look like a new one.
    expect(advance(paused, 1_000)).toBe(paused);
  });

  it('advances by wall time at speed 1', () => {
    expect(advance(playing(), 250).positionMs).toBe(START + 250);
  });

  it('scales wall time by the playback speed', () => {
    expect(advance(playing(), 250).positionMs).toBe(START + 250);
    expect(advance(setSpeed(playing(), 4), 250).positionMs).toBe(START + 1_000);
    expect(advance(setSpeed(playing(), 0.5), 250).positionMs).toBe(START + 125);
  });

  it('clamps at the end and stops playing there', () => {
    const ended = advance(playing(), 99_999);

    expect(ended.positionMs).toBe(END);
    expect(ended.playing).toBe(false);
  });

  it('ignores a non-finite or negative wall delta', () => {
    const state = playing();

    expect(advance(state, Number.NaN)).toBe(state);
    expect(advance(state, -100)).toBe(state);
  });
});

describe('play and pause', () => {
  it('pauses and resumes', () => {
    expect(play(createPlayback(START, END)).playing).toBe(true);
    expect(pause(playing()).playing).toBe(false);
  });

  it('restarts from the beginning when played from the end', () => {
    // Otherwise the play control is dead after the recording finishes, which
    // reads as a broken button rather than a finished flight.
    const atEnd = seekTo(createPlayback(START, END), END);

    expect(play(atEnd)).toMatchObject({ positionMs: START, playing: true });
  });
});

describe('seekTo', () => {
  it('clamps into the recording', () => {
    const state = createPlayback(START, END);

    expect(seekTo(state, START - 5_000).positionMs).toBe(START);
    expect(seekTo(state, END + 5_000).positionMs).toBe(END);
    expect(seekTo(state, START + 2_500).positionMs).toBe(START + 2_500);
  });

  it('ignores a non-finite target', () => {
    const state = createPlayback(START, END);

    expect(seekTo(state, Number.NaN)).toBe(state);
  });

  it('preserves whether it was playing', () => {
    expect(seekTo(playing(), START + 1_000).playing).toBe(true);
    expect(seekTo(createPlayback(START, END), START + 1_000).playing).toBe(false);
  });
});

describe('setSpeed', () => {
  it('rejects zero, negative, and non-finite speeds', () => {
    const state = createPlayback(START, END);

    expect(setSpeed(state, 0)).toBe(state);
    expect(setSpeed(state, -2)).toBe(state);
    expect(setSpeed(state, Number.POSITIVE_INFINITY)).toBe(state);
  });
});

describe('extendTo', () => {
  it('grows the end as later pages arrive', () => {
    expect(extendTo(createPlayback(START, END), END + 5_000).endedAtMs).toBe(END + 5_000);
  });

  it('never shrinks the end under the current position', () => {
    const state = seekTo(createPlayback(START, END), END);

    expect(extendTo(state, START + 1_000).endedAtMs).toBe(END);
  });
});

describe('dueCount', () => {
  const items: TimedItem[] = [
    { atMs: START },
    { atMs: START + 100 },
    { atMs: START + 200 },
    { atMs: START + 300 },
  ];

  it('counts the leading run that is due', () => {
    expect(dueCount(items, 0, START - 1)).toBe(0);
    expect(dueCount(items, 0, START)).toBe(1);
    expect(dueCount(items, 0, START + 250)).toBe(3);
    expect(dueCount(items, 0, START + 10_000)).toBe(4);
  });

  it('counts forward from the cursor', () => {
    expect(dueCount(items, 2, START + 250)).toBe(1);
    expect(dueCount(items, 4, START + 10_000)).toBe(0);
  });

  it('stops at the first item that is not due, even if a later one is', () => {
    // Recorded order is delivery order, not timestamp order. Emitting a later
    // event early to "catch up" would reorder telemetry the live path
    // delivered in sequence; one tick of delay is the cheaper error.
    const outOfOrder: TimedItem[] = [{ atMs: START + 500 }, { atMs: START + 100 }];

    expect(dueCount(outOfOrder, 0, START + 200)).toBe(0);
    expect(dueCount(outOfOrder, 0, START + 500)).toBe(2);
  });
});
