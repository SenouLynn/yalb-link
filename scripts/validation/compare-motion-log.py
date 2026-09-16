#!/usr/bin/env python3
"""Compare autopilot DataFlash POS/ATT with captured MAVLink by boot time.
Usage: PYTHON scripts/validation/compare-motion-log.py RUN
Requires pymavlink. Reports nearest-sample differences without inventing
wire identity: DataFlash and MAVLink publish at different instants.
"""
import bisect
import json
import math
from pathlib import Path
import sys
from pymavlink import mavutil
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
from sitl_motion import distance

run=Path(sys.argv[1])
rows=[json.loads(line) for line in (run/'flight/received.jsonl').read_text().splitlines()]
log=mavutil.mavlink_connection(str(run/'autopilot.BIN'))
samples={'POS':[],'ATT':[]}
while True:
    message=log.recv_match(type=list(samples))
    if message is None:break
    samples[message.get_type()].append(message.to_dict())
log.close()
result={}
for kind,telemetry in [('POS','GLOBAL_POSITION_INT'),('ATT','ATTITUDE')]:
    times=[m['TimeUS']/1000 for m in samples[kind]]
    diffs=[]
    for row in rows:
        m=row['message']
        if m['mavpackettype']!=telemetry:continue
        t=m['time_boot_ms'];i=bisect.bisect_left(times,t)
        if i==0 or i==len(times):continue
        j=min([i-1,i],key=lambda j:abs(times[j]-t));delta=abs(times[j]-t)
        if delta>50:continue
        source=samples[kind][j]
        d=dict(boot_alignment_ms=delta)
        if kind=='POS':
            d.update(horizontal_m=distance((source['Lat'],source['Lng']),(m['lat']/1e7,m['lon']/1e7)),msl_m=abs(source['Alt']-m['alt']/1000),relative_m=abs(source['RelHomeAlt']-m['relative_alt']/1000))
        else:
            for field in ['roll','pitch','yaw']:
                d[field+'_deg']=abs((source[field.title()]-math.degrees(m[field])+180)%360-180)
        diffs.append(d)
    result[kind]=dict(samples=len(diffs),max={k:max(d[k] for d in diffs) for k in diffs[0]} if diffs else {})
(run/'autopilot-comparison.json').write_text(json.dumps(result,indent=2))
print(json.dumps(result,indent=2))
