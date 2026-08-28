/**
 * Pure projections used by the resolution workflow.
 *
 * Vitest runs without a DOM here, so the attestation flow's interaction is not
 * reachable from these tests; it is a browser gate. What is testable is that
 * the countdown never depends on the browser's clock, and that a transaction
 * predating the resolution contract reads as unknown.
 */

import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';
import { ArmState, CommandResolutionSchema } from '@/gen/gcs/v1/commands_pb';
import { observedLabel, quarantineDurationMs } from './ArmControl';

describe('quarantineDurationMs', () => {
  it('measures the window from the server timestamps alone', () => {
    const resolution = create(CommandResolutionSchema, {
      attestedAt: timestampFromDate(new Date('2026-01-01T00:00:00Z')),
      quarantineUntil: timestampFromDate(new Date('2026-01-01T00:00:30Z')),
    });
    // A browser clock skewed by hours must not change this number.
    expect(quarantineDurationMs(resolution)).toBe(30_000);
  });

  it('is zero when either timestamp is missing', () => {
    expect(quarantineDurationMs(undefined)).toBe(0);
    expect(quarantineDurationMs(create(CommandResolutionSchema, {
      attestedAt: timestampFromDate(new Date('2026-01-01T00:00:00Z')),
    }))).toBe(0);
  });

  it('never reports a negative window', () => {
    const resolution = create(CommandResolutionSchema, {
      attestedAt: timestampFromDate(new Date('2026-01-01T00:00:30Z')),
      quarantineUntil: timestampFromDate(new Date('2026-01-01T00:00:00Z')),
    });
    expect(quarantineDurationMs(resolution)).toBe(0);
  });
});

describe('observedLabel', () => {
  it('names an explicit attestation', () => {
    expect(observedLabel(ArmState.ARMED)).toBe('armed');
    expect(observedLabel(ArmState.DISARMED)).toBe('disarmed');
  });

  it('reads a recording predating the contract as unknown', () => {
    // Never 'disarmed': an absent value must not read as a state the operator
    // reported, exactly as an absent reading never renders as a number.
    expect(observedLabel(ArmState.UNSPECIFIED)).toBe('an unknown state');
  });
});
