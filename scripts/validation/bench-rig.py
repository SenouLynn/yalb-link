#!/usr/bin/env python3
"""T-053: hold a synthetic ArduPilot-like aircraft on a PTY so the real browser UI
can be driven against the real backend. Standard library only.

  bench-rig.py --binary /abs/gcs [--profiles DIR]

Starts: backend (serial-only, commands disabled) on a fresh PTY, `vite dev`
proxied to it, headless Chrome with CDP. Prints the ports, then serves a tiny
control API on 127.0.0.1:CONTROL so a scenario can steer the aircraft:

  POST /pose      {"roll": deg, "pitch": deg}   attitude to transmit
  POST /attitude  {"on": bool}                  stop/start ATTITUDE only
  POST /heartbeat {"on": bool}                  stop/start HEARTBEAT only
  POST /backend   {"action": "stop"|"start"}    restart the backend (same PTY)
  GET  /outbound                                decoded outbound MAVLink counts
"""
import argparse
import collections
import json
import math
import os
import pty
import select
import signal
import socket
import struct
import subprocess
import sys
import tempfile
import threading
import time
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

spec = importlib.util.spec_from_file_location('acq', os.path.join(os.path.dirname(__file__), 'serial-acquisition.py'))
acq = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acq)

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'


def free_port():
    s = socket.socket()
    s.bind(('127.0.0.1', 0))
    port = s.getsockname()[1]
    s.close()
    return port


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--profiles')
    args = parser.parse_args()

    tmp = tempfile.mkdtemp(prefix='t053-')
    master, slave = pty.openpty()
    path = os.ttyname(slave)
    os.close(slave)
    backend_port, ui_port, cdp_port, control_port = (free_port() for _ in range(4))
    profiles = args.profiles or tmp + '/profiles.json'

    state = dict(roll=0.0, pitch=0.0, attitude=True, heartbeat=True)
    outbound = collections.Counter()
    stop = threading.Event()
    procs = {}

    def start_backend():
        env = dict(os.environ, GCS_MAVLINK_UDP_BIND='', GCS_COMMANDS_ENABLED='false',
                   GCS_SERIAL_PATHS=path, GCS_HTTP_ADDR='127.0.0.1:%d' % backend_port,
                   GCS_RECORDING_ENABLED='true', GCS_RECORDING_DB_PATH=tmp + '/recordings.db',
                   GCS_CONNECTION_PROFILES_PATH=profiles)
        procs['backend'] = subprocess.Popen([os.path.abspath(args.binary)], env=env,
                                            stdout=open(tmp + '/backend.log', 'a'), stderr=subprocess.STDOUT)

    def stop_backend():
        p = procs.get('backend')
        if p and p.poll() is None:
            p.send_signal(signal.SIGTERM)
            p.wait(timeout=5)

    def aircraft():
        boot = time.monotonic()
        next_hb = next_att = 0.0
        while not stop.is_set():
            now = time.monotonic()
            ms = int((now - boot) * 1000)
            try:
                if state['heartbeat'] and now >= next_hb:
                    os.write(master, acq.frame(0, struct.pack('<IBBBBB', 0, 2, 3, 0, 4, 3)))
                    next_hb = now + 1.0
                if state['attitude'] and now >= next_att:
                    r, p = math.radians(state['roll']), math.radians(state['pitch'])
                    os.write(master, acq.frame(30, struct.pack('<I6f', ms, r, p, 0, 0, 0, 0)))
                    next_att = now + 0.1
            except OSError:
                pass
            time.sleep(0.01)

    def reader():
        decoder = acq.Decoder()
        while not stop.is_set():
            ready, _, _ = select.select([master], [], [], 0.05)
            if master in ready:
                try:
                    data = os.read(master, 4096)
                except OSError:
                    time.sleep(0.01)
                    continue
                for mid, payload, sysid in decoder.feed(data):
                    outbound[mid] += 1

    class Control(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def reply(self, body):
            data = json.dumps(body).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(data)

        def do_GET(self):
            self.reply({str(k): v for k, v in outbound.items()})

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers.get('Content-Length', 0))) or b'{}')
            route = self.path
            if route == '/pose':
                state['roll'], state['pitch'] = body['roll'], body['pitch']
            elif route == '/attitude':
                state['attitude'] = body['on']
            elif route == '/heartbeat':
                state['heartbeat'] = body['on']
            elif route == '/backend':
                (stop_backend if body['action'] == 'stop' else start_backend)()
            self.reply(dict(state))

    start_backend()
    threading.Thread(target=aircraft, daemon=True).start()
    threading.Thread(target=reader, daemon=True).start()
    threading.Thread(target=ThreadingHTTPServer(('127.0.0.1', control_port), Control).serve_forever, daemon=True).start()

    procs['vite'] = subprocess.Popen(
        ['pnpm', 'exec', 'vite', '--port', str(ui_port), '--strictPort', '--host', '127.0.0.1'],
        cwd=ROOT + '/frontend', env=dict(os.environ, GCS_BACKEND_URL='http://127.0.0.1:%d' % backend_port),
        stdout=open(tmp + '/vite.log', 'w'), stderr=subprocess.STDOUT)
    procs['chrome'] = subprocess.Popen(
        [CHROME, '--headless=new', '--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader',
         '--remote-debugging-port=%d' % cdp_port, '--user-data-dir=' + tmp + '/chrome',
         '--window-size=1280,900', 'about:blank'],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    print(json.dumps(dict(tmp=tmp, pty=path, backend='http://127.0.0.1:%d' % backend_port,
                          ui='http://127.0.0.1:%d' % ui_port, cdp=cdp_port,
                          control='http://127.0.0.1:%d' % control_port, profiles=profiles)), flush=True)
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass
    finally:
        stop.set()
        for p in procs.values():
            if p.poll() is None:
                p.terminate()


if __name__ == '__main__':
    main()
