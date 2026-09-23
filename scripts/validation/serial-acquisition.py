#!/usr/bin/env python3
"""T-046: real backend, real native serial adapter, controlled PTY peer.

Standard-library only. Independent MAVLink v2 framing/CRC and outbound allowlist.
Run with --binary /absolute/path/to/gcs. Optional --sitl HOST:PORT forwards a
Plane 4.6.3 TCP stream through the PTY rather than generating synthetic frames.
"""
import argparse
import collections
import errno
import json
import os
import platform
import pty
import select
import signal
import socket
import sqlite3
import struct
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request

EXTRA = {0: 50, 20: 214, 22: 220, 30: 39, 33: 104, 43: 132, 44: 221,
         47: 153, 51: 196, 76: 152, 148: 178}


def crc(data):
    value = 0xffff
    for byte in data:
        tmp = byte ^ (value & 0xff)
        tmp ^= (tmp << 4) & 0xff
        value = (value >> 8) ^ (tmp << 8) ^ (tmp << 3) ^ (tmp >> 4)
    return value


sequence = 0


def frame(mid, payload, sysid=41, compid=1):
    global sequence
    sequence = (sequence + 1) % 256
    payload = payload.rstrip(b'\0') or b'\0'
    header = bytes([len(payload), 0, 0, sequence, sysid, compid]) + mid.to_bytes(3, 'little')
    return b'\xfd' + header + payload + struct.pack('<H', crc(header + payload + bytes([EXTRA[mid]])))


class Decoder:
    def __init__(self):
        self.buf = bytearray()

    def feed(self, data):
        self.buf.extend(data)
        out = []
        while self.buf:
            if self.buf[0] not in (0xfd, 0xfe):
                del self.buf[0]
                continue
            v2 = self.buf[0] == 0xfd
            header = 10 if v2 else 6
            if len(self.buf) < header:
                break
            n = header + self.buf[1] + 2 + (13 if v2 and self.buf[2] & 1 else 0)
            if len(self.buf) < n:
                break
            packet = bytes(self.buf[:n])
            del self.buf[:n]
            mid = int.from_bytes(packet[7:10], 'little') if v2 else packet[5]
            payload = packet[header:header+packet[1]]
            if mid in EXTRA:
                want = crc(packet[1:header] + payload + bytes([EXTRA[mid]]))
                assert struct.unpack_from('<H', packet, header+len(payload))[0] == want, ('bad CRC', mid)
            out.append((mid, payload.ljust(255, b'\0'), packet[5] if v2 else packet[3]))
        return out


def wait(predicate, description, seconds=8):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        value = predicate()
        if value:
            return value
        time.sleep(.025)
    raise AssertionError('deadline: ' + description)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--sitl')
    parser.add_argument('--output')
    parser.add_argument('--check-silence', action='store_true', help='exercise the real 60-second silence threshold')
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='t046-') as tmp:
        master, slave = pty.openpty()
        path = os.ttyname(slave)
        # Closing the initial slave proves subsequent opens are owned by the backend.
        os.close(slave)
        listener = socket.socket()
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
        listener.close()
        base = 'http://127.0.0.1:' + str(port)
        env = dict(os.environ, GCS_MAVLINK_UDP_BIND='', GCS_COMMANDS_ENABLED='false',
                   GCS_SERIAL_PATHS=path, GCS_HTTP_ADDR='127.0.0.1:'+str(port),
                   GCS_RECORDING_ENABLED='true', GCS_RECORDING_DB_PATH=tmp+'/recordings.db')
        log = open(tmp+'/backend.log', 'w+')
        process = subprocess.Popen([os.path.abspath(args.binary)], env=env, stdout=log, stderr=log)
        errors, outbound, events = [], [], []
        stop = threading.Event()
        mission_requested = threading.Event()
        released = threading.Event()
        sse_responses = []
        tcp = None
        inbound = []

        def api(route, body=None, want=200):
            request = urllib.request.Request(base+route, data=None if body is None else json.dumps(body).encode(),
                                             headers={'Content-Type': 'application/json'})
            try:
                with urllib.request.urlopen(request, timeout=5) as response:
                    data = response.read()
                    assert response.status == want, (response.status, data)
                    return json.loads(data) if data else None
            except urllib.error.HTTPError as exc:
                assert exc.code == want, (exc.code, exc.read())
                return None

        def healthy():
            try:
                with urllib.request.urlopen(base+'/healthz', timeout=.2) as response:
                    return response.status == 200
            except (OSError, urllib.error.URLError):
                assert process.poll() is None, 'backend exited during startup'
                return False

        def subscribe(target):
            # Longer than the backend's 15-second SSE keepalive during silence.
            response = urllib.request.urlopen(base+'/api/events', timeout=30)
            sse_responses.append(response)
            def read():
                name = ''
                try:
                    for line in response:
                        text = line.decode().strip()
                        if text.startswith('event: '):
                            name = text[7:]
                        elif text.startswith('data: '):
                            target.append((name, json.loads(text[6:])))
                except (OSError, ValueError):
                    if not stop.is_set():
                        errors.append('SSE reader failed')
            threading.Thread(target=read, daemon=True).start()

        def send(mid, payload, sysid=41):
            data = frame(mid, payload, sysid)
            for offset in range(0, len(data), 3):
                os.write(master, data[offset:offset+3])
                time.sleep(.001)

        def peer():
            decoder, sitl_decoder = Decoder(), Decoder()
            try:
                while not stop.is_set():
                    sources = [master] + ([tcp] if tcp else [])
                    ready, _, _ = select.select(sources, [], [], .05)
                    if master in ready:
                        try:
                            data = os.read(master, 4096)
                        except OSError as exc:
                            if exc.errno == errno.EIO:
                                time.sleep(.01)
                                continue
                            raise
                        for mid, payload, sysid in decoder.feed(data):
                            assert sysid == 255, ('unexpected GCS identity', sysid)
                            assert mid in (0, 76, 43, 51, 47), ('prohibited message', mid)
                            if mid == 76:
                                assert struct.unpack_from('<H', payload, 28)[0] == 511, 'prohibited command'
                                assert payload[30] in ((4,) if tcp else (41, 42)), 'wrong target system'
                                assert payload[31] == 1, 'wrong target component'
                            if mid in (43, 51, 47):
                                assert mission_requested.is_set(), 'unsolicited mission traffic'
                                offset = 2 if mid == 51 else 0
                                assert payload[offset] == (4 if tcp else 41), 'wrong mission target system'
                                assert payload[offset+1] == 1, 'wrong mission target component'
                                if mid == 43 and not tcp:
                                    os.write(master, frame(44, struct.pack('<HBBB', 0, 255, 190, 0)))
                            outbound.append(mid)
                        if tcp:
                            tcp.sendall(data)
                    if tcp and tcp in ready:
                        data = tcp.recv(8192)
                        if not data:
                            raise AssertionError('SITL closed')
                        inbound.extend(sitl_decoder.feed(data))
                        if not released.is_set():
                            os.write(master, data)
            except Exception as exc:
                if not stop.is_set():
                    errors.append(repr(exc))

        try:
            wait(healthy, 'backend healthy')
            inventory = api('/api/connections/devices')
            device = next(d for d in inventory if d['path'] == path)
            assert 'description' not in device and 'serial_number' not in device, 'fabricated PTY identity'
            assert api('/api/connections') == []
            api('/api/commands/arm', {'system_id': 41, 'arm': True}, want=404)
            api('/api/connections/connect', {'device_id': 'missing', 'settings': {'baud_rate': 57600}}, want=404)
            subscribe(events)
            recording = api('/api/recordings/start', {'name': 'T-046 serial'}, want=201)
            body = {'device_id': device['id'], 'settings': {'baud_rate': 57600}}
            opened = api('/api/connections/connect', body)
            assert opened['state'] == 'OPEN_AWAITING_TRAFFIC', opened
            assert api('/api/connections/connect', body)['opened_at_ms'] == opened['opened_at_ms']
            threading.Thread(target=peer, daemon=True).start()
            time.sleep(1.2)
            assert api('/api/connections')[0]['state'] == 'OPEN_AWAITING_TRAFFIC'
            os.write(master, b'not MAVLink\x00\xff')
            time.sleep(.1)
            assert api('/api/connections')[0]['state'] == 'OPEN_AWAITING_TRAFFIC'
            if args.sitl:
                host, tcpport = args.sitl.rsplit(':', 1)
                tcp = socket.create_connection((host, int(tcpport)), timeout=3)
                tcp.settimeout(None)
                wait(lambda: any(name == 'fleet' and e.get('vehicleId', {}).get('systemId') == 4 for name, e in events), 'QuadPlane fleet', 20)
                # Harness-originated read requests verify the same T-027 profile.
                tcp.sendall(frame(76, struct.pack('<7fHBBB', 148,0,0,0,0,0,0,512,4,1,0), 254, 190))
                for name in ('Q_ENABLE', 'Q_FRAME_CLASS'):
                    tcp.sendall(frame(20, struct.pack('<hBB16s', -1,4,1,name.encode()),254,190))
                wait(lambda: any(mid == 148 for mid, _, _ in inbound), 'AUTOPILOT_VERSION', 10)
                version = next(struct.unpack_from('<I', p, 16)[0] for mid,p,_ in inbound if mid == 148)
                assert version >> 8 == 0x040603, hex(version)
                def parameters():
                    return {p[8:24].split(b'\0')[0].decode():struct.unpack_from('<f',p)[0] for mid,p,_ in inbound if mid == 22}
                wait(lambda: parameters().get('Q_ENABLE') == 1 and parameters().get('Q_FRAME_CLASS') == 7, 'QuadPlane parameters')
                wait(lambda: any(name == 'telemetry' and 'attitude' in e for name,e in events), 'SITL attitude')
                wait(lambda: any(name == 'telemetry' and 'globalPosition' in e for name,e in events), 'SITL position')
            else:
                send(30, struct.pack('<I6f', 100, .25,0,0,0,0,0))
                wait(lambda: api('/api/connections')[0]['state'] == 'REPORTING', 'valid frame without heartbeat')
                assert not any(name == 'fleet' for name,_ in events), 'attitude invented vehicle liveness'
                if args.check_silence:
                    wait(lambda: api('/api/connections')[0]['state'] == 'INTERRUPTED', '60-second serial silence', 63)
                    assert api('/api/connections')[0]['vehicle_keys'] == [], 'stale route attribution'
                    assert process.poll() is None, 'silence stopped backend'
                for sysid in (41,42):
                    send(0, struct.pack('<IBBBBB', 0,2,3,0,4,3), sysid)
                    send(30, struct.pack('<I6f', 200,.25,0,0,0,0,0), sysid)
                wait(lambda: {41,42} <= {e.get('vehicleId',{}).get('systemId') for name,e in events if name == 'fleet'}, 'two fleet identities')
                wait(lambda: {41,42} <= {e.get('vehicleId',{}).get('systemId') for name,e in events if name == 'telemetry'}, 'two telemetry identities')
            late = []
            subscribe(late)
            wait(lambda: any(name == 'acquisition' and e['state'] == 'REPORTING' for name,e in late), 'late acquisition bootstrap')
            mission_requested.set()
            api('/api/vehicles/'+('4' if tcp else '41')+'/1/mission')
            wait(lambda: 43 in outbound, 'addressed mission read')
            api('/api/recordings/stop', {})
            with sqlite3.connect(tmp+'/recordings.db') as db:
                # Schema-independent inspection of event rows is reported below.
                tables = [r[0] for r in db.execute("select name from sqlite_master where type='table'")]
                assert 'recording_events' in tables, tables
                recorded = db.execute('select count(*) from recording_events').fetchone()[0]
                assert recorded > 0, 'serial telemetry was not recorded'
            released.set()
            api('/api/connections/disconnect', {'id':device['id']})
            assert api('/api/connections')[0]['state'] == 'RELEASED'
            time.sleep(.2)
            count = len(outbound)
            time.sleep(1.2)
            assert len(outbound) == count, 'outbound traffic after disconnect'
            # A second native OS opener proves release, without changing port flags.
            second = os.open(path, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
            os.close(second)
            if not tcp:
                reopened = api('/api/connections/connect', body)
                assert reopened['opened_at_ms'] > opened['opened_at_ms']
                send(0, struct.pack('<IBBBBB',0,2,3,0,4,3))
                wait(lambda: api('/api/connections')[0]['state'] == 'REPORTING', 'reopen telemetry')
                # Master removal is a PTY-specific device-loss fault, not USB evidence.
                stop.set()
                time.sleep(.1)
                os.close(master)
                master = None
                wait(lambda: api('/api/connections')[0]['state'] in ('DEVICE_LOST','TRANSPORT_FAILED'), 'PTY removal')
            assert not errors, errors
            stop.set()
            before = time.monotonic()
            process.send_signal(signal.SIGTERM)
            assert process.wait(timeout=5) == 0, 'unclean shutdown'
            result = dict(platform=platform.platform(), architecture=platform.machine(), port=path,
                          inventory=device, outbound=dict(collections.Counter(outbound)),
                          recording_events=recorded, mode='quadplane' if tcp else 'synthetic',
                          shutdown_seconds=round(time.monotonic()-before,3), silence_threshold_checked=args.check_silence, result='PASS')
            if args.sitl:
                result.update(firmware=hex(version), parameters=parameters())
            print(json.dumps(result, indent=2))
            if args.output:
                with open(args.output,'w') as output:
                    json.dump(result,output,indent=2)
        finally:
            stop.set()
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            if process.returncode != 0 or errors:
                log.seek(0)
                print(log.read())
            if master is not None:
                os.close(master)
            if tcp:
                tcp.close()
            log.close()


if __name__ == '__main__':
    main()
