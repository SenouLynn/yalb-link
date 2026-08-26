import { fromJson } from '@bufbuild/protobuf';
import { CommandTransactionSchema, type CommandTransaction } from '@/gen/gcs/v1/commands_pb';

export const ARM_PATH = '/api/commands/arm';

export class CommandHTTPError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
    this.name = 'CommandHTTPError';
  }
}

export async function postArm(systemId: number, arm: boolean): Promise<CommandTransaction> {
  const response = await globalThis.fetch(ARM_PATH, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ system_id: systemId, arm }),
  });
  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new CommandHTTPError(detail || `Command returned HTTP ${String(response.status)}.`, response.status);
  }
  return fromJson(CommandTransactionSchema, await response.json());
}
