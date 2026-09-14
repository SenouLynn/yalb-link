// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { useState } from 'react';
import { expect, it } from 'vitest';
import { Tabs } from './primitives';

/**
 * `Tabs` itself, not a section that happens to be tabbed.
 *
 * This used to mount the dev tier through `WorkspaceSlots` and drive it by
 * `#tab-families`/`#tab-sample`, because that was the one tabbed section in
 * the registry. It stopped being tabbed when Families/Raw sample became rail-
 * style accordions instead (so the developer tier can size to its own content
 * rather than a fixed strip), which left the registry with no tabbed section
 * at all — nothing to reach this behaviour through except a section built for
 * the test. `Tabs` is still a real primitive other panels may reach for, so it
 * keeps its own coverage, exercised directly rather than through a feature
 * that no longer uses it.
 */
it('moves focus and selection with arrows/Home/End and calls onSelect', () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);

  function Harness() {
    const [active, setActive] = useState('a');
    return (
      <Tabs
        label="Test tabs"
        tabs={[{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }]}
        active={active}
        onSelect={setActive}
      />
    );
  }

  act(() => { root.render(<Harness />); });
  const first = container.querySelector<HTMLButtonElement>('#tab-a');
  const second = container.querySelector<HTMLButtonElement>('#tab-b');
  expect(first?.getAttribute('aria-selected')).toBe('true');
  expect(second?.getAttribute('aria-selected')).toBe('false');

  const key = (target: HTMLElement | null, value: string) => {
    act(() => { target?.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true })); });
  };
  first?.focus();
  key(first, 'ArrowRight');
  expect(document.activeElement).toBe(second);
  expect(second?.getAttribute('aria-selected')).toBe('true');
  expect(second?.tabIndex).toBe(0);
  expect(first?.tabIndex).toBe(-1);
  key(second, 'ArrowRight');
  expect(document.activeElement).toBe(first);
  key(first, 'End');
  expect(document.activeElement).toBe(second);
  key(second, 'Home');
  expect(document.activeElement).toBe(first);
  key(first, 'ArrowLeft');
  expect(document.activeElement).toBe(second);

  act(() => { root.unmount(); });
  container.remove();
});
