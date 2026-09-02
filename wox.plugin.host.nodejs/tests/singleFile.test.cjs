const { test } = require("node:test")
const assert = require("node:assert/strict")
const fs = require("fs")
const os = require("os")
const path = require("path")
const ts = require("typescript")

const sourcePath = path.join(__dirname, "..", "src", "singleFile.ts")
const compiled = ts.transpileModule(fs.readFileSync(sourcePath, "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020 }
}).outputText
const sourceModule = { exports: {} }
new Function("require", "module", "exports", compiled)(require, sourceModule, sourceModule.exports)
const { assertPathWithinDirectory, evictCommonJSModule, getPluginExport } = sourceModule.exports

test("rejects path escape", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-sf-"))
  assert.throws(() => assertPathWithinDirectory(path.join(directory, "..", "secret.js"), directory))
  assert.equal(assertPathWithinDirectory(path.join(directory, "..plugin.js"), directory), path.join(directory, "..plugin.js"))
})

test("requires plugin export", () => {
  assert.throws(() => getPluginExport({}))
  assert.equal(getPluginExport({ plugin: { query() {} } }).query.name, "query")
})

test("clears require cache so reload sees new code", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-sf-"))
  const modulePath = path.join(directory, "plugin.js")
  fs.writeFileSync(modulePath, "module.exports.plugin = { value: 1 }\n")
  const first = require(modulePath)
  assert.equal(first.plugin.value, 1)
  fs.writeFileSync(modulePath, "module.exports.plugin = { value: 2 }\n")
  evictCommonJSModule(modulePath)
  const second = require(modulePath)
  assert.equal(second.plugin.value, 2)
})
