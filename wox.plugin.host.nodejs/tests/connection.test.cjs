const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("node:fs")
const path = require("node:path")
const ts = require("typescript")

const mocks = {
  "./logger": { logger: { info() {}, error() {} } },
  "./jsonrpc": { pluginInstances: new Map(), PluginJsonRpcTypeRequest: "WOX_JSONRPC_REQUEST" }
}

function loadSource(name) {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../src", name), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const module = { exports: {} }
  new Function("require", "module", "exports", compiled)(required => mocks[required] ?? require(required), module, module.exports)
  return module.exports
}

const connection = loadSource("connection.ts")
mocks["./connection"] = connection
const { PluginAPI } = loadSource("pluginAPI.ts")
const { WebSocket } = require("ws")

function context() {
  return { Values: {}, Get() { return "" } }
}

function openSocket() {
  const sent = []
  return {
    sent,
    readyState: WebSocket.OPEN,
    send(raw, callback) {
      sent.push(JSON.parse(raw))
      if (callback) callback()
    }
  }
}

test("invoke uses the replacement connection and keeps the same API object", async () => {
  const original = openSocket()
  const replacement = openSocket()
  connection.setCurrentConnection(original)
  const api = new PluginAPI(original, "tasknotes", "TaskNotes")
  connection.setCurrentConnection(replacement)

  const pending = api.invokeMethod(context(), "HideApp", {})
  await Promise.resolve()
  assert.equal(connection.closeCurrentConnection(original), false)
  assert.equal(original.sent.length, 0)
  assert.equal(replacement.sent[0].Method, "HideApp")
  connection.waitingForResponse[replacement.sent[0].Id].resolve(null)
  await pending
  assert.equal(connection.currentConnection, replacement)
})

test("closing the current connection fails a waiting call", async () => {
  const ws = openSocket()
  connection.setCurrentConnection(ws)
  const api = new PluginAPI(ws, "tasknotes", "TaskNotes")
  const pending = api.invokeMethod(context(), "HideApp", {})
  await Promise.resolve()
  assert.equal(connection.closeCurrentConnection(ws), true)
  await assert.rejects(pending, /host websocket disconnected/)
  assert.equal(Object.keys(connection.waitingForResponse).length, 0)
  await assert.rejects(api.invokeMethod(context(), "HideApp", {}), /host websocket is not connected/)
})

test("an error response rejects the waiting call", async () => {
  const ws = openSocket()
  connection.setCurrentConnection(ws)
  const api = new PluginAPI(ws, "tasknotes", "TaskNotes")
  const pending = api.invokeMethod(context(), "GetSetting", {})
  await Promise.resolve()
  const id = ws.sent[0].Id
  assert.equal(connection.settleResponse({ Id: id, Error: 'missing required parameter "Key"', Result: null }), true)
  await assert.rejects(pending, /missing required parameter "Key"/)
  assert.equal(connection.settleResponse({ Id: id, Result: null }), false)
})

test("a successful response resolves with its result", async () => {
  const ws = openSocket()
  connection.setCurrentConnection(ws)
  const api = new PluginAPI(ws, "tasknotes", "TaskNotes")
  const pending = api.invokeMethod(context(), "GetSetting", { Key: "k" })
  await Promise.resolve()
  connection.settleResponse({ Id: ws.sent[0].Id, Result: "value" })
  assert.equal(await pending, "value")
})
