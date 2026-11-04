import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const dist = resolve("docs/.vitepress/dist");
const plugins = JSON.parse(readFileSync("../store-plugin.json", "utf8"));
const sitemap = readFileSync(resolve(dist, "sitemap.xml"), "utf8");
const sample = plugins.find((plugin) => plugin.Name === "Obsidian");
assert.ok(sample);

for (const prefix of ["", "zh/"]) {
  const path = `${prefix}store/plugins/${sample.Id}.html`;
  const html = readFileSync(resolve(dist, path), "utf8");
  assert.match(html, /<h1[^>]*>Obsidian<\/h1>/);
  assert.ok(html.includes(`rel="canonical" href="https://www.woxlauncher.com/${path}"`));
  assert.ok(html.includes("Wox plugin to manage obsidian vault notes"));
  for (const [label, count] of [[prefix ? "Star 数量" : "Stars", sample.StarCount], [prefix ? "下载量" : "Downloads", sample.DownloadCount]]) {
    if (count !== undefined) {
      const formatted = count.toLocaleString("en-US");
      assert.match(html, new RegExp(`<dt[^>]*>${label}</dt><dd[^>]*>${formatted}</dd>`));
    }
  }
  assert.ok(sitemap.includes(`https://www.woxlauncher.com/${path}`));
  const gallery = readFileSync(resolve(dist, `${prefix}store/plugins.html`), "utf8");
  const mostDownloaded = [...plugins].sort((left, right) => (right.DownloadCount ?? -1) - (left.DownloadCount ?? -1))[0];
  const firstPluginLink = gallery.match(/<article class="card"[^>]*><a href="([^"]+)"/);
  assert.ok(firstPluginLink, "the gallery should render a plugin card");
  assert.equal(firstPluginLink[1], `/${prefix}store/plugins/${mostDownloaded.Id}.html`);
  assert.ok(gallery.includes(`/${path}`));
  const legacy = readFileSync(resolve(dist, `${prefix}store/plugin.html`), "utf8");
  assert.ok(legacy.includes('name="robots" content="noindex, follow"'));
  assert.ok(!sitemap.includes(`https://www.woxlauncher.com/${prefix}store/plugin.html`));
}
console.log("Plugin SEO build checks passed.");
