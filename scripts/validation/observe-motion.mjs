#!/usr/bin/env node
/** Live/replay T-019 observer. Node 22+, Chrome, and Vite are required.
 * Usage: node scripts/validation/observe-motion.mjs OUTPUT [SECONDS] [URL]
 * Observes rendered map sources; does not replace telemetry or predictions.
 */
import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { tmpdir } from 'node:os';
const [outArg, seconds='240', url='http://127.0.0.1:3000/'] = process.argv.slice(2);
if (!outArg || !Number.isFinite(+seconds) || +seconds <= 0) throw new Error('OUTPUT and positive duration required');
const out = resolve(outArg);
await mkdir(out, {recursive:true});
const profile = await mkdtemp(resolve(tmpdir(), 'yalb-motion-browser-'));
const browser = spawn(process.env.CHROME_BIN ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  ['--headless=new','--remote-debugging-port=0',`--user-data-dir=${profile}`,'--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader','--no-first-run','--no-default-browser-check','about:blank'], {stdio:'ignore'});
const sleep = ms => new Promise(r=>setTimeout(r,ms));
let socket;
const observations=[], errors=[];
try {
  let port;
  for(let n=0;n<100;n++) {try {port=(await readFile(resolve(profile,'DevToolsActivePort'),'utf8')).split('\n')[0];break;} catch {await sleep(100);}}
  if(!port) throw new Error('Chrome startup timeout');
  const targets=await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  socket=new WebSocket(targets.find(t=>t.type==='page').webSocketDebuggerUrl);
  await new Promise((res,rej)=>{socket.onopen=res;socket.onerror=rej;});
  let serial=0; const pending=new Map();
  socket.onmessage=message=>{const d=JSON.parse(message.data);if(d.id){const p=pending.get(d.id);if(!p)return;pending.delete(d.id);clearTimeout(p.timer);d.error?p.reject(new Error(JSON.stringify(d.error))):p.resolve(d.result);}else if(d.method==='Runtime.exceptionThrown')errors.push(d);};
  const send=(method,params={})=>new Promise((resolve,reject)=>{const id=++serial;pending.set(id,{resolve,reject,timer:setTimeout(()=>{pending.delete(id);reject(new Error(`CDP timeout ${method}`));},15000)});socket.send(JSON.stringify({id,method,params}));});
  const evaluate=async expression=>{const r=await send('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw new Error(JSON.stringify(r.exceptionDetails));return r.result.value;};
  await send('Runtime.enable');await send('Page.enable');
  await send('Emulation.setDeviceMetricsOverride',{width:1600,height:1000,deviceScaleFactor:1,mobile:false});
  await send('Page.navigate',{url});
  for(let n=0;n<100;n++){if(await evaluate(`!!document.querySelector('.maplibregl-map')`))break;await sleep(150);}
  await evaluate(`(async()=>{
    const moduleURL=performance.getEntriesByType('resource').map(r=>r.name).find(n=>n.includes('/deps/maplibre-gl.js'));
    if(!moduleURL)throw Error('MapLibre module not found');
    const {default:lib}=await import(moduleURL);
    window.motionMaps=[];
    window.motionTransitions=[];
    const update=lib.GeoJSONSource.prototype.setData;
    lib.GeoJSONSource.prototype.setData=function(data,...args){
      if(this.id==='trajectory'){
        const present=(data?.geometry?.coordinates?.length??0)>1;
        if(window.motionTransitions.at(-1)?.present!==present)window.motionTransitions.push({hostTime:Date.now()/1000,present});
      }
      return update.call(this,data,...args);
    };
    const add=lib.Map.prototype.addSource;
    lib.Map.prototype.addSource=function(id,...args){if(id==='trajectory')window.motionMaps.push(this);return add.call(this,id,...args);};
  })()`);
  if(new URL(url).searchParams.get('source')!=='replay'){
    // The fleet may still be empty right after navigation (bootstrap has not
    // arrived yet), which leaves the button disabled; a disabled button's
    // click is a silent no-op, so this must retry rather than click once.
    let opened=false;
    for(let n=0;n<100;n++){
      opened=await evaluate(`(()=>{
        const open=[...document.querySelectorAll('button')].find(b=>b.textContent==='Open selected vehicle'&&!b.disabled);
        if(!open)return false;
        open.click();return true;
      })()`);
      if(opened)break;
      await sleep(150);
    }
    if(!opened)throw new Error('Open selected vehicle never became available');
  } else {
    // Replay opens directly in the vehicle workspace. Read existing map refs
    // from React fibers; this observational hook is for dev validation only.
    await evaluate(`(()=>{
      for(const node of document.querySelectorAll('.maplibregl-map')){
        const key=Object.keys(node).find(k=>k.startsWith('__reactFiber$'));
        for(let fiber=node[key];fiber;fiber=fiber.return){
          for(let hook=fiber.memoizedState;hook;hook=hook.next){
            const ref=hook.memoizedState?.current;
            if(ref?.getSource && ref.getSource('trajectory') && !window.motionMaps.includes(ref))window.motionMaps.push(ref);
          }
        }
      }
    })()`);
  }
  if(new URL(url).searchParams.get('source')==='replay'){
    for(let n=0;n<100;n++){
      if(await evaluate(`!![...document.querySelectorAll('button')].find(b=>b.textContent==='Play'&&!b.disabled)`))break;
      await sleep(150);
    }
    const speed=Number(process.env.MOTION_REPLAY_SPEED??1);
    await evaluate(`(()=>{
      const select=document.querySelector('.replay select');
      if(!select || ![...select.options].some(o=>o.value===String(${speed})))throw Error('Unsupported replay speed');
      select.value=String(${speed});select.dispatchEvent(new Event('change',{bubbles:true}));
      const play=[...document.querySelectorAll('button')].find(b=>b.textContent==='Play'&&!b.disabled);
      if(!play)throw Error('Replay not ready');play.click();
    })()`);
  }
  const end=Date.now()+Number(seconds)*1000;
  let nextCapture=0;
  while(Date.now()<end){
    const sample=await evaluate(`(()=>{const map=window.motionMaps.find(m=>m.getSource('trajectory'));return {hostTime:Date.now()/1000,text:document.querySelector('#root').innerText,transitions:window.motionTransitions,trajectory:map?.getSource('trajectory')?._data,track:map?.getSource('track')?._data};})()`);
    observations.push(sample);
    if(Date.now()>=nextCapture){
      const png=await send('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});
      await writeFile(resolve(out,`capture-${observations.length}.png`),Buffer.from(png.data,'base64'));
      nextCapture=Date.now()+15000;
    }
    await sleep(200);
  }
} finally {
  await writeFile(resolve(out,'browser.json'),JSON.stringify({url,observations,errors}));
  socket?.close();browser.kill();
  await new Promise(res=>browser.exitCode!==null||browser.signalCode!==null?res():browser.once('exit',res));
  await rm(profile,{recursive:true,force:true});
}
if(errors.length)throw new Error('Browser runtime exceptions recorded');
