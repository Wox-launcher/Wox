import { Button, Column, createWoxView, woxViewNode } from "../types/index.js"

createWoxView({
  onError: console.error,
  render: session => Column({ children: [Button({ onPress: () => session.invalidate() }), woxViewNode("text-field", { value: "", onChange: (value: string) => console.log(value) })] })
})
// @ts-expect-error Unsupported component kinds are rejected.
woxViewNode("unknown")
// @ts-expect-error A number is not a valid text-field value.
woxViewNode("text-field", { value: 123 })
