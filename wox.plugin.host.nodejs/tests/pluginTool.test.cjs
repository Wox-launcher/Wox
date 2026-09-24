const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("node:fs")
const path = require("node:path")
const ts = require("typescript")

// Use the existing transpilation approach without starting the real host process.
const instances = new Map()
const mocks = {
  "./index": { waitingForResponse: {} },
  "./connection": { waitingForResponse: {}, currentConnection: undefined },
  "./logger": { logger: { info() {}, error() {} } },
  "./jsonrpc": { pluginInstances: instances }
}
function loadSource(name) {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../src", name), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const module = { exports: {} }
  new Function("require", "module", "exports", compiled)(name => mocks[name] ?? require(name), module, module.exports)
  return module.exports
}
const { PluginAPI } = loadSource("pluginAPI.ts")
mocks["./pluginAPI"] = { PluginAPI }
mocks["./singleFile"] = {}
const rpc = loadSource("jsonrpc.ts")
const context = () => ({
  Values: {},
  Get(key) {
    return this.Values[key]
  },
  Set(key, value) {
    this.Values[key] = value
  }
})

test("unload drains work even after Core stops waiting", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  let finish
  const execution = new Promise(resolve => {
    finish = resolve
  })
  api.pluginToolCalls.add(execution)
  let stopped = false
  const stopping = api.stopPluginTools().then(() => {
    stopped = true
  })
  await Promise.resolve()
  assert.equal(api.pluginToolsStopping, true)
  assert.equal(stopped, false)
  finish()
  await stopping
  assert.equal(stopped, true)
})

test("duplicate registration and failed unregister preserve the original handler", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  const results = [{ Error: null }, { Error: { Code: "TOOL_ALREADY_REGISTERED" } }, { Error: { Code: "EXECUTION_FAILED" } }, { Error: null }]
  api.invokeMethod = async () => results.shift()
  const option = { Tool: { Name: "ping" }, Handler: () => ({ Output: {} }) }
  await api.RegisterPluginTool(context(), option)
  const original = api.pluginToolCallbackIds.get("ping")
  await api.RegisterPluginTool(context(), option)
  assert.equal(api.pluginToolCallbackIds.get("ping"), original)
  assert.equal(api.pluginToolCallbacks.size, 1)
  await api.UnregisterPluginTool(context(), { Name: "ping" })
  assert.ok(api.pluginToolCallbacks.has(original))
  await api.UnregisterPluginTool(context(), { Name: "ping" })
  assert.equal(api.pluginToolCallbacks.size, 0)
  assert.equal(api.pluginToolCallbackIds.size, 0)
})

test("nested invocation forwards the parent token", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  rpc.pluginInstances.set("owner", { API: api })
  let sent
  api.invokeMethod = async (_ctx, _method, params) => {
    sent = params
    return { Output: {} }
  }
  api.pluginToolCallbacks.set("cb", async ctx => {
    await api.InvokePluginTool(ctx, { PluginId: "other", Name: "ping" })
    return { Output: {} }
  })
  await rpc.handleRequestFromWox(context(), {
    Method: "onInvokePluginTool",
    PluginId: "owner",
    PluginName: "Owner",
    Params: { CallbackId: "cb", Arguments: "{}", PluginToolCallId: "parent-token" }
  })
  assert.equal(sent.PluginToolCallId, "parent-token")
  assert.equal(api.pluginToolCalls.size, 0)
  api.pluginToolsStopping = true
  const rejected = await rpc.handleRequestFromWox(context(), {
    Method: "onInvokePluginTool",
    PluginId: "owner",
    PluginName: "Owner",
    Params: { CallbackId: "cb", Arguments: "{}" }
  })
  assert.equal(rejected.Error.Code, "PLUGIN_UNAVAILABLE")
  rpc.pluginInstances.delete("owner")
})
