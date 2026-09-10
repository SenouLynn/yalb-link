/**
 * The primitive vocabulary. Every panel in the app is built from these.
 *
 * The set is deliberately closed and deliberately small. Adding a value to the
 * screen must not require a design decision — that is the whole point, and it
 * is what the previous system failed at: with only `--gap` to work from, each
 * new panel invented its own spacing and type size until ten different values
 * meant nothing in particular.
 *
 * If something here cannot express a new panel, extend a primitive rather than
 * styling the panel directly. A one-off style is how the next drift starts.
 */

import type { ReactNode } from 'react';

import { Provenance } from './Provenance';
import type { Reading } from './readings';

/* --- group ---------------------------------------------------------------- */

export interface GroupProps {
  /** Uppercase section label. Names a category, not a sentence. */
  label: string;
  /** Right-aligned annotation on the header line — a count, an age, a source. */
  note?: string | undefined;
  /** Controls belonging to the group, placed on the header line. */
  actions?: ReactNode;
  /**
   * Provenance for every row in this group, rendered on the header line.
   *
   * Use this whenever a group's rows all resolve from one MAVLink family — the
   * source belongs to the table, not to each row, and repeating it per row
   * breaks the shared value edge the rows exist to form.
   */
  reading?: Reading<unknown> | undefined;
  /**
   * Shown instead of the body when there is nothing to report. Say what is
   * absent and why, never "no data" — an operator cannot act on that.
   */
  absent?: string | undefined;
  children?: ReactNode;
}

/** A labelled section of rows, separated from its neighbours by a hairline. */
export function Group({ label, note, actions, reading, absent, children }: GroupProps) {
  return (
    <section className="group">
      <div className="group__head">
        <span className="group__label">{label}</span>
        {actions}
        {note === undefined ? null : <span className="group__note">{note}</span>}
        {reading === undefined || absent !== undefined ? null : <Provenance reading={reading} />}
      </div>
      {absent === undefined ? (
        <div className="group__body">{children}</div>
      ) : (
        <p className="group__absent">{absent}</p>
      )}
    </section>
  );
}

/* --- row ------------------------------------------------------------------ */

/**
 * How a value is coloured.
 *
 * There is no `good` tone on purpose. Normal flight is unremarkable, and a row
 * that lights up to say nothing is wrong trains an operator to ignore it.
 */
export type Tone = 'normal' | 'caution' | 'dead';

export interface RowProps {
  label: string;
  /** Pre-formatted. Formatting decisions belong to `format.ts`, not here. */
  value: string;
  unit?: string | undefined;
  tone?: Tone | undefined;
  /** The one available emphasis: a group's headline value. Use once per group. */
  lead?: boolean | undefined;
  /** Wraps the value under the label — for coordinates and other long strings. */
  stacked?: boolean | undefined;
  /**
   * A second, dimmer line under the label.
   *
   * For meaning the operator cannot infer and that must not be truncated — a
   * sign convention like "+ below target" is the difference between reading a
   * value correctly and reading it backwards. Putting it on its own line is
   * deliberate: letting such a phrase wrap inside the label orphans its last
   * word beside the next row's number, which reads as a second value.
   */
  note?: string | undefined;
}

/**
 * The atom: label left, value right-aligned against a shared edge.
 *
 * The shared edge is the reason this is a grid rather than a flex row. A column
 * of rows has to scan as one table the eye can run down; values that each stop
 * wherever their string ends read as separate objects that happen to be nearby.
 */
export function Row({ label, value, unit, tone = 'normal', lead, stacked, note }: RowProps) {
  const classes = ['row'];

  if (tone !== 'normal') classes.push(`row--${tone}`);
  if (lead === true) classes.push('row--lead');
  if (stacked === true) classes.push('row--stacked');

  return (
    <div className={classes.join(' ')}>
      <span className="row__label">
        {label}
        {note === undefined ? null : <span className="row__note">{note}</span>}
      </span>
      <span className="row__value">
        {value}
        {unit === undefined || unit === '' ? null : <span className="row__unit">{unit}</span>}
      </span>
    </div>
  );
}

/* --- lever ---------------------------------------------------------------- */

export interface LeverProps {
  children: ReactNode;
  onClick?: (() => void) | undefined;
  /** Amber-edged. For an action whose effect on the aircraft is the hazard. */
  caution?: boolean | undefined;
  /** Fills its container — a form's commit action, not a lever in a row. */
  wide?: boolean | undefined;
  disabled?: boolean | undefined;
  /** Renders the toggled-on state for a control that holds a position. */
  pressed?: boolean | undefined;
  title?: string | undefined;
  'aria-controls'?: string | undefined;
  'aria-label'?: string | undefined;
}

/**
 * The one button shape in the app.
 *
 * Variants change colour and width, never form. A second button shape is how a
 * system stops meaning anything: if the commit action looks different in two
 * places, one of them is wrong.
 */
export function Lever({
  children,
  onClick,
  caution,
  wide,
  disabled,
  pressed,
  title,
  ...aria
}: LeverProps) {
  const classes = ['lever'];

  if (caution === true) classes.push('lever--caution');
  if (wide === true) classes.push('lever--wide');

  return (
    <button
      type="button"
      className={classes.join(' ')}
      onClick={onClick}
      disabled={disabled}
      title={title}
      {...(pressed === undefined ? {} : { 'aria-pressed': pressed })}
      {...aria}
    >
      {children}
    </button>
  );
}

/** A horizontal run of levers. The command surface of a panel. */
export function LeverRow({ children, label }: { children: ReactNode; label?: string }) {
  return (
    <div className="lever-row" {...(label === undefined ? {} : { role: 'group', 'aria-label': label })}>
      {children}
    </div>
  );
}

/* --- field ---------------------------------------------------------------- */

export interface FieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string | undefined;
  disabled?: boolean | undefined;
  inputMode?: 'decimal' | 'numeric' | 'text' | undefined;
}

/** A labelled input, sized on the target axis rather than the density axis. */
export function Field({ label, value, onChange, placeholder, disabled, inputMode }: FieldProps) {
  return (
    <label className="field">
      <span className="row__label">{label}</span>
      <input
        className="field__input"
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
        placeholder={placeholder}
        disabled={disabled}
        inputMode={inputMode}
        autoComplete="off"
      />
    </label>
  );
}

/**
 * A checkbox and the assertion it confirms.
 *
 * The assertion is spelled out in full rather than abbreviated to "Confirm":
 * the operator is attesting to something specific, and a checkbox beside the
 * word "confirm" attests to nothing.
 */
export function Confirm({
  assertion,
  checked,
  onChange,
  disabled,
}: {
  assertion: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <label className="field field--check">
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => {
          onChange(event.target.checked);
        }}
      />
      <span>{assertion}</span>
    </label>
  );
}

/* --- tabs ----------------------------------------------------------------- */

export interface TabDef {
  id: string;
  label: string;
}

/**
 * Several panels sharing one slot, one visible at a time.
 *
 * This is how the developer tier stays resident without spending a column on
 * every inspection view at once.
 */
export function Tabs({
  tabs,
  active,
  onSelect,
  note,
  label,
}: {
  tabs: readonly TabDef[];
  active: string;
  onSelect: (id: string) => void;
  /** Right-aligned status on the strip — a count, a rate, a receiving tally. */
  note?: string | undefined;
  label: string;
}) {
  return (
    <div className="tabs" role="tablist" aria-label={label}>
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          role="tab"
          className="tab"
          aria-selected={tab.id === active}
          onClick={() => {
            onSelect(tab.id);
          }}
        >
          {tab.label}
        </button>
      ))}
      {note === undefined ? null : <span className="tabs__note">{note}</span>}
    </div>
  );
}
