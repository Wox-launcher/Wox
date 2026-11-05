import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import sharp from "sharp";

const dist = resolve("docs/.vitepress/dist");
const site = "https://www.woxlauncher.com";
const plugins = JSON.parse(readFileSync("../store-plugin.json", "utf8"));
const themes = JSON.parse(readFileSync("../store-theme.json", "utf8"));
const sitemap = readFileSync(resolve(dist, "sitemap.xml"), "utf8");

function readPage(path) {
  return readFileSync(resolve(dist, path), "utf8");
}

function meta(html, key) {
  const match = html.match(new RegExp(`<meta (?:property|name)="${key}" content="([^"]*)"`));
  assert.ok(match, `missing ${key}`);
  return match[1].replaceAll("&amp;", "&").replaceAll("&quot;", '"').replaceAll("&#39;", "'").replaceAll("&lt;", "<").replaceAll("&gt;", ">");
}

function checkCard(path, image, type = "website") {
  const html = readPage(path);
  assert.equal(meta(html, "og:type"), type, path);
  assert.equal(meta(html, "og:image"), image, path);
  assert.equal(meta(html, "twitter:image"), image, path);
  assert.equal(meta(html, "twitter:card"), "summary_large_image", path);
  assert.equal(meta(html, "twitter:title"), meta(html, "og:title"), path);
  assert.equal(meta(html, "twitter:description"), meta(html, "og:description"), path);
  assert.ok(meta(html, "og:image:alt"), path);
  return html;
}

for (const prefix of ["", "zh/"]) {
  const home = checkCard(`${prefix}index.html`, "https://repository-images.githubusercontent.com/15315789/d33f10a3-fc19-4403-8680-8bbb74213a53");
  assert.equal(meta(home, "og:url"), `${site}/${prefix}`);

  const blog = resolve("docs", prefix, "blog");
  for (const file of readdirSync(blog).filter((name) => name.endsWith(".md") && name !== "index.md")) {
    const markdown = readFileSync(resolve(blog, file), "utf8");
    const cover = markdown.match(/^cover:\s*["']?([^\s"']+)/m)?.[1];
    assert.ok(cover, `${file} needs a cover`);
    assert.ok(existsSync(resolve("docs/public", `.${cover}`)), `${file} cover must exist`);
    const card = `${prefix}blog/${file.replace(/\.md$/, ".jpg")}`;
    checkCard(`${prefix}blog/${file.replace(/\.md$/, ".html")}`, `${site}/images/social/${card}`, "article");
    assert.ok(existsSync(resolve(dist, "images/social", card)), card);
  }

  for (const [kind, items] of [["plugins", plugins], ["themes", themes]]) {
    for (const item of items) {
      const path = `${prefix}store/${kind}/${item.Id}.html`;
      const card = `${prefix}store/${kind}/${item.Id}.jpg`;
      const html = checkCard(path, `${site}/images/social/${card}`);
      assert.ok(existsSync(resolve(dist, "images/social", card)), card);
      assert.equal(meta(html, "og:url"), `${site}/${path}`, path);
      assert.ok(sitemap.includes(`${site}/${path}`), path);
      if (kind === "themes") assert.match(html, /<h1[^>]*>[^<]+<\/h1>/, path);
    }
  }
  const legacy = readPage(`${prefix}store/theme.html`);
  assert.ok(legacy.includes('name="robots" content="noindex, follow"'));
  assert.ok(!sitemap.includes(`${site}/${prefix}store/theme.html`));
}

for (const image of ["blog/wox-privacy-settings.jpg", "zh/blog/wox-privacy-settings.jpg", `store/plugins/${plugins[0].Id}.jpg`, `store/themes/${themes[0].Id}.jpg`]) {
  const info = await sharp(resolve(dist, "images/social", image)).metadata();
  assert.equal(info.format, "jpeg", image);
  assert.equal(info.width, 1200, image);
  assert.equal(info.height, 630, image);
}

console.log("Social card build checks passed.");
