// @ts-check
/** @typedef {import("../types/index.js").WoxViewTree} WoxViewTree */

/** @type {WoxViewTree} */
const tree = {
  version: 1,
  root: { type: "button", props: { label: "Add", onPress: { $handler: "increment" } } }
}

void tree
