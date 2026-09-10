import json,time,urllib.request
from pymavlink import mavutil
out=[]
for port in [5760,5761]:
 m=mavutil.mavlink_connection('tcp:127.0.0.1:'+str(port),source_system=250)
 h=m.wait_heartbeat(timeout=10)
 assert h is not None
 system=h.get_srcSystem();comp=h.get_srcComponent()
 m.mav.mission_request_list_send(system,comp)
 count=m.recv_match(type='MISSION_COUNT',blocking=True,timeout=5)
 assert count is not None
 observed=[]
 for seq in range(count.count):
  m.mav.mission_request_int_send(system,comp,seq)
  item=m.recv_match(type='MISSION_ITEM_INT',blocking=True,timeout=5)
  assert item is not None and item.seq==seq
  observed.append(item.to_dict())
 m.mav.mission_ack_send(system,comp,0)
 backend=json.load(urllib.request.urlopen('http://localhost:8080/api/vehicles/%d/%d/mission'%(system,comp),timeout=10))
 assert len(backend.get('items',[]))==count.count
 for wire,app in zip(observed,backend.get('items',[])):
  assert app.get('seq',0)==wire['seq']
  assert app['command']=='MAV_CMD_NAV_WAYPOINT'
  assert abs(app.get('x',0)-wire['x']/1e7)<1e-7
  assert abs(app.get('y',0)-wire['y']/1e7)<1e-7
  assert abs(app.get('z',0)-wire['z'])<0.001
  assert app.get('autocontinue',False)==bool(wire['autocontinue'])
  for i in range(1,5):assert app.get('param'+str(i),0)==wire['param'+str(i)]
 out.append(dict(system=system,wire=observed,backend=backend,checked_at=time.time()))
 m.close()
print(json.dumps(out,indent=2))
