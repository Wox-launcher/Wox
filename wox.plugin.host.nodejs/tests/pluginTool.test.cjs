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
mocks["./resultActions"] = loadSource("resultActions.ts")
mocks["./pluginWindow"] = loadSource("pluginWindow.ts")
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

test("toast callbacks are one-shot and dismissal does not invoke them", async () => {
  const api = new PluginAPI({}, "toast-owner", "Owner")
  const calls = []
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "ShowToastMsg")
    calls.push(JSON.parse(params.option))
    return { Success: true, ErrMsg: "" }
  }
  rpc.pluginInstances.set("toast-owner", { API: api })
  const fired = []
  await api.ShowToastMsg(context(), { Message: "done", Action: { Name: "Open", Action: (_ctx, value) => fired.push(value.ContextData) } })
  const id = calls[0].Action.CallbackId
  const request = { PluginId: "toast-owner", PluginName: "Owner", Method: "toastMsgAction", Params: { CallbackId: id, Invoke: "true", ContextData: '{"path":"test"}' } }
  await rpc.handleRequestFromWox(context(), request, {})
  await rpc.handleRequestFromWox(context(), request, {})
  assert.deepEqual(fired, [{ path: "test" }])
  await api.ShowToastMsg(context(), { Message: "again", Action: { Name: "Open", Action: () => fired.push("unexpected") } })
  request.Params.CallbackId = calls[1].Action.CallbackId
  request.Params.Invoke = "false"
  await rpc.handleRequestFromWox(context(), request, {})
  assert.equal(api.toastCallbacks.size, 0)
  assert.equal(fired.length, 1)
  rpc.pluginInstances.delete("toast-owner")
})

test("toast callback is released when the request fails", async () => {
  const api = new PluginAPI({}, "toast-owner", "Owner")
  api.invokeMethod = async () => {
    throw new Error("disconnected")
  }
  await assert.rejects(api.ShowToastMsg(context(), { Message: "done", Action: { Name: "Open", Action: () => {} } }))
  assert.equal(api.toastCallbacks.size, 0)
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

test("GetTriggerKeywords forwards options and preserves the effective trigger order", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  const ctx = context()
  api.invokeMethod = async (actualContext, method, params) => {
    assert.equal(actualContext, ctx)
    assert.equal(method, "GetTriggerKeywords")
    assert.deepEqual(JSON.parse(params.option), {})
    return { TriggerKeywords: ["*", "custom", "runtime"] }
  }
  assert.deepEqual(await api.GetTriggerKeywords(ctx, {}), { TriggerKeywords: ["*", "custom", "runtime"] })
})

test("GetPluginInfo forwards the target plugin ID", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  const ctx = context()
  const info = { PluginId: "target", Installed: true, Enabled: true, Ready: false, Version: "1.0.0", TriggerKeywords: ["target"] }
  api.invokeMethod = async (actualContext, method, params) => {
    assert.equal(actualContext, ctx)
    assert.equal(method, "GetPluginInfo")
    assert.deepEqual(JSON.parse(params.option), { PluginId: "target" })
    return info
  }
  assert.deepEqual(await api.GetPluginInfo(ctx, { PluginId: "target" }), info)
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
