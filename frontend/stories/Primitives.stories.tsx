import { useState } from 'react';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip, Note, Confirm, Field, Group, Lever, LeverRow, Row, Tabs } from '@/ui/primitives';

/**
 * The whole vocabulary on one page.
 *
 * This is the reference: if a panel needs something that is not here, the
 * answer is to extend a primitive, not to style the panel. A one-off style is
 * how the last system drifted into ten spacing values that meant nothing.
 */
function Vocabulary() {
  const [confirmed, setConfirmed] = useState(false);
  const [latitude, setLatitude] = useState('47.393227');
  const [tab, setTab] = useState('families');

  return (
    <div style={{ width: 240, maxWidth: '100%' }}>
      <Group label="Rows" annotation="the atom">
        <Row label="Ground speed" value="17.5" unit="m/s" />
        <Row label="Altitude" value="501.3" unit="m" />
        <Row label="Armed" value="ARMED" tone="caution" />
        <Row label="EKF" value="- - -" tone="dead" />
        <Row label="Remaining" value="87" unit="%" lead />
        <Row label="Position" value="47.393227, 8.545423" stacked />
      </Group>

      <Group label="Levers" annotation="target axis">
        <LeverRow label="Flight modes">
          <Lever>Arm</Lever>
          <Lever>Take off</Lever>
          <Lever caution>Disarm</Lever>
          <Lever disabled>Land</Lever>
        </LeverRow>
        <LeverRow>
          <Lever pressed>Follow</Lever>
          <Lever pressed={false}>Track up</Lever>
        </LeverRow>
      </Group>

      <Group label="Notes and status">
        <Chip tone="active">Active</Chip> <Chip tone="caution">LOST</Chip>
        <Note>Snapshot retained until the next download.</Note>
        <Note tone="caution">Command outcome is unresolved.</Note>
        <Note tone="absent">Position has not been received.</Note>
      </Group>
      <Group label="Fields">
        <Field label="Latitude" value={latitude} onChange={setLatitude} inputMode="decimal" />
        <Field label="Longitude" value="" onChange={() => undefined} placeholder="required" />
        <Confirm
          assertion="I confirm target 1:1, its armed Guided state, and this bounded movement."
          checked={confirmed}
          onChange={setConfirmed}
        />
        <LeverRow>
          <Lever wide disabled={!confirmed}>
            Send reposition
          </Lever>
        </LeverRow>
      </Group>

      <Group label="Absent">
        <Row label="Home" value="- - -" tone="dead" />
      </Group>

      <Group
        label="Never heard"
        absent="Not received. ArduPilot sends home when home is set, not on an interval."
      />

      <Tabs
        label="Developer panels"
        tabs={[
          { id: 'families', label: 'Families' },
          { id: 'sample', label: 'Raw sample' },
        ]}
        active={tab}
        onSelect={setTab}
        note="19/19 receiving"
      />
      <div id="panel-families" role="tabpanel" aria-labelledby="tab-families" hidden={tab !== 'families'}>Family inspection</div>
      <div id="panel-sample" role="tabpanel" aria-labelledby="tab-sample" hidden={tab !== 'sample'}>Raw sample inspection</div>
    </div>
  );
}

const meta = {
  title: 'System/Vocabulary',
  component: Vocabulary,
} satisfies Meta<typeof Vocabulary>;
export default meta;
export const All: StoryObj<typeof meta> = {};
