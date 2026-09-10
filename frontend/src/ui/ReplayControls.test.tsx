// @vitest-environment happy-dom

/**
 * The recording picker's destructive path.
 *
 * Deleting a recording is guarded by `Confirm` rather than by the browser's
 * `confirm()` dialog, and it is deliberately not amber: it destroys data, not
 * the aircraft. Neither property is reachable from the mock captures — the
 * replay surface only renders under `?source=replay` — so it is pinned here.
 */

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { ReplayControls } from './ReplayControls';

const RECORDINGS = [{ id: 7, name: 'bench run', event_count: 412, status: 'closed' }];

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;

beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => { root.unmount(); });
  container.remove();
  vi.unstubAllGlobals();
});

/** Flushes the picker's mount fetch and the render it causes. */
async function mount() {
  await act(async () => {
    root.render(<ReplayControls source={null} />);
    await Promise.resolve();
  });
  await act(async () => { await Promise.resolve(); });
}

const lever = (text: string) =>
  [...container.querySelectorAll('button')].find((button) => button.textContent === text);

it('guards deletion with an attestation instead of a browser dialog, and never spends amber on it', async () => {
  const remove = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'DELETE') return remove(input, init) as Promise<Response>;
    return new Response(JSON.stringify(RECORDINGS), { status: 200, headers: { 'content-type': 'application/json' } });
  }));
  // A browser dialog would silently satisfy the flow without any of this.
  vi.stubGlobal('confirm', vi.fn(() => true));

  await mount();

  const del = lever('Delete');
  expect(del, 'the picker lists a recording with a delete lever').toBeDefined();
  // Destructive to data, not to the vehicle. Amber is reserved for the aircraft.
  expect(del?.className).not.toContain('lever--caution');

  // The recording opens through a real anchor, not a button wearing the class.
  const open = container.querySelector<HTMLAnchorElement>('a.lever');
  expect(open?.textContent).toBe('bench run');
  expect(open?.getAttribute('href')).toContain('7');

  act(() => { del?.click(); });
  expect(globalThis.confirm).not.toHaveBeenCalled();

  const commit = lever('Delete permanently');
  expect(commit?.disabled, 'unattested deletion stays disabled').toBe(true);

  const checkbox = container.querySelector<HTMLInputElement>('.field--check input');
  expect(checkbox?.parentElement?.textContent).toContain('Permanently delete bench run');

  await act(async () => {
    checkbox?.click();
    await Promise.resolve();
  });
  expect(lever('Delete permanently')?.disabled).toBe(false);

  await act(async () => {
    lever('Delete permanently')?.click();
    await Promise.resolve();
  });
  await act(async () => { await Promise.resolve(); });

  expect(remove).toHaveBeenCalledOnce();
  expect(container.textContent).toContain('No recordings yet.');
});

it('abandons the deletion when the attestation is withdrawn', async () => {
  vi.stubGlobal('fetch', vi.fn(() =>
    Promise.resolve(new Response(JSON.stringify(RECORDINGS), { status: 200, headers: { 'content-type': 'application/json' } }))));

  await mount();
  act(() => { lever('Delete')?.click(); });
  expect(lever('Delete permanently')).toBeDefined();

  act(() => { lever('Cancel')?.click(); });
  expect(lever('Delete permanently'), 'cancelling clears the guard').toBeUndefined();
  expect(container.querySelector('.field--check')).toBeNull();
});
