/** Transport controls for a recorded flight. */

import { Chip, Confirm, Field, Group, Lever, LeverRow, Note } from '@/ui/primitives';

import { useEffect, useState, useSyncExternalStore } from 'react';

import {
  deleteRecording,
  RECORDINGS_PATH,
  RecordingHTTPError,
  type RecordingWire,
  type ReplayEventSource,
} from '@/stream/replay';
import { replayUrl } from '@/stream/select';

import { NO_VALUE } from './format';

/** Offered playback speeds. Real time first, because it is the honest one. */
const SPEEDS = [1, 2, 4, 8];

export interface ReplayControlsProps {
  /** The running replay, or null when no recording has been chosen. */
  source: ReplayEventSource | null;
}

export function ReplayControls({ source }: ReplayControlsProps) {
  if (source === null) {
    return <RecordingPicker />;
  }

  return <Transport source={source} />;
}

function Transport({ source }: { source: ReplayEventSource }) {
  // Both arguments must be stable references: `subscribe` is a bound field on
  // the source and `status` is cached there, so React neither resubscribes nor
  // re-renders unless playback actually moved.
  const status = useSyncExternalStore(source.subscribe, source.status);

  const spanMs = status.endedAtMs - status.startedAtMs;
  const offsetMs = status.positionMs - status.startedAtMs;

  if (status.error !== null) {
    /*
     * The label names the panel, not its condition. Every other panel keeps its
     * own name and says what is wrong inside it; a panel that renames itself is
     * a section the operator cannot find twice.
     */
    return <Group label="Replay" className="replay" absent={status.error} />;
  }

  return (
    <Group label="Replay" className="replay" annotation={status.recording?.name ?? NO_VALUE}>
      <LeverRow label="Transport">
        <Lever
          onClick={() => {
            if (status.playing) {
              source.pause();
            } else {
              source.play();
            }
          }}
          disabled={status.loading || status.totalEvents === 0}
        >
          {status.playing ? 'Pause' : 'Play'}
        </Lever>

        <span className="replay__clock">
          {formatOffset(offsetMs)} / {formatOffset(spanMs)}
        </span>

        <input
          type="range"
          className="replay__scrub"
          min={0}
          max={Math.max(spanMs, 1)}
          step={100}
          value={Math.round(offsetMs)}
          aria-label="Playback position"
          onChange={(change) => {
            source.seekTo(status.startedAtMs + Number(change.target.value));
          }}
        />

        <Field label="Speed" value={String(status.speed)}
          onChange={(value) => { source.setSpeed(Number(value)); }}
          options={SPEEDS.map((speed) => ({ value: String(speed), label: `${String(speed)}×` }))} />
      </LeverRow>

      {status.loading && <Note>Loading…</Note>}
      {status.truncated && (
        <Note tone="caution">
          first {status.totalEvents} events only
        </Note>
      )}
    </Group>
  );
}

/** Lists recordings so one can be opened. Shown when none is selected. */
function RecordingPicker() {
  const [recordings, setRecordings] = useState<RecordingWire[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<number | null>(null);
  /*
   * Deleting a recording is destructive to data, not to the aircraft, so it is
   * not amber — amber says something is wrong with the vehicle, and spending it
   * here is how that meaning erodes. The guard is an explicit attestation
   * instead, which is also how the app stops needing the one native
   * `confirm()` dialog it had: a browser modal carries no design system, cannot
   * be styled, and states the name of the thing in a different voice.
   */
  const [pending, setPending] = useState<number | null>(null);
  const [confirmed, setConfirmed] = useState(false);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const response = await globalThis.fetch(RECORDINGS_PATH);

        if (response.status === 404) {
          // The routes exist only when recording is enabled, so a 404 here is
          // a configuration answer rather than a missing recording.
          throw new Error('Recording is disabled on this backend (GCS_RECORDING_ENABLED).');
        }
        if (!response.ok) {
          throw new Error(`The backend returned HTTP ${String(response.status)}.`);
        }
        const listed = (await response.json()) as RecordingWire[];

        if (!cancelled) {
          setRecordings(listed);
        }
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Group label="Replay" className="replay">
      {error !== null && <Note tone="caution">{error}</Note>}
      {error === null && recordings === null && <Note>Loading…</Note>}
      {error === null && recordings?.length === 0 && (
        <Note>No recordings yet.</Note>
      )}
      {recordings?.map((recording) => {
        const label = recording.name === '' ? `recording #${String(recording.id)}` : recording.name;
        const remove = () => {
          setDeleting(recording.id);
          setPending(null);
          setConfirmed(false);
          setError(null);
          void deleteRecording(recording.id)
            .then(() => {
              setRecordings((current) => current?.filter((item) => item.id !== recording.id) ?? []);
            })
            .catch((cause: unknown) => {
              if (cause instanceof RecordingHTTPError && cause.status === 409) {
                setError('Stop the recording first.');
              } else {
                setError(cause instanceof Error ? cause.message : String(cause));
              }
            })
            .finally(() => {
              setDeleting(null);
            });
        };

        return (
          <div key={recording.id}>
            <LeverRow label={label}>
              <Lever href={replayUrl(recording.id)}>
                {recording.name === '' ? `#${String(recording.id)}` : recording.name}
              </Lever>
              <Chip>{recording.event_count} events · {recording.status}</Chip>
              <Lever
                disabled={deleting === recording.id}
                aria-label={`Delete ${label}`}
                onClick={() => {
                  setPending(recording.id);
                  setConfirmed(false);
                }}
              >
                {deleting === recording.id ? 'Deleting…' : 'Delete'}
              </Lever>
            </LeverRow>
            {pending === recording.id && (
              <>
                <Confirm
                  assertion={`Permanently delete ${label}`}
                  checked={confirmed}
                  onChange={setConfirmed}
                />
                <LeverRow label={`Confirm deleting ${label}`}>
                  <Lever disabled={!confirmed} onClick={remove}>Delete permanently</Lever>
                  <Lever onClick={() => { setPending(null); setConfirmed(false); }}>Cancel</Lever>
                </LeverRow>
              </>
            )}
          </div>
        );
      })}
    </Group>
  );
}

/** Elapsed time into the recording, as m:ss. */
export function formatOffset(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return NO_VALUE;
  }

  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${String(minutes)}:${String(seconds).padStart(2, '0')}`;
}
