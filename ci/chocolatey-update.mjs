import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { createReadStream, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline/promises";
import { fileURLToPath } from "node:url";

const repository = "Wox-launcher/Wox";
const assetName = "wox-windows-amd64.exe";
const pushSource = "https://push.chocolatey.org/";
const scriptDir = dirname(fileURLToPath(import.meta.url));

// Pass arguments directly to executables so paths never go through shell interpolation.
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: "utf8", ...options });
  if (result.error) throw result.error;
  if (result.status !== 0)
    throw new Error(result.stderr?.trim() || `${command} failed with exit code ${result.status}`);
  return result.stdout?.trim();
}

function escapeXml(value) {
  return value.replace(
    /[&<>"']/g,
    (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&apos;" })[character],
  );
}

// Render only known placeholders; unknown fields must not silently produce a broken package.
function renderTemplate(name, values) {
  return readFileSync(join(scriptDir, "chocolatey", name), "utf8").replace(/@@([A-Z0-9_]+)@@/g, (_, key) => {
    if (!(key in values)) throw new Error(`Unknown template field: ${key}`);
    return values[key];
  });
}

try {
  const args = new Set(process.argv.slice(2));
  for (const arg of args) {
    if (!["--dry-run", "--pack-only", "--yes", "--help"].includes(arg)) throw new Error(`Unknown argument: ${arg}`);
  }
  if (args.has("--help")) {
    console.log(
      "Usage: make chocolatey-update CHOCOLATEY_ARGS='[--dry-run | --pack-only] [--yes]'\n--dry-run: generate sources without Chocolatey or submission\n--pack-only: build a local nupkg without submission\n--yes: submit without the interactive confirmation",
    );
    process.exit(0);
  }
  if (args.has("--dry-run") && args.has("--pack-only")) throw new Error("Choose either --dry-run or --pack-only");

  const release = JSON.parse(
    run("gh", ["release", "view", "--repo", repository, "--json", "tagName,url,assets,isDraft,isPrerelease"]),
  );
  const version = release.tagName.replace(/^v/, "");
  const asset = release.assets.find(({ name }) => name === assetName);
  if (
    release.isDraft ||
    release.isPrerelease ||
    !/^\d+\.\d+\.\d+$/.test(version) ||
    !asset ||
    asset.state !== "uploaded"
  ) {
    throw new Error(`Latest stable release ${release.tagName} has no completed ${assetName} asset`);
  }
  const installerUrl = `https://github.com/${repository}/releases/download/${release.tagName}/${assetName}`;
  if (asset.url !== installerUrl) throw new Error("Unexpected GitHub installer URL");

  // Query the exact version, including packages awaiting moderation, before attempting a duplicate push.
  const response = await fetch(`https://community.chocolatey.org/api/v2/Packages(Id='wox',Version='${version}')`, {
    signal: AbortSignal.timeout(30000),
  });
  if (response.ok) {
    if (!/<entry[\s>]/.test(await response.text()))
      throw new Error("Chocolatey returned an unexpected package response");
    console.log(`Wox ${version} is already present in Chocolatey; check its moderation status on the package page.`);
    process.exit(0);
  }
  if (response.status !== 404) throw new Error(`Unable to check Chocolatey package: HTTP ${response.status}`);

  if (!args.has("--dry-run")) run("choco", ["--version"]);
  const outputDir = join(scriptDir, "..", "release", "chocolatey", version);
  const downloadDir = mkdtempSync(join(tmpdir(), "wox-chocolatey-download-"));
  let checksum;
  try {
    run("gh", [
      "release",
      "download",
      release.tagName,
      "--repo",
      repository,
      "--pattern",
      assetName,
      "--dir",
      downloadDir,
    ]);
    const hash = createHash("sha256");
    for await (const chunk of createReadStream(join(downloadDir, assetName))) hash.update(chunk);
    checksum = hash.digest("hex");
    if (asset.digest && asset.digest !== `sha256:${checksum}`)
      throw new Error("Downloaded installer does not match the GitHub SHA256 digest");
  } finally {
    rmSync(downloadDir, { recursive: true, force: true });
  }

  const values = {
    VERSION: version,
    TAG: escapeXml(release.tagName),
    RELEASE_URL: escapeXml(release.url),
    INSTALLER_URL: installerUrl,
    SHA256: checksum,
  };
  mkdirSync(join(outputDir, "tools"), { recursive: true });
  writeFileSync(join(outputDir, "wox.nuspec"), renderTemplate("wox.nuspec", values));
  writeFileSync(join(outputDir, "tools", "chocolateyInstall.ps1"), renderTemplate("chocolateyInstall.ps1", values));
  writeFileSync(join(outputDir, "tools", "chocolateyUninstall.ps1"), renderTemplate("chocolateyUninstall.ps1", values));

  console.log(
    `Latest stable Wox version: ${version}\nInstaller: ${installerUrl}\nSHA256:    ${checksum}\nSources:   ${outputDir}`,
  );
  if (args.has("--dry-run")) {
    console.log("Dry run complete; no package was submitted.");
    process.exit(0);
  }

  const packagePath = join(outputDir, `wox.${version}.nupkg`);
  // Never submit a stale artifact if a previous packaging run left one behind.
  rmSync(packagePath, { force: true });
  run("choco", ["pack", join(outputDir, "wox.nuspec"), `--output-directory=${outputDir}`], { stdio: "inherit" });
  if (!existsSync(packagePath)) throw new Error(`Chocolatey did not create ${packagePath}`);
  console.log(`Package: ${packagePath}`);
  if (args.has("--pack-only")) process.exit(0);

  if (!args.has("--yes")) {
    if (!stdin.isTTY)
      throw new Error("Non-interactive submission requires --yes; use --pack-only to build without submitting");
    const terminal = createInterface({ input: stdin, output: stdout });
    let answer;
    try {
      answer = await terminal.question(`Type yes to submit Wox ${version} to ${pushSource}: `);
    } finally {
      terminal.close();
    }
    if (answer.trim().toLowerCase() !== "yes") {
      console.log("Cancelled; the local package is available for review.");
      process.exit(0);
    }
  }

  // Use Chocolatey's saved API key; do not expose credentials in this script's arguments or output.
  run("choco", ["push", packagePath, `--source=${pushSource}`], { stdio: "inherit" });
  console.log(`Submitted Wox ${version}. Moderation status: https://community.chocolatey.org/packages/wox/${version}`);
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
