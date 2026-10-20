# Blog cover illustrations

Use flat, two-dimensional editorial illustrations: thin charcoal-blue outlines, simple filled shapes, pale neutral gray backgrounds, warm white panels, and restrained slate-blue, sage, and peach accents. Keep a front-facing view with generous negative space. Avoid clay-like materials, bevels, extrusion, perspective, glossy highlights, gradients, and cast shadows. Images contain no text; the site owns headings and labels.

Compose one recognizable concept in a landscape 16:9 frame. Keep important objects in the central area for safe card cropping. Inspect at card size in both website themes.

Use the built-in image generation tool, one call per cover. Export web-sized WebP copies (1280px wide at quality 85 for the current set) into `www/docs/public/images/blog/`. Keep original generated files in place. Use the current covers as the style reference.

## Acceptance checks

- Match the approved flat covers currently in `www/docs/public/images/blog/`, rather than the earlier clay-style drafts. A new topic should still look like part of this set.
- Shapes should read as a drawing at card size: thin strokes, simple fills, and a front-facing view. Reject thick extruded panels, chunky physical keycaps, beveled edges, glossy highlights, or shadows that give the objects weight and volume.
- Keep one clear subject with few supporting objects. Preserve 16:9 framing and enough whitespace for safe cropping; do not fill the cover with tiny interface detail.
- Use restrained color accents and a light neutral background. Keep titles, labels, and branding in the page text, and use real product captures inside the article.
- Inspect the generated image before exporting it. If it drifts back toward 3D, correct that specific visual property with the image tool rather than accepting it because the subject is recognizable.

## Reusable prompt

The current covers use this shared prompt followed by a subject:

> Use case: stylized-concept. Create an original flat editorial illustration for a productivity blog cover. Landscape 16:9. Strictly two-dimensional vector-like artwork with simple filled shapes, thin clean charcoal-blue outlines, a pale neutral gray background (#f3f4f5), warm white panels and muted slate blue accents, tiny touches of sage and peach. A centered compact composition with generous negative space and consistent outline weight. Front-facing orthographic view. Crisp, restrained, friendly software editorial illustration. Absolutely no 3D rendering, no clay, no bevels, no extrusion, no perspective, no photorealistic texture, no gradients, no cast shadows, no glossy highlights. No text, letters, numbers, logos or watermark. Subject:

| Cover | Subject appended to the prompt |
| --- | --- |
| `workspace-layouts.webp` | three thin application window rectangles arranged as one large window on the left and two small windows stacked on the right. Simple abstract content blocks. One small flat restore arrow in front. |
| `file-preview.webp` | a flat document outline with a folded corner behind a flat image-preview card containing a minimal mountain-and-sun landscape. One thin blank spacebar shape beneath. All shapes front-facing and flat. |
| `app-launchpad.webp` | one flat rounded-square panel holding a 3 by 3 grid of small flat app tiles. Each tile contains a simple two-dimensional geometric pictogram. Thin outlines and restrained color fills. |
| `silent-translation.webp` | two flat outlined document panels with short horizontal lines representing text, linked by a simple blue arrow. One small blank shortcut key drawn as a plain outlined square below. |
| `webview-hotkey.webp` | one flat browser-window outline with a small toolbar and a simple globe line icon inside. Two thin blank keyboard-key shapes below. Clean flat geometric illustration. |

For a new topic, replace the subject and retain the shared visual direction. Reuse covers across languages. Do not put translated titles into images or copy third-party illustrations. Use real captures in the body when explaining Wox UI.

