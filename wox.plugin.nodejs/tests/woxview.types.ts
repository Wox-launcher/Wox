import type { WoxViewEventData, WoxViewTree } from "../types/index.js"

export const counter: WoxViewTree = {
  version: 1,
  root: {
    type: "column",
    children: [
      { type: "text", props: { text: "Count: 0" } },
      { type: "button", key: "increment", props: { label: "Add", onPress: { $handler: "increment" } } }
    ]
  }
}

export const event: WoxViewEventData = { Handler: "increment", Args: [], Key: "increment" }
// @ts-expect-error Callback functions are not part of the wire protocol.
const badHandler: WoxViewTree = { version: 1, root: { type: "button", props: { onPress: () => {} } } }
// @ts-expect-error Grid nodes require props with a column count.
const badGrid: WoxViewTree = { version: 1, root: { type: "grid" } }
// @ts-expect-error Leaf nodes cannot contain child nodes.
const badChildren: WoxViewTree = { version: 1, root: { type: "button", children: [{ type: "text" }] } }
// @ts-expect-error Image nodes need either an icon or a source.
const badImage: WoxViewTree = { version: 1, root: { type: "image", props: {} } }
void [badHandler, badGrid, badChildren, badImage]
