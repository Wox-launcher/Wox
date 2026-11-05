import { assertReadmeLatest, generateChangelogPages, getLatestRelease, readGitHubReleaseSnapshot, refreshGitHubReleaseSnapshot } from "./release-meta.mjs";

// Historical tags stay available from github-releases.json when GitHub cannot be reached.
try {
  await refreshGitHubReleaseSnapshot();
} catch (error) {
  const saved = readGitHubReleaseSnapshot();
  console.warn(`Using saved GitHub releases (${saved.length}): ${error instanceof Error ? error.message : error}`);
}

generateChangelogPages();

if (process.argv.includes("--check-readme")) {
  assertReadmeLatest(getLatestRelease());
}
