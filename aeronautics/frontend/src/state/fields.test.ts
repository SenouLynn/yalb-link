import { describe, expect, test } from 'vitest'
import { commitText, displayNumber, isEditingState } from './fields.ts'

describe('what counts as still being typed', () => {
  test('the states on the way to a number are not errors', () => {
    for (const text of ['', ' ', '-', '+', '.', '-.', '1.', '12.', '1e', '2.5e-']) {
      expect(isEditingState(text), text).toBe(true)
    }
  })

  test('a finished number is not an editing state', () => {
    for (const text of ['0', '1', '-1.5', '.5', '2.5e-3']) {
      expect(isEditingState(text), text).toBe(false)
    }
  })
})

describe('committing a field', () => {
  test('clearing a field withdraws the value and never means zero', () => {
    expect(commitText('')).toEqual({ kind: 'cleared' })
    expect(commitText('   ')).toEqual({ kind: 'cleared' })
    // The distinction matters: a supplied zero is a claim about the aircraft
    // and an empty field is the absence of one.
    expect(commitText('0')).toEqual({ kind: 'value', value: 0 })
  })

  test('ordinary numbers commit, in every form a person types them', () => {
    expect(commitText('1.2')).toEqual({ kind: 'value', value: 1.2 })
    expect(commitText(' -3 ')).toEqual({ kind: 'value', value: -3 })
    // Still an editing state while typing, and a number once committed.
    expect(commitText('1.')).toEqual({ kind: 'value', value: 1 })
    expect(commitText('.25')).toEqual({ kind: 'value', value: 0.25 })
    expect(commitText('2.5e-3')).toEqual({ kind: 'value', value: 0.0025 })
  })

  test('text that is not a number is refused with a reason, not swallowed', () => {
    for (const text of ['-', 'abc', '1,2', '1 2', 'Infinity', 'NaN', '0x10']) {
      const commitment = commitText(text)
      expect(commitment.kind, text).toBe('invalid')
      if (commitment.kind === 'invalid') {
        expect(commitment.reason).toContain(text)
      }
    }
  })
})

describe('display formatting', () => {
  test('a value is shown at readable precision without changing it', () => {
    expect(displayNumber(0.41694940476190476)).toBe('0.416949')
    expect(displayNumber(1.2)).toBe('1.2')
    expect(displayNumber(0)).toBe('0')
    // The rounding is presentation only: the number it was given is untouched,
    // and the field sends the committed text rather than the rendered one.
    expect(Number(displayNumber(1.23456789))).not.toBe(1.23456789)
  })
})
