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
