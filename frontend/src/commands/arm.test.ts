import { afterEach, describe, expect, it, vi } from 'vitest';
import { CommandState } from '@/gen/gcs/v1/commands_pb';
import { CommandHTTPError, postArm, postResolve } from './arm';

afterEach(() => vi.restoreAllMocks());

describe('postArm', () => {
  it('posts strict JSON and parses the synchronous transaction', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 1, vehicleId: { systemId: 7, componentId: 1 }, command: 'MAV_CMD_COMPONENT_ARM_DISARM',
      state: 'COMMAND_STATE_ACCEPTED', issuedAt: '2026-01-01T00:00:00Z', settledAt: '2026-01-01T00:00:01Z',
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    const tx = await postArm(7, true);
    expect(tx.state).toBe(CommandState.ACCEPTED);
    expect(fetch).toHaveBeenCalledWith('/api/commands/arm', expect.objectContaining({
      method: 'POST', body: JSON.stringify({ system_id: 7, arm: true }),
    }));
  });

  it('preserves a poisoned-key 409 message', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('previous command unresolved — restart the backend\n', { status: 409 }));
    await expect(postArm(1, false)).rejects.toEqual(
      new CommandHTTPError('previous command unresolved — restart the backend', 409),
    );
  });

  it('reads a poisoned 409 as the transaction that must be resolved', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      code: 'command_unresolved',
      transaction: {
        id: 7, registryEpoch: 'abc123', vehicleId: { systemId: 1, componentId: 1 },
        command: 'MAV_CMD_COMPONENT_ARM_DISARM', state: 'COMMAND_STATE_TIMED_OUT',
        requestedState: 'ARM_STATE_ARMED', operatorLabel: 'local-operator',
      },
    }), { status: 409, headers: { 'Content-Type': 'application/json' } }));

    // Without the reference the operator cannot say which ambiguity they saw.
    const error = await postArm(1, true).catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(CommandHTTPError);
    const failure = error as CommandHTTPError;
    expect(failure.code).toBe('command_unresolved');
    expect(failure.transaction?.id).toBe(7);
    expect(failure.transaction?.registryEpoch).toBe('abc123');
    expect(failure.transaction?.state).toBe(CommandState.TIMED_OUT);
  });

  it('reads a quarantine 409 as a wait, not an attestation', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      code: 'command_quarantined', retry_after_ms: 18420,
    }), { status: 409, headers: { 'Content-Type': 'application/json' } }));

    const error = await postArm(1, true).catch((cause: unknown) => cause);
    const failure = error as CommandHTTPError;
    expect(failure.code).toBe('command_quarantined');
    expect(failure.retryAfterMs).toBe(18420);
    // Nothing to attest to: the quarantine names no transaction.
    expect(failure.transaction).toBeUndefined();
  });

  it('distinguishes the two conflicts rather than merging them', async () => {
    const bodies = [
      { code: 'command_unresolved', transaction: { id: 1, registryEpoch: 'e' } },
      { code: 'command_quarantined', retry_after_ms: 500 },
    ];
    const codes: (string | undefined)[] = [];
    for (const body of bodies) {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(
        new Response(JSON.stringify(body), { status: 409, headers: { 'Content-Type': 'application/json' } }),
      );
      const error = await postArm(1, true).catch((cause: unknown) => cause);
      codes.push((error as CommandHTTPError).code);
    }
    expect(codes[0]).not.toBe(codes[1]);
  });

  it('falls back to the response text for an untyped 409', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('a command is already in flight for this vehicle\n', { status: 409 }),
    );
    const error = await postArm(1, true).catch((cause: unknown) => cause);
    const failure = error as CommandHTTPError;
    expect(failure.code).toBeUndefined();
    expect(failure.message).toBe('a command is already in flight for this vehicle');
  });
});

describe('postResolve', () => {
  it('sends the transaction reference and an explicit observed state', async () => {
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 7, state: 'COMMAND_STATE_TIMED_OUT',
      resolution: { observedState: 'ARM_STATE_DISARMED', operatorLabel: 'local-operator' },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }));

    const tx = await postResolve(1, 'abc123', 7, 'DISARMED');

    // The terminal state is never rewritten by an attestation.
    expect(tx.state).toBe(CommandState.TIMED_OUT);
    expect(fetch).toHaveBeenCalledWith('/api/commands/arm/resolve', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({
        system_id: 1, registry_epoch: 'abc123', transaction_id: 7, observed_state: 'DISARMED',
      }),
    }));
  });
});
