/**
 * The panel registry.
 *
 * Adding information to the screen is one entry here plus a renderer. There is
 * no layout CSS to write: slots are flex-sized, so whatever is mounted fits.
 *
 * Only composition knows the inventory. Feature modules never register
 * themselves — a module that could add itself to the screen would make the
 * screen's contents depend on import order, and nobody could answer "what is
 * on screen" by reading one file.
 */

/**
 * Where a panel lives.
 *
 * Slots are roles, not coordinates. `rail` is the always-on context column,
 * `center` is the thing being looked at, `aux` is the instrument stack, and
 * `dev` is the resident developer tier. Panels in `dev` share a tab strip;
 * panels in every other slot stack vertically.
 */
export type SlotId = 'rail' | 'center' | 'aux' | 'dev';

/**
 * Who a panel is for.
 *
 * Operators and developers have different information ceilings on the same
 * screen. `dev` panels may show raw wire values and decoder state; `operator`
 * panels may not, because a number an operator cannot act on is a number that
 * competes with one they can.
 */
export type PanelTier = 'operator' | 'dev';

export interface PanelDef {
  id: string;
  /** Shown in the panel header and in the Views menu. */
  label: string;
  slot: SlotId;
  tier: PanelTier;
  /** Whether the operator may hide it. A panel carrying the link's own health
   *  is not hideable: losing it is how you stop knowing the display is lying. */
  fixed?: boolean;
  /** Renders without a group header — the map and the instrument stack draw
   *  their own chrome, and a title bar above them is a second competing header. */
  bare?: boolean;
}

/**
 * Left-to-right, top-to-bottom order within each slot. This list is the single
 * source of order: the shell renders by mapping it, so DOM order and visual
 * order cannot drift apart.
 */
export const PANELS: readonly PanelDef[] = [
  { id: 'link', label: 'Link', slot: 'rail', tier: 'operator', fixed: true },
  { id: 'state', label: 'Target state', slot: 'rail', tier: 'operator' },
  { id: 'command', label: 'Command', slot: 'rail', tier: 'operator' },
  { id: 'position', label: 'Position', slot: 'rail', tier: 'operator' },
  { id: 'mission', label: 'Mission', slot: 'rail', tier: 'operator' },
  { id: 'guidance', label: 'Guidance', slot: 'rail', tier: 'operator', bare: true },
  { id: 'home', label: 'Home', slot: 'rail', tier: 'operator', bare: true },
  { id: 'radiolink', label: 'Radio link', slot: 'rail', tier: 'operator', bare: true },

  { id: 'map', label: 'Map', slot: 'center', tier: 'operator', bare: true },

  { id: 'instruments', label: 'Instruments', slot: 'aux', tier: 'operator', bare: true },

  { id: 'families', label: 'Families', slot: 'dev', tier: 'dev' },
  { id: 'sample', label: 'Raw sample', slot: 'dev', tier: 'dev' },
];

export type PanelVisibility = Readonly<Record<string, boolean>>;

/** Everything visible. The default: this is a monitoring surface, and a value
 *  the operator has to go and switch on is a value they will not have when it
 *  matters. */
export const DEFAULT_VISIBILITY: PanelVisibility = Object.fromEntries(
  PANELS.map((panel) => [panel.id, true]),
);

export const SLOT_ORDER: readonly SlotId[] = ['rail', 'center', 'aux', 'dev'];

export function panelsInSlot(slot: SlotId): readonly PanelDef[] {
  return PANELS.filter((panel) => panel.slot === slot);
}

/** Whether a slot has anything to show, which is what decides if it mounts. */
export function slotOccupied(slot: SlotId, visible: PanelVisibility): boolean {
  return panelsInSlot(slot).some((panel) => visible[panel.id] === true);
}

/** Panels the operator may toggle, grouped for the Views menu. */
export function toggleable(tier: PanelTier): readonly PanelDef[] {
  return PANELS.filter((panel) => panel.tier === tier && panel.fixed !== true);
}
