import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { PageData } from "vitepress";

const SITE = "https://www.woxlauncher.com";
const DOCS_ROOT = path.resolve(fileURLToPath(new URL("..", import.meta.url)));

export function canonicalUrl(relativePath: string): string {
  if (relativePath === "index.md") {
    return `${SITE}/`;
  }
  if (relativePath.endsWith("/index.md")) {
    return `${SITE}/${relativePath.slice(0, -"index.md".length)}`;
  }
  return `${SITE}/${relativePath.replace(/\.md$/, ".html")}`;
}

/** Add canonical, Open Graph, and paired hreflang tags for a docs page. */
export function applySeo(pageData: PageData): void {
  // Static hosting has no server redirect rules; retain old pages as noindex
  // redirects whose canonical URL points at the new article.
  const redirect = pageData.frontmatter.redirect;
  const canonical = redirect ? new URL(redirect, SITE).href : canonicalUrl(pageData.relativePath);
  const isHome = pageData.relativePath === "index.md" || pageData.relativePath === "zh/index.md";
  const title = isHome || pageData.frontmatter.titleTemplate === false ? pageData.title : `${pageData.title} | Wox`;
  const description = String(pageData.description || pageData.frontmatter.description || "");

  pageData.frontmatter.head ??= [];
  const head = pageData.frontmatter.head as Array<[string, Record<string, string>]>;
  head.push(["link", { rel: "canonical", href: canonical }]);
  if (redirect) {
    head.push(["meta", { name: "robots", content: "noindex, follow" }]);
    head.push(["meta", { "http-equiv": "refresh", content: `0; url=${redirect}` }]);
    return;
  }
  head.push(["meta", { property: "og:title", content: title }]);
  head.push(["meta", { property: "og:url", content: canonical }]);
  if (description) {
    head.push(["meta", { property: "og:description", content: description }]);
  }

  const alternate = alternateRelativePath(pageData.relativePath);
  if (!alternate || !pageExists(alternate)) {
    return;
  }

  const englishPath = pageData.relativePath.startsWith("zh/") ? alternate : pageData.relativePath;
  const chinesePath = pageData.relativePath.startsWith("zh/") ? pageData.relativePath : alternate;
  const englishUrl = canonicalUrl(englishPath);
  const chineseUrl = canonicalUrl(chinesePath);
  head.push(["link", { rel: "alternate", hreflang: "en-US", href: englishUrl }]);
  head.push(["link", { rel: "alternate", hreflang: "zh-CN", href: chineseUrl }]);
  head.push(["link", { rel: "alternate", hreflang: "x-default", href: englishUrl }]);
}

function alternateRelativePath(relativePath: string): string | null {
  if (relativePath === "zh/index.md") {
    return "index.md";
  }
  if (relativePath.startsWith("zh/")) {
    return relativePath.slice(3);
  }
  if (relativePath === "index.md") {
    return "zh/index.md";
  }
  return `zh/${relativePath}`;
}

function pageExists(relativePath: string): boolean {
  return fs.existsSync(path.join(DOCS_ROOT, relativePath));
}
