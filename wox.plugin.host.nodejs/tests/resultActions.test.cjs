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
  rpc.pluginInstances.set("owner", { Plugin: { query }, API: api, ResultActions: callbacks, ModulePath: "unused" })
  return { callbacks, api }
}

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
