import assert from "node:assert/strict"
import test from "node:test"
import { Button, Column, Loading, Text, createWoxView, woxViewNode } from "../dist/index.js"

test("Loading builds a loading node with optional style and key", () => {
  assert.deepEqual(Loading(), { type: "loading", props: {} })
  assert.deepEqual(Loading({ key: "pending", style: { width: 24, color: "$accent" } }), {
    type: "loading", key: "pending", props: { style: { width: 24, color: "$accent" } }
  })
})

const ctx = {
  Values: {},
  Get() {},
  Set() {},
  Exists() {
    return false
  }
}
const tick = () => new Promise(resolve => setTimeout(resolve, 10))

test("preview visibility aborts work on hide and disposes exactly once", async () => {
  const transitions = [], signals = [], errors = []
  let timer
  const preview = createWoxView({ onError: error => errors.push(error), render: () => Text({ text: "preview" }) }).asPreview({
    onShow: signal => {
      transitions.push("show")
      signals.push(signal)
      timer = setInterval(() => {}, 1000)
      signal.addEventListener("abort", () => { clearInterval(timer); transitions.push("abort") }, { once: true })
    },
    onHide: () => transitions.push("hide"),
    onDispose: () => transitions.push("dispose")
  })
  const binding = preview[Symbol.for("wox.preview.binding")]
  binding.bind(ctx, { UpdateResult: async () => true }, "row")
  await preview.OnWoxViewEvent(ctx, { Type: "show" })
  await preview.OnWoxViewEvent(ctx, { Type: "show" })
  await preview.OnWoxViewEvent(ctx, { Type: "hide" })
  await preview.OnWoxViewEvent(ctx, { Type: "hide" })
  await preview.OnWoxViewEvent(ctx, { Type: "show" })
  binding.release()
  binding.release()
  await preview.OnWoxViewEvent(ctx, { Type: "show" })
  assert.deepEqual(transitions, ["show", "abort", "hide", "show", "abort", "hide", "dispose"])
  assert.ok(signals.every(signal => signal.aborted))
  assert.deepEqual(errors, [])
})

test("hiding does not wait for an unfinished show hook", async () => {
  let finish, signal
  const wait = new Promise(resolve => { finish = resolve })
  const preview = createWoxView({ onError: () => {}, render: () => Text({ text: "preview" }) }).asPreview({
    onShow: async value => { signal = value; await wait }
  })
  await preview.OnWoxViewEvent(ctx, { Type: "show" })
  await preview.OnWoxViewEvent(ctx, { Type: "hide" })
  assert.equal(signal.aborted, true)
  finish()
  preview[Symbol.for("wox.preview.binding")].release()
})

test("preview callbacks update their result and stop after release", async () => {
  let count = 0
  const updates = [], errors = []
  const preview = createWoxView({
    onError: error => errors.push(error),
    render: session => Button({ label: String(count), onPress: value => { count += value; session.invalidate() } })
  }).asPreview()
  const binding = { ...preview }[Symbol.for("wox.preview.binding")]
  binding.bind(ctx, { UpdateResult: async (_ctx, update) => { updates.push(update); return true } }, "row")
  const handler = JSON.parse(preview.PreviewData).root.props.onPress.$handler
  await preview.OnWoxViewEvent(ctx, { Handler: handler, Args: [2], Key: "" })
  await tick()
  assert.equal(count, 2)
  assert.equal(updates[0].Id, "row")
  assert.equal(updates[0].Preview.WoxViewID, preview.WoxViewID)
  assert.equal(JSON.parse(updates[0].Preview.PreviewData).root.props.label, "2")
  binding.release()
  await preview.OnWoxViewEvent(ctx, { Handler: handler, Args: [2], Key: "" })
  assert.equal(count, 2)
  assert.deepEqual(errors, [])
})

test("a rejected preview update keeps the accepted callback registry", async () => {
  let session, value = "old"
  const calls = [], errors = []
  const preview = createWoxView({ onError: error => errors.push(error.message), render: s => {
    session = s
    const current = value
    return Button({ onPress: () => calls.push(current) })
  } }).asPreview()
  const binding = preview[Symbol.for("wox.preview.binding")]
  binding.bind(ctx, { UpdateResult: async () => false }, "row")
  const handler = JSON.parse(preview.PreviewData).root.props.onPress.$handler
  value = "new"
  session.invalidate()
  await tick()
  await preview.OnWoxViewEvent(ctx, { Handler: handler, Args: [], Key: "" })
  assert.deepEqual(calls, ["old"])
  assert.deepEqual(errors, ["WoxView preview update failed"])
  binding.release()
})

test("a window binding dispatches callbacks and retains handler identities", async () => {
  const errors = []
  const updates = []
  let count = 0
  const view = createWoxView({
    onError: error => errors.push(error),
    render: session =>
      Column({
        children: [
          Text({ text: String(count), key: "label" }),
          Button({
            label: "Add",
            key: "add",
            onPress: () => {
              count++
              session.invalidate()
            }
          })
        ]
      })
  })
  const option = view.asWindow()
  const binding = option[Symbol.for("wox.window.binding")]
  const initial = JSON.parse(option.Content.Data)
  const handler = initial.root.children[1].props.onPress.$handler
  await option.OnEvent(ctx, { Type: "ready", WindowID: "win", DocumentID: "doc" })
  binding.bind(ctx, {
    Content: {
      Update: async (_, update) => {
        updates.push(update)
        return { Success: true }
      }
    }
  })
  await tick()
  await option.OnEvent(ctx, { Type: "woxview_event", WindowID: "win", DocumentID: "doc", Data: { Handler: handler, Args: [], Key: "add" } })
  await tick()
  assert.equal(count, 1)
  assert.equal(updates.length, 1)
  assert.equal(updates[0].DocumentID, "doc")
  assert.equal(updates[0].Sequence, 1)
  const next = JSON.parse(updates[0].Data)
  assert.equal(next.root.children[0].props.text, "1")
  assert.equal(next.root.children[1].props.onPress.$handler, handler)
  binding.release()
  await option.OnEvent(ctx, { Type: "woxview_event", WindowID: "win", DocumentID: "doc", Data: { Handler: handler, Args: [], Key: "add" } })
  assert.equal(count, 1)
  assert.deepEqual(errors, [])
})

test("a rejected update retains the previous accepted callbacks", async () => {
  let callback = "old"
  let calls = []
  let session
  const errors = []
  const view = createWoxView({
    onError: error => errors.push(error.message),
    render: s => {
      session = s
      const value = callback
      return Button({ onPress: () => calls.push(value) })
    }
  })
  const option = view.asWindow()
  const handler = JSON.parse(option.Content.Data).root.props.onPress.$handler
  const binding = option[Symbol.for("wox.window.binding")]
  binding.bind(ctx, { Content: { Update: async () => ({ Success: false, ErrMsg: "rejected" }) } })
  await option.OnEvent(ctx, { Type: "ready", WindowID: "win", DocumentID: "doc" })
  callback = "new"
  session.invalidate()
  await tick()
  await option.OnEvent(ctx, { Type: "woxview_event", WindowID: "win", DocumentID: "doc", Data: { Handler: handler, Args: [], Key: "" } })
  assert.deepEqual(calls, ["old"])
  assert.deepEqual(errors, ["rejected"])
  binding.release()
})

test("changed node types and reordered menu entries retain correct identities", async () => {
  let session,
    type = "button",
    reverse = false
  const updates = []
  const view = createWoxView({
    onError: error => {
      throw error
    },
    render: s => {
      session = s
      const entries = ["one", "two"].map(label => ({ label, onPress: () => {} }))
      if (reverse) entries.reverse()
      return woxViewNode(type, { onPress: () => {}, contextMenu: entries })
    }
  })
  const option = view.asWindow(),
    binding = option[Symbol.for("wox.window.binding")]
  const initial = JSON.parse(option.Content.Data).root.props
  binding.bind(ctx, {
    Content: {
      Update: async (_, update) => {
        updates.push(JSON.parse(update.Data))
        return { Success: true }
      }
    }
  })
  await option.OnEvent(ctx, { Type: "ready", DocumentID: "doc" })
  reverse = true
  session.invalidate()
  await tick()
  const reordered = updates[0].root.props
  assert.equal(reordered.contextMenu[0].onPress.$handler, initial.contextMenu[1].onPress.$handler)
  type = "column"
  session.invalidate()
  await tick()
  assert.notEqual(updates[1].root.props.onPress.$handler, initial.onPress.$handler)
  binding.release()
})

test("events wait for pending updates and use the accepted callback", async () => {
  let session,
    label = "old",
    resolveUpdate
  const calls = []
  const view = createWoxView({
    onError: error => {
      throw error
    },
    render: s => {
      session = s
      const current = label
      return Button({ onPress: () => calls.push(current) })
    }
  })
  const option = view.asWindow(),
    binding = option[Symbol.for("wox.window.binding")]
  const handler = JSON.parse(option.Content.Data).root.props.onPress.$handler
  binding.bind(ctx, {
    Content: {
      Update: () =>
        new Promise(resolve => {
          resolveUpdate = resolve
        })
    }
  })
  await option.OnEvent(ctx, { Type: "ready", DocumentID: "doc" })
  label = "new"
  session.invalidate()
  const event = option.OnEvent(ctx, { Type: "woxview_event", DocumentID: "doc", Data: { Handler: handler, Args: [] } })
  await Promise.resolve()
  assert.deepEqual(calls, [])
  resolveUpdate({ Success: true })
  await event
  assert.deepEqual(calls, ["new"])
  binding.release()
})
