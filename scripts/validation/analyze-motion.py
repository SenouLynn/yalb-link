#!/usr/bin/env python3
"""Compare T-019 rendered predictions with independent received MAVLink.
Usage: python3 scripts/validation/analyze-motion.py RUN RECORDING_ID
Requires pymavlink==2.4.49. Downloads local recording pages; writes analysis.json.
"""
import bisect
import json
import math
from pathlib import Path
import sys
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from sitl_motion import distance


def percentile(values, fraction=.95):
    return sorted(values)[max(0, math.ceil(len(values)*fraction)-1)] if values else None


def coordinates(source):
    if not source:
        return []
    if source.get('type') == 'FeatureCollection':
        return next((f['geometry']['coordinates'] for f in source['features'] if f['geometry']['type']=='LineString'), [])
    return source.get('geometry', {}).get('coordinates', [])


def main():
    run, recording = Path(sys.argv[1]), int(sys.argv[2])
    rows = [json.loads(line) for line in (run/'flight/received.jsonl').read_text().splitlines()]
    raw = [r for r in rows if r['message']['mavpackettype']=='GLOBAL_POSITION_INT']
    phases = [json.loads(line) for line in (run/'flight/phases.jsonl').read_text().splitlines()]
    times = {r['phase']:r['host_time'] for r in phases if r['event']=='start'}
    browser = json.loads((run/'browser/browser.json').read_text())
    events=[]; seq=1
    while seq:
        with urllib.request.urlopen(f'http://localhost:8080/api/recordings/{recording}/events?from_seq={seq}&limit=1000', timeout=10) as response:
            page=json.load(response)
        events.extend(page['events']); seq=page['next_seq']
    (run/'recording.json').write_text(json.dumps(events))
    backend = [e for e in events if 'globalPosition' in e['event']]
    from pymavlink import mavutil
    parser=mavutil.mavlink.MAVLink(None)
    udp=[]
    for line in (run/'flight/udp-received.jsonl').read_text().splitlines():
        datagram=json.loads(line)
        for m in parser.parse_buffer(bytes.fromhex(datagram['data'])) or []:
            if m.get_type()=='GLOBAL_POSITION_INT':
                udp.append(dict(host_time=datagram['host_time'],message=m.to_dict()))
    by_boot = {r['message']['time_boot_ms']:r for r in udp}
    comparisons=[]
    for event in backend:
        p=event['event']['globalPosition']; r=by_boot.get(p.get('timeBootMs'))
        if r is None: continue
        m=r['message']
        comparisons.append(dict(position_m=distance((p.get('latDeg',0),p.get('lonDeg',0)),(m['lat']/1e7,m['lon']/1e7)),
            altitude_m=max(abs(p.get('altMslM',0)-m['alt']/1000),abs(p.get('altRelativeM',0)-m['relative_alt']/1000)),
            velocity_ms=max(abs(p.get('vxMS',0)-m['vx']/100),abs(p.get('vyMS',0)-m['vy']/100)),
            receipt_skew_s=abs(event['occurred_at_ms']/1000-r['host_time'])))
    raw_times=[r['host_time'] for r in raw]
    predictions={phase:[] for phase in ['straight-north','turn-east','whole-north-leg']}
    for sample in browser['observations']:
        t=sample['hostTime']; path=coordinates(sample.get('trajectory'))
        active=[phase for phase,start,end in [('straight-north','straight-north','approach-north'),('turn-east','turn-east','settle'),('whole-north-leg','accelerate-north','turn-east')] if times[start]<=t<times[end]]
        if not active or len(path)<2:continue
        i=bisect.bisect_left(raw_times,t+5)
        if i==0 or i==len(raw):continue
        r=min(raw[i-1:i+1],key=lambda r:abs(r['host_time']-t-5))
        alignment=abs(r['host_time']-t-5)
        if alignment>.15:continue
        m=r['message']
        for phase in active:
            predictions[phase].append(dict(time=t,error_m=distance((path[-1][1],path[-1][0]),(m['lat']/1e7,m['lon']/1e7)), alignment_s=alignment))
    stop,restore=times['position-interruption'],times['position-recovery']
    last=max(e['occurred_at_ms']/1000 for e in backend if e['occurred_at_ms']/1000<stop+.2)
    first=min(e['occurred_at_ms']/1000 for e in backend if e['occurred_at_ms']/1000>restore)
    samples=browser['observations']
    transitions=samples[-1].get('transitions',[]) if samples else []
    stale=next((s['hostTime'] for s in samples if last+1<s['hostTime']<restore and not coordinates(s.get('trajectory'))),None)
    fresh=next((s['hostTime'] for s in samples if s['hostTime']>restore and coordinates(s.get('trajectory'))),None)
    if transitions:
        stale=next((s['hostTime'] for s in transitions if last+1<s['hostTime']<restore and not s['present']),None)
        fresh=next((s['hostTime'] for s in transitions if s['hostTime']>restore and s['present']),None)
    summary=dict(recording_id=recording,phases=phases,
        matched_boot_samples=len(comparisons),
        decoding_max={k:max((r[k] for r in comparisons),default=None) for k in ['position_m','altitude_m','velocity_ms','receipt_skew_s']},
        prediction={phase:dict(samples=len(values),p95_m=percentile([v['error_m'] for v in values]),max_alignment_s=max((v['alignment_s'] for v in values),default=None)) for phase,values in predictions.items()},
        interruption=dict(backend_gap_s=first-last,stale_after_last_position_s=None if stale is None else stale-last,recovery_after_restore_s=None if fresh is None else fresh-restore),
        browser_exceptions=browser['errors'],
        max_browser_sample_gap_s=max((b['hostTime']-a['hostTime'] for a,b in zip(samples,samples[1:])),default=None),
        max_speed_ms={phase:max((math.hypot(r['message']['vx'],r['message']['vy'])/100 for r in raw if r['phase']==phase),default=None) for phase in predictions})
    (run/'prediction-errors.json').write_text(json.dumps(predictions))
    checks={
        'endpoint_bounds': all(p['horizontal_error_m']<=3 and abs(p['relative_alt_m']-20)<=2 for p in phases if p['event']=='endpoint') and sum(p['event']=='endpoint' for p in phases)==2,
        'land_deadline': times['complete']-times['land']<=90,
        'decoded_positions': len(comparisons)>100 and summary['decoding_max']['position_m']<=.02,
        'decoded_altitudes': len(comparisons)>100 and summary['decoding_max']['altitude_m']<=.002,
        'decoded_velocities': len(comparisons)>100 and summary['decoding_max']['velocity_ms']<=.002,
        'straight_prediction': len(predictions['straight-north'])>=20 and summary['prediction']['straight-north']['p95_m']<=10,
        'turn_prediction': len(predictions['turn-east'])>=20 and summary['prediction']['turn-east']['p95_m']<=30,
        'stale_deadline': stale is not None and 5<=stale-last<=5.25,
        'recovery_deadline': fresh is not None and 0<=fresh-restore<=2,
        'backend_position_interrupted': first-last>=8,
        'browser_runtime': not browser['errors'],
    }
    summary['checks']=checks
    (run/'analysis.json').write_text(json.dumps(summary,indent=2))
    print(json.dumps(summary,indent=2))
    if not all(checks.values()):raise SystemExit('Baseline checks failed; inspect analysis.json')


if __name__=='__main__':main()
