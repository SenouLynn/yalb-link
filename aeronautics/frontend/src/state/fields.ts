// The numeric field editing model.
//
// A worksheet field has two separate things in it: the text the builder is
// typing, and the value that has been committed. They are kept apart on
// purpose. "", "-" and "1." are perfectly ordinary things to have on the way to
// a number, and treating them as errors — or worse, as zero — makes a field
// fight the person filling it in.
//
// Enter and blur commit. Escape restores the committed value. An invalid commit
// keeps the text so nothing the builder typed is lost, and says why.

/** Commitment is what committing a field's text would do. */
export type Commitment =
  | { readonly kind: 'value'; readonly value: number }
  | { readonly kind: 'cleared' }
  | { readonly kind: 'invalid'; readonly reason: string }

// A decimal number, with an optional sign, an optional fractional part whose
// digits may be absent ("1." is a number a person is finished typing), and an
// optional exponent. It deliberately does not accept "Infinity" or "NaN".
const decimal = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/

/**
 * isEditingState reports whether text is on the way to a number rather than a
 * finished one. A field in an editing state shows no error: the builder has not
 * finished, and complaining mid-keystroke is noise.
 */
export function isEditingState(text: string): boolean {
  const trimmed = text.trim()
  if (trimmed === '') return true
  if (/^[+-]?$/.test(trimmed)) return true
  if (/^[+-]?\.$/.test(trimmed)) return true
  // A trailing dot is an unfinished decimal. Committing it is still fine — a
  // person who types "1." and presses Enter means 1 — but nothing complains
  // about it while the next digit is on its way.
  if (trimmed.endsWith('.')) return true
  return /[eE][+-]?$/.test(trimmed)
}

/**
 * commitText decides what committing this text means. Empty means the value is
 * withdrawn, never zero: a builder who clears a field has not said "nought".
 */
export function commitText(text: string): Commitment {
  const trimmed = text.trim()
  if (trimmed === '') return { kind: 'cleared' }
  if (!decimal.test(trimmed)) {
    return { kind: 'invalid', reason: `${text} is not a number` }
  }
  const value = Number(trimmed)
  if (!Number.isFinite(value)) {
    return { kind: 'invalid', reason: `${text} is not a finite number` }
  }
  return { kind: 'value', value }
}

/**
 * displayNumber renders a committed value for an input box. It is presentation
 * only: the underlying value is whatever the boundary holds, and rounding here
 * never changes what is saved or evaluated.
 *
 * Values are shown to a fixed number of significant digits rather than a fixed
 * number of decimals, so a span in metres and an area in square centimetres are
 * both readable without either losing its meaning.
 */
export function displayNumber(value: number, significantDigits = 6): string {
  if (!Number.isFinite(value)) return ''
  if (value === 0) return '0'
  const rendered = value.toPrecision(significantDigits)
  // toPrecision keeps trailing zeros and can produce exponent form for
  // ordinary magnitudes; Number round-trips it back to the shortest exact form.
  const shortest = String(Number(rendered))
  return shortest
}
