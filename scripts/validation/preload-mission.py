"""Executed external SITL setup artifact; args: TCP port, optional item count.

Run only against the isolated Compose simulators. A count of zero clears the
test mission. No arming or mode commands are issued.
"""
import sys,time,json
from pymavlink import mavutil
port=int(sys.argv[1]); count=int(sys.argv[2]) if len(sys.argv)>2 else 6
m=mavutil.mavlink_connection('tcp:127.0.0.1:'+str(port),source_system=250)
h=m.wait_heartbeat(timeout=15)
if h is None: raise RuntimeError('heartbeat timeout')
system=h.get_srcSystem(); comp=h.get_srcComponent()
m.mav.mission_count_send(system,comp,count)
end=time.monotonic()+20
sent=[]
while time.monotonic()<end:
 msg=m.recv_match(type=['MISSION_REQUEST','MISSION_REQUEST_INT','MISSION_ACK'],blocking=True,timeout=1)
 if msg is None:continue
 if msg.get_type()=='MISSION_ACK':
  if msg.type!=0:raise RuntimeError(str(msg))
  break
 seq=msg.seq
 lat=37.7749+seq*0.0002;lon=-122.4194+(seq%2)*0.0003
 # seq 0 is ArduPilot's home slot; five commanded waypoints follow it.
 m.mav.mission_item_int_send(system,comp,seq,6,16,0,1,0,0,0,0,round(lat*1e7),round(lon*1e7),20+seq)
 sent.append(dict(seq=seq,lat=lat,lon=lon,alt=20+seq))
else:raise RuntimeError('upload timeout')
print(json.dumps(dict(system=system,uploaded=sent)))
m.close()
