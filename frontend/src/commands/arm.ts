import { fromJson, type JsonValue } from '@bufbuild/protobuf';
import { CommandTransactionSchema, type CommandTransaction } from '@/gen/gcs/v1/commands_pb';

export const ARM_PATH = '/api/commands/arm';
export const RESOLVE_PATH = '/api/commands/arm/resolve';

/**
 * The two conflicts the arm endpoint distinguishes.
 *
 * `command_unresolved` needs an operator attestation about what the vehicle
 * actually did; `command_quarantined` only needs waiting. Conflating them would
 * either ask for an attestation that has already been given, or leave the
 * operator with nothing to do but retry.
 */
export type CommandErrorCode = 'command_unresolved' | 'command_quarantined';

/** The armed state an operator reports having observed. */
export type ObservedArmState = 'ARMED' | 'DISARMED';

export class CommandHTTPError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: CommandErrorCode,
    /** The ambiguous transaction, when the backend named one. */
    readonly transaction?: CommandTransaction,
    /**
     * Milliseconds of quarantine left, as the server reported them. Relative on
     * purpose: the browser never decides expiry from its own clock.
     */
    readonly retryAfterMs?: number,
  ) {
    super(message);
    this.name = 'CommandHTTPError';
  }
}

/** Reads a failed response, preferring the typed conflict bodies. */
async function failure(response: Response): Promise<CommandHTTPError> {
  const raw = (await response.text()).trim();
  const fallback = raw || `Command returned HTTP ${String(response.status)}.`;

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return new CommandHTTPError(fallback, response.status);
  }
  if (typeof parsed !== 'object' || parsed === null) {
    return new CommandHTTPError(fallback, response.status);
  }

  const body = parsed as { code?: unknown; transaction?: unknown; retry_after_ms?: unknown };

  if (body.code === 'command_unresolved' && body.transaction !== undefined) {
    return new CommandHTTPError(
      'The previous command outcome is unresolved.',
      response.status,
      'command_unresolved',
      fromJson(CommandTransactionSchema, body.transaction as JsonValue),
    );
  }
  if (body.code === 'command_quarantined') {
    const remaining = typeof body.retry_after_ms === 'number' ? body.retry_after_ms : 0;
    return new CommandHTTPError(
      'Commanding is paused after the resolution.',
      response.status,
      'command_quarantined',
      undefined,
      remaining,
    );
  }
  return new CommandHTTPError(fallback, response.status);
}

async function postCommand(path: string, body: unknown): Promise<CommandTransaction> {
  const response = await globalThis.fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw await failure(response);
  }
  const payload: unknown = await response.json();
  return fromJson(CommandTransactionSchema, payload as JsonValue);
}

export async function postArm(systemId: number, arm: boolean): Promise<CommandTransaction> {
  return postCommand(ARM_PATH, { system_id: systemId, arm });
}

/**
 * Attests to the armed state the operator observed, clearing one ambiguity.
 *
 * The transaction reference is sent back verbatim so a stale tab cannot resolve
 * an ambiguity it never saw; the backend rejects a mismatched epoch or id.
 */
export async function postResolve(
  systemId: number,
  registryEpoch: string,
  transactionId: number,
  observedState: ObservedArmState,
): Promise<CommandTransaction> {
  return postCommand(RESOLVE_PATH, {
    system_id: systemId,
    registry_epoch: registryEpoch,
    transaction_id: transactionId,
    observed_state: observedState,
  });
}
