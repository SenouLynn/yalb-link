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

import type { ReactNode, Ref, AriaAttributes } from 'react';

import { Provenance } from './Provenance';
import type { Reading } from './readings';

/* --- group ---------------------------------------------------------------- */

export interface GroupProps {
  className?: string;
  'aria-label'?: string;
  /** Uppercase section label. Names a category, not a sentence. */
  label: string;
  /**
   * Right-aligned annotation on the header line — a count, an age, a source.
   *
   * Named `annotation` rather than `note` because `Row.note` is a different
   * thing in the opposite place: a dimmer second line *under* a label. One prop
   * name meaning two opposite things is how a vocabulary stops being one.
   */
  annotation?: string | undefined;
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
export function Group({ label, annotation, actions, reading, absent, children, className, 'aria-label': ariaLabel }: GroupProps) {
  return (
    <section className={`group${className ? ` ${className}` : ''}`} aria-label={ariaLabel}>
      <div className="group__head">
        <span className="group__label">{label}</span>
        {actions}
        {annotation === undefined ? null : <span className="group__annotation">{annotation}</span>}
        {reading === undefined || absent !== undefined ? null : <Provenance reading={reading} />}
      </div>
      {absent === undefined ? (
        <div className="group__body">{children}</div>
      ) : (
        <Note tone="absent">{absent}</Note>
      )}
    </section>
  );
}

export function Chip({ children, tone = 'normal' }: { children: ReactNode; tone?: 'normal' | 'active' | 'caution' | 'dead' }) {
  return <span className={`chip chip--${tone}`}>{children}</span>;
}

export function Note({ children, tone = 'normal', role }: { children: ReactNode; tone?: 'normal' | 'caution' | 'absent'; role?: 'status' | 'alert' }) {
  return <p className={`note note--${tone}`} role={role}>{children}</p>;
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

export interface LeverProps extends AriaAttributes {
  ref?: Ref<HTMLButtonElement>;
  children: ReactNode;
  onClick?: (() => void) | undefined;
  /**
   * Navigates instead of acting. Renders an anchor, which is what a thing that
   * changes the address has to be — it must open in a new tab, be copied, and
   * be read as a link by assistive technology.
   */
  href?: string | undefined;
  /** Amber-edged. For an action whose effect on the aircraft is the hazard. */
  caution?: boolean | undefined;
  /** Fills its container — a group's committing action, not a lever in a row. */
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
 *
 * There are exactly two placements and no third. A lever either sits inline in
 * a `LeverRow` at its content width, or it is `wide` and fills its group as
 * that group's one committing action. Anything else — a lever floated beside
 * its own status text, a lever stretched by a rule in a feature stylesheet — is
 * a placement decision being made at a call site, which is the thing this
 * vocabulary exists to prevent.
 *
 * The label is written in sentence case and uppercased by CSS, the same way
 * `Group` and `Row` labels are. Screaming the string at the call site produces
 * the identical pixels by a second mechanism, and then half the app is written
 * one way and half the other.
 */
export function Lever({
  children,
  onClick,
  href,
  caution,
  wide,
  disabled,
  pressed,
  title,
  ref,
  ...aria
}: LeverProps) {
  const classes = ['lever'];

  if (caution === true) classes.push('lever--caution');
  if (wide === true) classes.push('lever--wide');

  if (href !== undefined) {
    return (
      <a className={classes.join(' ')} href={href} title={title} {...aria}>
        {children}
      </a>
    );
  }

  return (
    <button
      type="button"
      className={classes.join(' ')}
      onClick={onClick}
      disabled={disabled}
      title={title}
      ref={ref}
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
  options?: readonly { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string | undefined;
  disabled?: boolean | undefined;
  inputMode?: 'decimal' | 'numeric' | 'text' | undefined;
}

/** A labelled input, sized on the target axis rather than the density axis. */
export function Field({ label, value, onChange, placeholder, disabled, inputMode, options }: FieldProps) {
  return (
    <label className="field">
      <span className="row__label">{label}</span>
      {options ? <select className="field__input" value={value} disabled={disabled}
        onChange={(event) => { onChange(event.target.value); }}>
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select> : <input
        className="field__input"
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
        placeholder={placeholder}
        disabled={disabled}
        inputMode={inputMode}
        autoComplete="off"
      />}
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
          id={`tab-${tab.id}`}
          aria-controls={`panel-${tab.id}`}
          tabIndex={tab.id === active ? 0 : -1}
          onKeyDown={(event) => {
            const index = tabs.findIndex((item) => item.id === tab.id);
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1
              : event.key === 'ArrowRight' ? (index + 1) % tabs.length
              : event.key === 'ArrowLeft' ? (index - 1 + tabs.length) % tabs.length : null;
            if (next === null) return;
            event.preventDefault();
            const target = tabs[next];
            if (target) {
              onSelect(target.id);
              document.getElementById(`tab-${target.id}`)?.focus();
            }
          }}
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
