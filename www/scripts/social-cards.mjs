import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import sharp from "sharp";

const cards = new Map();
const images = new Map();
const publicDir = path.resolve("docs/public");
const allowedImageHosts = new Set(["raw.githubusercontent.com", "github.com", "gist.github.com"]);
const logoImage = sharp(path.join(publicDir, "logo.png")).resize(41, 41).png().toBuffer();

// Store pages need their own image URLs before VitePress renders the HTML.
export function queueSocialCard(pageData) {
  const relativePath = pageData.relativePath;
  const match = relativePath.match(/^(zh\/)?(?:blog\/(.+)|store\/(plugins|themes)\/(.+))\.md$/);
  if (!match || pageData.frontmatter.redirect) return;

  const kind = match[2] ? "blog" : match[3];
  const source = pageData.frontmatter.socialImage || pageData.frontmatter.cover;
  if (kind === "blog" && !source) return;
  const imagePath = `/images/social/${relativePath.replace(/\.md$/, ".jpg")}`;
  cards.set(imagePath, {
    kind,
    chinese: Boolean(match[1]),
    title: String(pageData.title || "Wox"),
    description: String(pageData.description || ""),
    source: String(source || ""),
  });
  pageData.frontmatter.socialImage = imagePath;
}

function escapeXml(value) {
  return value.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

// Approximate glyph widths to keep mixed Chinese and English readable without clipping.
function wrapText(value, maxWidth, maxLines) {
  const segments = [...new Intl.Segmenter(undefined, { granularity: "word" }).segment(value.replace(/\s+/g, " ").trim())].map((part) => part.segment);
  const lines = [];
  let line = "";
  let width = 0;
  let truncated = false;
  const glyphWidth = (char) => /[\u2e80-\u9fff\uf900-\ufaff\uff00-\uffef]/u.test(char) ? 1 : /[MW@]/.test(char) ? 0.85 : 0.55;
  for (const segment of segments) {
    const segmentWidth = [...segment].reduce((sum, char) => sum + glyphWidth(char), 0);
    if (line.trim() && width + segmentWidth > maxWidth) {
      lines.push(line.trim());
      if (lines.length === maxLines) {
        truncated = true;
        break;
      }
      line = "";
      width = 0;
    }
    for (const char of segment) {
      if (!line && /\s/u.test(char)) continue;
      const charWidth = glyphWidth(char);
      if (width + charWidth > maxWidth && line.trim()) {
        lines.push(line.trim());
        if (lines.length === maxLines) {
          truncated = true;
          break;
        }
        line = "";
        width = 0;
      }
      line += char;
      width += charWidth;
    }
    if (truncated) break;
  }
  if (!truncated && line.trim()) lines.push(line.trim());
  if (truncated) lines[maxLines - 1] = `${lines[maxLines - 1].trimEnd().replace(/[\s,.，。;；:：]+$/u, "")}…`;
  return lines;
}

async function loadImage(source) {
  if (!source) return null;
  if (source.startsWith("/")) {
    const file = path.resolve(publicDir, `.${source}`);
    if (!file.startsWith(`${publicDir}${path.sep}`)) return null;
    return readFile(file).catch(() => null);
  }
  try {
    const url = new URL(source);
    if (url.protocol !== "https:" || !allowedImageHosts.has(url.hostname)) return null;
    const response = await fetch(url, { signal: AbortSignal.timeout(15000) });
    if (!response.ok || !response.headers.get("content-type")?.startsWith("image/")) return null;
    if (Number(response.headers.get("content-length")) > 8_000_000) return null;
    const chunks = [];
    let size = 0;
    for await (const chunk of response.body) {
      size += chunk.length;
      if (size > 8_000_000) return null;
      chunks.push(chunk);
    }
    return Buffer.concat(chunks);
  } catch {
    return null;
  }
}

async function cardImage(source) {
  if (!images.has(source)) images.set(source, loadImage(source));
  const input = await images.get(source);
  if (!input) return null;
  try {
    return await sharp(input, { limitInputPixels: 40_000_000 }).resize(500, 320, { fit: "contain", background: "#e9edf2" }).png().toBuffer();
  } catch {
    return null;
  }
}

async function renderCard(card) {
  const titleLines = wrapText(card.title, 10.5, 3);
  const descriptionLines = wrapText(card.description, 20, 4);
  const labels = card.chinese ? { blog: "博客", plugins: "插件", themes: "主题" } : { blog: "BLOG", plugins: "PLUGIN", themes: "THEME" };
  const titleSvg = titleLines.map((line, index) => `<text x="72" y="${210 + index * 55}" font-size="46" font-weight="700">${escapeXml(line)}</text>`).join("");
  const descriptionY = 220 + titleLines.length * 55 + 24;
  const descriptionSvg = descriptionLines.map((line, index) => `<text x="72" y="${descriptionY + index * 33}" font-size="25" fill="#536276">${escapeXml(line)}</text>`).join("");
  const preview = await cardImage(card.source);
  const fallback = preview ? "" : `<text x="886" y="310" text-anchor="middle" font-size="54" font-weight="700" fill="#8d9aaa">Wox</text>`;
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630">
    <rect width="1200" height="630" fill="#f5f7fa"/>
    <circle cx="1070" cy="90" r="245" fill="#e7efff"/>
    <rect x="626" y="133" width="520" height="340" rx="20" fill="#e9edf2"/>
    <g font-family="Noto Sans CJK SC, Microsoft YaHei, sans-serif" fill="#172337">
      <rect x="72" y="56" width="41" height="41" rx="10" fill="white" stroke="#172337" stroke-width="1.5"/>
      <text x="128" y="87" font-size="28" font-weight="700">Wox</text>
      <text x="72" y="150" font-size="19" font-weight="700" letter-spacing="1" fill="#226b78">${labels[card.kind]}</text>
      ${titleSvg}${descriptionSvg}${fallback}
      <text x="72" y="570" font-size="19" fill="#66778a">woxlauncher.com</text>
    </g>
  </svg>`;
  const layers = [{ input: await logoImage, left: 72, top: 56 }];
  if (preview) layers.push({ input: preview, left: 636, top: 143 });
  return { data: await sharp(Buffer.from(svg)).composite(layers).jpeg({ quality: 85 }).toBuffer(), hasPreview: Boolean(preview) };
}

async function writeCard(outputDir, imagePath, card) {
  const { data, hasPreview } = await renderCard(card);
  const out = path.join(outputDir, imagePath.slice(1));
  await mkdir(path.dirname(out), { recursive: true });
  await writeFile(out, data);
  return hasPreview;
}

// VitePress dev serves the same generated images without writing into the source tree.
export function socialCardsDevPlugin() {
  return {
    name: "wox-social-cards",
    configureServer(server) {
      server.middlewares.use(async (request, response, next) => {
        const imagePath = request.url?.split("?")[0];
        const card = cards.get(imagePath);
        if (!card) return next();
        try {
          const { data } = await renderCard(card);
          response.setHeader("Content-Type", "image/jpeg");
          response.end(data);
        } catch (error) {
          next(error);
        }
      });
    },
  };
}

// Render static JPEGs so social crawlers never need JavaScript or external images.
export async function writeSocialCards(outputDir) {
  const pending = [...cards.entries()];
  let missing = 0;
  await Promise.all(Array.from({ length: 8 }, async () => {
    while (pending.length) {
      const [imagePath, card] = pending.pop();
      if (!await writeCard(outputDir, imagePath, card)) missing++;
    }
  }));
  if (missing) console.warn(`Social cards: ${missing} previews could not be loaded; their text cards were generated without screenshots.`);
}
