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

test("window instances route operations and early events through RPC", async () => {
  const api = new PluginAPI({}, "window-owner", "Owner")
  rpc.pluginInstances.set("window-owner", { API: api })
  const fired = [],
    calls = []
  let callbackID
  api.invokeMethod = async (_ctx, method, params) => {
    const option = JSON.parse(params.option)
    calls.push({ method, option, id: params.windowID })
    if (method === "CreateWindow") {
      callbackID = option.CallbackID
      assert.equal(option.OnEvent, undefined)
      assert.equal(option.Window.Presentation, "window")
      await rpc.handleRequestFromWox(
        context(),
        { PluginId: "window-owner", Method: "windowEvent", Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "ready", WindowID: "native-1", DocumentID: "doc-1" }) } },
        {}
      )
      return { Success: true, WindowID: "native-1" }
    }
    return { Success: true }
  }
  const tree = JSON.stringify({ version: 1, root: { type: "text", props: { text: "hello" } } })
  const created = await api.CreateWindow(context(), {
    Content: { Type: "woxview", Data: tree },
    Window: { Presentation: "window", Topmost: false, OffsetX: 0, Movable: true, DragContent: true, Transparent: true, PassThroughTransparent: true },
    OnEvent: (_ctx, event) => fired.push(event)
  })
  assert.equal(fired[0].Type, "ready")
  assert.equal(created.Window.ID, "native-1")
  await created.Window.Show(context(), {})
  await created.Window.Hide(context(), {})
  await created.Window.SetOptions(context(), { Topmost: false, OffsetY: 0 })
  await created.Window.Content.Set(context(), { Content: { Type: "woxview", Data: tree } })
  assert.deepEqual(calls.at(-1).option.Content, { Type: "woxview", Data: tree })
  assert.equal((await created.Window.Content.Update(context(), { Data: tree, DocumentID: "doc-1", Sequence: 7 })).Success, true)
  assert.deepEqual(calls.at(-1), { method: "WindowContentUpdate", id: "native-1", option: { Data: tree, DocumentID: "doc-1", Sequence: 7 } })
  for (const Command of ["focus", "scrollTo", "selectAll"]) {
    const option = { DocumentID: "doc-1", Command, Key: "draft/世界" }
    assert.equal((await created.Window.Content.Command(context(), option)).Success, true)
    assert.deepEqual(calls.at(-1), { method: "WindowContentCommand", id: "native-1", option })
  }
  const payload = { Handler: "save", Args: [{ name: "hello" }] }
  await rpc.handleRequestFromWox(
    context(),
    { PluginId: "window-owner", Method: "windowEvent", Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "woxview_event", WindowID: "native-1", DocumentID: "doc-1", Data: payload }) } },
    {}
  )
  assert.equal(fired.at(-1).Type, "woxview_event")
  assert.deepEqual(fired.at(-1).Data, payload)
  for (const Dark of [false, true]) {
    await rpc.handleRequestFromWox(
      context(),
      {
        PluginId: "window-owner",
        Method: "windowEvent",
        Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "theme_changed", WindowID: "native-1", DocumentID: "doc-1", Data: { Dark } }) }
      },
      {}
    )
    assert.equal(fired.at(-1).Type, "theme_changed")
    assert.equal(fired.at(-1).DocumentID, "doc-1")
    assert.deepEqual(fired.at(-1).Data, { Dark })
  }
  assert.equal(calls[0].option.Window.Topmost, false)
  assert.equal(calls[0].option.Window.DragContent, true)
  assert.equal(calls[0].option.Window.Transparent, true)
  assert.equal(calls[0].option.Window.PassThroughTransparent, true)
  assert.deepEqual(calls[0].option.Content, { Type: "woxview", Data: tree })
  await rpc.handleRequestFromWox(context(), { PluginId: "window-owner", Method: "windowEvent", Params: { CallbackID: callbackID, Release: "true" } }, {})
  assert.equal(api.windowCallbacks.size, 0)
  const delivered = fired.length
  await rpc.handleRequestFromWox(
    context(),
    { PluginId: "window-owner", Method: "windowEvent", Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "theme_changed", WindowID: "native-1", Data: { Dark: false } }) } },
    {}
  )
  assert.equal(fired.length, delivered)
  const count = calls.length
  assert.equal((await created.Window.Show(context(), {})).Success, false)
  assert.equal((await created.Window.Content.Update(context(), { Data: tree })).Success, false)
  assert.equal((await created.Window.Content.Command(context(), { Command: "focus", Key: "draft" })).Success, false)
  assert.equal((await created.Window.Close(context(), {})).Success, true)
  assert.equal(calls.length, count)
})

test("window failure and unload release every callback", async () => {
  const api = new PluginAPI({}, "window-owner", "Owner")
  api.invokeMethod = async () => {
    throw new Error("disconnected")
  }
  await assert.rejects(api.CreateWindow(context(), { Content: { Type: "woxview", Data: JSON.stringify({ version: 1, root: { type: "text", props: { text: "" } } }) } }), /disconnected/)
  assert.equal(api.windowCallbacks.size, 0)
  api.invokeMethod = async () => ({ Success: false, ErrMsg: "unsupported" })
  assert.equal((await api.CreateWindow(context(), { Content: { Type: "woxview", Data: JSON.stringify({ version: 1, root: { type: "text", props: { text: "" } } }) } })).Success, false)
  assert.equal(api.windowCallbacks.size, 0)
  api.invokeMethod = async () => ({ Success: true, WindowID: "native-2" })
  const created = await api.CreateWindow(context(), { Content: { Type: "woxview", Data: JSON.stringify({ version: 1, root: { type: "text", props: { text: "" } } }) } })
  api.releaseWindows()
  assert.equal(api.windowCallbacks.size, 0)
  assert.equal((await created.Window.Hide(context(), {})).Success, false)
})

test("window bindings attach after creation and release exactly once", async () => {
  const api = new PluginAPI({}, "window-owner", "Owner")
  const events = []
  const binding = { bind: (_ctx, window) => events.push(`bound:${window.ID}`), release: () => events.push("released") }
  const option = { Content: { Type: "woxview", Data: "{}" }, [Symbol.for("wox.window.binding")]: binding }
  api.invokeMethod = async (_ctx, method, params) => {
    assert.equal(method, "CreateWindow")
    assert.equal(JSON.stringify(params).includes("binding"), false)
    return { Success: true, WindowID: "native-3" }
  }
  const created = await api.CreateWindow(context(), option)
  assert.equal(created.Success, true)
  assert.deepEqual(events, ["bound:native-3"])
  api.releaseWindows()
  api.releaseWindows()
  assert.deepEqual(events, ["bound:native-3", "released"])
  const failed = { Content: { Type: "woxview", Data: "{}" }, [Symbol.for("wox.window.binding")]: binding }
  api.invokeMethod = async () => ({ Success: false, ErrMsg: "unsupported" })
  assert.equal((await api.CreateWindow(context(), failed)).Success, false)
  assert.deepEqual(events, ["bound:native-3", "released", "released"])
})

test("WoxView works through host window RPC", async () => {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../../wox.plugin.nodejs/src/woxviewSession.ts"), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const sdk = { exports: {} }
  new Function("require", "module", "exports", compiled)(require, sdk, sdk.exports)
  let params
  rpc.pluginInstances.set("view-owner", {
    Plugin: {
      init: async (_ctx, value) => {
        params = value
      }
    }
  })
  await rpc.handleRequestFromWox(context(), { PluginId: "view-owner", PluginName: "View", Method: "init", Params: { PluginDirectory: "" } }, {})
  const api = params.API,
    calls = [],
    errors = []
  let callbackID,
    count = 0,
    handler
  api.invokeMethod = async (ctx, method, args) => {
    const option = JSON.parse(args.option)
    calls.push({ method, option })
    if (method === "CreateWindow") {
      callbackID = option.CallbackID
      handler = JSON.parse(option.Content.Data).root.props.onPress.$handler
      await rpc.handleRequestFromWox(
        ctx,
        { PluginId: "view-owner", Method: "windowEvent", Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "ready", WindowID: "win", DocumentID: "doc" }) } },
        {}
      )
      return { Success: true, WindowID: "win" }
    }
    return { Success: true }
  }
  const view = sdk.exports.createWoxView({
    onError: error => errors.push(error),
    render: session =>
      sdk.exports.Button({
        label: String(count),
        onPress: () => {
          count++
          session.invalidate()
        }
      })
  })
  await api.CreateWindow(context(), { ...view.asWindow() })
  await rpc.handleRequestFromWox(
    context(),
    {
      PluginId: "view-owner",
      Method: "windowEvent",
      Params: { CallbackID: callbackID, Event: JSON.stringify({ Type: "woxview_event", WindowID: "win", DocumentID: "doc", Data: { Handler: handler, Args: [], Key: "" } }) }
    },
    {}
  )
  await new Promise(resolve => setTimeout(resolve, 10))
  assert.equal(count, 1)
  const update = calls.find(call => call.method === "WindowContentUpdate")
  assert.equal(JSON.parse(update.option.Data).root.props.label, "1")
  assert.equal(update.option.DocumentID, "doc")
  assert.deepEqual(errors, [])
  api.releaseWindows()
  rpc.pluginInstances.delete("view-owner")
})
