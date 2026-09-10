#!/usr/bin/env node
/** CDP-only mock review. Run host Vite on 3001 first. Node 22+; Chrome with WebGL. */
import { spawn, execFileSync } from 'node:child_process';
import { mkdtemp, mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const storybook = process.argv.includes('--storybook');
let url = process.env.SHOOT_URL ?? 'http://127.0.0.1:3001/?source=mock';
if (!storybook && new URL(url).searchParams.get('source') !== 'mock') throw new Error('This driver operates mock fixtures only.');
const output = resolve(root, process.env.SHOOT_OUT ?? `docs/temp/evidence/T-014/${new Date().toISOString().replace(/[:.]/g, '-')}`);
const chromePath = process.env.CHROME_BIN ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const sleep = ms => new Promise(r => setTimeout(r, ms));
await mkdir(output, { recursive: true });
const observations = [];

async function capture(name, width, height) {
  const profile = await mkdtemp(resolve(tmpdir(), 'yalb-shoot-'));
  const browser = spawn(chromePath, ['--headless=new', '--remote-debugging-port=0', `--user-data-dir=${profile}`,
    '--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--no-first-run', '--no-default-browser-check', 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
  let stderr = '';
  browser.stderr.on('data', chunk => { stderr += chunk; });
  let socket;
  const events = [];
  try {
    let port;
    for (let i = 0; i < 100; i++) {
      try { port = (await readFile(resolve(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]; break; } catch { await sleep(100); }
    }
    if (!port) throw new Error(`Chrome did not start: ${stderr}`);
    const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
    socket = new WebSocket(targets.find(target => target.type === 'page').webSocketDebuggerUrl);
    await new Promise((res, rej) => { socket.addEventListener('open', res, { once: true }); socket.addEventListener('error', rej, { once: true }); });
    let serial = 0;
    const pending = new Map();
    socket.addEventListener('message', message => {
      const data = JSON.parse(message.data);
      if (data.id) {
        const call = pending.get(data.id);
        if (!call) return;
        pending.delete(data.id);
        clearTimeout(call.timer);
        data.error ? call.reject(new Error(JSON.stringify(data.error))) : call.resolve(data.result);
      } else if (['Runtime.consoleAPICalled', 'Runtime.exceptionThrown', 'Log.entryAdded'].includes(data.method)) events.push(data);
    });
    const send = (method, params = {}) => new Promise((res, rej) => {
      const id = ++serial;
      pending.set(id, { resolve: res, reject: rej, timer: setTimeout(() => { pending.delete(id); rej(new Error(`CDP timeout: ${method}`)); }, 15000) });
      socket.send(JSON.stringify({ id, method, params }));
    });
    const evaluate = async (fn, ...args) => {
      const result = await send('Runtime.evaluate', { expression: `(${fn.toString()})(...${JSON.stringify(args)})`, returnByValue: true, awaitPromise: true });
      if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
      return result.result.value;
    };
    const check = async (fn, message) => { if (!await evaluate(fn)) throw new Error(message); };
    const click = async text => {
      await evaluate(text => {
        // Lever labels are uppercased by CSS, so innerText is not the source string.
        const button = [...document.querySelectorAll('button')].find(button => button.innerText.toUpperCase() === text.toUpperCase() && button.getClientRects().length);
        if (!button) throw new Error(`Button missing: ${text}`);
        button.click();
      }, text);
      await sleep(75);
    };
    const screenshot = async suffix => {
      const png = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
      await writeFile(resolve(output, `${name}${suffix}.png`), Buffer.from(png.data, 'base64'));
      await writeFile(resolve(output, `${name}${suffix}.txt`), await evaluate(() => (document.getElementById('root') ?? document.getElementById('storybook-root') ?? document.body).innerText));
    };
    await send('Runtime.enable');
    await send('Log.enable');
    await send('Page.enable');
    await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
    await send('Page.navigate', { url });
    if (storybook) {
      for (let attempt = 0; attempt < 100; attempt++) {
        if (await evaluate(() => !!document.querySelector('#storybook-root .group, #storybook-docs .group'))) break;
        await sleep(150);
      }
      await sleep(1000);
      await screenshot('');
      const rendered = await evaluate(() => {
        const group = document.querySelector('#storybook-root .group, #storybook-docs .group');
        return { text: (document.getElementById('storybook-root')?.innerText || document.getElementById('storybook-docs')?.innerText) ?? '',
          token: getComputedStyle(document.documentElement).getPropertyValue('--lume').trim(),
          labelColor: group ? getComputedStyle(group.querySelector('.group__label')).color : null,
          labelSize: group ? getComputedStyle(group.querySelector('.group__label')).fontSize : null,
          error: document.body.classList.contains('sb-show-errordisplay') ? document.querySelector('.sb-errordisplay')?.innerText : '' };
      });
      const errors = events.filter(event => event.method === 'Runtime.exceptionThrown' || event.params?.type === 'error');
      observations.push({ name, url, rendered, errors });
      if (!rendered.text || rendered.token !== '#d8dde3' || rendered.labelSize !== '10px' || errors.length) {
        throw new Error(`Story rendering failed: ${JSON.stringify({ name, rendered, errors })}`);
      }
      return;
    }
    for (let attempt = 0; attempt < 100; attempt++) {
      if (await evaluate(() => !!document.querySelector('[aria-label="Vehicle 3:1"]'))) break;
      await sleep(150);
    }
    await sleep(3200);
    await check(() => document.querySelector('[aria-label="Vehicle 3:1"]')?.innerText.includes('LOST'), 'LOST fixture missing');
    await screenshot('-fleet');
    await click('Open selected vehicle');
    // Software WebGL/startup compilation can delay timers. Wait for the actual
    // fixture state, not merely a nominal wall-clock budget.
    const warmed = () => {
      const roll = [...document.querySelectorAll('#panel-instruments .row')].find(row => row.querySelector('.row__label')?.textContent === 'Roll');
      return document.querySelector('#panel-families .group__annotation')?.textContent === '10/10 fresh'
        && Math.abs(Number.parseFloat(roll?.querySelector('.row__value')?.textContent ?? '0')) > 0;
    };
    for (let attempt = 0; attempt < 120; attempt++) {
      if (await evaluate(warmed)) break;
      await sleep(150);
    }
    await check(warmed, 'Full fresh mock telemetry did not settle');
    // MapLibre adds WebGL route layers only after its initial raster load.
    await sleep(3500);
    await check(() => document.querySelector('.mission-panel')?.innerText.includes('Complete'), 'Mock mission did not load');
    await check(() => document.querySelector('[aria-label="Select vehicle"]')?.querySelectorAll('button').length === 3, 'Vehicle selector missing');
    await screenshot('');
    const layout = await evaluate(() => {
      const rect = selector => { const r = document.querySelector(selector)?.getBoundingClientRect(); return r ? { x: r.x, y: r.y, width: r.width, height: r.height } : null; };
      const instruments = document.querySelector('#panel-instruments');
      return { width: innerWidth, height: innerHeight, pageOverflow: document.documentElement.scrollWidth > innerWidth,
        aux: rect('.slot__stack > .slot--aux'), dev: rect('.slot__stack > .slot--dev'), position: rect('#panel-position'),
        rollPadding: getComputedStyle(instruments.querySelector('.row')).paddingLeft,
        markerFill: document.querySelector('.vehicle-marker polygon') ? getComputedStyle(document.querySelector('.vehicle-marker polygon')).fill : null,
        positionHeader: document.querySelector('#panel-position .group__head')?.innerText };
    });
    if (layout.pageOverflow || layout.aux.y >= layout.dev.y) throw new Error(`Layout failed: ${JSON.stringify(layout)}`);
    if (width < 960) {
      await evaluate(() => document.querySelector('#panel-instruments').scrollIntoView({ block: 'start' }));
      await screenshot('-instruments');
      await evaluate(() => { document.querySelector('.vehicle-workspace .shell__body').scrollTop = 0; });
    }
    await click('Views (12)');
    const ids = await evaluate(() => [...document.querySelectorAll('.views__option input')].map(input => input.getAttribute('aria-controls')));
    for (const id of ids) {
      await evaluate(id => document.querySelector(`.views__option [aria-controls="${id}"]`).click(), id);
      await sleep(25);
      await check(() => document.querySelector('#panel-link')?.hidden === false, 'Fixed link must stay visible');
      const hidden = await evaluate(id => document.getElementById(id).hidden, id);
      if (!hidden) throw new Error(`Panel was not hidden: ${id}`);
    }
    for (const id of ids) { await evaluate(id => document.querySelector(`.views__option [aria-controls="${id}"]`).click(), id); await sleep(25); }
    await evaluate(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })));
    await check(() => !document.querySelector('.views__popover'), 'Escape did not dismiss Views');
    for (let i = 0; i < 2; i++) {
      await click('Refresh mission');
      await check(() => document.querySelector('.mission-panel').innerText.includes('Complete'), 'Refresh failed');
    }
    await click('2:1');
    await check(() => document.querySelector('[aria-label="Select vehicle"] [aria-pressed="true"]').innerText === '2:1', 'Vehicle switch failed');
    await check(() => document.querySelector('.mission-panel').innerText.includes('Complete'), 'Second vehicle mission missing');
    await click('1:1');
    await evaluate(() => { const tab = document.querySelector('#tab-families'); tab.focus(); tab.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })); });
    await sleep(50);
    await check(() => document.activeElement?.id === 'tab-sample' && !document.querySelector('#panel-sample').hidden, 'Keyboard tabs failed');
    const errors = events.filter(event => event.method === 'Runtime.exceptionThrown' || event.params?.type === 'error');
    const basemapErrors = errors.filter(event => event.params?.type === 'error' && event.params.args?.some(arg => /^ct: AJAXError: Failed to fetch .*https:\/\/server\.arcgisonline\.com\//.test(arg.description ?? '')));
    const runtimeErrors = errors.filter(event => !basemapErrors.includes(event));
    observations.push({ name, layout, smoke: 'passed', runtimeErrors, basemapErrors });
    if (runtimeErrors.length) throw new Error(`Runtime errors: ${JSON.stringify(runtimeErrors)}`);
  } finally {
    socket?.close();
    browser.kill('SIGTERM');
    await new Promise(res => { if (browser.exitCode !== null || browser.signalCode !== null) res(); else browser.once('exit', res); });
    await writeFile(resolve(output, `${name}-console.json`), JSON.stringify(events, null, 2));
    await writeFile(resolve(output, `${name}-chrome.log`), stderr);
    await rm(profile, { recursive: true, force: true });
  }
}

if (storybook) {
  const base = process.env.STORYBOOK_URL ?? 'http://127.0.0.1:6010';
  const storyIds = process.env.SHOOT_STORY_IDS?.split(',') ?? ['system-vocabulary--all', 'panels-instruments--normal', 'composition-layout-shell--wide', 'frontend-current-app--vehicle', 'composition-layout-shell--docs'];
  for (const id of storyIds) {
    url = `${base}/iframe.html?id=${id}&viewMode=${id.endsWith('--docs') ? 'docs' : 'story'}`;
    await capture(id, 1600, 1000);
  }
} else {
  await capture('desktop', 1600, 1000);
  await capture('mobile', 560, 900);
}
const revision = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
const diff = execFileSync('git', ['diff', '--', 'frontend'], { cwd: root });
await writeFile(resolve(output, 'observations.json'), JSON.stringify(observations, null, 2));
await writeFile(resolve(output, 'README.md'), storybook ? `# Storybook rendering check\n\nDate: ${new Date().toISOString()}\nBase revision: ${revision}\nCommand: node scripts/shoot.mjs --storybook\n\nThe stories/docs listed in observations.json rendered non-empty content with the system palette and 10px group labels. Screenshots, text, computed values and console events accompany this check. Synthetic fixtures only; no live telemetry claim.\n` : `# Mock UI review\n\nDate: ${new Date().toISOString()}\nBase revision: ${revision}\nFrontend diff SHA256: ${createHash('sha256').update(diff).digest('hex')}\nURL: ${url}\n\nCommand: node scripts/shoot.mjs (host Vite on 3001). Chrome CDP with software WebGL, fresh profile per viewport, real wall-clock settling.\n\nExpected and observed: two streaming vehicles, LOST observer, auto-loaded missions, readable provenance, instrument tier above developer tier, no page overflow. Panel hide/restore, mission refresh twice, vehicle switching and keyboard tabs passed at both widths. Application runtime exceptions/errors: none. Public basemap request errors, if any, are recorded separately in observations.json.\n\nPNG captures and matching text dumps include fleet, vehicle and stacked instruments; JSON records computed layout and console events. Basemap loading depends on public internet; these synthetic fixtures do not establish live telemetry behavior.\n`);
console.log(output);
