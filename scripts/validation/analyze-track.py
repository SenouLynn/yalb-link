#!/usr/bin/env python3
"""T-052 live/replay distance and northward track checks from fresh evidence.

Usage: python3 scripts/validation/analyze-track.py RUN
Run the T-019 baseline analyzer first to download recording.json. The browser
observer captures rendered map data and text; this script uses independent raw
MAVLink positions and a spherical distance calculation, not the UI odometer.
"""
import json
import math
from pathlib import Path
import re
import sys

RADIUS = 6371008.8


def distance(a, b):
    lat1, lon1, lat2, lon2 = map(math.radians, (*a, *b))
    h = math.sin((lat2-lat1)/2)**2 + math.cos(lat1)*math.cos(lat2)*math.sin((lon2-lon1)/2)**2
    return 2*RADIUS*math.asin(min(1, math.sqrt(h)))


def flown(sample):
    match = re.search(r'\bFLOWN\s+([\d.]+)\s*(km|m)\b', sample['text'])
    if not match:
        raise ValueError('Flown readout missing')
    return float(match[1]) * (1000 if match[2] == 'km' else 1)


def coordinates(sample):
    return sample.get('track', {}).get('geometry', {}).get('coordinates', [])


def replay_seconds(sample):
    match = re.search(r'\n(\d+):(\d+) / \d+:\d+\n', sample['text'])
    if not match:
        raise ValueError('Replay clock missing')
    return int(match[1])*60 + int(match[2])


def main():
    run = Path(sys.argv[1])
    phases = [json.loads(line) for line in (run/'flight/phases.jsonl').read_text().splitlines()]
    times = {p['phase']: p['host_time'] for p in phases if p['event'] == 'start'}
    rows = [json.loads(line) for line in (run/'flight/received.jsonl').read_text().splitlines()]
    raw = [r for r in rows if r['message']['mavpackettype'] == 'GLOBAL_POSITION_INT'
           and r['system'] == 1 and r['component'] == 1]
    # The stationary settle interval avoids timing ambiguity at an endpoint.
    comparison_time = times['position-interruption'] - 1
    positions = [(r['message']['lat']/1e7, r['message']['lon']/1e7)
                 for r in raw if times['accelerate-north'] <= r['host_time'] <= comparison_time]
    raw_distance = sum(distance(a, b) for a, b in zip(positions, positions[1:]))
    live = json.loads((run/'browser/browser.json').read_text())
    replay = json.loads((run/'replay/browser.json').read_text())
    recording = json.loads((run/'recording.json').read_text())
    start = recording[0]['occurred_at_ms']/1000
    live_sample = min(live['observations'], key=lambda s: abs(s['hostTime']-comparison_time))
    replay_sample = min(replay['observations'], key=lambda s: abs(start+replay_seconds(s)-comparison_time))

    steady = [r for r in raw if r['phase'] == 'straight-north']
    low = min(r['message']['lat']/1e7 for r in steady)
    high = max(r['message']['lat']/1e7 for r in steady)
    def max_backstep(observations):
        steps = []
        for sample in observations:
            coords = coordinates(sample)
            for a, b in zip(coords, coords[1:]):
                # Restrict to the measured northward corridor and steady section,
                # excluding ground jitter, approach and the east-turn transition.
                if (low <= a[1] <= high and low <= b[1] <= high
                        and abs(a[0]+122.4194) < .00002 and abs(b[0]+122.4194) < .00002):
                    steps.append(max(0, math.radians(a[1]-b[1])*RADIUS))
        if not steps:
            raise ValueError('No observed steady north track segments')
        return max(steps)
    raw_backstep = max(max(0, math.radians((a['message']['lat']-b['message']['lat'])/1e7)*RADIUS)
                       for a, b in zip(steady, steady[1:]))
    result = {
        'raw_route_m': raw_distance,
        'live_flown_m': flown(live_sample),
        'replay_flown_m': flown(replay_sample),
        'live_comparison_skew_s': abs(live_sample['hostTime']-comparison_time),
        'replay_clock_skew_s': abs(start+replay_seconds(replay_sample)-comparison_time),
        'raw_max_north_backstep_m': raw_backstep,
        'live_max_north_backstep_m': max_backstep(live['observations']),
        'replay_max_north_backstep_m': max_backstep(replay['observations']),
        'browser_exceptions': live['errors']+replay['errors'],
    }
    result['checks'] = {
        'live_distance': abs(result['live_flown_m']-raw_distance) <= 5,
        'replay_distance': abs(result['replay_flown_m']-raw_distance) <= 5,
        'live_time_alignment': result['live_comparison_skew_s'] <= .5,
        'replay_time_alignment': result['replay_clock_skew_s'] <= 1.5,
        'live_backtracking': result['live_max_north_backstep_m'] <= max(.2, raw_backstep),
        'replay_backtracking': result['replay_max_north_backstep_m'] <= max(.2, raw_backstep),
        'browser_runtime': not result['browser_exceptions'],
    }
    (run/'track-analysis.json').write_text(json.dumps(result, indent=2))
    print(json.dumps(result, indent=2))
    if not all(result['checks'].values()):
        raise SystemExit('Track acceptance failed')


if __name__ == '__main__':
    main()
