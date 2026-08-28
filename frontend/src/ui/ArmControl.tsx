import { useEffect, useState } from 'react';
import { CommandState, type CommandResolution, type CommandTransaction } from '@/gen/gcs/v1/commands_pb';
import { ArmState } from '@/gen/gcs/v1/commands_pb';
import { FleetEventType } from '@/gen/gcs/v1/fleet_pb';
import type { VehicleView } from '@/fleet/state';
import type { StreamSource } from '@/stream/select';
import { CommandHTTPError, postArm, postResolve, type ObservedArmState } from '@/commands/arm';

const CONFIRM_MS = 3000;

/**
 * The quarantine window, measured from the server's own two timestamps.
 *
 * Both come from the backend, so their difference is a duration the browser can
 * count down locally without ever comparing a server instant to its own clock.
 * The countdown only explains the wait: the server refuses a premature command
 * regardless, and answers with a fresh remaining time when it does.
 */
export function quarantineDurationMs(resolution: CommandResolution | undefined): number {
  const until = resolution?.quarantineUntil;
  const attested = resolution?.attestedAt;
  if (until === undefined || attested === undefined) return 0;
  const seconds = Number(until.seconds - attested.seconds);
  return Math.max(0, seconds * 1000 + Math.round((until.nanos - attested.nanos) / 1e6));
}

export function ArmControl({ view, connected, source, latest, nowMs }: {
  view: VehicleView;
  connected: boolean;
  source: StreamSource;
  latest?: CommandTransaction | undefined;
  nowMs: number;
}) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<CommandTransaction | undefined>(latest);
  const [error, setError] = useState<string | null>(null);
  const [unresolved, setUnresolved] = useState<CommandTransaction | null>(null);
  const [quarantineMs, setQuarantineMs] = useState<number | null>(null);
  const arm = !(view.heartbeat?.armed ?? false);

  useEffect(() => {
    setResult(latest);
  }, [latest]);
  useEffect(() => {
    if (!confirming) return;
    const handle = globalThis.setTimeout(() => {
      setConfirming(false);
    }, CONFIRM_MS);
    return () => {
      globalThis.clearTimeout(handle);
    };
  }, [confirming]);
  useEffect(() => {
    if (quarantineMs === null) return;
    if (quarantineMs <= 0) {
      setQuarantineMs(null);
      return;
    }
    const handle = globalThis.setTimeout(() => {
      setQuarantineMs((ms) => (ms === null ? null : Math.max(0, ms - 1000)));
    }, 1000);
    return () => {
      globalThis.clearTimeout(handle);
    };
  }, [quarantineMs]);

  const absorb = (cause: unknown) => {
    if (cause instanceof CommandHTTPError) {
      if (cause.code === 'command_unresolved' && cause.transaction !== undefined) {
        // Discoverable after a reload: the backend named the transaction.
        setUnresolved(cause.transaction);
        setError(null);
        return;
      }
      if (cause.code === 'command_quarantined') {
        setUnresolved(null);
        setQuarantineMs(cause.retryAfterMs ?? 0);
        setError(null);
        return;
      }
      setError(cause.message);
      return;
    }
    setError(cause instanceof Error ? cause.message : 'Command failed.');
  };

  const act = async () => {
    if (!confirming) {
      setConfirming(true);
      setError(null);
      return;
    }
    setConfirming(false);
    setBusy(true);
    setError(null);
    try {
      setResult(await postArm(view.sysId, arm));
    } catch (cause) {
      absorb(cause);
    } finally {
      setBusy(false);
    }
  };

  const attest = async (observed: ObservedArmState) => {
    if (unresolved === null) return;
    setBusy(true);
    setError(null);
    try {
      const resolved = await postResolve(view.sysId, unresolved.registryEpoch, unresolved.id, observed);
      setUnresolved(null);
      setResult(resolved);
      // No automatic retry: the operator must decide again, deliberately.
      setQuarantineMs(quarantineDurationMs(resolved.resolution));
    } catch (cause) {
      absorb(cause);
    } finally {
      setBusy(false);
    }
  };

  if (unresolved !== null) {
    return (
      <div className="panel command-control command-control--unresolved">
        <p className="command-control__ambiguous" role="status">
          {`Command ${String(unresolved.id)} ${terminalPhrase(unresolved.state)}. What the vehicle did is unknown.`}
        </p>
        <p className="command-control__evidence">{heartbeatEvidence(view, nowMs)}</p>
        <p className="command-control__prompt">Report the armed state you observed:</p>
        <div className="command-control__attest">
          <button type="button" disabled={busy} onClick={() => void attest('ARMED')}>
            OBSERVED ARMED
          </button>
          <button type="button" disabled={busy} onClick={() => void attest('DISARMED')}>
            OBSERVED DISARMED
          </button>
        </div>
        {error !== null && <span className="command-control__state" role="alert">{error}</span>}
      </div>
    );
  }

  const remainingS = quarantineMs === null ? 0 : Math.ceil(quarantineMs / 1000);
  const quarantined = remainingS > 0;
  const disabled = source !== 'live'
    || !connected
    || view.lifecycle === FleetEventType.VEHICLE_LOST
    || busy
    || quarantined;
  const label = arm ? 'ARM' : 'DISARM';

  return (
    <div className="panel command-control">
      <button type="button" disabled={disabled} onClick={() => void act()}>
        {busy ? 'WAITING…' : confirming ? `CONFIRM ${label}` : label}
      </button>
      <span className="command-control__state" role="status">
        {quarantined
          ? `Commanding paused for ${String(remainingS)}s after the resolution`
          : (error ?? (result === undefined ? 'No command issued' : stateLabel(result)))}
      </span>
    </div>
  );
}

/** The heartbeat is evidence for the operator, never proof of the outcome. */
function heartbeatEvidence(view: VehicleView, nowMs: number): string {
  if (view.heartbeat === undefined) {
    return 'No heartbeat has been received from this vehicle.';
  }
  const state = view.heartbeat.armed ? 'ARMED' : 'DISARMED';
  if (view.lastFleetAtMs === undefined) {
    return `Last heartbeat reported ${state}. Evidence only.`;
  }
  const ageS = Math.max(0, Math.round((nowMs - view.lastFleetAtMs) / 1000));
  return `Last heartbeat reported ${state}, ${String(ageS)}s ago. Evidence only.`;
}

function terminalPhrase(state: CommandState): string {
  switch (state) {
    case CommandState.TIMED_OUT: return 'timed out';
    case CommandState.CANCELLED: return 'was cancelled';
    case CommandState.SEND_FAILED: return 'may or may not have been delivered';
    default: return 'did not settle';
  }
}

function stateLabel(tx: CommandTransaction): string {
  if (tx.resolution !== undefined) {
    return `Resolved: operator observed ${observedLabel(tx.resolution.observedState)}`;
  }
  switch (tx.state) {
    case CommandState.PENDING: return 'Command pending';
    case CommandState.ACCEPTED: return 'Command accepted';
    case CommandState.REJECTED: return 'Command rejected';
    case CommandState.TIMED_OUT: return 'Command timed out — outcome unresolved';
    case CommandState.CANCELLED: return 'Command cancelled — outcome unresolved';
    case CommandState.SEND_FAILED: return 'Send failed — delivery unknown, outcome unresolved';
    default: return 'Command state unknown';
  }
}

/**
 * Recordings predating the resolution contract decode as UNSPECIFIED. They read
 * as unknown rather than defaulting to either state, the same way an absent
 * reading never renders as a number.
 */
export function observedLabel(state: ArmState): string {
  switch (state) {
    case ArmState.ARMED: return 'armed';
    case ArmState.DISARMED: return 'disarmed';
    default: return 'an unknown state';
  }
}
