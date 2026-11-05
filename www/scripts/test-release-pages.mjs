import assert from "node:assert/strict";
import fs from "node:fs";
import { test } from "node:test";
import { buildReleaseNavigation, generateChangelogPages, getStableReleases, readGitHubReleaseSnapshot } from "./release-meta.mjs";

test("release navigation prefers local notes and links historical tags to cards", () => {
  const local = [{ version: "2.0.0", date: "2026-01-01", year: 2026 }];
  const github = [
    { tag: "v2.0.0", publishedAt: "2026-01-02T00:00:00Z", body: "GitHub notes" },
    { tag: "1.0.0-beta", publishedAt: "2014-01-01T00:00:00Z", body: "Historical notes" },
  ];
  assert.deepEqual(buildReleaseNavigation(local, github), [
    { tag: "v2.0.0", year: 2026, date: "2026-01-01", href: "#release-v2-0-0" },
    { tag: "v1.0.0-beta", year: 2014, date: "2014-01-01", href: "#release-v1-0-0-beta" },
  ]);
});

test("both generated feeds include historical notes and a card for each navigation entry", () => {
  generateChangelogPages();
  const snapshot = readGitHubReleaseSnapshot();
  const navigation = buildReleaseNavigation(getStableReleases(), snapshot);
  const historical = snapshot.find((release) => release.tag === "v1.0-beta");
  assert.ok(historical?.body.includes("Most function is ready for use"));
  for (const url of ["../docs/changelog/index.md", "../docs/zh/changelog/index.md"]) {
    const feed = fs.readFileSync(new URL(url, import.meta.url), "utf8");
    for (const item of navigation) {
      assert.ok(feed.includes(`id="${item.href.slice(1)}"`), `Missing ${item.tag} in ${url}`);
    }
    assert.ok(feed.includes(url.includes("/zh/") ? "大多数功能可供使用" : "Most function is ready for use"));
    assert.ok(feed.includes("https://github.com/Wox-launcher/Wox/releases/tag/1.3.475"));
  }
});

test("historical Chinese translations preserve release links and code", () => {
  const snapshot = readGitHubReleaseSnapshot();
  const translations = JSON.parse(fs.readFileSync(new URL("./github-releases.zh_CN.json", import.meta.url), "utf8"));
  const protectedContent = /https?:\/\/[^\s)<>]+|`+[^`\n]*`+|<[^\n>]+>/g;
  for (const release of snapshot) {
    const translation = translations[release.tag];
    if (!translation) continue;
    assert.ok(translation.body.trim(), `Missing translation for ${release.tag}`);
    assert.deepEqual((translation.body.match(protectedContent) || []).sort(), (release.body.match(protectedContent) || []).sort(), release.tag);
  }
});
