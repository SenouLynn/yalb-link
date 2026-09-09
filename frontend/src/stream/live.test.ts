import { afterEach, expect, it, vi } from 'vitest';
import { LiveEventSource } from './live';
class FakeSource extends EventTarget {
  readyState = 0;
  close = vi.fn();
}
afterEach(() => { vi.useRealTimers(); });
it('recreates terminal sources, leaves native reconnect alone, and cancels pending retries on cleanup', () => {
  vi.useFakeTimers();
  const sources: FakeSource[] = [];
  const create = vi.fn(() => { const source = new FakeSource(); sources.push(source); return source as unknown as EventSource; });
  const events = vi.fn();
  const stop = new LiveEventSource({ create }).start(events);
  const first = sources[0];
  if (!first) throw new Error('first source missing');
  first.dispatchEvent(new Event('error'));
  vi.advanceTimersByTime(3000);
  expect(create).toHaveBeenCalledTimes(1);
  first.readyState = 2;
  first.dispatchEvent(new Event('error'));
  expect(first.close).toHaveBeenCalledTimes(1);
  vi.advanceTimersByTime(2000);
  expect(create).toHaveBeenCalledTimes(2);
  const second = sources[1];
  if (!second) throw new Error('second source missing');
  second.dispatchEvent(new Event('open'));
  expect(events).toHaveBeenLastCalledWith(expect.objectContaining({ kind: 'connection', connected: true }));
  second.readyState = 2;
  second.dispatchEvent(new Event('error'));
  stop();
  vi.advanceTimersByTime(5000);
  expect(create).toHaveBeenCalledTimes(2);
  expect(vi.getTimerCount()).toBe(0);
});

it('backs off to a ceiling while the backend stays down, and resets on the next open', () => {
  vi.useFakeTimers();
  const sources: FakeSource[] = [];
  const create = vi.fn(() => { const source = new FakeSource(); sources.push(source); return source as unknown as EventSource; });
  const stop = new LiveEventSource({ create }).start(vi.fn());
  // Each new source is terminal the moment it is made, so every retry is a
  // failed one and the delay should keep growing.
  const fail = () => {
    const source = sources[sources.length - 1];
    if (source === undefined) throw new Error('no source');
    source.readyState = 2;
    source.dispatchEvent(new Event('error'));
  };
  for (const delay of [2000, 4000, 8000, 16000, 30000, 30000]) {
    const before = create.mock.calls.length;
    fail();
    vi.advanceTimersByTime(delay - 1);
    expect(create).toHaveBeenCalledTimes(before);
    vi.advanceTimersByTime(1);
    expect(create).toHaveBeenCalledTimes(before + 1);
  }
  sources[sources.length - 1]?.dispatchEvent(new Event('open'));
  const reset = create.mock.calls.length;
  fail();
  vi.advanceTimersByTime(2000);
  expect(create).toHaveBeenCalledTimes(reset + 1);
  stop();
});
