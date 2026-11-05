import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(SCRIPT_DIR, "../..");
const CHANGELOG_PATH = path.join(REPO_ROOT, "CHANGELOG.md");
const ZH_CHANGELOG_PATH = path.join(REPO_ROOT, "CHANGELOG.zh_CN.md");
const README_PATH = path.join(REPO_ROOT, "README.md");
const EN_CHANGELOG_DIR = path.join(REPO_ROOT, "www/docs/changelog");
const ZH_CHANGELOG_DIR = path.join(REPO_ROOT, "www/docs/zh/changelog");
const GITHUB_RELEASES_PATH = path.join(SCRIPT_DIR, "github-releases.json");
const GITHUB_RELEASES_ZH_PATH = path.join(SCRIPT_DIR, "github-releases.zh_CN.json");
const GITHUB_RELEASES_API = "https://api.github.com/repos/Wox-launcher/Wox/releases";

const STABLE_HEADING = /^## v(\d+\.\d+\.\d+) - (\d{4}-\d{2}-\d{2})\s*$/;
const MONTH_NAMES_EN = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];
const MONTH_SHORT_EN = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function changelogPath() {
  return CHANGELOG_PATH;
}

export function changelogZhPath() {
  return ZH_CHANGELOG_PATH;
}

export function formatDateParts(isoDate) {
  const [yearText, monthText, dayText] = isoDate.split("-");
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  return {
    year,
    month,
    day,
    monthNameEn: MONTH_NAMES_EN[month - 1],
    monthNameEnShort: MONTH_SHORT_EN[month - 1],
    dateLongEn: `${day} ${MONTH_NAMES_EN[month - 1]} ${year}`,
    dateShortEn: `${MONTH_SHORT_EN[month - 1]} ${year}`,
    dateLongZh: `${year} 年 ${month} 月 ${day} 日`,
    dateMonthZh: `${year} 年 ${month} 月`,
  };
}

export function publicReleaseMeta(release) {
  return {
    version: release.version,
    tag: release.tag,
    date: release.date,
    year: release.year,
    month: release.month,
    day: release.day,
    monthNameEn: release.monthNameEn,
    monthNameEnShort: release.monthNameEnShort,
    dateLongEn: release.dateLongEn,
    dateShortEn: release.dateShortEn,
    dateLongZh: release.dateLongZh,
    dateMonthZh: release.dateMonthZh,
    highlight: release.highlight,
    githubReleaseUrl: `https://github.com/Wox-launcher/Wox/releases/tag/${release.tag}`,
    changelogUrl: `/changelog/${release.version}`,
  };
}

/** Parse stable `## vX.Y.Z - YYYY-MM-DD` sections from CHANGELOG.md, skipping betas. */
export function parseChangelog(markdown = fs.readFileSync(CHANGELOG_PATH, "utf8")) {
  const lines = markdown.split(/\r?\n/);
  const releases = [];
  let current = null;
  let bodyLines = [];

  const flush = () => {
    if (!current) {
      return;
    }
    const rawBody = bodyLines.join("\n").replace(/^\n+/, "").replace(/\n+$/, "");
    const { highlight, rest } = splitHighlight(rawBody);
    releases.push({
      ...current,
      highlight,
      body: rest,
    });
    current = null;
    bodyLines = [];
  };

  for (const line of lines) {
    const match = line.match(STABLE_HEADING);
    if (match) {
      flush();
      current = {
        version: match[1],
        tag: `v${match[1]}`,
        date: match[2],
        ...formatDateParts(match[2]),
      };
      continue;
    }
    if (!current) {
      continue;
    }
    if (line.startsWith("## ")) {
      flush();
      continue;
    }
    bodyLines.push(line);
  }
  flush();
  return releases;
}

export function getStableReleases() {
  return parseChangelog();
}

/** Parse stable sections from CHANGELOG.zh_CN.md when the file exists. */
export function getChineseReleases() {
  if (!fs.existsSync(ZH_CHANGELOG_PATH)) {
    return [];
  }
  return parseChangelog(fs.readFileSync(ZH_CHANGELOG_PATH, "utf8"));
}

function chineseReleaseMap(releases = getChineseReleases()) {
  return new Map(releases.map((release) => [release.version, release]));
}

export function getLatestRelease() {
  const releases = getStableReleases();
  if (releases.length === 0) {
    throw new Error(`No stable releases found in ${CHANGELOG_PATH}`);
  }
  return publicReleaseMeta(releases[0]);
}

export function assertReadmeLatest(latest = getLatestRelease()) {
  const readme = fs.readFileSync(README_PATH, "utf8");
  const needle = `Latest: ${latest.tag} · ${latest.monthNameEnShort} ${latest.year}`;
  if (!readme.includes(needle)) {
    throw new Error(`README.md is missing "${needle}". Update it when cutting a stable release.`);
  }
}

/** Load the saved GitHub release list used when the API cannot be reached. */
export function readGitHubReleaseSnapshot() {
  if (!fs.existsSync(GITHUB_RELEASES_PATH)) {
    return [];
  }
  const parsed = JSON.parse(fs.readFileSync(GITHUB_RELEASES_PATH, "utf8"));
  if (!Array.isArray(parsed)) {
    throw new Error(`Invalid GitHub release snapshot: ${GITHUB_RELEASES_PATH}`);
  }
  return parsed;
}

/** Refresh the saved Wox release list from GitHub so new tags stay in the changelog navigation. */
export async function refreshGitHubReleaseSnapshot() {
  let releases;
  try {
    releases = await fetchGitHubReleaseList();
  } catch (error) {
    // Unauthenticated REST calls are easy to rate-limit. A logged-in gh CLI can still read public releases.
    releases = readGitHubReleasesWithGh();
    if (!releases) {
      throw error;
    }
  }
  if (releases.length === 0) {
    throw new Error("GitHub releases response was empty");
  }
  releases.sort((left, right) => right.publishedAt.localeCompare(left.publishedAt) || right.tag.localeCompare(left.tag));
  writeIfChanged(GITHUB_RELEASES_PATH, `${JSON.stringify(releases, null, 2)}\n`);
  return releases;
}

function githubRequestHeaders() {
  const headers = {
    Accept: "application/vnd.github+json",
    "User-Agent": "wox-docs",
    "X-GitHub-Api-Version": "2022-11-28",
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

function normalizeGitHubReleases(payload) {
  if (!Array.isArray(payload)) {
    throw new Error("GitHub releases response was not a list");
  }
  const releases = [];
  const seen = new Set();
  for (const release of payload) {
    if (release.draft || !release.tag_name || !release.published_at) {
      continue;
    }
    if (seen.has(release.tag_name)) {
      continue;
    }
    seen.add(release.tag_name);
    releases.push({
      tag: release.tag_name,
      publishedAt: release.published_at,
      body: release.body || "",
    });
  }
  return releases;
}

async function fetchGitHubReleaseList() {
  const pages = [];
  for (let page = 1; page <= 10; page += 1) {
    const response = await fetch(`${GITHUB_RELEASES_API}?per_page=100&page=${page}`, {
      headers: githubRequestHeaders(),
      signal: AbortSignal.timeout(15000),
    });
    if (!response.ok) {
      throw new Error(`GitHub releases request failed (${response.status})`);
    }
    const payload = await response.json();
    if (!Array.isArray(payload)) {
      throw new Error("GitHub releases response was not a list");
    }
    pages.push(...payload);
    if (payload.length < 100) {
      break;
    }
  }
  return normalizeGitHubReleases(pages);
}

function readGitHubReleasesWithGh() {
  try {
    const output = execFileSync("gh", ["api", "--paginate", "--slurp", "repos/Wox-launcher/Wox/releases?per_page=100"], {
      encoding: "utf8",
      maxBuffer: 20 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"],
    });
    return normalizeGitHubReleases(JSON.parse(output).flat());
  } catch {
    return null;
  }
}

function releaseIdentity(tag) {
  return tag.replace(/^v/i, "").toLowerCase();
}

function displayTag(tag) {
  return /^v/i.test(tag) ? `v${tag.slice(1)}` : `v${tag}`;
}

function releaseAnchor(version) {
  return `release-v${version.replaceAll(".", "-")}`;
}

/** Merge local changelog entries with every GitHub release, newest first. */
export function buildReleaseNavigation(documentedReleases, githubReleases = readGitHubReleaseSnapshot()) {
  const items = new Map();
  for (const release of githubReleases) {
    const date = String(release.publishedAt || "").slice(0, 10);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !release.tag) {
      continue;
    }
    items.set(releaseIdentity(release.tag), {
      tag: displayTag(release.tag),
      year: Number(date.slice(0, 4)),
      date,
      href: `#${releaseAnchor(releaseIdentity(release.tag))}`,
    });
  }
  for (const release of documentedReleases) {
    items.set(releaseIdentity(release.version), {
      tag: `v${release.version}`,
      year: release.year,
      date: release.date,
      href: `#${releaseAnchor(release.version)}`,
    });
  }
  return [...items.values()].sort((left, right) => right.date.localeCompare(left.date) || right.tag.localeCompare(left.tag, "en", { numeric: true }));
}

/** Write per-version changelog pages and language indexes from CHANGELOG.md and CHANGELOG.zh_CN.md. */
export function generateChangelogPages() {
  const releases = getStableReleases();
  if (releases.length === 0) {
    throw new Error(`No stable releases found in ${CHANGELOG_PATH}`);
  }
  const githubReleases = readGitHubReleaseSnapshot();
  const githubChinese = JSON.parse(fs.readFileSync(GITHUB_RELEASES_ZH_PATH, "utf8"));
  const navigation = buildReleaseNavigation(releases, githubReleases).map(({ tag, year, href }) => ({ tag, year, href }));
  const localByIdentity = new Map(releases.map((release) => [releaseIdentity(release.tag), release]));
  const githubByIdentity = new Map(githubReleases.map((release) => [releaseIdentity(release.tag), release]));
  const feed = navigation.map((item) => {
    const identity = releaseIdentity(item.tag);
    const local = localByIdentity.get(identity);
    if (local) return local;
    const github = githubByIdentity.get(identity);
    const date = github.publishedAt.slice(0, 10);
    return { version: identity, tag: item.tag, githubTag: github.tag, date, ...formatDateParts(date), body: github.body || "", github: true };
  });

  const chineseByVersion = chineseReleaseMap();
  fs.mkdirSync(EN_CHANGELOG_DIR, { recursive: true });
  fs.mkdirSync(ZH_CHANGELOG_DIR, { recursive: true });

  const expectedEnglish = new Set(["index.md"]);
  const expectedChinese = new Set(["index.md"]);
  for (const release of releases) {
    const fileName = `${release.version}.md`;
    expectedEnglish.add(fileName);
    writeIfChanged(path.join(EN_CHANGELOG_DIR, fileName), renderVersionPage(release));

    const chineseRelease = chineseByVersion.get(release.version);
    if (chineseRelease) {
      expectedChinese.add(fileName);
      writeIfChanged(path.join(ZH_CHANGELOG_DIR, fileName), renderChineseVersionPage(chineseRelease));
    }
  }
  writeIfChanged(path.join(EN_CHANGELOG_DIR, "index.md"), renderEnglishIndex(feed, navigation, releases[0]));
  writeIfChanged(path.join(ZH_CHANGELOG_DIR, "index.md"), renderChineseIndex(feed, chineseByVersion, navigation, releases[0], githubChinese, githubByIdentity));

  removeStaleMarkdown(EN_CHANGELOG_DIR, expectedEnglish);
  removeStaleMarkdown(ZH_CHANGELOG_DIR, expectedChinese);
}

function splitHighlight(body) {
  const lines = body.split("\n");
  let index = 0;
  while (index < lines.length && lines[index].trim() === "") {
    index += 1;
  }

  const highlightLines = [];
  while (index < lines.length) {
    const trimmed = lines[index].trim();
    if (trimmed === "" || trimmed.startsWith("- ") || trimmed.startsWith("!") || trimmed.startsWith("#")) {
      break;
    }
    highlightLines.push(trimmed);
    index += 1;
  }
  while (index < lines.length && lines[index].trim() === "") {
    index += 1;
  }

  const rest = lines.slice(index).join("\n").replace(/\n+$/, "");
  let highlight = highlightLines.join(" ");
  if (!highlight) {
    const bullet = rest.match(/^\s{2,}- \s*(.+)$/m);
    highlight = bullet ? stripMarkdown(bullet[1]) : `Stable release notes for this version.`;
  }
  return { highlight, rest };
}

function stripMarkdown(text) {
  return text
    .replace(/!\[.*?\]\(.*?\)/g, "")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/`+/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function seoDescription(text) {
  const oneLine = text.replace(/\s+/g, " ").trim();
  if (oneLine.length <= 160) {
    return oneLine;
  }
  return `${oneLine.slice(0, 157).replace(/\s+\S*$/, "")}...`;
}

function renderVersionPage(release) {
  const description = seoDescription(`${release.highlight} (${release.dateShortEn})`);
  const body = release.body ? `\n\n${release.body}\n` : "\n";
  return `---
title: ${JSON.stringify(`Wox ${release.version}`)}
description: ${JSON.stringify(description)}
lastUpdated: ${release.date}
---

<!-- Generated from CHANGELOG.md. Do not edit. -->

# Wox ${release.version}

**${release.dateLongEn}** · [GitHub Release ${release.tag}](https://github.com/Wox-launcher/Wox/releases/tag/${release.tag})

${release.highlight}
${body}`;
}

function renderChineseVersionPage(release) {
  const description = seoDescription(`${release.highlight}（${release.dateMonthZh}）`);
  const body = release.body ? `\n\n${release.body}\n` : "\n";
  return `---
title: ${JSON.stringify(`Wox ${release.version}`)}
description: ${JSON.stringify(description)}
lastUpdated: ${release.date}
---

<!-- Generated from CHANGELOG.zh_CN.md. Do not edit. -->

# Wox ${release.version}

**${release.dateLongZh}** · [GitHub Release ${release.tag}](https://github.com/Wox-launcher/Wox/releases/tag/${release.tag})

${release.highlight}
${body}`;
}

function renderEnglishIndex(releases, navigation, latest) {
  return `---
title: "Changelog"
description: ${JSON.stringify(`Wox release notes. Latest ${latest.tag} (${latest.dateShortEn}).`)}
lastUpdated: ${latest.date}
---

<!-- Generated from CHANGELOG.md. Do not edit. -->

# Changelog

${renderReleaseFeed(releases, navigation, false)}
`;
}

function renderChineseIndex(releases, chineseByVersion, navigation, latest, githubChinese, githubByIdentity) {
  return `---
title: "更新日志"
description: ${JSON.stringify(`Wox 正式版更新记录。最新 ${latest.tag}（${latest.dateMonthZh}）。`)}
lastUpdated: ${latest.date}
---

<!-- Generated from CHANGELOG.md and CHANGELOG.zh_CN.md. Do not edit. -->

# 更新日志

${renderReleaseFeed(releases, navigation, true, chineseByVersion, githubChinese, githubByIdentity)}
`;
}

/** Keep release content in Markdown so VitePress renders it into searchable static HTML. */
function renderReleaseFeed(releases, navigation, chinese, chineseByVersion = new Map(), githubChinese = {}, githubByIdentity = new Map()) {
  const cards = releases.map((release, index) => {
    const translated = chineseByVersion.get(release.version);
    const github = githubByIdentity.get(releaseIdentity(release.tag));
    const githubTranslation = github && githubChinese[github.tag];
    // A changed GitHub release must not be paired with a stale translation.
    const translatedBody = githubTranslation && githubTranslation.sourceSha256 === createHash("sha256").update(github.body || "").digest("hex") ? githubTranslation.body : "";
    const content = chinese ? translated || release : release;
    const page = chinese && !translated ? `/changelog/${release.version}` : `./${release.version}`;
    const id = releaseAnchor(release.version);
    const latest = index === 0 && !release.github ? `<span class="release-badge">${chinese ? "最新版本" : "Latest"}</span>` : "";
    const fallback = chinese && !translated && !translatedBody && (!release.github || release.body.trim())
      ? `\n\n> 此版本暂无中文说明，以下为${release.github ? " GitHub" : "英文"}原文。`
      : "";
    const title = release.github ? `Wox ${release.tag}` : `<a href="${page}">Wox ${release.tag}</a>`;
    const body = chinese && !translated && translatedBody
      ? translatedBody
      : release.github ? (release.body || (chinese ? "此版本暂无发布说明。" : "No release notes were provided.")) : `${content.highlight}\n\n${content.body}`;
    return `<section class="release-card" id="${id}" data-release="${release.tag}" tabindex="-1" aria-labelledby="${id}-title">
<header class="release-header">
<div class="release-title-row">
<h2 id="${id}-title">${title}</h2>
${latest}
</div>
<div class="release-meta">
<time datetime="${release.date}">${chinese ? release.dateLongZh : release.dateLongEn}</time>
<a href="https://github.com/Wox-launcher/Wox/releases/tag/${encodeURIComponent(release.githubTag || release.tag)}">GitHub Release ↗</a>
</div>
</header>
${fallback}

${renderFeedMedia(body, chinese)}

</section>`;
  });
  return `<script setup>
import ChangelogIndex from "../${chinese ? "../" : ""}.vitepress/theme/components/ChangelogIndex.vue";
</script>

<ChangelogIndex :releases='${JSON.stringify(navigation)}' :chinese="${chinese}">

${cards.join("\n\n")}

</ChangelogIndex>`;
}

/** Release feeds defer large media and render Markdown-linked MP4 files as playable videos. */
function renderFeedMedia(markdown, chinese) {
  return markdown.replace(/^([ \t]*)!\[([^\]]*)\]\((https?:\/\/[^\s)]+)\)[ \t]*$/gm, (_, indent, alt, url) => {
    const escapedUrl = escapeHtmlAttribute(url);
    const escapedAlt = escapeHtmlAttribute(alt);
    if (/\.mp4(?:\?|$)/i.test(url)) {
      const label = escapedAlt || (chinese ? "版本功能演示" : "Release feature demonstration");
      return `${indent}<video src="${escapedUrl}" aria-label="${label}" controls playsinline preload="none"></video>`;
    }
    return `${indent}<img src="${escapedUrl}" alt="${escapedAlt}" loading="lazy" decoding="async" />`;
  });
}

function escapeHtmlAttribute(value) {
  return value.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}

function writeIfChanged(filePath, content) {
  if (fs.existsSync(filePath) && fs.readFileSync(filePath, "utf8") === content) {
    return false;
  }
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, content);
  return true;
}

function removeStaleMarkdown(directory, expectedNames) {
  if (!fs.existsSync(directory)) {
    return;
  }
  for (const name of fs.readdirSync(directory)) {
    if (!name.endsWith(".md") || expectedNames.has(name)) {
      continue;
    }
    fs.unlinkSync(path.join(directory, name));
  }
}
