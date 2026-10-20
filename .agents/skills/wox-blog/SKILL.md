---
name: wox-blog
description: Create or revise Wox blog articles with practical workflows, paired English and Chinese Markdown, and consistent illustrated covers. Use for Wox blog writing and editorial standards, not release changelogs or general documentation.
---

# Wox Blog

Write for people using Wox: one concrete task, its useful outcome, and enough detail to reproduce it. Support product stories and engineering articles when requested; practical tips and workflows are the default.

## Establish the facts

- Read relevant feature documentation and implementation before describing commands, setting labels, defaults, or platform support. Distinguish released behavior from development-only changes; do not imply unreleased features are available in the current release.
- Inspect nearby articles and `www/docs/.vitepress/theme/components/blog.data.mts` for the metadata contract. Do not turn a blog request into an app feature change.
- For existing articles, preserve the slug, publication date, and useful media unless the request explicitly changes them.

## Write the article pair

- Read [the writing style guide](references/writing-style.md) before drafting or substantially rewriting. It contains the Mole sample analysis, Wox-specific examples, and an editorial review. Borrow explanatory techniques rather than copying an author's voice.
- Default to English in `www/docs/blog/<slug>.md` and Simplified Chinese in `www/docs/zh/blog/<slug>.md`, using the same slug, date, category, and cover. Honor explicit single-language requests.
- Use [the article template](assets/article-template.md) for frontmatter and a starting point, replacing every template token. It is not a mandatory outline.
- Choose a specific title about a task or benefit. Avoid a repeated “Did You Know” / “你知道吗” prefix, exaggerated claims, and generic introductions. The description should add useful context rather than repeat the title.
- Develop the situation, a concrete example, the reason for consequential settings, and the observable result as needed. Use connected prose for the explanation and steps for actions. Add useful context where an article feels thin; do not pad it with adjectives, repeated benefits, or a mandatory FAQ/conclusion. Keep length and structure proportional to the task.
- Use a calm, conversational voice with specific nouns and verbs. Do not fabricate personal experience, measured improvements, user quotations, or test results to make the writing lively. Keep card descriptions compact; put the fuller explanation in the body.
- Translate naturally. Keep commands, setting semantics, platform constraints, and outcomes equivalent across languages. Make frontmatter `title` match the article H1.
- New articles use the actual publication date in `YYYY-MM-DD`. Use a stable descriptive kebab-case slug; omit the legacy `did-you-know-` prefix for new articles.

## Supply card metadata and a cover

Required frontmatter: `title`, `description`, `date`, `category`, `cover`, and `coverAlt`.

| Category ID | English label | Chinese label |
| --- | --- | --- |
| `workspace` | Windows & apps | 窗口与应用 |
| `preview` | Files & previews | 文件与预览 |
| `automation` | Shortcuts & automation | 快捷键与自动化 |
| `product` | Inside Wox | Wox 动态 |

Choose the closest category. Add a category only when the article genuinely falls outside these themes, updating loader validation and both localized labels in `BlogIndex.vue` together.

- The default cover style is the approved flat illustration family: front-facing 2D shapes, thin charcoal-blue outlines, pale gray backgrounds, muted blue/sage/peach accents, and generous whitespace. Avoid clay-like 3D materials, bevels, thick keys, perspective, and heavy shadows. Use no text or logos in covers.
- Read [the cover style reference](references/cover-style.md) when creating or replacing a cover. Use the built-in image generation tool for original illustrations; inspect an existing cover as a style reference when useful.
- Save selected images under `www/docs/public/images/blog/` and reference `/images/blog/<name>.webp` (or the actual image extension). Localize `coverAlt` concisely.
- Covers express the topic, not evidence of product behavior. Preserve real screenshots and videos in the body. Never substitute generated mock UI for factual screenshots.
- When a real Wox product screenshot is needed but unavailable, continue drafting with a descriptive text placeholder at the intended position, such as `> Screenshot needed: Wox workspace settings showing the saved window layout on Windows.` Specify the screen, relevant state or action, and what the image should demonstrate; use the article language. Do not create an image link to a nonexistent file. At delivery, notify the user and list the article locations and screenshots they need to supply. These intentional screenshot placeholders may remain in the draft; they do not block completing the text and local validation.
- If image generation is unavailable, finish text preparation and report the missing cover. Do not leave broken references or silently switch to a different medium.
- The homepage discovers Markdown automatically and sorts by date, then URL. Do not manually add links to the blog indexes or hard-code cards.

## Verify and deliver

- Run the editorial pass in the writing guide: check that the reader can recognize the situation, follow the example, and tell what success looks like; remove unsupported claims and repeated filler.
- Check both languages, links, actual asset paths, metadata, and claims against the source feature. Preserve existing URLs when revising.
- From `www`, run `pnpm docs:build`; run `git diff --check` from the repository root.
- Verify the article appears on its language homepage with its cover, complete title, and correct date. Inspect article media and the card at desktop and mobile widths.
- Report article paths and verification limits. Default to local files and local verification; do not commit, push, or deploy unless requested.
