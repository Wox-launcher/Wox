const { test } = require("node:test")
const assert = require("node:assert/strict")
const { spawn } = require("node:child_process")
const fs = require("node:fs")
const net = require("node:net")
const os = require("node:os")
const path = require("node:path")
const { WebSocket } = require("ws")

// Run the real TypeScript entry without requiring a previously built bundle.
const entry = `
  const fs = require('fs'), ts = require('typescript');
  process.argv = [process.execPath, 'host', process.env.TEST_PORT, process.env.TEST_LOG, process.env.TEST_PID];
  require.extensions['.ts'] = (module, filename) => module._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020 }
  }).outputText, filename);
  require('./src/index.ts');
`

function launch(port, directory, secret) {
  const env = { ...process.env, TEST_PORT: String(port), TEST_LOG: directory, TEST_PID: String(process.pid) }
  delete env.WOX_HOST_SECRET
  if (secret) env.WOX_HOST_SECRET = secret
  const child = spawn(process.execPath, ["-e", entry], { cwd: path.join(__dirname, ".."), env, stdio: ["ignore", "pipe", "pipe"] })
  child.diagnostics = ""
  child.stderr.on("data", data => { child.diagnostics += data })
  return child
}

function connect(port, headers = {}) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://127.0.0.1:${port}`, { headers })
    ws.once("open", () => resolve(ws))
    ws.once("error", reject)
  })
}

function close(ws) {
  return new Promise(resolve => {
    ws.once("close", resolve)
    ws.close()
  })
}

test("host rejects missing secrets at startup", { timeout: 20000 }, async t => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-host-security-"))
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }))
  const child = launch(0, directory)
  t.after(() => child.kill())
  const [code] = await new Promise(resolve => child.once("exit", (...args) => resolve(args)))
  assert.equal(code, 1)
})

test("host authenticates, blocks connection takeover, and permits reconnect", { timeout: 20000 }, async t => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "wox-host-security-"))
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }))
  const probe = net.createServer()
  await new Promise(resolve => probe.listen(0, "127.0.0.1", resolve))
  const port = probe.address().port
  await new Promise(resolve => probe.close(resolve))
  const child = launch(port, directory, "test-secret")
  t.after(async () => {
    if (child.exitCode === null) {
      child.kill()
      await new Promise(resolve => child.once("exit", resolve))
    }
  })

  let ready = false
  for (let i = 0; i < 150; i++) {
    try {
      await connect(port)
    } catch (error) {
      if (/Unexpected server response: (401|403)/.test(error.message)) { ready = true; break }
    }
    await new Promise(resolve => setTimeout(resolve, 30))
  }
  const logPath = path.join(directory, "node.log")
  const logs = fs.existsSync(logPath) ? fs.readFileSync(logPath, "utf8") : "no log"
  assert.equal(ready, true, `host did not start (exit ${child.exitCode}): ${child.diagnostics} ${logs}`)
  const externalAddress = Object.values(os.networkInterfaces()).flat().find(address => address && address.family === "IPv4" && !address.internal)
  if (externalAddress) {
    const reachable = await new Promise(resolve => {
      const socket = net.connect({ host: externalAddress.address, port })
      socket.setTimeout(1000)
      socket.once("connect", () => { socket.destroy(); resolve(true) })
      socket.once("error", () => resolve(false))
      socket.once("timeout", () => { socket.destroy(); resolve(false) })
    })
    assert.equal(reachable, false, "host must not listen on a non-loopback address")
  }
  await assert.rejects(connect(port, { "X-Wox-Host-Secret": "wrong" }))
  await assert.rejects(connect(port, { "X-Wox-Host-Secret": "test-secret", Origin: "https://example.com" }))
  const headers = { "X-Wox-Host-Secret": "test-secret" }
  const active = await connect(port, headers)
  await assert.rejects(connect(port, headers))

  // The authorized connection still loads a plugin from a user-selected directory.
  fs.writeFileSync(path.join(directory, "plugin.cjs"), "module.exports.plugin = {}\n")
  const response = new Promise(resolve => active.on("message", data => {
    const message = JSON.parse(String(data))
    if (message.Id === "test") resolve(message)
  }))
  active.send(JSON.stringify({
    Type: "WOX_JSONRPC_REQUEST", Id: "test", TraceId: "test", Method: "loadPlugin", PluginId: "test", PluginName: "test",
    Params: { PluginDirectory: directory, Entry: "plugin.cjs", EntryMode: "singleFile" }
  }))
  assert.equal((await response).Error, undefined)
  await close(active)

  let reconnected
  for (let i = 0; i < 100; i++) {
    try { reconnected = await connect(port, headers); break } catch { await new Promise(resolve => setTimeout(resolve, 20)) }
  }
  assert.ok(reconnected, "host did not accept reconnect")
  await close(reconnected)
})
