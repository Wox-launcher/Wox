import assert from "node:assert/strict"
import test from "node:test"
import { Button, Column, Text, createWoxView, woxViewNode } from "../dist/woxviewSession.js"

const ctx = {
  Values: {},
  Get() {},
  Set() {},
  Exists() {
    return false
  }
}
const tick = () => new Promise(resolve => setTimeout(resolve, 10))

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
