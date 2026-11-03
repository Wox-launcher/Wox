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

test("hotkey RPC delivers release while start is pending and isolates owners", async () => {
  const api = new PluginAPI({}, "hotkey-owner", "Owner")
  const other = new PluginAPI({}, "other", "Other")
  rpc.pluginInstances.set("hotkey-owner", { API: api })
  rpc.pluginInstances.set("other", { API: other })
  let callbackID, finish
  const pending = new Promise(resolve => {
    finish = resolve
  })
  const fired = []
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "OnHotkey")
    callbackID = JSON.parse(params.option).CallbackID
    return { Success: true }
  }
  assert.equal(
    (
      await api.OnHotkey(context(), {
        Callback: async (_ctx, event) => {
          fired.push(event.Phase)
          if (event.Phase === "started") await pending
        }
      })
    ).Success,
    true
  )
  const send = (owner, phase) =>
    rpc.handleRequestFromWox(
      context(),
      {
        PluginId: owner,
        Method: "hotkeyEvent",
        Params: { CallbackID: callbackID, Event: JSON.stringify({ Phase: phase, ActivationID: "hold-1" }) }
      },
      {}
    )
  await send("hotkey-owner", "started")
  await send("other", "ended")
  assert.deepEqual(fired, ["started"])
  await send("hotkey-owner", "ended")
  assert.deepEqual(fired, ["started", "ended"])
  finish()
  api.releaseHotkeys()
  await send("hotkey-owner", "triggered")
  assert.equal(fired.length, 2)
  assert.equal(api.hotkeyCallbacks.size, 0)
})

test("failed hotkey subscription does not retain callbacks", async () => {
  const api = new PluginAPI({}, "owner", "Owner")
  api.invokeMethod = async () => ({ Success: false, ErrMsg: "already subscribed" })
  assert.equal((await api.OnHotkey(context(), { Callback() {} })).Success, false)
  assert.equal(api.hotkeyCallbacks.size, 0)
  api.invokeMethod = async () => {
    throw new Error("offline")
  }
  await assert.rejects(api.OnHotkey(context(), { Callback() {} }), /offline/)
  assert.equal(api.hotkeyCallbacks.size, 0)
})
