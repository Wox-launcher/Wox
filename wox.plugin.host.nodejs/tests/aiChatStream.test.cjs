const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("node:fs")
const path = require("node:path")
const ts = require("typescript")

const mocks = {
  "./index": { waitingForResponse: {} },
  "./connection": { waitingForResponse: {}, currentConnection: undefined },
  "./logger": { logger: { info() {}, error() {} } },
  "./jsonrpc": { pluginInstances: new Map() }
}

function loadSource(name) {
  const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "../src", name), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2022 }
  }).outputText
  const module = { exports: {} }
  new Function("require", "module", "exports", compiled)(name => mocks[name] ?? require(name), module, module.exports)
  return module.exports
}

const { PluginAPI } = loadSource("pluginAPI.ts")
mocks["./pluginAPI"] = { PluginAPI }
mocks["./singleFile"] = {}
const { handleRequestFromWox, pluginInstances } = loadSource("jsonrpc.ts")
const context = () => ({
  Values: {},
  Get(key) {
    return this.Values[key]
  },
  Set(key, value) {
    this.Values[key] = value
  }
})

test("AIChatStream sends the core method, model, conversations, and options", async () => {
  const api = new PluginAPI({}, "plugin", "Plugin")
  let sent
  api.invokeMethod = async (_ctx, method, params) => {
    sent = { method, params }
  }

  await api.AIChatStream(
    context(),
    { Provider: "deepseek", Name: "deepseek-chat", ProviderAlias: "work" },
    [{ Role: "user", Text: "Hello", Timestamp: 1, Images: [Buffer.from("png")] }],
    { ThinkingMode: "thinking", LoopPolicy: { MaxIterations: -1, RetryOnFailure: true, TimeoutMs: 1500 } },
    () => {}
  )

  assert.equal(sent.method, "AIChatStream")
  assert.equal(typeof sent.params.callbackId, "string")
  assert.deepEqual(JSON.parse(sent.params.model), {
    Name: "deepseek-chat",
    Provider: "deepseek",
    ProviderAlias: "work"
  })
  const conversations = JSON.parse(sent.params.conversations)
  assert.equal(conversations[0].Role, "user")
  assert.equal(conversations[0].Text, "Hello")
  assert.equal(conversations[0].Images[0].ImageType, "base64")
  assert.match(conversations[0].Images[0].ImageData, /^data:image\/png;base64,/)
  assert.deepEqual(JSON.parse(sent.params.options), {
    ThinkingMode: "thinking",
    LoopPolicy: { MaxIterations: -1, RetryOnFailure: true, Timeout: 1500 * 1_000_000 }
  })
})

test("a chat without a model fails before calling core", async () => {
  const api = new PluginAPI({}, "plugin", "Plugin")
  api.invokeMethod = async () => {
    throw new Error("core should not be called")
  }
  await assert.rejects(() => api.AIChatStream(context(), { Provider: "", Name: "" }, [], {}, () => {}))
  assert.equal(api.llmStreamCallbacks.size, 0)
})

for (const terminalStatus of ["finished", "error"]) {
  for (const throws of [false, true]) {
    test(`AIChatStream releases ${terminalStatus} callbacks, throwing=${throws}`, async () => {
      const api = new PluginAPI({}, "plugin", "Plugin")
      pluginInstances.set("plugin", { API: api })
      try {
        const statuses = []
        const failure = new Error("plugin callback failed")
        let callbackId
        api.invokeMethod = async (_ctx, _method, params) => {
          callbackId = params.callbackId
        }
        const callback = data => {
          statuses.push(data.Status)
          if (throws && data.Status === terminalStatus) throw failure
        }
        await api.AIChatStream(context(), { Provider: "openai", Name: "test" }, [], {}, callback)
        const otherCallback = () => {}
        api.llmStreamCallbacks.set("other", otherCallback)
        for (const status of ["streaming", "streamed", "running_tool_call", terminalStatus]) {
          const invocation = handleRequestFromWox(
            context(),
            {
              Method: "onLLMStream",
              PluginId: "plugin",
              Params: { CallbackId: callbackId, StreamType: status, Data: "response" }
            },
            {}
          )
          if (throws && status === terminalStatus) {
            await assert.rejects(invocation, error => error === failure)
          } else {
            await invocation
          }
          assert.equal(api.llmStreamCallbacks.has(callbackId), status !== terminalStatus)
          assert.equal(api.llmStreamCallbacks.get("other"), otherCallback)
        }
        assert.deepEqual(statuses, ["streaming", "streamed", "running_tool_call", terminalStatus])
      } finally {
        pluginInstances.delete("plugin")
      }
    })
  }
}

test("AIChatStream releases callbacks when the request fails", async () => {
  const api = new PluginAPI({}, "plugin", "Plugin")
  const failure = new Error("host websocket disconnected")
  api.invokeMethod = async () => {
    throw failure
  }
  await assert.rejects(
    api.AIChatStream(context(), { Provider: "openai", Name: "test" }, [], {}, () => {}),
    error => error === failure
  )
  assert.equal(api.llmStreamCallbacks.size, 0)
})

test("AIChatStream releases callbacks when payload serialization fails", async () => {
  const api = new PluginAPI({}, "plugin", "Plugin")
  api.invokeMethod = async () => assert.fail("core should not be called")
  await assert.rejects(
    api.AIChatStream(context(), { Provider: "openai", Name: "test" }, [{ Role: "user", Text: "Hi", Timestamp: 1n }], {}, () => {}),
    TypeError
  )
  assert.equal(api.llmStreamCallbacks.size, 0)
})
