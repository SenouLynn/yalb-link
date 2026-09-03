import { create } from '@bufbuild/protobuf';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { MissionItemSchema, MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { MavCmd, MavFrame } from '@/gen/gcs/v1/types_pb';
import { MissionPanel } from './MissionPanel';
import { missionGeometry } from './model';

describe('MissionPanel', () => {
  it('shows all item fields, active state, and unsupported geometry explanation', () => {
    const snapshot = create(MissionSnapshotSchema, { items: [create(MissionItemSchema, {
      seq: 4, command: MavCmd.DO_SET_SERVO, frame: MavFrame.MISSION,
      param1: 1, param2: 2, param3: 3, param4: 4, x: 5, y: 6, z: 7, autocontinue: true,
    })] });
    const html = renderToStaticMarkup(<MissionPanel status="complete" snapshot={snapshot} error={null} geometry={missionGeometry(snapshot)} activeSeq={4} onDownload={() => undefined} />);
    expect(html).toContain('#4 DO_SET_SERVO');
    expect(html).toContain('params [1, 2, 3, 4]');
    expect(html).toContain('autocontinue yes');
    expect(html).toContain('Not mapped:');
    expect(html).toContain('Active');
  });

  it.each([
    ['idle', 'Not downloaded'], ['loading', 'Loading current vehicle'],
    ['error', 'Download failed'],
  ] as const)('distinguishes %s posture', (status, label) => {
    const html = renderToStaticMarkup(<MissionPanel status={status} snapshot={null} error={status === 'error' ? 'link failed' : null} geometry={{ points: [], omitted: {} }} activeSeq={null} onDownload={() => undefined} />);
    expect(html).toContain(label);
  });
});
