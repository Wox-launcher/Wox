import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createContentLoader } from "vitepress";

export interface BlogPost {
  url: string;
  lang: "en" | "zh";
  title: string;
  description: string;
  date: string;
  category: "workspace" | "preview" | "automation" | "product";
  cover: string;
  coverDark: string;
  coverAlt: string;
}

export declare const data: BlogPost[];

export default createContentLoader(["blog/*.md", "zh/blog/*.md"], {
  // Metadata is the only index: adding a translated Markdown pair adds both cards.
  transform(pages): BlogPost[] {
    return pages
      .filter(({ url }) => !/^\/(?:zh\/)?blog\/(?:index\.html)?$/.test(url))
      .map(({ url, frontmatter }) => {
        for (const key of ["title", "description", "category", "cover", "coverDark", "coverAlt"]) {
          if (typeof frontmatter[key] !== "string" || !frontmatter[key].trim()) {
            throw new Error(`${url}: blog frontmatter requires ${key}`);
          }
        }
        if (!["workspace", "preview", "automation", "product"].includes(frontmatter.category)) {
          throw new Error(`${url}: unsupported blog category ${frontmatter.category}`);
        }
        const date = frontmatter.date instanceof Date ? frontmatter.date.toISOString().slice(0, 10) : String(frontmatter.date);
        if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !Number.isFinite(Date.parse(date)) || new Date(date).toISOString().slice(0, 10) !== date) {
          throw new Error(`${url}: blog date must be a valid YYYY-MM-DD date`);
        }
        for (const cover of [frontmatter.cover, frontmatter.coverDark]) {
          if (!cover.startsWith("/images/blog/") || !existsSync(fileURLToPath(new URL(`../../../public${cover}`, import.meta.url)))) {
            throw new Error(`${url}: blog cover must exist under public/images/blog/`);
          }
        }
        return {
          url,
          lang: url.startsWith("/zh/") ? "zh" : "en",
          title: frontmatter.title,
          description: frontmatter.description,
          date,
          category: frontmatter.category,
          cover: frontmatter.cover,
          coverDark: frontmatter.coverDark,
          coverAlt: frontmatter.coverAlt,
        } satisfies BlogPost;
      })
      .sort((a, b) => b.date.localeCompare(a.date) || (a.url < b.url ? -1 : a.url > b.url ? 1 : 0));
  },
});
