// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { WorkspaceSlots, DEFAULT_VISIBILITY } from '@/workspace/Workspace';

it('moves focus and selection with arrows/Home/End while keeping tab panels mounted', () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  act(() => { root.render(<WorkspaceSlots visible={DEFAULT_VISIBILITY} content={{ families: 'Families body', sample: 'Sample body' }} />); });
  const first = container.querySelector<HTMLButtonElement>('#tab-families');
  const second = container.querySelector<HTMLButtonElement>('#tab-sample');
  const panel = container.querySelector<HTMLElement>('#panel-sample');
  expect(panel?.hidden).toBe(true);
  expect(panel?.getAttribute('role')).toBe('tabpanel');
  expect(panel?.getAttribute('aria-labelledby')).toBe(second?.id);
  const key = (target: HTMLElement | null, value: string) => {
    act(() => { target?.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true })); });
  };
  first?.focus();
  key(first, 'ArrowRight');
  expect(document.activeElement).toBe(second);
  expect(second?.tabIndex).toBe(0);
  expect(first?.tabIndex).toBe(-1);
  expect(panel?.hidden).toBe(false);
  key(second, 'ArrowRight');
  expect(document.activeElement).toBe(first);
  key(first, 'End');
  expect(document.activeElement).toBe(second);
  key(second, 'Home');
  expect(document.activeElement).toBe(first);
  key(first, 'ArrowLeft');
  expect(document.activeElement).toBe(second);
  expect(container.querySelector('#panel-sample')).toBe(panel);
  act(() => { root.unmount(); });
  container.remove();
});
