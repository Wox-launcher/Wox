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
  assert.ok(sitemap.includes(`https://www.woxlauncher.com/${path}`));
  const gallery = readFileSync(resolve(dist, `${prefix}store/plugins.html`), "utf8");
  assert.ok(gallery.includes(`/${path}`));
  const legacy = readFileSync(resolve(dist, `${prefix}store/plugin.html`), "utf8");
  assert.ok(legacy.includes('name="robots" content="noindex, follow"'));
  assert.ok(!sitemap.includes(`https://www.woxlauncher.com/${prefix}store/plugin.html`));
}
console.log("Plugin SEO build checks passed.");
