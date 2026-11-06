import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"

test("the typed Node.js example matches the Core fixture", async () => {
  const source = await readFile(new URL("./woxview.types.ts", import.meta.url), "utf8")
  const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
  const example = await import(`data:text/javascript,${encodeURIComponent(output)}`)
  const fixture = JSON.parse(await readFile(new URL("../../wox.core/ui/woxview/testdata/sdk-counter.json", import.meta.url), "utf8"))
  assert.deepEqual(example.counter, fixture)
})
