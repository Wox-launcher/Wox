import { Button, Column, Loading, createWoxView, woxViewNode } from "../types/index.js"

Loading()
Loading({ key: "pending", style: { width: 24, color: "$accent" } })
// @ts-expect-error Width must be a size, not a boolean.
Loading({ style: { width: true } })

const view = createWoxView({
  onError: console.error,
  render: session => Column({ children: [Button({ onPress: () => session.invalidate() }), woxViewNode("text-field", { value: "", onChange: (value: string) => console.log(value) })] })
})
view.asPreview({ onShow: signal => signal.addEventListener("abort", () => {}), onHide: () => {}, onDispose: () => {} })
// @ts-expect-error Visibility hooks receive an AbortSignal.
view.asPreview({ onShow: (signal: number) => console.log(signal) })
// @ts-expect-error Unsupported component kinds are rejected.
woxViewNode("unknown")
// @ts-expect-error A number is not a valid text-field value.
woxViewNode("text-field", { value: 123 })
