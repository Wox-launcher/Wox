# Icons

Use this reference when a plugin needs polished `result` and `action` icons.

## Defaults

- Prefer inline SVG constants in `icons.ts` or `icons.py`.
- Use colorful `48x48` SVG for primary plugin/result visuals.
- Keep `result` and `action` icons in the same Iconify family.
- Prefer simple shapes that remain legible at 16-32 px after downscaling.
- Check `assets/iconify/` first. If a bundled generic icon already matches the requested behavior, reuse it instead of searching for a new one.

## Bundled Generic Icons

- `assets/iconify/open.svg`: prefer for open, launch, open-in-browser, and go-to style actions.
- `assets/iconify/copy.svg`: prefer for copy, duplicate, and clipboard style actions.

When one of these already matches the intent, treat it as the default choice. Only search Iconify again when the user asks for a different metaphor or the plugin needs a stronger domain-specific icon.

## Family Selection

- Use monochrome families such as `tabler`, `lucide`, or `material-symbols` when the plugin already has a colored card/background treatment.
- Use palette-enabled families only when the plugin UI clearly benefits from multicolor icons.
- Avoid mixing stroke-heavy outline icons with dense filled icons in the same plugin.

## Search Workflow

1. Search by behavior or noun first: `list`, `click`, `launch`, `copy`, `search`.
2. Check whether `assets/iconify/` already has a matching generic icon.
3. Filter to one or two families.
4. Prefer icons whose silhouette still reads when scaled down.
5. Fetch the selected SVG and store it as a shared constant in `icons.ts` or `icons.py`.

Helper commands:

```bash
python3 scripts/search_iconify.py search "result" --prefixes tabler,lucide --palette monotone
python3 scripts/search_iconify.py fetch tabler:list-details --height 48 --format ts --const-name RESULT_ICON_SVG
python3 scripts/search_iconify.py fetch tabler:hand-click --height 48 --format py --const-name ACTION_ICON_SVG
```

## API Notes

- Search API: `https://api.iconify.design/search`
- SVG API: `https://api.iconify.design/{prefix}/{name}.svg`
- The Search API enforces a minimum `limit` of `32`.
- Collection metadata includes whether a family uses a color palette, which is useful for filtering.

## Output Placement

- Put shared constants in `icons.ts` or `icons.py`.
- If designers need to replace assets manually, put the chosen SVGs under the plugin's own `icons/` directory and reference them with `relative`.
- Do not reference files from the skill folder at runtime.
