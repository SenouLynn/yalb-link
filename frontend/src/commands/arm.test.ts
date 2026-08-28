import { afterEach, describe, expect, it, vi } from 'vitest';
import { CommandState } from '@/gen/gcs/v1/commands_pb';
import { CommandHTTPError, postArm } from './arm';

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
});
