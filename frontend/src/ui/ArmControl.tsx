import { useEffect, useState } from 'react';
import { CommandState, type CommandTransaction } from '@/gen/gcs/v1/commands_pb';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { VehicleView } from '@/fleet/state';
import type { StreamSource } from '@/stream/select';
import { CommandHTTPError, postArm } from '@/commands/arm';

const CONFIRM_MS = 3000;

export function ArmControl({ view, connected, source, latest }: {
  view: VehicleView;
  connected: boolean;
  source: StreamSource;
  latest?: CommandTransaction | undefined;
}) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<CommandTransaction | undefined>(latest);
  const [error, setError] = useState<string | null>(null);
  const arm = !(view.heartbeat?.armed ?? false);

  useEffect(() => setResult(latest), [latest]);
  useEffect(() => {
    if (!confirming) return;
    const handle = globalThis.setTimeout(() => setConfirming(false), CONFIRM_MS);
    return () => globalThis.clearTimeout(handle);
  }, [confirming]);

  const disabled = source !== 'live' || !connected || view.lifecycle === FleetEventType.VEHICLE_LOST || busy;
  const label = `${arm ? 'ARM' : 'DISARM'}`;

  const act = async () => {
    if (!confirming) { setConfirming(true); setError(null); return; }
    setConfirming(false); setBusy(true); setError(null);
    try { setResult(await postArm(view.sysId, arm)); }
    catch (cause) {
      if (cause instanceof CommandHTTPError && cause.status === 409) {
        setError(cause.message.includes('unresolved') ? 'Previous command unresolved — restart the backend.' : cause.message);
      } else { setError(cause instanceof Error ? cause.message : 'Command failed.'); }
    } finally { setBusy(false); }
  };

  return (
    <div className="panel command-control">
      <button type="button" disabled={disabled} onClick={() => void act()}>
        {busy ? 'WAITING…' : confirming ? `CONFIRM ${label}` : label}
      </button>
      <span className="command-control__state" role="status">
        {error ?? (result === undefined ? 'No command issued' : stateLabel(result.state))}
      </span>
    </div>
  );
}

function stateLabel(state: CommandState): string {
  switch (state) {
    case CommandState.PENDING: return 'Command pending';
    case CommandState.ACCEPTED: return 'Command accepted';
    case CommandState.REJECTED: return 'Command rejected';
    case CommandState.TIMED_OUT: return 'Command timed out — backend restart required';
    case CommandState.CANCELLED: return 'Command cancelled — backend restart required';
    case CommandState.SEND_FAILED: return 'Send failed — delivery unknown; backend restart required';
    default: return 'Command state unknown';
  }
}
