import { fromJson, type JsonValue } from '@bufbuild/protobuf';

import { MissionSnapshotSchema, type MissionSnapshot } from '@/gen/gcs/v1/missions_pb';

interface ErrorBody { code?: string; message?: string }

export async function downloadMission(sysId: number, compId: number, signal?: AbortSignal): Promise<MissionSnapshot> {
  const response = await fetch(`/api/vehicles/${String(sysId)}/${String(compId)}/mission`, signal === undefined ? {} : { signal });
  const body: unknown = await response.json();
  if (!response.ok) {
    const error = body as ErrorBody;
    throw new Error(error.message ?? error.code ?? `mission download failed (${String(response.status)})`);
  }
  return fromJson(MissionSnapshotSchema, body as JsonValue);
}
