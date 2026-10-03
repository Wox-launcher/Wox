import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));

// Run the real CLI with isolated command/network boundaries; no test can publish or download a package.
function runCase(t, options = {}, args = []) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "wox-chocolatey-test-")));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  // The CLI writes ../release relative to its script. Keep that path inside this fixture.
  const ci = join(root, "ci");
  mkdirSync(ci);
  cpSync(join(scriptDir, "chocolatey-update.mjs"), join(ci, "chocolatey-update.mjs"));
  cpSync(join(scriptDir, "chocolatey"), join(ci, "chocolatey"), { recursive: true });
  writeFileSync(join(root, "options.json"), JSON.stringify(options));
  writeFileSync(join(root, "calls.json"), "[]");
  const preload = join(root, "mock.mjs");
  writeFileSync(
    preload,
    `
import childProcess from 'node:child_process';
import { syncBuiltinESMExports } from 'node:module';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
const options = JSON.parse(readFileSync('options.json', 'utf8'));
const bytes = Buffer.from('fixture executable');
const assetName = 'wox-windows-amd64.exe';
const url = 'https://github.com/Wox-launcher/Wox/releases/download/v2.4.5/' + assetName;
childProcess.spawnSync = (command, args) => {
  const calls = JSON.parse(readFileSync('calls.json', 'utf8'));
  calls.push([command, ...args]);
  writeFileSync('calls.json', JSON.stringify(calls));
  let stdout = '';
  if (command === 'gh' && args[1] === 'view') {
    stdout = JSON.stringify({ tagName: 'v2.4.5', url: 'https://github.com/Wox-launcher/Wox/releases/tag/v2.4.5',
      isDraft: false, isPrerelease: options.prerelease ?? false,
      assets: options.missingAsset ? [] : [{ name: assetName, url, state: 'uploaded',
        digest: 'sha256:' + (options.badDigest ? '0'.repeat(64) : createHash('sha256').update(bytes).digest('hex')) }] });
  } else if (command === 'gh' && args[1] === 'download') {
    writeFileSync(join(args[args.indexOf('--dir') + 1], assetName), bytes);
  } else if (command === 'choco' && args[0] === 'pack' && !options.packFailure && !options.missingPackage) {
    writeFileSync(join(args.find(arg => arg.startsWith('--output-directory=')).split('=')[1], 'wox.2.4.5.nupkg'), 'fixture package');
  } else if (!(command === 'choco' && ['--version', 'pack', 'push'].includes(args[0]))) {
    throw new Error('Unexpected command: ' + command);
  }
  const failed = command === 'choco' && ((args[0] === 'pack' && options.packFailure) || (args[0] === 'push' && options.pushFailure));
  return { status: failed ? 1 : 0, stdout, stderr: failed ? 'fixture command failure' : '' };
};
syncBuiltinESMExports();
globalThis.fetch = async () => new Response(options.responseBody ?? '<entry xmlns="http://www.w3.org/2005/Atom" />', { status: options.httpStatus ?? 404 });
`,
  );
  const result = spawnSync(process.execPath, ["--import", preload, join(ci, "chocolatey-update.mjs"), ...args], {
    cwd: root,
    encoding: "utf8",
  });
  if (result.error) throw result.error;
  const calls = JSON.parse(readFileSync(join(root, "calls.json"), "utf8"));
  return { ...result, calls, outputDir: join(root, "release", "chocolatey", "2.4.5") };
}

test("dry run generates pinned sources without running Chocolatey", (t) => {
  const result = runCase(t, {}, ["--dry-run"]);
  assert.equal(result.status, 0, result.stderr);
  assert.ok(result.calls.every(([command]) => command === "gh"));
  const nuspec = readFileSync(join(result.outputDir, "wox.nuspec"), "utf8");
  const installer = readFileSync(join(result.outputDir, "tools", "chocolateyInstall.ps1"), "utf8");
  assert.match(nuspec, /<version>2\.4\.5<\/version>/);
  assert.match(installer, /releases\/download\/v2\.4\.5\/wox-windows-amd64\.exe/);
  assert.match(installer, /-Checksum64 '[a-f0-9]{64}'/);
  assert.doesNotMatch(nuspec + installer, /@@[A-Z0-9_]+@@/);
});

test("an existing version is skipped before downloading or packing", (t) => {
  const result = runCase(t, { httpStatus: 200 }, ["--yes"]);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.calls.length, 1);
  assert.match(result.stdout, /already present/);
});

for (const [name, options] of [
  ["feed failure", { httpStatus: 503 }],
  ["HTML instead of package metadata", { httpStatus: 200, responseBody: "<html>Blocked</html>" }],
  ["prerelease", { prerelease: true }],
  ["missing installer", { missingAsset: true }],
  ["checksum mismatch", { badDigest: true }],
  ["pack failure", { packFailure: true }],
  ["missing generated package", { missingPackage: true }],
]) {
  test(`${name} prevents submission`, (t) => {
    const result = runCase(t, options, ["--yes"]);
    assert.notEqual(result.status, 0);
    assert.ok(!result.calls.some(([command, action]) => command === "choco" && action === "push"));
  });
}

test("pack-only retains a local package without submitting", (t) => {
  const result = runCase(t, {}, ["--pack-only"]);
  assert.equal(result.status, 0, result.stderr);
  assert.ok(existsSync(join(result.outputDir, "wox.2.4.5.nupkg")));
  assert.ok(!result.calls.some(([command, action]) => command === "choco" && action === "push"));
});

test("non-interactive invocation requires explicit --yes to submit", (t) => {
  const result = runCase(t);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /requires --yes/);
  assert.ok(!result.calls.some(([command, action]) => command === "choco" && action === "push"));
});

test("--yes submits the generated package to the fixed community feed", (t) => {
  const result = runCase(t, {}, ["--yes"]);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(result.calls.at(-1), [
    "choco",
    "push",
    join(result.outputDir, "wox.2.4.5.nupkg"),
    "--source=https://push.chocolatey.org/",
  ]);
});

test("push failure returns failure without reporting success", (t) => {
  const result = runCase(t, { pushFailure: true }, ["--yes"]);
  assert.notEqual(result.status, 0);
  assert.doesNotMatch(result.stdout, /Submitted Wox/);
});
