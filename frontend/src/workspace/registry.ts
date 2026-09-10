/**
 * The panel registry: what is on screen, in what order, in whose column.
 *
 * Two tables and nothing else. `SECTIONS` says which columns exist and how they
 * divide the width; `PANELS` says which section each panel belongs to. Moving a
 * panel between columns is an edit to one field, and adding a column is one row
 * here plus its share of the width — there is no layout CSS to write, because a
 * column's geometry is derived from its `grow` and its panels are flex-sized
 * inside it.
 *
 * That is the whole reason the modularity is worth its cost. The map, the
 * waypoint list and the position rows were three panels in three different
 * columns; making them one Map section is four `section:` fields, not a
 * re-layout.
 *
 * Only composition knows the inventory. Feature modules never register
 * themselves — a module that could add itself to the screen would make the
 * screen's contents depend on import order, and nobody could answer "what is on
 * screen" by reading one file.
 */

/**
 * Who a panel is for.
 *
 * Operators and developers have different information ceilings on the same
 * screen. `dev` panels may show raw wire values and decoder state; `operator`
 * panels may not, because a number an operator cannot act on is a number that
 * competes with one they can.
 */
export type PanelTier = 'operator' | 'dev';

/**
 * A column, named for the job it does rather than for where it sits.
 *
 * `vehicle` is what this aircraft is and what state it is in — the column that
 * is never not wanted. `map` is where it is, including the route it was given
 * and the fixes it has reported. `instruments` is everything derived from those
 * two: attitude, the flight path, the consulted families. `dev` is the resident
 * developer tier, stacked under the instruments.
 */
export type SectionId = 'vehicle' | 'map' | 'instruments' | 'dev';

export interface SectionDef {
  id: SectionId;
  /** Shown as the section heading in the Views menu. */
  label: string;
  tier: PanelTier;
  /**
   * Share of the width left over after the fixed columns.
   *
   * `0` is a fixed column sized in CSS. The map outweighs the instruments 3:1,
   * which is what "the map takes the bulk" means as a number — and because the
   * share is a share and not a width, a section left alone beside the sidebar
   * grows into everything the hidden ones gave up.
   */
  grow: number;
  /** Renders stacked inside another section's column instead of beside it. */
  stackedIn?: SectionId;
  /** Panels share one tab strip rather than stacking. */
  tabbed?: boolean;
}

export const SECTIONS: readonly SectionDef[] = [
  { id: 'vehicle', label: 'Vehicle', tier: 'operator', grow: 0 },
  { id: 'map', label: 'Map', tier: 'operator', grow: 3 },
  { id: 'instruments', label: 'Instruments', tier: 'operator', grow: 1 },
  { id: 'dev', label: 'Developer', tier: 'dev', grow: 0, stackedIn: 'instruments', tabbed: true },
];

export interface PanelDef {
  id: string;
  /** Shown in the panel header and in the Views menu. */
  label: string;
  section: SectionId;
  /** Whether the operator may hide it. A panel carrying the link's own health
   *  is not hideable: losing it is how you stop knowing the display is lying. */
  fixed?: boolean;
  /**
   * Owns the space its section's other panels do not use.
   *
   * At most one per section. The map is the only one: it is a viewport rather
   * than a run of rows, so it takes the column's remaining height edge to edge
   * while the route and the fix sit under it in a bounded, scrolling region.
   * Hide those and the map has the column.
   */
  bleed?: boolean;

  /**
   * Which side of a split row region the panel sits on.
   *
   * A section whose rows declare sides lays them out as two columns rather than
   * as one stack, and only while both sides have something to show — one side
   * alone takes the full width rather than leaving a gutter where the other
   * would have been. The waypoint list is a long column of its own and the
   * readings that describe progress along it are short: side by side they fit
   * in the band under the map, stacked they would push it off the screen.
   */
  side?: 'lead' | 'trail';
}

/**
 * Top-to-bottom order within each section. This list is the single source of
 * order: the shell renders by mapping it, so DOM order and visual order cannot
 * drift apart.
 */
export const PANELS: readonly PanelDef[] = [
  { id: 'link', label: 'Link', section: 'vehicle', fixed: true },
  { id: 'state', label: 'Target state', section: 'vehicle' },
  { id: 'command', label: 'Command', section: 'vehicle' },
  { id: 'mission', label: 'Mission', section: 'vehicle' },

  { id: 'map', label: 'Map', section: 'map', bleed: true },
  { id: 'waypoints', label: 'Waypoints', section: 'map', side: 'lead' },
  { id: 'position', label: 'Position', section: 'map', side: 'trail' },
  { id: 'guidance', label: 'Guidance', section: 'map', side: 'trail' },

  { id: 'instruments', label: 'Instruments', section: 'instruments' },
  { id: 'radiolink', label: 'Radio link', section: 'instruments' },

  { id: 'families', label: 'Families', section: 'dev' },
  { id: 'sample', label: 'Raw sample', section: 'dev' },
];

export type PanelVisibility = Readonly<Record<string, boolean>>;

/** Everything visible. The default: this is a monitoring surface, and a value
 *  the operator has to go and switch on is a value they will not have when it
 *  matters. */
export const DEFAULT_VISIBILITY: PanelVisibility = Object.fromEntries(
  PANELS.map((panel) => [panel.id, true]),
);

/** The sections that mount as their own column, left to right. */
export const COLUMN_SECTIONS: readonly SectionDef[] = SECTIONS.filter(
  (section) => section.stackedIn === undefined,
);

/** The sections stacked inside `parent`'s column, in order, beneath its own. */
export function stackedIn(parent: SectionId): readonly SectionDef[] {
  return SECTIONS.filter((section) => section.stackedIn === parent);
}

export function sectionOf(id: SectionId): SectionDef | undefined {
  return SECTIONS.find((section) => section.id === id);
}

export function panelsInSection(section: SectionId): readonly PanelDef[] {
  return PANELS.filter((panel) => panel.section === section);
}

/** Whether a section has anything to show, which is what decides if it mounts. */
export function sectionOccupied(section: SectionId, visible: PanelVisibility): boolean {
  return panelsInSection(section).some((panel) => visible[panel.id] === true);
}

/**
 * Whether a column has anything to show — its own section and anything stacked
 * inside it. A column whose instruments are hidden still mounts while the
 * developer tier under it is on.
 */
export function columnOccupied(section: SectionId, visible: PanelVisibility): boolean {
  return (
    sectionOccupied(section, visible) ||
    stackedIn(section).some((nested) => sectionOccupied(nested.id, visible))
  );
}

/** Panels the operator may hide, in a section. */
export function toggleable(section: SectionId): readonly PanelDef[] {
  return panelsInSection(section).filter((panel) => panel.fixed !== true);
}

/**
 * How a section's own checkbox reads: on when every hideable panel in it is on,
 * off when none is, mixed in between.
 *
 * Mixed is a real third state and not a rounding of the other two. A section
 * whose map is on and whose waypoints are off is not "off", and rendering it so
 * would put the operator one click away from hiding the map they are flying on.
 */
export type SectionCheck = 'on' | 'off' | 'mixed';

export function sectionCheck(section: SectionId, visible: PanelVisibility): SectionCheck {
  const panels = toggleable(section);

  if (panels.length === 0) {
    return sectionOccupied(section, visible) ? 'on' : 'off';
  }

  const shown = panels.filter((panel) => visible[panel.id] === true).length;

  if (shown === 0) return 'off';

  return shown === panels.length ? 'on' : 'mixed';
}

/**
 * The visibility a section's own checkbox produces when clicked.
 *
 * Anything short of fully on turns the whole section on — including mixed, so
 * the section heading always reads as "give me all of this" rather than as a
 * toggle whose direction the operator has to work out from the current state.
 */
export function toggleSection(section: SectionId, visible: PanelVisibility): PanelVisibility {
  const target = sectionCheck(section, visible) !== 'on';

  return {
    ...visible,
    ...Object.fromEntries(toggleable(section).map((panel) => [panel.id, target])),
  };
}
