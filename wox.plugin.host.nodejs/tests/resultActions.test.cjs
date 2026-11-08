const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("node:fs")
const path = require("node:path")
const ts = require("typescript")
const warnings = []

const mocks = {
  "./connection": { waitingForResponse: {}, currentConnection: undefined },
  "./logger": {
    logger: {
      info() {},
      error() {},
      warn(_ctx, message) {
        warnings.push(message)
      }
    }
  },
  "./singleFile": { evictCommonJSModule() {} }
}

// Load the actual RPC and API code without starting a host process.
function loadSource(name) {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../src", name), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const module = { exports: {} }
  new Function("require", "module", "exports", compiled)(name => mocks[name] ?? require(name), module, module.exports)
  return module.exports
}
mocks["./resultActions"] = loadSource("resultActions.ts")
const { ResultActions } = mocks["./resultActions"]
mocks["./jsonrpc"] = { pluginInstances: new Map() }
mocks["./pluginWindow"] = loadSource("pluginWindow.ts")
const { PluginAPI } = loadSource("pluginAPI.ts")
mocks["./pluginAPI"] = { PluginAPI }
const rpc = loadSource("jsonrpc.ts")
mocks["./jsonrpc"].pluginInstances = rpc.pluginInstances

const context = (session = "", query = "") => ({
  Values: { SessionId: session, QueryId: query },
  Get(key) {
    return this.Values[key]
  }
})
const call = (method, session, query, params = {}) =>
  rpc.handleRequestFromWox(context(), {
    PluginId: "owner",
    PluginName: "Owner",
    Method: method,
    SessionId: session,
    QueryId: query,
    ManagedQuery: true,
    Params: params
  })

function fixture(query) {
  const callbacks = new ResultActions()
  const api = new PluginAPI({}, "owner", "Owner", callbacks)
  api.invokeMethod = async () => true
  rpc.pluginInstances.set("owner", { Plugin: { query }, API: api, ResultActions: callbacks, ToolbarMsgActions: new Map(), ModulePath: "unused" })
  return { callbacks, api }
}

test("preview events stay scoped to their result and released document", async () => {
  const calls = [], released = []
  const { callbacks } = fixture(async () => {
    const preview = { PreviewType: "woxview", PreviewData: "{}", WoxViewID: "doc", OnWoxViewEvent: (ctx, event) => calls.push([ctx.Get("SessionId"), event.Args]) }
    Object.defineProperty(preview, Symbol.for("wox.preview.binding"), { value: { bind() {}, release: () => released.push(true) } })
    return { Results: [{ Id: "row", Preview: preview }] }
  })
  await call("query", "a", "one")
  await call("query", "b", "one")
  await call("woxviewPreviewEvent", "a", "one", { ResultId: "row", DocumentID: "doc", Event: JSON.stringify({ Handler: "h", Args: [3], Key: "" }) })
  await call("woxviewPreviewEvent", "a", "one", { ResultId: "row", DocumentID: "old", Event: "{}" })
  callbacks.release("a", "one")
  await call("woxviewPreviewEvent", "a", "one", { ResultId: "row", DocumentID: "doc", Event: "{}" })
  await call("woxviewPreviewEvent", "b", "one", { ResultId: "row", DocumentID: "doc", Event: JSON.stringify({ Args: [4] }) })
  assert.deepEqual(calls, [["a", [3]], ["b", [4]]])
  assert.equal(released.length, 1)
})

test("plugin preview survives its query and releases its callbacks on context exit", async () => {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../../wox.plugin.nodejs/src/woxviewSession.ts"), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const sdk = { exports: {} }
  new Function("require", "module", "exports", compiled)(require, sdk, sdk.exports)
  let count = 0, disposed = 0
  const updates = []
  const preview = sdk.exports.createWoxView({
    onError: error => { throw error },
    render: session => sdk.exports.Button({ label: String(count), onPress: () => { count++; session.invalidate() } })
  }).asPreview({ onDispose: () => disposed++ })
  const { api, callbacks } = fixture(async () => ({ Results: [], PluginPreview: preview }))
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "UpdatePluginPreview")
    updates.push(JSON.parse(params.option))
    return { Success: true }
  }
  const result = "plugin-preview:owner"
  const event = (method, params) => call(method, "a", "first", { ResultId: result, DocumentID: preview.WoxViewID, ...params })
  await call("query", "a", "first")
  await event("woxviewPreviewEvent", { Event: JSON.stringify({ Type: "show" }) })
  callbacks.release("a", "first")
  await call("query", "a", "second")
  callbacks.release("a", "second")
  const handler = JSON.parse(preview.PreviewData).root.props.onPress.$handler
  await event("woxviewPreviewEvent", { Event: JSON.stringify({ Handler: handler, Args: [] }) })
  await new Promise(resolve => setTimeout(resolve, 10))
  assert.equal(count, 1)
  assert.equal(JSON.parse(updates[0].Preview.PreviewData).root.props.label, "1")
  await event("releasePluginPreview", {})
  await event("woxviewPreviewEvent", { Event: JSON.stringify({ Handler: handler, Args: [] }) })
  assert.equal(count, 1)
  assert.equal(disposed, 1)
})

test("plugin preview data sends a chart snapshot without a full document", async () => {
  const { api } = fixture(async () => ({ Results: [] }))
  const option = { WoxViewID: "doc", WoxViewNodeKey: "chart", WoxViewData: { pointRadii: [0, 2] } }
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "UpdatePluginPreviewData")
    assert.deepEqual(JSON.parse(params.option), option)
    return { Success: true }
  }
  assert.deepEqual(await api.UpdatePluginPreviewData({}, option), { Success: true })
})

test("new plugin preview can show before the previous document is released", async () => {
  const disposed = [], seen = []
  const make = id => {
    const preview = { PreviewType: "woxview", PreviewData: "{}", WoxViewID: id, OnWoxViewEvent: () => seen.push(id) }
    Object.defineProperty(preview, Symbol.for("wox.preview.binding"), { value: { bind() {}, release: () => disposed.push(id) } })
    return preview
  }
  const old = make("old"), current = make("new")
  let next = old
  fixture(async () => ({ Results: [], PluginPreview: next }))
  const invoke = (method, query, doc, extra = {}) => call(method, "a", query, { ResultId: "plugin-preview:owner", DocumentID: doc, ...extra })
  await call("query", "a", "q1")
  await invoke("woxviewPreviewEvent", "q1", "old", { Event: JSON.stringify({ Type: "show" }) })
  next = current
  await call("query", "a", "q2")
  await invoke("woxviewPreviewEvent", "q2", "new", { Event: JSON.stringify({ Type: "show" }) })
  await invoke("releasePluginPreview", "q1", "old")
  await invoke("woxviewPreviewEvent", "q2", "new", { Event: "{}" })
  assert.deepEqual(seen, ["old", "new", "new"])
  assert.deepEqual(disposed, ["old"])
  await invoke("releasePluginPreview", "q2", "new")
})

test("SDK preview binds through query RPC and publishes invalidation", async () => {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../../wox.plugin.nodejs/src/woxviewSession.ts"), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const sdk = { exports: {} }
  new Function("require", "module", "exports", compiled)(require, sdk, sdk.exports)
  let count = 0
  const errors = [], updates = [], transitions = []
  const preview = sdk.exports.createWoxView({
    onError: error => errors.push(error),
    render: session => sdk.exports.Button({ label: String(count), onPress: () => { count++; session.invalidate() } })
  }).asPreview({ onShow: () => transitions.push("show"), onHide: () => transitions.push("hide"), onDispose: () => transitions.push("dispose") })
  const { api, callbacks } = fixture(async () => ({ Results: [{ Id: "row", Preview: preview }] }))
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "UpdateResult")
    updates.push(JSON.parse(params.result))
    return true
  }
  await call("query", "a", "query")
  await call("woxviewPreviewEvent", "a", "query", { ResultId: "row", DocumentID: preview.WoxViewID, Event: JSON.stringify({ Type: "show" }) })
  const handler = JSON.parse(preview.PreviewData).root.props.onPress.$handler
  await call("woxviewPreviewEvent", "a", "query", { ResultId: "row", DocumentID: preview.WoxViewID, Event: JSON.stringify({ Handler: handler, Args: [], Key: "" }) })
  await new Promise(resolve => setTimeout(resolve, 10))
  assert.equal(count, 1)
  assert.equal(JSON.parse(updates[0].Preview.PreviewData).root.props.label, "1")
  assert.equal(updates[0].Preview.WoxViewID, preview.WoxViewID)
  await call("woxviewPreviewEvent", "a", "query", { ResultId: "row", DocumentID: preview.WoxViewID, Event: JSON.stringify({ Type: "hide" }) })
  await call("woxviewPreviewEvent", "a", "query", { ResultId: "row", DocumentID: preview.WoxViewID, Event: JSON.stringify({ Type: "show" }) })
  callbacks.release("a", "query")
  assert.deepEqual(transitions, ["show", "hide", "show", "hide", "dispose"])
  assert.deepEqual(errors, [])
})

test("a rejected replacement disposes only the new preview", async () => {
  const disposed = []
  const preview = name => {
    const value = { PreviewType: "woxview", PreviewData: "{}", WoxViewID: name, OnWoxViewEvent() {} }
    value[Symbol.for("wox.preview.binding")] = { bind() {}, release: () => disposed.push(name) }
    return value
  }
  const { api, callbacks } = fixture(async () => ({ Results: [{ Id: "row", Preview: preview("old") }] }))
  await call("query", "a", "query")
  api.invokeMethod = async () => false
  assert.equal(await api.UpdateResult(context("a", "query"), { Id: "row", Preview: preview("new") }), false)
  assert.equal(callbacks.find("a", "query", "row").previews.get("row").id, "old")
  assert.deepEqual(disposed, ["new"])
  callbacks.release("a", "query")
  assert.deepEqual(disposed, ["new", "old"])
})

test("a preview returned after query release is disposed", async () => {
  let complete, disposed = 0
  const pending = new Promise(resolve => { complete = resolve })
  const { callbacks } = fixture(async () => pending)
  const query = call("query", "a", "query")
  callbacks.release("a", "query")
  const preview = { PreviewType: "woxview", PreviewData: "{}", WoxViewID: "doc", OnWoxViewEvent() {} }
  preview[Symbol.for("wox.preview.binding")] = { bind() {}, release: () => disposed++ }
  complete({ Results: [{ Id: "row", Preview: preview }] })
  await query
  assert.equal(disposed, 1)
})

test("toolbar callbacks with the same public ID stay isolated and retire by snapshot", async () => {
  const { api } = fixture(async () => [])
  const calls = []
  const sent = []
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "ShowToolbarMsg")
    assert.equal(params.managedToolbar, "true")
    sent.push(JSON.parse(params.msg))
  }
  for (const session of ["one", "two"]) {
    await api.ShowToolbarMsg(context(session), { Id: "status", Actions: [{ Id: "open", Action: (_ctx, action) => calls.push(`${session}/${action.ToolbarMsgActionId}`) }] })
  }
  const first = sent[0]
  const second = sent[1]
  assert.equal(first.Actions[0].Id, "open")
  assert.equal(second.Actions[0].Id, "open")
  assert.notEqual(first.HostSnapshotId, second.HostSnapshotId)
  await call("toolbarMsgAction", "one", "", { ActionId: "open", HostSnapshotId: first.HostSnapshotId, ToolbarMsgId: "status", ToolbarMsgActionId: "open" })
  await call("toolbarMsgAction", "two", "", { ActionId: "open", HostSnapshotId: second.HostSnapshotId, ToolbarMsgId: "status", ToolbarMsgActionId: "open" })
  assert.deepEqual(calls, ["one/open", "two/open"])
  await call("retireToolbarSnapshot", "one", "", { HostSnapshotId: first.HostSnapshotId })
  assert.equal(rpc.pluginInstances.get("owner").ToolbarMsgActions.has(first.HostSnapshotId), false)
  assert.equal(rpc.pluginInstances.get("owner").ToolbarMsgActions.has(second.HostSnapshotId), true)
})

test("failed toolbar requests discard newly registered callbacks", async () => {
  const { api } = fixture(async () => [])
  api.invokeMethod = async () => { throw new Error("disconnected") }
  await assert.rejects(api.ShowToolbarMsg(context("one"), { Id: "status", Actions: [{ Id: "run", Action: () => {} }] }))
  assert.equal(rpc.pluginInstances.get("owner").ToolbarMsgActions.size, 0)
})

test("a managed message without a callback cannot invoke an older snapshot", async () => {
  const { api } = fixture(async () => [])
  const sent = []
  let invoked = false
  api.invokeMethod = async (_ctx, _method, params) => sent.push(JSON.parse(params.msg))
  await api.ShowToolbarMsg(context("one"), { Id: "old", Actions: [{ Id: "run", Action: () => { invoked = true } }] })
  await api.ShowToolbarMsg(context("one"), { Id: "new", Actions: [{ Id: "run" }] })
  await call("toolbarMsgAction", "one", "", { ActionId: "run", HostSnapshotId: sent[1].HostSnapshotId })
  assert.equal(invoked, false)
})

test("two windows and two rows may reuse action IDs without losing or crossing callbacks", async () => {
  const calls = []
  fixture(async (_ctx, query) => ({
    Results: ["one", "two"].map(id => ({
      Id: id,
      Actions: [{ Id: "open", Action: async () => calls.push(`${query.SessionId}/${query.Id}/${id}`) }]
    }))
  }))
  await call("query", "a", "old")
  await call("query", "b", "other")
  await call("query", "a", "new")
  for (const [session, query, result] of [
    ["a", "old", "one"],
    ["b", "other", "one"],
    ["a", "new", "two"]
  ]) {
    await call("action", session, query, { ResultId: result, ActionId: "open" })
  }
  assert.deepEqual(calls, ["a/old/one", "b/other/one", "a/new/two"])
  await call("action", "a", "missing", { ResultId: "one", ActionId: "open" })
  await call("action", "", "", { ResultId: "one", ActionId: "open" })
  assert.equal(calls.length, 3)
})

test("out-of-order responses retain their own closures; released late responses stay released", async () => {
  const pending = new Map()
  const calls = []
  const { callbacks } = fixture(async (_ctx, query) => {
    await new Promise(resolve => pending.set(query.Id, resolve))
    return { Results: [{ Id: "row", Actions: [{ Id: "open", Action: async () => calls.push(query.Id) }] }] }
  })
  const old = call("query", "a", "old")
  const current = call("query", "a", "current")
  pending.get("current")()
  await current
  pending.get("old")()
  await old
  await call("action", "a", "old", { ResultId: "row", ActionId: "open" })
  await call("action", "a", "current", { ResultId: "row", ActionId: "open" })
  assert.deepEqual(calls, ["old", "current"])
  const late = call("query", "a", "late")
  await call("releaseQueryResults", "a", "late")
  pending.get("late")()
  await late
  assert.equal(callbacks.find("a", "late", "row"), undefined)
})

test("push, update, form restoration and MRU share the scoped callback store", async () => {
  const calls = []
  const { api, callbacks } = fixture(async () => ({ Results: [] }))
  await call("query", "a", "query")
  await call("query", "b", "query")
  const ctx = context("a", "query")
  await api.PushResults(context("b", "query"), { SessionId: "a", Id: "query" }, [
    {
      Id: "row",
      Actions: [{ Id: "edit", Type: "form", OnSubmit: async () => calls.push("form") }]
    }
  ])
  await call("formAction", "a", "query", { ResultId: "row", ActionId: "edit", Values: "{}" })
  api.invokeMethod = async () => JSON.stringify({ Actions: [{ Id: "edit", Type: "form" }] })
  const restored = await api.GetUpdatableResult(ctx, "row")
  assert.equal(typeof restored.Actions[0].OnSubmit, "function")
  api.invokeMethod = async () => true
  await api.UpdateResult(ctx, { Id: "row", Actions: [{ Id: "edit", Type: "form", Name: "Rename" }] })
  assert.equal(callbacks.find("a", "query", "row").results.get("row").get("edit").OnSubmit, restored.Actions[0].OnSubmit)
  await api.UpdateResult(ctx, { Id: "row", Actions: [{ Id: "edit", Action: async () => calls.push("updated") }] })
  await call("formAction", "a", "query", { ResultId: "row", ActionId: "edit" })
  await call("action", "a", "query", { ResultId: "row", ActionId: "edit" })
  await api.UpdateResult(ctx, { Id: "row", Actions: [] })
  await call("action", "a", "query", { ResultId: "row", ActionId: "edit" })
  api.mruRestoreCallbacks.set("mru", async () => ({ Id: "row", Actions: [{ Id: "edit", Action: async () => calls.push("mru") }] }))
  await call("onMRURestore", "a", "home", { CallbackId: "mru" })
  await call("query", "a", "next")
  await call("action", "a", "home", { ResultId: "row", ActionId: "edit" })
  assert.deepEqual(calls, ["form", "updated", "mru"])
  await call("releaseQueryResults", "a", "home")
  await api.PushResults(ctx, { SessionId: "a", Id: "home" }, [{ Id: "row", Actions: [] }])
  assert.equal(callbacks.find("a", "home", "row"), undefined)
  assert.ok(callbacks.find("b", "query", ""))
  await call("unloadPlugin", "", "")
  assert.equal(callbacks.begin("a", "late", true).active, false)
})

test("legacy retention is bounded and unscoped recovery requires a unique result", () => {
  const callbacks = new ResultActions()
  for (let i = 0; i < 33; i++) callbacks.begin("a", String(i), false).cache({ Id: String(i), Actions: [] })
  assert.equal(callbacks.find("a", "0", "0"), undefined)
  assert.ok(callbacks.find("", "", "1"))
  callbacks.begin("b", "1", false).cache({ Id: "1", Actions: [] })
  assert.equal(callbacks.find("", "", "1"), undefined)
})

test("ambiguous updates warn without replacing either query's callbacks", async () => {
  warnings.length = 0
  const { api, callbacks } = fixture(async () => ({ Results: [] }))
  const original = async () => {}
  const replacement = async () => {}
  for (const query of ["old", "new"]) {
    callbacks.begin("a", query, true).cache({ Id: "row", Actions: [{ Id: "open", Action: original }] })
  }
  assert.equal(await api.UpdateResult(context(), { Id: "row", Actions: [{ Id: "open", Action: replacement }] }), true)
  assert.equal(warnings.length, 1)
  assert.match(warnings[0], /Owner.*row.*ambiguous.*SessionId\/QueryId/)
  for (const query of ["old", "new"]) {
    assert.equal(callbacks.find("a", query, "row").results.get("row").get("open").Action, original)
  }
  await api.UpdateResult(context("a", "new"), { Id: "row", Actions: [{ Id: "open", Action: replacement }] })
  assert.equal(callbacks.find("a", "new", "row").results.get("row").get("open").Action, replacement)
  assert.equal(warnings.length, 1)
})
test("removeResult releases callbacks only after an accepted removal", async () => {
  const { api, callbacks } = fixture(async () => ({ Results: [] }))
  for (const query of ["old", "current"]) callbacks.begin("a", query, true).cache({ Id: "row", Actions: [] })
  const ctx = context("a", "current")
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "RemoveResult")
    assert.deepEqual(JSON.parse(params.option), { ResultId: "row", SelectedResultId: "next" })
    return { Success: false }
  }
  assert.equal((await api.RemoveResult(ctx, { ResultId: "row", SelectedResultId: "next" })).Success, false)
  assert.ok(callbacks.find("a", "current", "row").results.has("row"))
  api.invokeMethod = async () => ({ Success: true })
  assert.equal((await api.RemoveResult(ctx, { ResultId: "row" })).Success, true)
  assert.equal(callbacks.find("a", "current", "row").results.has("row"), false)
  assert.ok(callbacks.find("a", "old", "row").results.has("row"))
})
