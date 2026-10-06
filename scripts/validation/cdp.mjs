#!/usr/bin/env node
// Minimal one-shot Chrome DevTools driver (Node >= 22, global WebSocket).
//   cdp.mjs PORT eval 'js expression'   -> prints the JSON result
//   cdp.mjs PORT navigate URL
//   cdp.mjs PORT shot FILE.png
// The page persists between invocations; each call attaches, acts, detaches.
import { writeFileSync } from 'node:fs';

const [port, command, arg] = process.argv.slice(2);

const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
const page = targets.find((target) => target.type === 'page');
if (!page) throw new Error('no page target');

const socket = new WebSocket(page.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});

let next = 0;
const waiting = new Map();
socket.onmessage = (message) => {
  const data = JSON.parse(message.data);
  const resolve = waiting.get(data.id);
  if (resolve) {
    waiting.delete(data.id);
    resolve(data);
  }
};
const call = (method, params = {}) =>
  new Promise((resolve) => {
    const id = ++next;
    waiting.set(id, resolve);
    socket.send(JSON.stringify({ id, method, params }));
  });

let code = 0;
if (command === 'eval') {
  const reply = await call('Runtime.evaluate', {
    expression: arg,
    awaitPromise: true,
    returnByValue: true,
  });
  if (reply.result.exceptionDetails) {
    console.error(JSON.stringify(reply.result.exceptionDetails));
    code = 1;
  } else {
    console.log(JSON.stringify(reply.result.result.value));
  }
} else if (command === 'navigate') {
  await call('Page.enable');
  await call('Page.navigate', { url: arg });
} else if (command === 'shot') {
  const reply = await call('Page.captureScreenshot', { format: 'png' });
  writeFileSync(arg, Buffer.from(reply.result.data, 'base64'));
} else {
  console.error('unknown command');
  code = 2;
}
socket.close();
process.exit(code);
