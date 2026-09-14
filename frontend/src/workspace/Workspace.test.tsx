// @vitest-environment happy-dom
import { act, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';

import {
  DEFAULT_MIRRORED,
  DEFAULT_VISIBILITY,
  WorkspaceSlots,
  type PanelContent,
  type PanelVisibility,
  type SectionId,
} from './Workspace';

const CONTENT: PanelContent = {
  position: 'Rail position',
  guidance: 'Rail guidance',
  radiolink: 'Rail radio link',
  map: 'Map',
  waypoints: 'Waypoints',
  instruments: 'Instruments',
};

const MIRROR_CONTENT: PanelContent = {
  position: 'Mirrored position',
  guidance: 'Mirrored guidance',
  radiolink: 'Mirrored radio link',
};

function renderSlots(props: {
  visible?: PanelVisibility;
  mirrored?: PanelVisibility;
  sectionHeaders?: Partial<Record<SectionId, ReactNode>>;
} = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  act(() => {
    root.render(
      <WorkspaceSlots
        visible={props.visible ?? DEFAULT_VISIBILITY}
        mirrored={props.mirrored ?? DEFAULT_MIRRORED}
        content={CONTENT}
        mirrorContent={MIRROR_CONTENT}
        sectionHeaders={props.sectionHeaders}
      />,
    );
  });
  return {
    container,
    unmount: () => {
      act(() => { root.unmount(); });
      container.remove();
    },
  };
}

const allOff = (visibility: PanelVisibility): PanelVisibility =>
  Object.fromEntries(Object.keys(visibility).map((id) => [id, false]));

describe('mirrored panels', () => {
  it('renders under both their rail home and their mirror target, as two distinct nodes', () => {
    const { container, unmount } = renderSlots();
    const home = container.querySelector<HTMLElement>('#panel-position');
    const mirror = container.querySelector<HTMLElement>('#panel-position-mirror');
    expect(home?.textContent).toBe('Rail position');
    expect(mirror?.textContent).toBe('Mirrored position');
    expect(home?.hidden).toBe(false);
    expect(mirror?.hidden).toBe(false);
    unmount();
  });

  it('hides the mirror when the panel itself is turned off, even while still mirrored on', () => {
    const { container, unmount } = renderSlots({
      visible: { ...DEFAULT_VISIBILITY, position: false },
    });
    expect(container.querySelector<HTMLElement>('#panel-position-mirror')?.hidden).toBe(true);
    unmount();
  });

  it('hides the mirror when mirroring is off, even while the panel itself stays visible', () => {
    const { container, unmount } = renderSlots({
      mirrored: { ...DEFAULT_MIRRORED, position: false },
    });
    expect(container.querySelector<HTMLElement>('#panel-position')?.hidden).toBe(false);
    expect(container.querySelector<HTMLElement>('#panel-position-mirror')?.hidden).toBe(true);
    unmount();
  });

  it('keeps a column mounted from a mirror alone, even with every one of its own panels hidden', () => {
    const { container, unmount } = renderSlots({
      visible: { ...DEFAULT_VISIBILITY, map: false, waypoints: false },
    });
    const mapSlot = container.querySelector<HTMLElement>('[aria-label="Map"]');
    expect(mapSlot?.hidden).toBe(false);
    expect(container.querySelector<HTMLElement>('#panel-position-mirror')?.hidden).toBe(false);
    unmount();
  });
});

describe('section headers', () => {
  it('stay mounted and visible even when nothing in that section has anything to show', () => {
    const { container, unmount } = renderSlots({
      visible: allOff(DEFAULT_VISIBILITY),
      mirrored: allOff(DEFAULT_MIRRORED),
      sectionHeaders: { map: <div data-testid="critical-bar">Command</div> },
    });
    const mapSlot = container.querySelector<HTMLElement>('[aria-label="Map"]');
    expect(mapSlot?.hidden).toBe(false);
    expect(container.querySelector('[data-testid="critical-bar"]')).not.toBeNull();
    unmount();
  });

  it('do not appear in a section that has no header of its own', () => {
    const { container, unmount } = renderSlots({
      visible: allOff(DEFAULT_VISIBILITY),
      mirrored: allOff(DEFAULT_MIRRORED),
      sectionHeaders: { map: <div>Command</div> },
    });
    const vehicleSlot = container.querySelector<HTMLElement>('[aria-label="Vehicle"]');
    expect(vehicleSlot?.hidden).toBe(true);
    unmount();
  });
});
