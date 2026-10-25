const Module = require('node:module');
const readline = require('node:readline');
const React = require('react');
const jsxRuntime = require('react/jsx-runtime');
const { createRenderer } = require('./renderer.cjs');
const { createAPI } = require('./api.cjs');
const { createMapper } = require('./mapping.cjs');

const write = process.stdout.write.bind(process.stdout);
const send = message => write(JSON.stringify(message) + '\n');
const pending = new Map();
let nextCall = 0;
let renderer, mapper, config, container, snapshot, renderTimer, failure;
let stack = [];
let launched = false;

// Console output uses the same structured channel so Wox owns diagnostic formatting.
for (const level of ['log', 'info', 'warn', 'error', 'debug']) console[level] = (...args) => send({ type: 'log', level, text: args.map(String).join(' ') });
process.stdout.write = (data, encoding, callback) => {
  send({ type: 'log', level: 'info', text: String(data) });
  if (typeof encoding === 'function') encoding(); else callback?.();
  return true;
};

function hostCall(method, params) {
  const id = String(++nextCall);
  return new Promise((resolve, reject) => {
    // User confirmation has no response deadline; Go cancels it when the owning query/session leaves.
    const timer = method === 'confirmAlert' ? null : setTimeout(() => { pending.delete(id); reject(new Error(`Wox API ${method} timed out`)); }, 30000);
    pending.set(id, { resolve, reject, timer });
    send({ type: 'api', id, method, params });
  });
}

function report(error) {
  failure = error?.message || String(error);
  send({ type: 'error', error: failure });
}

function publish() {
  if (!mapper || !container || failure) return;
  try {
    const { Loading, ...response } = mapper.map(container);
    snapshot = { response, loading: !!Loading };
    send({ type: 'render', ...snapshot });
  }
  catch (error) { report(error); }
}

// Coalesce React commits, and flush explicitly for query replies to avoid refresh loops.
function onCommit(next) {
  container = next;
  clearImmediate(renderTimer);
  renderTimer = setImmediate(publish);
}

const navigation = {
  push(element) { stack.push(element); renderer.render(element); },
  pop() { if (stack.length > 1) { stack.pop(); renderer.render(stack.at(-1)); } },
  root() { if (stack.length) { stack = [stack[0]]; renderer.render(stack[0]); } },
  depth: () => stack.length
};

// The process is dedicated to one command, so these overrides cannot leak into Wox SDK plugins.
async function launch(params) {
  if (launched) throw new Error('Raycast command already started');
  launched = true;
  config = params;
  const api = createAPI(config, hostCall, navigation);
  const original = Module.prototype.require;
  const overrides = { react: React, 'react/jsx-runtime': jsxRuntime, '@raycast/api': api };
  Module.prototype.require = function (id) { return overrides[id] || original.call(this, id); };
  // Raycast's CLI uses esbuild's classic jsxFactory, whose remaining arguments
  // are children. Automatic JSX-runtime imports use the separate require override.
  globalThis._jsx = React.createElement;
  globalThis._jsxs = (type, props, key) => jsxRuntime.jsxs(type, props || {}, key);
  globalThis._jsxFragment = React.Fragment;
  const module = Module.createRequire(config.entry)(config.entry);
  const command = module.default || module;
  if (typeof command !== 'function' && !(config.command.mode === 'view' && command?.$$typeof)) {
    throw new Error('Raycast command must have a default component or function export');
  }
  const props = { arguments: config.arguments || {}, launchType: 'userInitiated', fallbackText: config.search || '' };
  if (config.command.mode === 'no-view') { await command(props); return { done: true }; }
  mapper = createMapper(config, navigation);
  await mapper.input(config.search || '', config.refinements || {});
  renderer = createRenderer(onCommit, report);
  const root = React.createElement(command, props);
  stack = [root];
  renderer.render(root);
  await settle();
  // Controls exist only after the first commit. Deliver the initial Wox input then,
  // including refinements restored by Wox before this command was mounted.
  await mapper.input(config.search || '', config.refinements || {});
  await settle();
  return { ...snapshot, error: failure };
}

async function settle() {
  await new Promise(setImmediate);
  await new Promise(setImmediate);
  clearImmediate(renderTimer);
  publish();
}

async function request(message) {
  switch (message.method) {
    case 'launch': return launch(message.params);
    case 'query':
      await mapper.input(message.params.search || '', message.params.refinements || {});
      await settle();
      return { ...snapshot, error: failure };
    case 'action':
      await mapper.dispatch(message.params.id, message.params.values || {});
      await settle();
      return { ...snapshot, error: failure };
    case 'stop':
      renderer?.unmount();
      setTimeout(() => process.exit(0), 30).unref();
      return {};
    default: throw new Error(`Unknown runtime method ${message.method}`);
  }
}

// API replies bypass command serialization: a command may be waiting for the host to answer.
let queue = Promise.resolve();
readline.createInterface({ input: process.stdin }).on('line', line => {
  let message;
  try { message = JSON.parse(line); } catch (error) { report(error); return; }
  if (message.type === 'apiReply') {
    const call = pending.get(message.id);
    if (!call) return;
    pending.delete(message.id);
    clearTimeout(call.timer);
    if (message.error) call.reject(new Error(message.error)); else call.resolve(message.result);
    return;
  }
  queue = queue.then(async () => {
    try { send({ type: 'reply', id: message.id, ...await request(message) }); }
    catch (error) { send({ type: 'reply', id: message.id, error: error.message || String(error) }); }
  });
}).on('close', () => process.exit(0));
process.on('uncaughtException', report);
process.on('unhandledRejection', report);
