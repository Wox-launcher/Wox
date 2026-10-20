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

/** Write per-version changelog pages and language indexes from CHANGELOG.md and CHANGELOG.zh_CN.md. */
export function generateChangelogPages() {
  const releases = getStableReleases();
  if (releases.length === 0) {
    throw new Error(`No stable releases found in ${CHANGELOG_PATH}`);
  }

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
  writeIfChanged(path.join(EN_CHANGELOG_DIR, "index.md"), renderEnglishIndex(releases));
  writeIfChanged(path.join(ZH_CHANGELOG_DIR, "index.md"), renderChineseIndex(releases, chineseByVersion));

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

function renderEnglishIndex(releases) {
  const latest = releases[0];
  return `---
title: "Changelog"
description: ${JSON.stringify(`Wox release notes. Latest ${latest.tag} (${latest.dateShortEn}).`)}
lastUpdated: ${latest.date}
---

<!-- Generated from CHANGELOG.md. Do not edit. -->

# Changelog

${renderReleaseFeed(releases, false)}
`;
}

function renderChineseIndex(releases, chineseByVersion) {
  const latest = releases[0];
  return `---
title: "更新日志"
description: ${JSON.stringify(`Wox 正式版更新记录。最新 ${latest.tag}（${latest.dateMonthZh}）。`)}
lastUpdated: ${latest.date}
---

<!-- Generated from CHANGELOG.md and CHANGELOG.zh_CN.md. Do not edit. -->

# 更新日志

${renderReleaseFeed(releases, true, chineseByVersion)}
`;
}

/** Keep release content in Markdown so VitePress renders it into searchable static HTML. */
function renderReleaseFeed(releases, chinese, chineseByVersion = new Map()) {
  const cards = releases.map((release, index) => {
    const translated = chineseByVersion.get(release.version);
    const content = chinese ? translated || release : release;
    const page = chinese && !translated ? `/changelog/${release.version}` : `./${release.version}`;
    const id = `release-v${release.version.replaceAll(".", "-")}`;
    const latest = index === 0 ? `<span class="release-badge">${chinese ? "最新版本" : "Latest"}</span>` : "";
    const fallback = chinese && !translated ? "\n\n> 此版本暂无中文说明，以下为英文原文。" : "";
    return `<section class="release-card" id="${id}" tabindex="-1" aria-labelledby="${id}-title">
<header class="release-header">
<div class="release-title-row">
<h2 id="${id}-title"><a href="${page}">Wox ${release.tag}</a></h2>
${latest}
</div>
<div class="release-meta">
<time datetime="${release.date}">${chinese ? release.dateLongZh : release.dateLongEn}</time>
<a href="https://github.com/Wox-launcher/Wox/releases/tag/${release.tag}">GitHub Release ↗</a>
</div>
</header>
${fallback}

${renderFeedMedia(`${content.highlight}\n\n${content.body}`, chinese)}

</section>`;
  });
  return `<script setup>
import ChangelogIndex from "../${chinese ? "../" : ""}.vitepress/theme/components/ChangelogIndex.vue";
</script>

<ChangelogIndex :versions='${JSON.stringify(releases.map((release) => release.version))}' :chinese="${chinese}">

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
