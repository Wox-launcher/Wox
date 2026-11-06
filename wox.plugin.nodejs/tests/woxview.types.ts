import type { WoxViewEventData, WoxViewTree } from "../types/index.js"

export const counter: WoxViewTree = {
  version: 1,
  root: {
    type: "column",
    children: [
      { type: "text", props: { text: "Count: 0", inputRegion: true } },
      { type: "button", key: "increment", props: { label: "Add", onPress: { $handler: "increment" } } }
    ]
  }
}

export const event: WoxViewEventData = { Handler: "increment", Args: [], Key: "increment" }
export const morph: WoxViewTree = {
  version: 1,
  root: { type: "morph-path", key: "shape", props: {
    keyframes: [
      { at: 0, path: [{ op: "M", values: [0, 0] }, { op: "C", values: [0, 1, 1, 1, 1, 0] }, { op: "Z" }] },
      { at: 1, path: [{ op: "M", values: [0, 0] }, { op: "C", values: [0, .5, 1, .5, 1, 0] }, { op: "Z" }] }
    ],
    progress: 1, duration: 200, curve: "easeInOut", style: { width: 40, height: 20, color: "$accent" }
  } }
}
// @ts-expect-error Path morphs require keyframes.
const badMorph: WoxViewTree = { version: 1, root: { type: "morph-path", props: {} } }
// @ts-expect-error Callback functions are not part of the wire protocol.
const badHandler: WoxViewTree = { version: 1, root: { type: "button", props: { onPress: () => {} } } }
// @ts-expect-error Grid nodes require props with a column count.
const badGrid: WoxViewTree = { version: 1, root: { type: "grid" } }
// @ts-expect-error Leaf nodes cannot contain child nodes.
const badChildren: WoxViewTree = { version: 1, root: { type: "button", children: [{ type: "text" }] } }
// @ts-expect-error Image nodes need either an icon or a source.
const badImage: WoxViewTree = { version: 1, root: { type: "image", props: {} } }
void [badHandler, badGrid, badChildren, badImage, badMorph]
