#!/usr/bin/env python3
"""Bounded T-019 motion experiment, exclusively for the local Compose Copter.

Requires pymavlink==2.4.49. Recreate the SITL container before each run.
Received frames and decoded observations have host receipt timestamps; these
are a separate TCP stream from the backend's UDP stream, not wire-identical.
"""
import argparse
import json
import math
from pathlib import Path
import time
import select
import socket
import threading

HOME = (37.7749, -122.4194)
RADIUS = 6371008.8


def offset(north, east):
    return (HOME[0] + math.degrees(north / RADIUS),
            HOME[1] + math.degrees(east / (RADIUS * math.cos(math.radians(HOME[0])))))


def distance(a, b):
    lat1, lon1, lat2, lon2 = map(math.radians, (*a, *b))
    h = math.sin((lat2-lat1)/2)**2 + math.cos(lat1)*math.cos(lat2)*math.sin((lon2-lon1)/2)**2
    return 2 * RADIUS * math.asin(min(1, math.sqrt(h)))


def bounded_wait(receive, predicate, timeout, label, clock=time.monotonic):
    end = clock() + timeout
    while clock() < end:
        message = receive()
        if message is not None and predicate(message):
            return message
    raise TimeoutError(label)


class Relay:
    """Forward original datagrams; inject rate commands on the same SITL link."""
    def __init__(self, out):
        self.capture = (out / 'udp-received.jsonl').open('w')
        self.vehicle = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.vehicle.bind(('0.0.0.0', 14560))
        self.backend = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.backend.connect(('127.0.0.1', 14550))
        self.peer = None
        self.stopped = threading.Event()
        self.error = None
        self.thread = threading.Thread(target=self.run, daemon=True)
        self.thread.start()

    def run(self):
        try:
            while not self.stopped.is_set():
                ready, _, _ = select.select([self.vehicle, self.backend], [], [], 0.2)
                for source in ready:
                    data, address = source.recvfrom(65535)
                    if source is self.vehicle:
                        self.capture.write(json.dumps(dict(host_time=time.time(), data=data.hex())) + '\n')
                        self.capture.flush()
                        self.peer = address
                        self.backend.send(data)
                    elif self.peer:
                        self.vehicle.sendto(data, self.peer)
        except Exception as error:
            self.error = error

    def send(self, data):
        if self.error:
            raise RuntimeError('UDP relay failed') from self.error
        if self.peer is None:
            raise RuntimeError('SITL must stream to host.docker.internal:14560')
        self.vehicle.sendto(data, self.peer)

    def close(self):
        self.stopped.set()
        self.thread.join(timeout=2)
        self.vehicle.close()
        self.backend.close()
        self.capture.close()


class Experiment:
    def __init__(self, connection, out, mav, relay):
        self.link, self.out, self.mav = connection, out, mav
        self.relay = relay
        self.phase = 'identify'
        self.system, self.component = 1, 1
        self.latest = {}
        self.raw = (out / 'received.mavlink').open('wb')
        self.decoded = (out / 'received.jsonl').open('w')
        self.commands = (out / 'commands.jsonl').open('w')
        self.events = (out / 'phases.jsonl').open('w')
        self.last_heartbeat = 0

    def record(self, file, **data):
        file.write(json.dumps(dict(host_time=time.time(), monotonic=time.monotonic(), phase=self.phase, **data)) + '\n')
        file.flush()

    def receive(self):
        if self.relay.error:
            raise RuntimeError("UDP relay failed") from self.relay.error
        if time.monotonic() - self.last_heartbeat >= 1:
            self.link.mav.heartbeat_send(self.mav.MAV_TYPE_GCS, self.mav.MAV_AUTOPILOT_INVALID, 0, 0, 0)
            self.last_heartbeat = time.monotonic()
        msg = self.link.recv_match(blocking=True, timeout=0.2)
        if msg is not None:
            self.raw.write(msg.get_msgbuf())
            self.record(self.decoded, system=msg.get_srcSystem(), component=msg.get_srcComponent(), message=msg.to_dict())
            if (msg.get_srcSystem(), msg.get_srcComponent()) != (self.system, self.component):
                return None
            self.latest[msg.get_type()] = msg
        return msg

    def wait(self, predicate, timeout, label):
        return bounded_wait(self.receive, predicate, timeout, label)

    def hold(self, seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            self.receive()

    def phase_start(self, phase, **data):
        self.phase = phase
        self.record(self.events, event='start', **data)
        print(phase, flush=True)

    def command(self, command, *params):
        params = list(params) + [0] * (7-len(params))
        self.record(self.commands, command=command, params=params)
        self.link.mav.command_long_send(self.system, self.component, command, 0, *params)
        ack = self.wait(lambda m: m.get_type() == 'COMMAND_ACK' and m.command == command, 10, f'ACK {command}')
        if ack.result != self.mav.MAV_RESULT_ACCEPTED:
            raise RuntimeError(f'command {command} rejected: {ack.result}')

    def mode(self, mode):
        self.command(self.mav.MAV_CMD_DO_SET_MODE, self.mav.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED, mode)
        self.wait(lambda m: m.get_type() == 'HEARTBEAT' and m.custom_mode == mode, 10, f'mode {mode}')

    def target(self, north, east, baseline=False):
        lat, lon = offset(north, east)
        self.record(self.commands, target=dict(lat=lat, lon=lon, relative_alt_m=20))
        self.link.mav.set_position_target_global_int_send(0, 1, 1,
            self.mav.MAV_FRAME_GLOBAL_RELATIVE_ALT_INT, 3576,
            round(lat*1e7), round(lon*1e7), 20, 0, 0, 0, 0, 0, 0, 0, 0)
        # Enter position control first: initialization resets the speed limit.
        self.command(self.mav.MAV_CMD_DO_CHANGE_SPEED, 1, 5, -1)
        if baseline:
            self.wait(lambda m: m.get_type() == 'GLOBAL_POSITION_INT'
                and math.hypot(m.vx, m.vy)/100 >= 4.5, 30, 'straight baseline speed')
            self.phase_start('straight-north')
            self.wait(lambda m: m.get_type() == 'GLOBAL_POSITION_INT'
                and distance((m.lat/1e7, m.lon/1e7), (lat, lon)) <= 35,
                90, 'straight baseline end')
            self.phase_start('approach-north')
        msg = self.wait(lambda m: m.get_type() == 'GLOBAL_POSITION_INT'
            and distance((m.lat/1e7, m.lon/1e7), (lat, lon)) <= 3
            and abs(m.relative_alt/1000 - 20) <= 2, 90, f'endpoint {north},{east}')
        self.record(self.events, event='endpoint', horizontal_error_m=distance((msg.lat/1e7,msg.lon/1e7),(lat,lon)), relative_alt_m=msg.relative_alt/1000)

    def run(self):
        heartbeat = self.wait(lambda m: m.get_type() == 'HEARTBEAT', 20, 'heartbeat')
        if heartbeat.autopilot != self.mav.MAV_AUTOPILOT_ARDUPILOTMEGA or heartbeat.type != self.mav.MAV_TYPE_QUADROTOR:
            raise RuntimeError('Expected ArduPilot quadrotor')
        if heartbeat.base_mode & self.mav.MAV_MODE_FLAG_SAFETY_ARMED:
            raise RuntimeError('Expected disarmed fresh simulator')
        self.command(self.mav.MAV_CMD_REQUEST_MESSAGE, self.mav.MAVLINK_MSG_ID_AUTOPILOT_VERSION)
        version = self.latest.get('AUTOPILOT_VERSION') or self.wait(lambda m: m.get_type() == 'AUTOPILOT_VERSION', 10, 'firmware')
        (self.out / 'firmware.json').write_text(json.dumps(version.to_dict(), indent=2))
        if version.flight_sw_version >> 8 != (4 << 16 | 7 << 8):
            raise RuntimeError('Expected Copter 4.7.0')
        for message, rate in [(33, 5), (30, 10), (74, 5), (24, 1), (193, 1)]:
            self.command(self.mav.MAV_CMD_SET_MESSAGE_INTERVAL, message, 1e6/rate)
        position = self.wait(lambda m: m.get_type() == 'GLOBAL_POSITION_INT' and m.lat != 0, 45, 'position')
        if distance((position.lat/1e7, position.lon/1e7), HOME) > 3 or abs(position.alt/1000-10) > 3:
            raise RuntimeError('Simulator home does not match T-019')
        self.wait(lambda m: m.get_type() == 'GPS_RAW_INT' and m.fix_type >= 3, 45, 'GPS fix')
        self.wait(lambda m: m.get_type() == 'EKF_STATUS_REPORT' and m.flags & 1 and m.flags & 16, 45, 'EKF attitude and absolute position')
        if self.relay.peer is None:
            raise RuntimeError('Expected simulator telemetry through the configured UDP relay')
        self.phase_start('stationary-ground')
        self.hold(10)
        self.mode(4)
        self.command(self.mav.MAV_CMD_COMPONENT_ARM_DISARM, 1)
        self.wait(lambda m: m.get_type() == 'HEARTBEAT' and m.base_mode & self.mav.MAV_MODE_FLAG_SAFETY_ARMED, 10, 'armed')
        self.phase_start('takeoff')
        self.command(self.mav.MAV_CMD_NAV_TAKEOFF, 0, 0, 0, 0, 0, 0, 20)
        self.wait(lambda m: m.get_type() == 'GLOBAL_POSITION_INT' and abs(m.relative_alt/1000-20) <= 2, 90, 'takeoff altitude')
        self.phase_start('stationary-air')
        self.hold(10)
        self.phase_start('accelerate-north')
        self.target(100, 0, baseline=True)
        self.phase_start('turn-east')
        self.target(100, 100)
        self.phase_start('settle')
        self.hold(10)
        self.phase_start('position-interruption')
        self.backend_rate(-1)
        self.hold(8)
        self.phase_start('position-recovery')
        self.backend_rate(200000)
        self.hold(10)
        self.phase_start('land')
        self.mode(9)
        self.wait(lambda m: m.get_type() == 'HEARTBEAT' and not m.base_mode & self.mav.MAV_MODE_FLAG_SAFETY_ARMED, 90, 'land and disarm')
        self.phase_start('complete')
        # Keep UDP forwarding through several heartbeats after TCP disarm.
        self.hold(3)

    def backend_rate(self, interval):
        self.record(self.commands, channel='backend-udp', command=511, message_id=33, interval_us=interval)
        packet = self.mav.MAVLink_command_long_message(1, 1, 511, 0, 33, interval, 0, 0, 0, 0, 0)
        self.relay.send(packet.pack(self.link.mav))

    def close(self):
        self.relay.close()
        for file in [self.raw, self.decoded, self.commands, self.events]:
            file.close()
        self.link.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    from pymavlink import mavutil
    relay = Relay(args.output)
    try:
        link = mavutil.mavlink_connection('tcp:127.0.0.1:5760', source_system=250, retries=2)
    except BaseException:
        relay.close()
        raise
    experiment = Experiment(link, args.output, mavutil.mavlink, relay)
    try:
        experiment.run()
    except BaseException as error:
        experiment.record(experiment.events, event='failure', error=str(error))
        # No force-disarm in air. Best-effort restore stream and request LAND;
        # original failure remains visible even if the connection is gone.
        heartbeat = experiment.latest.get('HEARTBEAT')
        if heartbeat and heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED:
            try:
                experiment.backend_rate(200000)
                experiment.mode(9)
            except Exception as cleanup:
                experiment.record(experiment.events, event='cleanup-failure', error=str(cleanup))
        raise
    finally:
        experiment.close()


if __name__ == '__main__':
    main()
