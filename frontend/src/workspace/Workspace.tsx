/**
 * The shell: an app bar, a view bar, and the section columns.
 *
 * The shell knows about sections and never about panels. A panel is a registry
 * entry plus a rendered node handed in by composition, which is what keeps
 * "what is on screen" answerable by reading `registry.ts` alone.
 */

import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';

import { Tabs, Lever } from '@/ui/primitives';
import {
  COLUMN_SECTIONS,
  PANELS,
  SECTIONS,
  columnOccupied,
  panelsInSection,
  sectionCheck,
  sectionOccupied,
  stackedIn,
  toggleable,
  type PanelDef,
  type PanelVisibility,
  type SectionDef,
  type SectionId,
} from './registry';

export {
  DEFAULT_VISIBILITY,
  PANELS,
  SECTIONS,
  sectionCheck,
  toggleSection,
} from './registry';
export type {
  PanelVisibility,
  PanelDef,
  SectionDef,
  SectionId,
  PanelTier,
  SectionCheck,
} from './registry';

/** What composition hands the shell: a node per registered panel id. */
export type PanelContent = Readonly<Record<string, ReactNode>>;

/* --- shell ---------------------------------------------------------------- */

export function WorkspaceShell({
  nav,
  meta,
  children,
}: {
  /**
   * Where the operator is and which vehicle they are on.
   *
   * The bar used to carry the application's name and nothing else, which is a
   * row of chrome spent on a fact that never changes and that the browser tab
   * already states. Navigation earns the row; a title does not.
   */
  nav?: ReactNode;
  /** Right-aligned app-level context — the source this page is reading. */
  meta?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="shell">
      <header className="shell__bar">
        {nav}
        {meta === undefined ? null : <span className="shell__meta shell__spacer">{meta}</span>}
      </header>
      {children}
    </div>
  );
}

/**
 * The bar under the app bar, owned by the view rather than by the shell.
 *
 * Panel visibility is a property of the thing being looked at, so the control
 * for it belongs to the view that has panels — not to an application bar that
 * outlives it. `trailing` is pushed to the right edge: the bar reads
 * left-to-right as what is playing, then what is shown.
 */
export function ViewBar({ children, trailing }: { children?: ReactNode; trailing?: ReactNode }) {
  return (
    <div className="shell__bar shell__bar--view">
      {children}
      {trailing === undefined ? null : <span className="shell__end">{trailing}</span>}
    </div>
  );
}

/* --- columns -------------------------------------------------------------- */

/**
 * Mounts every column.
 *
 * A column is one section, plus whatever is stacked inside it — today only the
 * developer tier under the instruments, which is bounded so it cannot crowd
 * them out.
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
   * Every column and every panel stays mounted; hiding is `hidden`, never an
   * unmount.
   *
   * This is not a detail. Unmounting a hidden panel tears down its MapLibre
   * context and drops a mission download that is still in flight, so a glance
   * at a different panel would cost the operator the request they were waiting
   * on. `display: none` on a flex item leaves the layout identical to a column
   * that was never there, so the reflow is free either way.
   */
  return (
    <main className="shell__body">
      {anyVisible ? null : (
        <p className="slot__empty">
          {emptyHint ?? 'Every panel is hidden. Use Views to show one.'}
        </p>
      )}

      {COLUMN_SECTIONS.map((section) => (
        <Column key={section.id} section={section} visible={visible} content={content} />
      ))}
    </main>
  );
}

/**
 * One column of the workspace.
 *
 * Its width is `--grow`, read off the registry rather than written per section
 * in CSS: a column's share of the leftover width is a registry fact, and a
 * second copy of it in a stylesheet is how the two stop agreeing.
 */
function Column({
  section,
  visible,
  content,
}: {
  section: SectionDef;
  visible: PanelVisibility;
  content: PanelContent;
}) {
  const nested = stackedIn(section.id);

  if (nested.length === 0) {
    return <Slot section={section} visible={visible} content={content} />;
  }

  return (
    <div
      className={`slot slot--column slot--${section.id}`}
      style={{ '--grow': section.grow } as CSSProperties}
      hidden={!columnOccupied(section.id, visible)}
    >
      <div className="slot__stack">
        <Slot section={section} visible={visible} content={content} nested />
        {nested.map((child) => (
          <Slot key={child.id} section={child} visible={visible} content={content} nested />
        ))}
      </div>
    </div>
  );
}

function Slot({
  section,
  visible,
  content,
  nested,
}: {
  section: SectionDef;
  visible: PanelVisibility;
  content: PanelContent;
  /** Inside a stack, so its flex sizing is a height rather than a width. */
  nested?: boolean;
}) {
  const panels = panelsInSection(section.id);
  const shown = panels.filter((panel) => visible[panel.id] === true);

  const tabbed = section.tabbed === true;
  const [active, setActive] = useState(panels[0]?.id ?? '');
  const current = shown.some((panel) => panel.id === active) ? active : (shown[0]?.id ?? '');

  /*
   * A section splits into two regions: the one panel that owns the leftover
   * space, and the rows underneath it. Only the Map section has both today,
   * which is exactly the arrangement the operator asked for — the route and
   * the readings that describe progress along it fold in under the map, and
   * turning them off gives the map the column.
   */
  const bleeding = panels.filter((panel) => panel.bleed === true);
  const rows = panels.filter((panel) => panel.bleed !== true);
  const isShown = (panel: PanelDef) =>
    visible[panel.id] === true && (!tabbed || panel.id === current);
  const anyShown = (group: readonly PanelDef[]) => group.some(isShown);

  /*
   * The row region's two sides. Split only while both have something on them:
   * one side alone takes the full width rather than leaving a gutter where the
   * other would have been.
   */
  const lead = rows.filter((panel) => panel.side === 'lead');
  const trail = rows.filter((panel) => panel.side === 'trail');
  const sided = lead.length > 0 || trail.length > 0;
  const split = anyShown(lead) && anyShown(trail);

  const mount = (panel: PanelDef) => (
    <div
      key={panel.id}
      id={`panel-${panel.id}`}
      className="panel-mount"
      hidden={!isShown(panel)}
      role={tabbed ? 'tabpanel' : undefined}
      aria-labelledby={tabbed ? `tab-${panel.id}` : undefined}
      tabIndex={tabbed ? 0 : undefined}
    >
      {content[panel.id]}
    </div>
  );

  return (
    <section
      className={`slot slot--${section.id}${bleeding.length > 0 ? ' slot--bleeding' : ''}`}
      style={nested === true ? undefined : ({ '--grow': section.grow } as CSSProperties)}
      aria-label={section.label}
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

      <div className="slot__body">
        {bleeding.length === 0 ? null : (
          <div className="slot__bleed" hidden={!anyShown(bleeding)}>
            {bleeding.map(mount)}
          </div>
        )}
        {rows.length === 0 ? null : (
          <div
            className={`slot__rows${split ? ' slot__rows--split' : ''}`}
            hidden={!anyShown(rows)}
          >
            {sided ? (
              <>
                <div className="slot__side slot__side--lead" hidden={!anyShown(lead)}>
                  {lead.map(mount)}
                </div>
                <div className="slot__side slot__side--trail" hidden={!anyShown(trail)}>
                  {trail.map(mount)}
                </div>
              </>
            ) : (
              rows.map(mount)
            )}
          </div>
        )}
      </div>
    </section>
  );
}

/* --- views ---------------------------------------------------------------- */

/**
 * Which panels are on screen, grouped the way the screen is.
 *
 * The menu's shape is the layout's shape: one heading per column, its panels
 * under it. That is not decoration — it is what lets "show me the map" be one
 * click that brings the route and the fix along with it, while leaving the
 * operator a second click to drop the waypoint list and keep the map.
 *
 * A button plus a checkbox popover rather than a row of toggles: the row grew
 * with every panel added and pushed the navigation it sat beside off the bar.
 * A native `<select multiple>` cannot be styled to match and is awkward with a
 * pointer.
 */
export function ViewsMenu({
  visible,
  onToggle,
  onToggleSection,
}: {
  visible: PanelVisibility;
  onToggle: (id: string) => void;
  onToggleSection: (id: SectionId) => void;
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
          {SECTIONS.map((section) => {
            const panels = toggleable(section.id);

            if (panels.length === 0) {
              return null;
            }

            const check = sectionCheck(section.id, visible);

            return (
              <div key={section.id}>
                <label className="views__tier">
                  <Tristate
                    check={check}
                    label={section.label}
                    onChange={() => {
                      onToggleSection(section.id);
                    }}
                  />
                  <span>{section.label}</span>
                  {sectionOccupied(section.id, visible) ? null : (
                    <span className="views__off">off</span>
                  )}
                </label>
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

/**
 * A checkbox that can say "some of this".
 *
 * `indeterminate` is a DOM property with no attribute behind it, so React
 * cannot set it from JSX and it has to be written on the node. `aria-checked`
 * is not a substitute: a native checkbox reports `mixed` from the property, and
 * setting the attribute as well makes two sources of truth for one state.
 */
function Tristate({
  check,
  label,
  onChange,
}: {
  check: 'on' | 'off' | 'mixed';
  label: string;
  onChange: () => void;
}) {
  return (
    <input
      type="checkbox"
      ref={(node) => {
        if (node !== null) node.indeterminate = check === 'mixed';
      }}
      checked={check === 'on'}
      aria-label={`${label} section`}
      onChange={onChange}
    />
  );
}
