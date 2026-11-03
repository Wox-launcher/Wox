const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("node:fs")
const path = require("node:path")
const vm = require("node:vm")

// Execute the exact embedded page bridge without a native engine for malformed-input coverage.
function page(iframe = false) {
  const sent = [],
    handlers = {}
  const window = { chrome: { webview: { postMessage: message => sent.push(JSON.parse(message.slice(12))) } } }
  window.top = iframe ? {} : window
  const source = fs.readFileSync(path.join(__dirname, "../../wox.core/ui/pluginwindow/bridge.js"), "utf8").replace("__WOX_DOCUMENT_TOKEN__", '"document-1"')
  vm.runInNewContext(source, {
    window,
    TextEncoder,
    document: {
      addEventListener: (name, fn) => {
        handlers[name] = fn
      }
    }
  })
  return { window, sent, handlers }
}

test("HTML bridge has no iframe capability and rejects stale document delivery", () => {
  assert.equal(page(true).window.woxWindow, undefined)
  const { window, sent, handlers } = page()
  handlers.DOMContentLoaded()
  assert.equal(sent[0].Type, "ready")
  const received = []
  const unsubscribe = window.woxWindow.onMessage(data => received.push(data))
  window.__woxWindowReceive("old", "ignored")
  window.__woxWindowReceive("document-1", { value: false })
  unsubscribe()
  window.__woxWindowReceive("document-1", "ignored")
  assert.deepEqual(received, [{ value: false }])
  window.woxWindow.postMessage({ action: "cancel", snake_case: false })
  assert.deepEqual(sent.at(-1).Data, { action: "cancel", snake_case: false })
  assert.equal(sent.at(-1).Token, "document-1")
})

test("HTML bridge bounds UTF-8 payloads and reports listener errors", async () => {
  const { window, sent, handlers } = page()
  assert.throws(() => window.woxWindow.postMessage(undefined), /must be JSON/)
  assert.throws(() => window.woxWindow.postMessage("界".repeat(100000)), /too large/)
  assert.throws(() => window.woxWindow.onMessage(null), /must be a function/)
  window.woxWindow.onMessage(() => {
    throw new Error("sync failure")
  })
  window.woxWindow.onMessage(async () => {
    throw new Error("async failure")
  })
  window.__woxWindowReceive("document-1", {})
  await new Promise(resolve => setImmediate(resolve))
  assert.deepEqual(
    sent.map(event => event.Type),
    ["error", "error"]
  )
  let prevented = false
  handlers.click({
    target: { closest: () => ({ href: "https://example.com/" }) },
    preventDefault: () => {
      prevented = true
    }
  })
  assert.equal(prevented, true)
  assert.equal(sent.at(-1).Type, "link")
})
