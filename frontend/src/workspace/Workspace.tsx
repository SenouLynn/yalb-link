/**
 * The shell: an app bar, a view bar, and the slots.
 *
 * The shell knows about slots and never about panels. A panel is a registry
 * entry plus a rendered node handed in by composition, which is what keeps
 * "what is on screen" answerable by reading `registry.ts` alone.
 */

import { useEffect, useRef, useState, type ReactNode } from 'react';

import { Tabs, Lever } from '@/ui/primitives';
import {
  PANELS,
  panelsInSlot,
  slotOccupied,
  toggleable,
  type PanelVisibility,
  type SlotId,
} from './registry';

export { DEFAULT_VISIBILITY, PANELS } from './registry';
export type { PanelVisibility, PanelDef, SlotId, PanelTier } from './registry';

/** What composition hands the shell: a node per registered panel id. */
export type PanelContent = Readonly<Record<string, ReactNode>>;

/* --- shell ---------------------------------------------------------------- */

export function WorkspaceShell({
  title,
  meta,
  viewBar,
  children,
}: {
  title: string;
  /** Right-aligned app-level context — operator identity, build, clock. */
  meta?: ReactNode;
  /** Per-view chrome: navigation, scope, panel visibility. */
  viewBar?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="shell">
      <header className="shell__bar">
        <span className="shell__title">{title}</span>
        {meta === undefined ? null : <span className="shell__meta shell__spacer">{meta}</span>}
      </header>
      <div className="shell__bar shell__bar--view">{viewBar}</div>
      {children}
    </div>
  );
}

/* --- slots ---------------------------------------------------------------- */

/**
 * Mounts every slot.
 *
 * The aux and dev slots share one column: the instruments sit above a resident
 * developer tier, which is bounded so it cannot crowd them out.
 */
export function WorkspaceSlots({
  visible,
  content,
  emptyHint,
}: {
  visible: PanelVisibility;
  content: PanelContent;
  /** Shown when the operator has hidden everything. */
  emptyHint?: string;
}) {
  const anyVisible = PANELS.some((panel) => visible[panel.id] === true);

  /*
   * Every slot and every panel stays mounted; hiding is `hidden`, never an
   * unmount.
   *
   * This is not a detail. Unmounting a hidden panel tears down its MapLibre
   * context and drops a mission download that is still in flight, so a glance
   * at a different panel would cost the operator the request they were waiting
   * on. `display: none` on a flex item leaves the layout identical to a slot
   * that was never there, so the reflow is free either way.
   */
  return (
    <main className="shell__body">
      {anyVisible ? null : (
        <p className="slot__empty">
          {emptyHint ?? 'Every panel is hidden. Use Views to show one.'}
        </p>
      )}

      {(['rail', 'center'] as const).map((slot) => (
        <Slot key={slot} slot={slot} visible={visible} content={content} />
      ))}

      <div className="slot slot--aux" hidden={!slotOccupied('aux', visible) && !slotOccupied('dev', visible)}>
        <div className="slot__stack">
          <Slot slot="aux" visible={visible} content={content} />
          <Slot slot="dev" visible={visible} content={content} />
        </div>
      </div>
    </main>
  );
}

function Slot({
  slot,
  visible,
  content,
}: {
  slot: SlotId;
  visible: PanelVisibility;
  content: PanelContent;
}) {
  const panels = panelsInSlot(slot);
  const shown = panels.filter((panel) => visible[panel.id] === true);

  // The dev slot shares one tab strip; every other slot stacks.
  const tabbed = slot === 'dev';
  const [active, setActive] = useState(panels[0]?.id ?? '');
  const current = shown.some((panel) => panel.id === active) ? active : (shown[0]?.id ?? '');

  // The map owns its slot edge to edge; a scroll body would clip its controls.
  const bleed = slot === 'center';

  return (
    <section
      className={`slot slot--${slot}`}
      aria-label={slot}
      hidden={shown.length === 0}
    >
      {tabbed && shown.length > 0 ? (
        <Tabs
          label="Developer panels"
          tabs={shown.map((panel) => ({ id: panel.id, label: panel.label }))}
          active={current}
          onSelect={setActive}
        />
      ) : null}

      <div className={`slot__body${bleed ? ' slot__body--bleed' : ''}`}>
        {panels.map((panel) => {
          const hidden = visible[panel.id] !== true || (tabbed && panel.id !== current);
          const node = content[panel.id];

          return (
              <div key={panel.id} id={`panel-${panel.id}`} className="panel-mount" hidden={hidden}
                role={tabbed ? 'tabpanel' : undefined}
                aria-labelledby={tabbed ? `tab-${panel.id}` : undefined}
                tabIndex={tabbed ? 0 : undefined}>
                {node}
              </div>
          );
        })}
      </div>
    </section>
  );
}

/* --- views ---------------------------------------------------------------- */

/**
 * Which panels are on screen.
 *
 * A button plus a checkbox popover rather than a row of toggles: the row grew
 * with every panel added and pushed the navigation it sat beside off the bar.
 * A native `<select multiple>` cannot be styled to match and is awkward with a
 * pointer.
 */
export function ViewsMenu({
  visible,
  onToggle,
}: {
  visible: PanelVisibility;
  onToggle: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement | null>(null);

  // Dismiss on outside click or Escape, the two ways anyone closes a popover.
  useEffect(() => {
    if (!open) {
      return;
    }

    const onPointerDown = (event: PointerEvent) => {
      if (root.current !== null && !root.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);

    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const shown = PANELS.filter((panel) => visible[panel.id] === true).length;

  return (
    <div className="views" ref={root}>
      <Lever
        aria-expanded={open}
        aria-haspopup="true"
        onClick={() => {
          setOpen((previous) => !previous);
        }}
      >
        Views ({shown})
      </Lever>

      {open ? (
        <div className="views__popover" role="group" aria-label="Visible panels">
          {(['operator', 'dev'] as const).map((tier) => {
            const panels = toggleable(tier);

            if (panels.length === 0) {
              return null;
            }

            return (
              <div key={tier}>
                <div className="views__tier">{tier === 'dev' ? 'Developer' : 'Operator'}</div>
                {panels.map((panel) => (
                  <label key={panel.id} className="views__option">
                    <input
                      type="checkbox"
                      checked={visible[panel.id] === true}
                      aria-controls={`panel-${panel.id}`}
                      onChange={() => {
                        onToggle(panel.id);
                      }}
                    />
                    <span>{panel.label}</span>
                  </label>
                ))}
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
