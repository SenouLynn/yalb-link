#!/usr/bin/env python3
"""Generate golden MAVLink byte fixtures for the Go codec tests.

Run deliberately, never at build time:

    python3 -m venv .venv && .venv/bin/pip install pymavlink
    .venv/bin/python scripts/gen_mavlink_fixtures.py

Output lands in contracts/mavlink/ and is committed. The fixtures are the
primary source, not a fallback: every committed fixture must be regenerable
from this script and its pinned Python dependencies.

Each family produces a pair:
  <name>.bin   raw frame bytes, exactly as they arrive on the wire
  <name>.json  the fields that were encoded, plus the proto values the codec
               is expected to normalise them into

The .json `fields` block is in **wire units** — that is what was encoded. The
`expected_proto` block, present only where a unit changes, pins what the codec
must produce. Keeping both in one file is what makes a fixture evidence for a
NORM-* matrix row rather than just a decode test.

Determinism matters: every value here is a literal, nothing reads the clock,
so regenerating produces byte-identical output. If a diff appears in
contracts/ after a regeneration, the dialect changed and that is worth knowing.
"""

import json
import pathlib
import sys

try:
    from pymavlink.dialects.v20 import ardupilotmega as mav2
    from pymavlink.dialects.v10 import ardupilotmega as mav1
except ImportError:
    sys.exit("pymavlink not installed: pip install pymavlink")

OUT = pathlib.Path(__file__).resolve().parent.parent / "contracts" / "mavlink"

# Vehicle 1, autopilot component. The GCS is (255, 190); anything inbound is
# from a vehicle, so these are vehicle-side identities.
SYS_ID = 1
COMP_ID = 1

# Fixed sequence number so regeneration is byte-stable.
SEQ = 42

# Seattle. Plausible values on purpose — a reviewer spots a 1e7 scaling error
# instantly in 47.6 and never in 476000000.
LAT_DEG = 47.6062
LON_DEG = -122.3321
LAT_E7 = int(round(LAT_DEG * 1e7))
LON_E7 = int(round(LON_DEG * 1e7))


class Sink:
    """Collects the bytes MAVLink.send() writes."""

    def __init__(self):
        self.buf = bytearray()

    def write(self, data):
        self.buf.extend(data)


def encode(dialect, builder, seq=SEQ):
    """Encode one message and return its complete frame bytes."""
    sink = Sink()
    link = dialect.MAVLink(sink, srcSystem=SYS_ID, srcComponent=COMP_ID)
    link.seq = seq
    link.send(builder(link))

    return bytes(sink.buf)


def write(name, raw, meta):
    """Write a .bin/.json fixture pair."""
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / f"{name}.bin").write_bytes(raw)
    (OUT / f"{name}.json").write_text(json.dumps(meta, indent=2, sort_keys=True) + "\n")
    print(f"  {name}.bin ({len(raw)} bytes)")


def fixture(name, message_id, message_name, dialect, builder, fields,
            expected_proto=None, seq=SEQ, note=None):
    raw = encode(dialect, builder, seq=seq)
    meta = {
        "message_id": message_id,
        "message_name": message_name,
        "sys_id": SYS_ID,
        "comp_id": COMP_ID,
        "seq": seq,
        "fields": fields,
    }

    if expected_proto is not None:
        meta["expected_proto"] = expected_proto

    if note is not None:
        meta["note"] = note

    write(name, raw, meta)


# --- receive: streaming telemetry (13 families -> TelemetryEvent) -----------


def gen_telemetry():
    print("receive / telemetry:")

    fixture(
        "heartbeat_v2", 0, "HEARTBEAT", mav2,
        lambda m: m.heartbeat_encode(
            type=2,          # MAV_TYPE_QUADROTOR
            autopilot=3,     # MAV_AUTOPILOT_ARDUPILOTMEGA
            base_mode=209,   # armed | custom mode enabled | stabilize | guided
            custom_mode=4,   # ArduCopter GUIDED
            system_status=4, # MAV_STATE_ACTIVE
        ),
        fields={
            "type": 2, "autopilot": 3, "base_mode": 209,
            "custom_mode": 4, "system_status": 4,
        },
        note="base_mode bit 7 (128) is MAV_MODE_FLAG_SAFETY_ARMED — this vehicle is armed",
    )

    fixture(
        "sys_status_v2", 1, "SYS_STATUS", mav2,
        lambda m: m.sys_status_encode(
            onboard_control_sensors_present=327695,
            onboard_control_sensors_enabled=327695,
            onboard_control_sensors_health=327695,
            load=250,                # 25.0% in centi-percent
            voltage_battery=12587,   # mV
            current_battery=1050,    # cA
            battery_remaining=87,    # %
            drop_rate_comm=0,
            errors_comm=0,
            errors_count1=0, errors_count2=0, errors_count3=0, errors_count4=0,
        ),
        fields={
            "load": 250, "voltage_battery": 12587,
            "current_battery": 1050, "battery_remaining": 87,
        },
    )

    fixture(
        "gps_raw_int_v2", 24, "GPS_RAW_INT", mav2,
        lambda m: m.gps_raw_int_encode(
            time_usec=1234567890,
            fix_type=3,              # 3D fix
            lat=LAT_E7, lon=LON_E7,
            alt=120500,              # mm MSL
            eph=121, epv=200,
            vel=1500,                # cm/s
            cog=9000,                # centidegrees
            satellites_visible=14,
        ),
        fields={
            "fix_type": 3, "lat": LAT_E7, "lon": LON_E7, "alt": 120500,
            "vel": 1500, "cog": 9000, "satellites_visible": 14,
        },
        expected_proto={
            "lat_deg": LAT_DEG, "lon_deg": LON_DEG,
            "alt_msl_m": 120.5,
            # vel and cog keep wire units in the proto: the 65535 unknown
            # sentinel is defined in those units and is lost across a divide.
            "vel_cm_s": 1500, "cog_cdeg": 9000,
        },
        note="GPS_RAW_INT altitude is MSL, never relative — the datum differs from GLOBAL_POSITION_INT",
    )

    fixture(
        "attitude_v2", 30, "ATTITUDE", mav2,
        lambda m: m.attitude_encode(
            time_boot_ms=12345,
            roll=0.0174533,      # 1 degree
            pitch=-0.0087266,    # -0.5 degrees
            yaw=1.5708,          # 90 degrees
            rollspeed=0.01, pitchspeed=-0.02, yawspeed=0.15,
        ),
        fields={
            "time_boot_ms": 12345,
            "roll": 0.0174533, "pitch": -0.0087266, "yaw": 1.5708,
            "rollspeed": 0.01, "pitchspeed": -0.02, "yawspeed": 0.15,
        },
        note="radians on the wire and radians in the proto — no conversion on this path",
    )

    fixture(
        "global_position_int_v2", 33, "GLOBAL_POSITION_INT", mav2,
        lambda m: m.global_position_int_encode(
            time_boot_ms=12345,
            lat=LAT_E7, lon=LON_E7,
            alt=120500,          # mm MSL
            relative_alt=50250,  # mm above home
            vx=1200, vy=-300, vz=150,  # cm/s, NED
            hdg=9000,            # centidegrees
        ),
        fields={
            "lat": LAT_E7, "lon": LON_E7, "alt": 120500,
            "relative_alt": 50250, "vx": 1200, "vy": -300, "vz": 150,
            "hdg": 9000,
        },
        expected_proto={
            "lat_deg": LAT_DEG, "lon_deg": LON_DEG,
            "alt_msl_m": 120.5, "alt_relative_m": 50.25,
            "vx_m_s": 12.0, "vy_m_s": -3.0, "vz_m_s": 1.5,
            "hdg_cdeg": 9000,
        },
        note="vz is positive DOWN; climb rate is -vz_m_s and there is no second divide",
    )

    fixture(
        "mission_current_v2", 42, "MISSION_CURRENT", mav2,
        lambda m: m.mission_current_encode(seq=3),
        fields={"seq": 3},
    )

    fixture(
        "nav_controller_output_v2", 62, "NAV_CONTROLLER_OUTPUT", mav2,
        lambda m: m.nav_controller_output_encode(
            nav_roll=5.5, nav_pitch=-2.25,
            nav_bearing=90, target_bearing=95,
            wp_dist=250,
            alt_error=1.5, aspd_error=0.25, xtrack_error=3.75,
        ),
        fields={
            "nav_roll": 5.5, "nav_pitch": -2.25, "nav_bearing": 90,
            "target_bearing": 95, "wp_dist": 250, "alt_error": 1.5,
            "aspd_error": 0.25, "xtrack_error": 3.75,
        },
    )

    fixture(
        "vfr_hud_v2", 74, "VFR_HUD", mav2,
        lambda m: m.vfr_hud_encode(
            airspeed=18.5, groundspeed=17.25,
            heading=90,
            throttle=55,
            alt=120.5,
            climb=2.5,
        ),
        fields={
            "airspeed": 18.5, "groundspeed": 17.25, "heading": 90,
            "throttle": 55, "alt": 120.5, "climb": 2.5,
        },
        note="climb is already positive-up; heading is int16 and cannot hold the 65535 sentinel",
    )

    fixture(
        "vfr_hud_negative_heading_v2", 74, "VFR_HUD", mav2,
        lambda m: m.vfr_hud_encode(
            airspeed=18.5, groundspeed=17.25,
            heading=-90,
            throttle=55, alt=120.5, climb=-1.5,
        ),
        fields={"heading": -90, "climb": -1.5},
        expected_proto={"heading_deg": -90},
        note="ArduPilot emits negative headings on this int16 field; normalisation to [0,360) is the resolver's job",
    )

    fixture(
        "radio_status_v2", 109, "RADIO_STATUS", mav2,
        lambda m: m.radio_status_encode(
            rssi=195, remrssi=190, txbuf=98,
            noise=42, remnoise=40, rxerrors=3, fixed=1,
        ),
        fields={
            "rssi": 195, "remrssi": 190, "txbuf": 98,
            "noise": 42, "remnoise": 40, "rxerrors": 3, "fixed": 1,
        },
        note="txbuf is the uplink back-pressure signal a duplicated GCS heartbeat would inflate",
    )

    fixture(
        "battery_status_v2", 147, "BATTERY_STATUS", mav2,
        lambda m: m.battery_status_encode(
            id=0,
            battery_function=1,
            type=3,
            temperature=2500,
            voltages=[4200, 4195, 4198, 4201] + [65535] * 6,
            current_battery=1050,
            current_consumed=1200,
            energy_consumed=-1,
            battery_remaining=87,
        ),
        fields={
            "id": 0, "temperature": 2500,
            "voltages": [4200, 4195, 4198, 4201] + [65535] * 6,
            "current_battery": 1050, "current_consumed": 1200,
            "battery_remaining": 87,
        },
        note="65535 in a cell voltage slot means the cell is not present, not 65.535 V",
    )

    fixture(
        "home_position_v2", 242, "HOME_POSITION", mav2,
        lambda m: m.home_position_encode(
            latitude=LAT_E7, longitude=LON_E7,
            altitude=120000,
            x=0.0, y=0.0, z=0.0,
            q=[1.0, 0.0, 0.0, 0.0],
            approach_x=0.0, approach_y=0.0, approach_z=0.0,
        ),
        fields={"latitude": LAT_E7, "longitude": LON_E7, "altitude": 120000},
        expected_proto={"lat_deg": LAT_DEG, "lon_deg": LON_DEG, "alt_msl_m": 120.0},
    )

    fixture(
        "statustext_v2", 253, "STATUSTEXT", mav2,
        lambda m: m.statustext_encode(
            severity=6,  # MAV_SEVERITY_INFO
            text=b"EKF2 IMU0 is using GPS",
        ),
        fields={"severity": 6, "text": "EKF2 IMU0 is using GPS"},
    )

    fixture(
        "ekf_status_report_v2", 193, "EKF_STATUS_REPORT", mav2,
        lambda m: m.ekf_status_report_encode(
            flags=831,
            velocity_variance=0.15,
            pos_horiz_variance=0.22,
            pos_vert_variance=0.18,
            compass_variance=0.09,
            terrain_alt_variance=0.0,
        ),
        fields={
            "flags": 831, "velocity_variance": 0.15,
            "pos_horiz_variance": 0.22, "pos_vert_variance": 0.18,
            "compass_variance": 0.09, "terrain_alt_variance": 0.0,
        },
        note="ardupilotmega dialect, not common — this is why the node uses ardupilotmega.Dialect",
    )


# --- receive: transaction responses (5 families -> ProtocolEvent) ----------


def gen_transactions():
    print("receive / transactions:")

    fixture(
        "param_value_v2", 22, "PARAM_VALUE", mav2,
        lambda m: m.param_value_encode(
            param_id=b"ARMING_CHECK",
            param_value=1.0,
            param_type=6,   # MAV_PARAM_TYPE_INT32
            param_count=1372,
            param_index=142,
        ),
        fields={
            "param_id": "ARMING_CHECK", "param_value": 1.0,
            "param_type": 6, "param_count": 1372, "param_index": 142,
        },
        note="representative integer parameter response",
    )

    fixture(
        "mission_count_v2", 44, "MISSION_COUNT", mav2,
        lambda m: m.mission_count_encode(
            target_system=255, target_component=190, count=5,
        ),
        fields={"target_system": 255, "target_component": 190, "count": 5},
        note="addressed to the GCS (255, 190)",
    )

    fixture(
        "mission_ack_v2", 47, "MISSION_ACK", mav2,
        lambda m: m.mission_ack_encode(
            target_system=255, target_component=190,
            type=0,  # MAV_MISSION_ACCEPTED
        ),
        fields={"target_system": 255, "target_component": 190, "type": 0},
    )

    fixture(
        "mission_item_int_v2", 73, "MISSION_ITEM_INT", mav2,
        lambda m: m.mission_item_int_encode(
            target_system=255, target_component=190,
            seq=1,
            frame=6,     # MAV_FRAME_GLOBAL_RELATIVE_ALT_INT
            command=16,  # MAV_CMD_NAV_WAYPOINT
            current=0, autocontinue=1,
            param1=0.0, param2=0.0, param3=0.0, param4=0.0,
            x=LAT_E7, y=LON_E7, z=50.0,
        ),
        fields={
            "seq": 1, "frame": 6, "command": 16, "autocontinue": 1,
            "x": LAT_E7, "y": LON_E7, "z": 50.0,
        },
        expected_proto={"x": LAT_DEG, "y": LON_DEG, "z": 50.0},
        note="x/y are degE7 ints but z is already float metres — only two of the three convert",
    )

    fixture(
        "command_ack_v2", 77, "COMMAND_ACK", mav2,
        lambda m: m.command_ack_encode(
            command=400,  # MAV_CMD_COMPONENT_ARM_DISARM
            result=0,     # MAV_RESULT_ACCEPTED
        ),
        fields={"command": 400, "result": 0},
        note="legacy zero-target ACK; decoded but deliberately ineligible for operator correlation",
    )

    fixture(
        "command_ack_addressed_v2", 77, "COMMAND_ACK", mav2,
        lambda m: m.command_ack_encode(
            command=400, result=0, progress=0, result_param2=0,
            target_system=255, target_component=190,
        ),
        fields={"command": 400, "result": 0, "target_system": 255, "target_component": 190},
        note="eligible ACK: frame sender is the vehicle and payload target is this GCS",
    )

    fixture(
        "command_ack_foreign_v2", 77, "COMMAND_ACK", mav2,
        lambda m: m.command_ack_encode(
            command=400, result=0, progress=0, result_param2=0,
            target_system=42, target_component=99,
        ),
        fields={"command": 400, "result": 0, "target_system": 42, "target_component": 99},
        note="foreign ACK: addressed to another GCS and ineligible for correlation",
    )


# --- send (11 families) ----------------------------------------------------


def gen_send():
    """Encoder references.

    These are what the GCS transmits, so they carry the GCS identity (255, 190)
    rather than a vehicle's. They exist so an encoder test can compare against
    an independent implementation's bytes rather than against gomavlib's own
    output, which would only prove gomavlib agrees with itself.
    """
    print("send:")

    def gcs_encode(builder, seq=SEQ):
        sink = Sink()
        link = mav2.MAVLink(sink, srcSystem=255, srcComponent=190)
        link.seq = seq
        link.send(builder(link))

        return bytes(sink.buf)

    def send_fixture(name, message_id, message_name, builder, fields, note=None):
        raw = gcs_encode(builder)
        meta = {
            "message_id": message_id,
            "message_name": message_name,
            "sys_id": 255,
            "comp_id": 190,
            "seq": SEQ,
            "fields": fields,
        }

        if note is not None:
            meta["note"] = note

        write(name, raw, meta)

    send_fixture(
        "heartbeat_gcs_out", 0, "HEARTBEAT",
        lambda m: m.heartbeat_encode(
            type=6,          # MAV_TYPE_GCS
            autopilot=0,     # MAV_AUTOPILOT_GENERIC
            base_mode=0, custom_mode=0,
            system_status=4, # MAV_STATE_ACTIVE
        ),
        fields={
            "type": 6, "autopilot": 0, "base_mode": 0,
            "custom_mode": 0, "system_status": 4,
        },
        note="gomavlib emits this itself from its own config; the fixture pins the field values",
    )

    send_fixture(
        "command_long_arm_out", 76, "COMMAND_LONG",
        lambda m: m.command_long_encode(
            target_system=1, target_component=1,
            command=400,  # MAV_CMD_COMPONENT_ARM_DISARM
            confirmation=0,
            param1=1.0,   # 1 = arm
            param2=0.0, param3=0.0, param4=0.0,
            param5=0.0, param6=0.0, param7=0.0,
        ),
        fields={"command": 400, "param1": 1.0, "param2": 0.0},
        note="param2=21196 would request force-arm; this encoder does not validate policy",
    )

    send_fixture(
        "set_position_target_global_int_out", 86, "SET_POSITION_TARGET_GLOBAL_INT",
        lambda m: m.set_position_target_global_int_encode(
            time_boot_ms=0,
            target_system=1, target_component=1,
            coordinate_frame=6,  # MAV_FRAME_GLOBAL_RELATIVE_ALT_INT
            type_mask=0xDF8,
            lat_int=LAT_E7, lon_int=LON_E7, alt=50.0,
            vx=0.0, vy=0.0, vz=0.0,
            afx=0.0, afy=0.0, afz=0.0,
            yaw=0.0, yaw_rate=0.0,
        ),
        fields={
            "coordinate_frame": 6, "type_mask": 0xDF8,
            "lat_int": LAT_E7, "lon_int": LON_E7, "alt": 50.0,
        },
        note="type_mask 0xDF8 = 3576 = position-only; the FORCE_SET bit (9) is deliberately clear",
    )

    send_fixture(
        "param_set_out", 23, "PARAM_SET",
        lambda m: m.param_set_encode(
            target_system=1, target_component=1,
            param_id=b"ARMING_CHECK",
            param_value=1.0,
            param_type=6,
        ),
        fields={"param_id": "ARMING_CHECK", "param_value": 1.0, "param_type": 6},
    )

    send_fixture(
        "param_request_list_out", 21, "PARAM_REQUEST_LIST",
        lambda m: m.param_request_list_encode(target_system=1, target_component=1),
        fields={"target_system": 1, "target_component": 1},
    )

    send_fixture(
        "param_request_read_out", 20, "PARAM_REQUEST_READ",
        lambda m: m.param_request_read_encode(
            target_system=1, target_component=1,
            param_id=b"ARMING_CHECK",
            param_index=-1,
        ),
        fields={"param_id": "ARMING_CHECK", "param_index": -1},
        note="param_index -1 means look up by name; a valid index ignores param_id",
    )

    send_fixture(
        "mission_count_out", 44, "MISSION_COUNT",
        lambda m: m.mission_count_encode(
            target_system=1, target_component=1, count=5,
        ),
        fields={"target_system": 1, "target_component": 1, "count": 5},
    )

    send_fixture(
        "mission_item_int_out", 73, "MISSION_ITEM_INT",
        lambda m: m.mission_item_int_encode(
            target_system=1, target_component=1,
            seq=1, frame=6, command=16,
            current=0, autocontinue=1,
            param1=0.0, param2=0.0, param3=0.0, param4=0.0,
            x=LAT_E7, y=LON_E7, z=50.0,
        ),
        fields={"seq": 1, "frame": 6, "command": 16, "x": LAT_E7, "y": LON_E7, "z": 50.0},
    )

    send_fixture(
        "mission_request_int_out", 51, "MISSION_REQUEST_INT",
        lambda m: m.mission_request_int_encode(
            target_system=1, target_component=1, seq=1,
        ),
        fields={"target_system": 1, "target_component": 1, "seq": 1},
    )

    send_fixture(
        "mission_ack_out", 47, "MISSION_ACK",
        lambda m: m.mission_ack_encode(
            target_system=1, target_component=1, type=0,
        ),
        fields={"target_system": 1, "target_component": 1, "type": 0},
    )

    send_fixture(
        "mission_clear_all_out", 45, "MISSION_CLEAR_ALL",
        lambda m: m.mission_clear_all_encode(
            target_system=1, target_component=1,
        ),
        fields={"target_system": 1, "target_component": 1},
        note="encoder exists; no service or UI calls it",
    )


# --- framing cases ---------------------------------------------------------


def gen_framing():
    """Frames that exercise the parser rather than any one decoder."""
    print("framing:")

    # v1 frame. The node runs OutVersion: V2 but still accepts inbound v1 —
    # that is gomavlib's default receive behaviour, and this fixture is what
    # proves it rather than assuming it.
    raw_v1 = encode(
        mav1,
        lambda m: m.heartbeat_encode(
            type=2, autopilot=3, base_mode=209, custom_mode=4, system_status=4,
        ),
    )
    write("v1_heartbeat", raw_v1, {
        "message_id": 0,
        "message_name": "HEARTBEAT",
        "sys_id": SYS_ID,
        "comp_id": COMP_ID,
        "seq": SEQ,
        "fields": {"type": 2, "autopilot": 3, "base_mode": 209, "custom_mode": 4},
        "note": "MAVLink v1 framing (STX 0xFE). Must decode without error on a V2-configured node.",
    })

    # Bad CRC: flip a payload byte after framing so the checksum no longer
    # matches. gomavlib raises EventParseError and no EventFrame surfaces.
    good = encode(
        mav2,
        lambda m: m.heartbeat_encode(
            type=2, autopilot=3, base_mode=209, custom_mode=4, system_status=4,
        ),
    )
    # Corrupt a PAYLOAD byte, not a header byte. The v2 header is 10 bytes
    # (STX, len, incompat, compat, seq, sysid, compid, msgid x3), so index 10 is
    # the first payload byte. Flipping inside the msgid field instead produces
    # an unknown message ID with a self-consistent frame, which gomavlib
    # surfaces as a MessageRaw — a completely different path that tests nothing
    # about CRC validation.
    corrupt = bytearray(good)
    corrupt[10] ^= 0xFF
    write("bad_crc", bytes(corrupt), {
        "message_id": 0,
        "message_name": "HEARTBEAT",
        "sys_id": SYS_ID,
        "comp_id": COMP_ID,
        "seq": SEQ,
        "fields": {},
        "note": (
            "Payload byte flipped after framing. Assert: no TelemetryEvent, no panic, "
            "parse-error counter incremented. Do NOT assert an error return from Decode — "
            "it returns (nil, nil) for parse errors too."
        ),
    })

    # Truncated: cut the frame mid-payload. Same observable behaviour as a bad
    # CRC, reached by a different path in the parser.
    write("truncated", good[: len(good) // 2], {
        "message_id": 0,
        "message_name": "HEARTBEAT",
        "sys_id": SYS_ID,
        "comp_id": COMP_ID,
        "seq": SEQ,
        "fields": {},
        "note": (
            "Frame cut mid-payload, with no following bytes. Unlike bad_crc, "
            "gomavlib's parser is stream-oriented and "
            "simply WAITS for the remaining bytes, so no EventParseError is emitted and "
            "no counter moves. Verified by execution. The observable contract is only "
            "'no frame surfaces, no panic'. A truncated frame followed by more traffic "
            "is the damaging case: the parser consumes the next frame's bytes as this "
            "one's remainder, so both are lost."
        ),
    })


def main():
    print(f"writing fixtures to {OUT}\n")
    gen_telemetry()
    gen_transactions()
    gen_send()
    gen_framing()

    count = len(list(OUT.glob("*.bin")))
    print(f"\n{count} fixtures written")


if __name__ == "__main__":
    main()
