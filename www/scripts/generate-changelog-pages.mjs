import { assertReadmeLatest, generateChangelogPages, getLatestRelease } from "./release-meta.mjs";

generateChangelogPages();

if (process.argv.includes("--check-readme")) {
  assertReadmeLatest(getLatestRelease());
}
