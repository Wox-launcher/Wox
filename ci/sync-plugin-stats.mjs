import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const catalogPath = new URL("../store-plugin.json", import.meta.url);

// Prefer the declared website, falling back to the download URL for plugins with another homepage.
export function githubSource(plugin) {
  for (const value of [plugin.Website, plugin.DownloadUrl]) {
    let url;
    try { url = new URL(value); } catch { continue; }
    if (url.protocol !== "https:") continue;
    const parts = url.pathname.split("/").filter(Boolean);
    if (url.hostname === "github.com" && parts.length >= 2 && /^[\w.-]+$/.test(parts[0]) && /^[\w.-]+$/.test(parts[1])) {
      return { type: "repository", owner: parts[0], name: parts[1].replace(/\.git$/, "") };
    }
    if ((url.hostname === "gist.github.com" || url.hostname === "gist.githubusercontent.com") && parts.length >= 2 && /^[\w-]+$/.test(parts[0]) && /^[a-f\d]+$/i.test(parts[1])) {
      return { type: "gist", owner: parts[0], name: parts[1] };
    }
  }
  return null;
}

// Fetch counts in batches; inaccessible sources stay unknown rather than appearing to have zero stars.
export async function fetchPluginStars(plugins, token, fetchImpl = fetch) {
  const sources = plugins.map((plugin) => ({ id: plugin.Id, source: githubSource(plugin) })).filter((item) => item.source);
  const stars = {};
  for (let offset = 0; offset < sources.length; offset += 25) {
    const batch = sources.slice(offset, offset + 25);
    const fields = batch.map(({ source }, index) => {
      const owner = JSON.stringify(source.owner);
      const name = JSON.stringify(source.name);
      return source.type === "gist"
        ? `p${index}: user(login: ${owner}) { gist(name: ${name}) { stargazerCount } }`
        : `p${index}: repository(owner: ${owner}, name: ${name}) { stargazerCount }`;
    });
    const response = await fetchImpl("https://api.github.com/graphql", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "User-Agent": "Wox-plugin-store" },
      body: JSON.stringify({ query: `query { ${fields.join(" ")} }` }),
      signal: AbortSignal.timeout(30000),
    });
    if (!response.ok) throw new Error(`GitHub GraphQL returned HTTP ${response.status}`);
    const result = await response.json();
    const unexpectedErrors = result.errors?.filter((error) => error.type !== "NOT_FOUND") || [];
    if (unexpectedErrors.length || !result.data) throw new Error(`GitHub GraphQL: ${JSON.stringify(unexpectedErrors.length ? unexpectedErrors : result.errors)}`);
    batch.forEach(({ id, source }, index) => {
      const entry = result.data[`p${index}`];
      const count = source.type === "gist" ? entry?.gist?.stargazerCount : entry?.stargazerCount;
      if (Number.isInteger(count) && count >= 0) stars[id] = count;
    });
  }
  return stars;
}

// Only release assets have GitHub download counts; a website repository alone is insufficient.
export function releaseAssetSource(plugin) {
  let url;
  try { url = new URL(plugin.DownloadUrl); } catch { return null; }
  const parts = url.pathname.split("/").filter(Boolean);
  if (url.protocol !== "https:" || url.hostname !== "github.com" || parts.length !== 6 || parts[2] !== "releases" || !((parts[3] === "latest" && parts[4] === "download") || parts[3] === "download")) return null;
  if (!/^[\w.-]+$/.test(parts[0]) || !/^[\w.-]+$/.test(parts[1])) return null;
  try {
    return { owner: parts[0], repo: parts[1], name: decodeURIComponent(parts[5]) };
  } catch {
    return null;
  }
}

// Batch repositories and count matching assets in at most 200 releases, ordered by creation time.
export async function fetchPluginDownloads(plugins, token, fetchImpl = fetch) {
  const downloads = {};
  let pending = plugins.map((plugin) => ({ id: plugin.Id, source: releaseAssetSource(plugin), cursor: null, pages: 0, total: 0, matched: false })).filter((item) => item.source);
  while (pending.length) {
    const batch = pending.slice(0, 25);
    pending = pending.slice(25);
    const fields = batch.map(({ source, cursor }, index) => {
      const after = cursor ? `, after: ${JSON.stringify(cursor)}` : "";
      return `p${index}: repository(owner: ${JSON.stringify(source.owner)}, name: ${JSON.stringify(source.repo)}) { releases(first: 100, orderBy: {field: CREATED_AT, direction: DESC}${after}) { pageInfo { hasNextPage endCursor } nodes { isDraft releaseAssets(first: 1, name: ${JSON.stringify(source.name)}) { nodes { name downloadCount } } } } }`;
    });
    const response = await fetchImpl("https://api.github.com/graphql", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "User-Agent": "Wox-plugin-store" },
      body: JSON.stringify({ query: `query { ${fields.join(" ")} }` }),
      signal: AbortSignal.timeout(30000),
    });
    if (!response.ok) throw new Error(`GitHub GraphQL returned HTTP ${response.status}`);
    const result = await response.json();
    const unexpectedErrors = result.errors?.filter((error) => error.type !== "NOT_FOUND") || [];
    if (unexpectedErrors.length || !result.data) throw new Error(`GitHub GraphQL: ${JSON.stringify(unexpectedErrors.length ? unexpectedErrors : result.errors)}`);
    batch.forEach((item, index) => {
      const releases = result.data[`p${index}`]?.releases;
      if (!releases) return;
      for (const release of releases.nodes) {
        if (release.isDraft) continue;
        for (const asset of release.releaseAssets.nodes) {
          // GitHub matches release asset names without regard to case.
          if (asset.name.toLowerCase() === item.source.name.toLowerCase()) {
            if (!Number.isSafeInteger(asset.downloadCount) || asset.downloadCount < 0) throw new Error(`Invalid download count for ${item.source.owner}/${item.source.repo}/${item.source.name}`);
            item.total += asset.downloadCount;
            item.matched = true;
          }
        }
      }
      item.pages++;
      if (releases.pageInfo.hasNextPage && item.pages < 2) {
        if (!releases.pageInfo.endCursor || releases.pageInfo.endCursor === item.cursor) throw new Error("Invalid GitHub releases pagination cursor");
        item.cursor = releases.pageInfo.endCursor;
        pending.push(item);
      } else if (item.matched) {
        downloads[item.id] = item.total;
      }
    });
  }
  return downloads;
}

// Update only statistics, retaining every other catalog field.
export function updatePluginStats(plugins, stars, downloads) {
  for (const plugin of plugins) {
    if (Object.hasOwn(stars, plugin.Id)) plugin.StarCount = stars[plugin.Id];
    else delete plugin.StarCount;
    if (Object.hasOwn(downloads, plugin.Id)) plugin.DownloadCount = downloads[plugin.Id];
    else delete plugin.DownloadCount;
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  if (!process.env.GITHUB_TOKEN) throw new Error("GITHUB_TOKEN is required to fetch plugin statistics");
  const plugins = JSON.parse(await readFile(catalogPath, "utf8"));
  const stars = await fetchPluginStars(plugins, process.env.GITHUB_TOKEN);
  const downloads = await fetchPluginDownloads(plugins, process.env.GITHUB_TOKEN);
  updatePluginStats(plugins, stars, downloads);
  await writeFile(catalogPath, `${JSON.stringify(plugins, null, 2)}\n`);
  console.log(`Fetched star counts for ${Object.keys(stars).length} and download counts for ${Object.keys(downloads).length} of ${plugins.length} plugins.`);
}
