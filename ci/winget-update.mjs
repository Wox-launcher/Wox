import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";

const repository = "Wox-launcher/Wox";
const packageId = "Wox.Wox";
const assetName = "wox-windows-amd64.exe";

function runGh(args) {
  const result = spawnSync("gh", args, { encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(result.stderr.trim() || `gh ${args.join(" ")} failed`);
  return result.stdout.trim();
}

try {
  const release = JSON.parse(runGh(["release", "view", "--repo", repository, "--json", "tagName,url,assets,body"]));
  const version = release.tagName.replace(/^v/, "");
  const asset = release.assets.find(({ name }) => name === assetName);
  if (!/^\d+\.\d+\.\d+$/.test(version) || !asset) {
    throw new Error(`Latest stable release ${release.tagName} has no ${assetName} asset`);
  }

  const manifestVersions = runGh([
    "api",
    "repos/microsoft/winget-pkgs/contents/manifests/w/Wox/Wox",
    "--jq",
    ".[].name",
  ]).split(/\r?\n/);
  if (manifestVersions.includes(version)) {
    console.log(`Wox ${version} is already present in winget.`);
    process.exit(0);
  }

  console.log(`Latest stable Wox version: ${version}`);
  console.log(`Installer: ${asset.url}`);
  console.log(`Release:   ${release.url}`);
  const wingetCreateCheck = spawnSync("wingetcreate", ["--help"], { stdio: "ignore" });
  const installWingetCreate = wingetCreateCheck.error?.code === "ENOENT";
  if (wingetCreateCheck.error && !installWingetCreate) throw wingetCreateCheck.error;
  if (installWingetCreate) console.log("wingetcreate is missing; it will be installed with winget before submission.");
  const terminal = createInterface({ input: stdin, output: stdout });
  const answer = await terminal.question("Type yes to continue with the winget update: ");
  terminal.close();
  if (answer.trim().toLowerCase() !== "yes") {
    console.log("Cancelled; no winget PR was submitted.");
    process.exit(0);
  }

  if (installWingetCreate) {
    const install = spawnSync(
      "winget",
      ["install", "wingetcreate", "--accept-source-agreements", "--accept-package-agreements"],
      { stdio: "inherit" },
    );
    if (install.error) throw install.error;
    if (install.status !== 0) throw new Error(`winget install wingetcreate failed with exit code ${install.status}`);
  }

  const token = runGh(["auth", "token"]);
  // Keep the GitHub token out of command-line arguments, where it may be logged.
  const env = { ...process.env, WINGET_CREATE_GITHUB_TOKEN: token };
  const outputDir = mkdtempSync(join(tmpdir(), "wox-winget-update-"));
  try {
    const generated = spawnSync(
      "wingetcreate",
      ["update", packageId, "--version", version, "--urls", asset.url, "--out", outputDir],
      { stdio: "inherit", env },
    );
    if (generated.error) throw generated.error;
    if (generated.status !== 0) throw new Error(`wingetcreate update failed with exit code ${generated.status}`);

    const manifestDir = join(outputDir, "manifests", "w", "Wox", "Wox", version);
    const localePath = join(manifestDir, "Wox.Wox.locale.en-US.yaml");
    const localeManifest = readFileSync(localePath, "utf8");
    if (!localeManifest.includes("ReleaseNotesUrl:")) {
      throw new Error("Generated locale manifest has no ReleaseNotesUrl field");
    }
    const notes = release.body.trim().replace(/\r\n/g, "\n");
    const notesYaml = `ReleaseNotes: |-${notes ? `\n${notes.split("\n").map((line) => `  ${line}`).join("\n")}` : ""}\n`;
    writeFileSync(localePath, localeManifest.replace("ReleaseNotesUrl:", `${notesYaml}ReleaseNotesUrl:`));

    const submitted = spawnSync(
      "wingetcreate",
      ["submit", "--prtitle", `New version: ${packageId} version ${version}`, manifestDir],
      { stdio: "inherit", env },
    );
    if (submitted.error) throw submitted.error;
    process.exitCode = submitted.status ?? 1;
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
} catch (error) {
  console.error(error.message);
  process.exit(1);
}
