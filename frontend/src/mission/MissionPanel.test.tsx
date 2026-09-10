import { create } from '@bufbuild/protobuf';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { MissionItemSchema, MissionSnapshotSchema } from '@/gen/gcs/v1/missions_pb';
import { MavCmd, MavFrame } from '@/gen/gcs/v1/types_pb';
import { MissionPanel, WaypointList } from './MissionPanel';
import { missionGeometry } from './model';

const snapshot = create(MissionSnapshotSchema, { items: [create(MissionItemSchema, {
  seq: 4, command: MavCmd.DO_SET_SERVO, frame: MavFrame.MISSION,
  param1: 1, param2: 2, param3: 3, param4: 4, x: 5, y: 6, z: 7, autocontinue: true,
})] });

describe('WaypointList', () => {
  it('shows all item fields, active state, and unsupported geometry explanation', () => {
    const html = renderToStaticMarkup(<WaypointList snapshot={snapshot} geometry={missionGeometry(snapshot)} activeSeq={4} />);
    expect(html).toContain('#4 DO_SET_SERVO');
    expect(html).toContain('Param 1');
    expect(html).toContain('Autocontinue');
    expect(html).toContain('yes');
    expect(html).toContain('Not mapped:');
    expect(html).toContain('Active');
  });

  it('names the panel that fetches the mission rather than showing an empty list', () => {
    const html = renderToStaticMarkup(<WaypointList snapshot={null} geometry={{ points: [], omitted: {} }} activeSeq={null} />);
    expect(html).toContain('Mission panel');
    expect(html).not.toContain('DO_SET_SERVO');
  });

  it('separates an empty onboard mission from one that was never downloaded', () => {
    const empty = create(MissionSnapshotSchema, { items: [] });
    const html = renderToStaticMarkup(<WaypointList snapshot={empty} geometry={{ points: [], omitted: {} }} activeSeq={null} />);
    expect(html).toContain('empty mission');
  });
});

describe('MissionPanel', () => {
  it.each([
    ['idle', 'Not downloaded'], ['loading', 'Downloading…'],
    ['error', 'Download failed'],
  ] as const)('distinguishes %s posture', (status, label) => {
    const html = renderToStaticMarkup(<MissionPanel status={status} snapshot={null} error={status === 'error' ? 'link failed' : null} activeSeq={null} onDownload={() => undefined} />);
    expect(html).toContain(label);
  });

  it('names the current waypoint from the downloaded snapshot', () => {
    const html = renderToStaticMarkup(<MissionPanel status="complete" snapshot={snapshot} error={null} activeSeq={4} onDownload={() => undefined} />);
    expect(html).toContain('#4 DO_SET_SERVO');
  });

  it('reports the sequence alone when no snapshot names it', () => {
    // MISSION_CURRENT arrives on its own; the name needs a download. Withholding
    // the number until then would hide route state the vehicle volunteered.
    const html = renderToStaticMarkup(<MissionPanel status="idle" snapshot={null} error={null} activeSeq={2} onDownload={() => undefined} />);
    expect(html).toContain('#2');
  });

  it('carries no waypoint list; that is the map section\'s panel', () => {
    const html = renderToStaticMarkup(<MissionPanel status="complete" snapshot={snapshot} error={null} activeSeq={4} onDownload={() => undefined} />);
    expect(html).not.toContain('Autocontinue');
  });
});
