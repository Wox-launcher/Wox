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
const sdk = { createWoxView: () => "view", NewContext: () => "context" }
new Function("require", "module", "exports", compiled)(name => name === "@wox-launcher/wox-plugin" ? sdk : require(name), sourceModule, sourceModule.exports)
const { assertPathWithinDirectory, evictCommonJSModule, getPluginExport, loadCommonJSModule } = sourceModule.exports

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
  const first = loadCommonJSModule(modulePath)
  assert.equal(first.plugin.value, 1)
  fs.writeFileSync(modulePath, "module.exports.plugin = { value: 2 }\n")
  evictCommonJSModule(modulePath)
  const second = loadCommonJSModule(modulePath)
  assert.equal(second.plugin.value, 2)
  evictCommonJSModule(modulePath)
})

test("single-file plugin imports the host SDK at load time and later", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-sf-sdk-"))
  const modulePath = path.join(directory, "plugin.js")
  fs.writeFileSync(path.join(directory, "local.js"), "module.exports = 42\n")
  fs.writeFileSync(modulePath, `const sdk = require('@wox-launcher/wox-plugin')
module.exports.plugin = { value: sdk.createWoxView(), later: () => require('@wox-launcher/wox-plugin').NewContext(), local: require('./local.js'), builtin: require('path').basename('/x/y') }
`)
  const loaded = loadCommonJSModule(modulePath)
  assert.deepEqual([loaded.plugin.value, loaded.plugin.later(), loaded.plugin.local, loaded.plugin.builtin], ["view", "context", 42, "y"])
  assert.equal(loadCommonJSModule(modulePath), loaded)
  assert.throws(() => require.resolve("@wox-launcher/wox-plugin", { paths: [directory] }), { code: "MODULE_NOT_FOUND" })
  evictCommonJSModule(modulePath)
})

test("failed single-file loading clears the module cache", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-sf-error-"))
  const modulePath = path.join(directory, "plugin.js")
  fs.writeFileSync(modulePath, "throw new Error('broken')\n")
  assert.throws(() => loadCommonJSModule(modulePath), /broken/)
  fs.writeFileSync(modulePath, "module.exports.plugin = { value: 2 }\n")
  assert.equal(loadCommonJSModule(modulePath).plugin.value, 2)
  evictCommonJSModule(modulePath)
})
