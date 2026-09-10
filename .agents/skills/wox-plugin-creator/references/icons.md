# Icons

Use this reference when a plugin needs polished `result` and `action` icons.

## Result vs action

Treat result icons and Action Panel icons as different contracts.

- **Plugin metadata and result-row icons** may be colorful. Brand marks, app icons, favicons, emoji, and mixed-color SVGs belong here.
- **Action Panel leading icons must be monochrome theme-adaptive SVGs.** Prefer verbs that match Wox's built-in `action.*` catalog: copy, open, execute (lightning), delete, edit, paste, add, search, settings.
- Do not reuse the plugin mark, a colored result icon, an emoji, or a brand logo as the leading action glyph. The Action Panel only tints SVGs that contain `var(--wox-theme-icon-color)`. A colored or emoji action stays authored and looks out of place next to system actions.
- Execute actions use the lightning verb, not a gear or a play triangle.
- Result identity can stay colorful even when the same row's actions are monochrome.

Third-party plugins cannot call `icons.Get`. Copy a bundled action SVG from `assets/iconify/action/`, or author the same stroke style with the theme variable.

## Defaults

- Prefer inline SVG constants in `icons.ts` or `icons.py`.
- Use colorful `48x48` SVG only for plugin/result identity.
- Keep action icons in one monochrome family (Wox verbs or Lucide/Tabler outlines).
- Prefer simple shapes that remain legible at 16-32 px after downscaling.
- Check `assets/iconify/action/` first for Action Panel verbs. Only search Iconify when no bundled verb matches.

## SVG Theme Colors

- For paints that should adapt to Wox appearance, explicitly use `fill="var(--wox-theme-icon-color)"` or `stroke="var(--wox-theme-icon-color)"`. Wox resolves this variable to white (`#ffffff`) in dark themes and black (`#000000`) in light themes.
- The Action Panel then tints those theme-adaptive SVGs to the row label (`ActionText` / `ActionSelectedText`). The variable itself is appearance black/white, not a per-row text color.
- Apply the variable only to theme-adaptive parts. Preserve fixed brand colors, gradients, and SVG mask colors; do not infer theme behavior from authored black/white paints or recolor an entire mixed-color SVG.
- Standard SVG `currentColor` is not the Wox theme variable. When adapting an Iconify SVG for an action, replace theme-adaptive `currentColor` paints with `var(--wox-theme-icon-color)` (`scripts/search_iconify.py fetch` does this by default).
- The variable is resolved by Wox's shared launcher image pipeline, not by a browser or arbitrary SVG renderer. Do not assume it is automatically defined inside a plugin's HTML preview.

```xml
<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--wox-theme-icon-color)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
  <rect x="8" y="8" width="12" height="12" rx="2"/>
  <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>
</svg>
```

Script plugins prefix the same markup with `svg:`. SDK plugins pass it as `WoxImage` type `svg`.

## Bundled Action Verbs

Reuse these files as the default Action Panel glyphs. They are byte-for-byte mirrors of Wox's own `action.*` catalog in `wox.core/common/icons/default_action.go` (24 grid, 2-unit stroke, about 3 units of margin), so a plugin action sits next to system actions without a visible weight or size change. `TestPluginCreatorSkillActionAssetsMatchCatalog` in `wox.core/common/icons` fails when they drift; when a catalog verb changes, copy the new SVG here.

| Verb | File | Use for |
| --- | --- | --- |
| Execute | `assets/iconify/action/execute.svg` | Execute, run command. Lightning, not settings or play. |
| Copy | `assets/iconify/action/copy.svg` | Copy, duplicate, clipboard |
| Open | `assets/iconify/action/open.svg` | Open, launch, open in browser |
| Open folder | `assets/iconify/action/open-containing-folder.svg` | Reveal / open containing folder |
| Delete | `assets/iconify/action/delete.svg` | Delete, remove, trash |
| Edit | `assets/iconify/action/edit.svg` | Edit, rename, fill parameters |
| Paste | `assets/iconify/action/paste.svg` | Paste |
| Add | `assets/iconify/action/add.svg` | New, create, add |
| Search | `assets/iconify/action/search.svg` | Search |
| Settings | `assets/iconify/action/settings.svg` | Open settings (not Execute) |

When one of the action verbs already matches the intent, treat it as the default choice. Only search Iconify again when the user asks for a different metaphor or the plugin needs a stronger domain-specific **result** icon.

## Family Selection

- Action icons: monochrome families such as `tabler`, `lucide`, or `material-symbols`, then rewrite paints to `var(--wox-theme-icon-color)`.
- Result icons: palette-enabled families only when the plugin identity clearly benefits from multicolor icons.
- Avoid mixing stroke-heavy outline actions with dense filled actions in the same plugin.

## Search Workflow

1. For an action, pick a bundled verb from the table above.
2. If no verb matches, search Iconify for a monochrome outline and apply the theme variable.
3. For a result or plugin mark, colorful SVG or emoji is acceptable.
4. Prefer icons whose silhouette still reads when scaled down.
5. Store the selected SVG as a shared constant in `icons.ts` or `icons.py`.

Helper commands:

```bash
python3 scripts/search_iconify.py search "copy" --prefixes tabler,lucide --palette monotone
python3 scripts/search_iconify.py fetch tabler:copy --height 24 --format ts --const-name ACTION_COPY_SVG
python3 scripts/search_iconify.py fetch tabler:list-details --height 48 --format py --const-name RESULT_ICON_SVG --no-wox-theme
```

`fetch` rewrites `currentColor` to `var(--wox-theme-icon-color)` unless `--no-wox-theme` is set. Use `--no-wox-theme` only for brand or mixed-color **result** icons.

## API Notes

- Search API: `https://api.iconify.design/search`
- SVG API: `https://api.iconify.design/{prefix}/{name}.svg`
- The Search API enforces a minimum `limit` of `32`.
- Collection metadata includes whether a family uses a color palette, which is useful for filtering.

## Output Placement

- Put shared constants in `icons.ts` or `icons.py`.
- If designers need to replace assets manually, put the chosen SVGs under the plugin's own `icons/` directory and reference them with `relative` for **results** only. Action icons still need the theme variable in the SVG itself.
- Do not reference files from the skill folder at runtime.
- Single-file SDK plugins cannot use relative image paths. Inline the SVG (or use emoji/URL/base64 for result identity only).
