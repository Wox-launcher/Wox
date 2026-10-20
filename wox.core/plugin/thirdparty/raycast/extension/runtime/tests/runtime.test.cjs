const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const readline = require('node:readline');
const esbuild = require('esbuild');
const net = require('node:net');

// Exercise real React effects and the process protocol, including callbacks that await Go-side APIs.
function runtime(t, source, mode = 'view') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'wox-raycast-'));
  fs.writeFileSync(path.join(dir, 'command.js'), source);
  const child = spawn(process.execPath, [path.resolve(__dirname, '../../../../../../resource/hosts/raycast-host.js')], { stdio: ['pipe', 'pipe', 'pipe'], windowsHide: true });
  const pending = new Map();
  const renders = [];
  const calls = [];
  let errors = '';
  let next = 0;
  child.stderr.on('data', data => { errors += data; });
  readline.createInterface({ input: child.stdout }).on('line', line => {
    const message = JSON.parse(line);
    if (message.type === 'reply') { pending.get(message.id)?.(message); pending.delete(message.id); }
    if (message.type === 'render') renders.push(message.response);
    if (message.type === 'api') {
      calls.push(message);
      const result = ['readClipboard', 'selectedText'].includes(message.method) ? 'clipboard value' : message.method === 'storageAll' ? {} : null;
      child.stdin.write(JSON.stringify({ type: 'apiReply', id: message.id, result }) + '\n');
    }
  });
  child.on('exit', code => { for (const resolve of pending.values()) resolve({ error: `exit ${code}: ${errors}` }); });
  t.after(async () => {
    child.kill();
    if (child.exitCode == null) await new Promise(resolve => child.once('exit', resolve));
    fs.rmSync(dir, { recursive: true, force: true });
  });
  const call = (method, params = {}) => new Promise((resolve, reject) => {
    const id = String(++next);
    const timer = setTimeout(() => reject(new Error(`timeout ${method}: ${errors}`)), 5000);
    pending.set(id, message => { clearTimeout(timer); resolve(message); });
    child.stdin.write(JSON.stringify({ id, method, params }) + '\n');
  });
  return { call, renders, calls, child, dir, launch: (input = {}) => call('launch', {
    entry: path.join(dir, 'command.js'), command: { name: 'command', title: 'Test', mode },
    extensionName: 'test', author: 'wox', assetsPath: dir, supportPath: dir, preferences: {}, arguments: {}, refinements: {}, ...input
  }) };
}

test('List hooks, async updates, refinement, preview and awaited clipboard action', async t => {
  const r = runtime(t, `
    const React = require('react');
    const {List, ActionPanel, Action, Clipboard} = require('@raycast/api');
    const h = React.createElement;
    module.exports.default = function Command() {
      const [query, setQuery] = React.useState('');
      const [category, setCategory] = React.useState('all');
      const [title, setTitle] = React.useState('loading');
      React.useEffect(() => { const timer = setTimeout(() => setTitle('loaded'), 80); return () => clearTimeout(timer); }, []);
      return h(List, {onSearchTextChange: setQuery, searchBarAccessory: h(List.Dropdown, {value: category, onChange: setCategory},
        h(List.Dropdown.Item, {value:'all', title:'All'}), h(List.Dropdown.Item, {value:'one',title:'One'}))},
        h(List.Item, {id:'item', title: title + query + category,
          detail: h(List.Item.Detail, {markdown:'# Detail'}),
          actions: h(ActionPanel, null, h(Action, {title:'Copy', onAction: async () => {await Clipboard.copy(await Clipboard.readText()); setTitle('copied');}}))}));
    };`);
  const first = await r.launch();
  assert.equal(first.error, undefined);
  assert.equal(first.response.Results[0].Id, 'item');
  assert.equal(first.response.Results[0].Preview.PreviewData, '# Detail');
  assert.equal(first.response.Refinements[0].Type, 'singleSelect');
  await new Promise(resolve => setTimeout(resolve, 160));
  assert.ok(r.renders.some(value => value.Results[0].Title === 'loadedall'));
  const next = await r.call('query', { search: 'x', refinements: { 'raycast.dropdown': 'one' } });
  assert.equal(next.response.Results[0].Title, 'loadedxone');
  const clicked = await r.call('action', {id: next.response.Results[0].Actions[0].Id});
  assert.equal(clicked.response.Results[0].Title, 'copiedxone');
  assert.equal(r.calls.at(-1).params.text, 'clipboard value');
});

test('initial search and persisted refinement reach React after mounting', async t => {
  const r = runtime(t, `const R=require('react'); const {List}=require('@raycast/api');const h=R.createElement;
    exports.default=function(){const [text,setText]=R.useState('');const [category,setCategory]=R.useState('all');
      return h(List,{onSearchTextChange:setText,searchBarAccessory:h(List.Dropdown,{value:category,onChange:setCategory},h(List.Dropdown.Item,{value:'one',title:'One'}))},h(List.Item,{title:text+category}));};`);
  const first = await r.launch({ search:'initial-', refinements:{'raycast.dropdown':'one'} });
  assert.equal(first.response.Results[0].Title, 'initial-one');
});

test('stopping unmounts React and runs effect cleanup', async t => {
  const r = runtime(t, `const R=require('react');const fs=require('node:fs');const path=require('node:path');const {List,environment}=require('@raycast/api');
    exports.default=function(){R.useEffect(()=>()=>fs.writeFileSync(path.join(environment.supportPath,'cleaned'),'yes'),[]);return R.createElement(List,null,R.createElement(List.Item,{title:'Ready'}));};`);
  await r.launch();
  await r.call('stop');
  if (r.child.exitCode == null) await new Promise(resolve => r.child.once('exit', resolve));
  assert.equal(fs.readFileSync(path.join(r.dir, 'cleaned'), 'utf8'), 'yes');
});

test('Raycast CLI classic JSX factory preserves nested children, fragments and actions', async t => {
  // Match the published Raycast CLI's esbuild jsxFactory/jsxFragment configuration.
  const built = esbuild.transformSync(`import {useState} from 'react';import {List,Action,ActionPanel,Clipboard} from '@raycast/api';
    export default function Command(){const [text,setText]=useState('Built');return <List><>
      <List.Item title={text} actions={<ActionPanel><Action title="Copy" onAction={async()=>{await Clipboard.copy(text);setText('Clicked')}}/></ActionPanel>}/>
      <List.Item title="Second"/>
    </></List>}`, { loader:'tsx', format:'cjs', jsxFactory:'_jsx', jsxFragment:'_jsxFragment', target:'node20' }).code;
  const r = runtime(t, built);
  const first = await r.launch();
  assert.equal(first.error, undefined);
  assert.deepEqual(first.response.Results.map(row => row.Title), ['Built', 'Second']);
  const clicked = await r.call('action', {id:first.response.Results[0].Actions[0].Id});
  assert.equal(clicked.response.Results[0].Title, 'Clicked');
  assert.equal(r.calls.at(-1).params.text, 'Built');
});

test('Grid uses Wox layout and local filtering preserves stable item ids', async t => {
  const r = runtime(t, `const React=require('react'); const {Grid}=require('@raycast/api');
    exports.default=React.memo(()=>React.createElement(Grid,{columns:3}, ...['Alpha','Beta'].map(title=>React.createElement(Grid.Item,{key:title,id:title,title,content:'icon.png'}))));`);
  const first = await r.launch();
  assert.equal(first.error, undefined);
  assert.equal(first.response.Layout.GridLayout.Columns, 3);
  const next = await r.call('query', {search:'beta'});
  assert.deepEqual(next.response.Results.map(row => row.Id), ['Beta']);
});

test('Form submit uses Wox definitions and typed checkbox values', async t => {
  const r = runtime(t, `const React=require('react'); const {Form,ActionPanel,Action,Clipboard}=require('@raycast/api'); const h=React.createElement;
    exports.default=()=>h(Form,{actions:h(ActionPanel,null,h(Action.SubmitForm,{title:'Save',onSubmit:values=>Clipboard.copy(JSON.stringify(values))}))},
      h(Form.TextField,{id:'name',title:'Name'}),h(Form.Checkbox,{id:'checked',title:'Enabled',defaultValue:true}));`);
  const first = await r.launch();
  assert.equal(first.error, undefined);
  const action = first.response.Results[0].Actions[0];
  assert.equal(action.Type, 'form');
  assert.equal(action.Form[1].Type, 'checkbox');
  const submitted = await r.call('action', {id:action.Id, values:{name:'Ada', checked:'true'}});
  assert.equal(submitted.error, undefined);
  assert.deepEqual(JSON.parse(r.calls.at(-1).params.text), {name:'Ada', checked:true});
});

test('no-view completes, unsupported services fail without pretending success', async t => {
  const r = runtime(t, `const {Clipboard}=require('@raycast/api'); exports.default=async()=>{await Clipboard.copy('done')};`, 'no-view');
  assert.equal((await r.launch()).done, true);
  assert.equal(r.calls[0].params.text, 'done');
  const unsupported = runtime(t, `const {OAuth}=require('@raycast/api'); exports.default=async()=>{new OAuth.PKCEClient({})};`, 'no-view');
  assert.match((await unsupported.launch()).error, /OAuth.*not supported/);
});

test('unsupported optional actions preserve usable lists and capability checks exclude AI', async t => {
  const r = runtime(t, `const R=require('react');const {List,ActionPanel,Action,Clipboard,AI,environment}=require('@raycast/api');
    exports.default=()=>R.createElement(List,null,R.createElement(List.Item,{title:String(environment.canAccess(AI))+':'+String(environment.canAccess(Clipboard.copy)),
      actions:R.createElement(ActionPanel,null,R.createElement(Action.CopyToClipboard,{content:'ready'}),R.createElement(Action.CreateQuicklink,{title:'Quicklink'}))}));`);
  const first = await r.launch();
  assert.equal(first.error, undefined);
  assert.equal(first.response.Results[0].Title, 'false:true');
  const actions = first.response.Results[0].Actions;
  assert.equal((await r.call('action', { id: actions[0].Id })).error, undefined);
  assert.match((await r.call('action', { id: actions[1].Id })).error, /Action.CreateQuicklink.*not supported/);
});

test('navigation and unmount replace actions rather than retaining stale handlers', async t => {
  const r = runtime(t, `const React=require('react');const {List,Detail,Action,ActionPanel,useNavigation}=require('@raycast/api');const h=React.createElement;
    exports.default=function(){const {push}=useNavigation();return h(List,null,h(List.Item,{title:'Item',actions:h(ActionPanel,null,h(Action,{title:'Next',onAction:()=>push(h(Detail,{markdown:'next'}))}))}));};`);
  const first = await r.launch();
  const id = first.response.Results[0].Actions[0].Id;
  const next = await r.call('action', {id});
  assert.equal(next.response.Layout.ResultPreviewWidthRatio, 0);
  assert.equal(next.response.Results[0].Preview.PreviewData, 'next');
  assert.match((await r.call('action', {id})).error, /no longer available/);
  const back = await r.call('action', {id:'navigation:back'});
  assert.equal(back.response.Results[0].Title, 'Item');
});

// Opt-in verification uses installed bundles with synthetic host data and never performs OS actions.
test('installed Raycast commands render and invoke Wox APIs', { skip: !process.env.WOX_RAYCAST_VERIFY_EXTENSIONS }, async t => {
  const root = process.env.WOX_RAYCAST_VERIFY_EXTENSIONS;
  const installed = fs.readdirSync(root).flatMap(name => {
    const directory = path.join(root, name);
    try { return [{ directory, manifest: JSON.parse(fs.readFileSync(path.join(directory, 'package.json'), 'utf8')) }]; } catch { return []; }
  });
  for (const example of [
    { name: 'lorem-ipsum', command: 'words', preferences: { action: 'clipboard' }, arguments: { numberOfWords: '3' } },
    { name: 'random', command: 'index', preferences: { defaultAction: 'copy' } },
    { name: 'change-case', command: 'convert-upper-case', preferences: { source: 'clipboard', action: 'copy' } }
  ]) {
    await t.test(example.name, async t => {
      const extension = installed.find(item => item.manifest.name === example.name);
      assert.ok(extension, `missing local extension ${example.name}`);
      const command = extension.manifest.commands.find(item => item.name === example.command);
      assert.ok(command);
      const r = runtime(t, '', command.mode);
      const copy = path.join(r.dir, 'extension');
      fs.cpSync(extension.directory, copy, { recursive: true });
      const defaults = Object.fromEntries([...(extension.manifest.preferences || []), ...(command.preferences || [])].map(pref => [pref.name, pref.default]));
      let reply = await r.launch({
        entry: path.join(copy, `${command.name}.js`), command, extensionName: example.name,
        assetsPath: path.join(copy, 'assets'), preferences: { ...defaults, ...example.preferences }, arguments: example.arguments || {}
      });
      assert.equal(reply.error, undefined);
      if (command.mode === 'no-view') {
        assert.equal(reply.done, true);
        const copied = r.calls.find(call => call.method === 'copy');
        if (example.name === 'lorem-ipsum') assert.equal(copied?.params.text.trim().split(/\s+/).length, 3);
        else assert.equal(copied?.params.text, 'CLIPBOARD VALUE');
      } else {
        for (let attempt = 0; attempt < 20 && !reply.response?.Results?.some(row => row.Actions?.some(action => /copy/i.test(action.Name))); attempt++) {
          await new Promise(resolve => setTimeout(resolve, 100));
          reply = await r.call('query');
          assert.equal(reply.error, undefined);
        }
        const action = reply.response.Results.flatMap(row => row.Actions || []).find(action => /copy/i.test(action.Name));
        assert.ok(action, `expected copy action: ${JSON.stringify(reply.response.Results.map(row => ({title:row.Title,actions:row.Actions})))}`);
        const copied = await r.call('action', { id: action.Id });
        assert.equal(copied.error, undefined);
        assert.ok(r.calls.some(call => call.method === 'copy' && call.params.text), 'expected a text copy through the host');
      }
      assert.equal((await r.call('stop')).error, undefined);
    });
  }
});

// Open Ports enumerates actual listeners; only a test-owned listener is searched and no kill actions run.
test('installed Port Manager subscribes, filters and updates cached view state', { skip: !process.env.WOX_RAYCAST_VERIFY_EXTENSIONS, timeout: 30000 }, async t => {
  const root = process.env.WOX_RAYCAST_VERIFY_EXTENSIONS;
  const source = fs.readdirSync(root).map(name => path.join(root, name)).find(directory => {
    try { return JSON.parse(fs.readFileSync(path.join(directory, 'package.json'), 'utf8')).name === 'port-manager'; } catch { return false; }
  });
  assert.ok(source, 'Port Manager must be installed locally');
  const server = net.createServer();
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const port = String(server.address().port);
  const r = runtime(t, '');
  const copy = path.join(r.dir, 'extension');
  fs.cpSync(source, copy, { recursive: true });
  const manifest = JSON.parse(fs.readFileSync(path.join(copy, 'package.json'), 'utf8'));
  let reply = await r.launch({ entry: path.join(copy, 'open-ports.js'), command: manifest.commands.find(command => command.name === 'open-ports'),
    extensionName: manifest.name, author: manifest.author, assetsPath: path.join(copy, 'assets'), search: port,
    preferences: { killSignal: 'ask', primaryPortAction: 'copyInfo' } });
  assert.equal(reply.error, undefined);
  for (let attempt = 0; attempt < 60 && !reply.response?.Results?.some(row => row.Preview?.PreviewData?.includes(port)); attempt++) {
    await new Promise(resolve => setTimeout(resolve, 250));
    reply = await r.call('query', { search: port });
    assert.equal(reply.error, undefined);
  }
  const row = reply.response.Results.find(row => row.Preview?.PreviewData?.includes(port));
  assert.ok(row, `test listener was not listed: ${JSON.stringify(reply.response)}`);
  assert.equal(reply.response.Refinements[0].Type, 'singleSelect');
  const toggle = row.Actions.find(action => action.Name === 'Show Details');
  assert.ok(toggle);
  assert.equal(reply.response.Layout.ResultPreviewWidthRatio, 0.4);
  const toggled = await r.call('action', { id: toggle.Id });
  assert.equal(toggled.error, undefined);
  assert.equal(toggled.response.Layout.ResultPreviewWidthRatio, undefined);
  const filtered = await r.call('query', { search: port, refinements: { 'raycast.dropdown': 'loopback' } });
  assert.equal(filtered.error, undefined);
  assert.ok(filtered.response.Results.some(row => row.Preview?.PreviewData?.includes(port)));
  const stored = JSON.stringify(fs.readdirSync(r.dir).filter(name => /^cache-.*\.json$/.test(name)).map(name => JSON.parse(fs.readFileSync(path.join(r.dir, name), 'utf8'))));
  assert.match(stored, /loopback/);
  assert.equal((await r.call('stop')).error, undefined);
});
