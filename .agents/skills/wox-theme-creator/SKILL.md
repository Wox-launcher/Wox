---
name: wox-theme-creator
description: Design, create, and refine Wox theme JSON files, including schema v2 base colors, optional styles, transparency, platform overrides, and local debugging. Use for authoring themes or converting an existing theme while preserving its appearance.
---

# Wox Theme Creator

Create a deliberately designed theme with a coherent palette, readable states, and a valid authored JSON document. Prefer schema v2 for new themes. Preserve the requested output location and existing theme identity when editing; generate a fresh UUID for a new theme.

## Embedded theme editor

When invoked by Wox's theme editor, work on the supplied current draft and editable property list. These are the authoritative available fields. Apply the design guidance below; return only the requested JSON patch of changed properties, preserving unrelated values and theme identity. Do not require repository access, filesystem tools, or a new theme file. The editor validates the patch and handles preview, undo, and saving. A normal AI Chat or repository authoring session continues to use the document workflow below.

## Source of truth

Paths below are relative to the Wox repository root. Read the current implementation before choosing fields; do not invent theme tokens or copy resolved runtime values into a new theme.

- `wox.core/common/theme_schema_v2.go`: complete v2 document, accepted fields, color/geometry defaults, validation, and platform resolution.
- `wox.core/resource/themes/`: built-in themes as working examples.
- `wox.core/common/theme_surfaces.go` and `theme_package.go`: image surface declarations, resource validation, and package parsing.
- `wox.core/common/theme_schema_v1.go`: legacy wire format and behavior, needed when converting an old theme.
- `wox.core/common/theme_runtime.go` and `theme.go`: independent runtime representation, schema dispatch, and Wox version checks. These are not the authored JSON schema.

Each schema owns its complete document and parser. Missing `SchemaVersion` or historical zero means v1. Loading must not rewrite old files. Changing v2 default semantics would change sparse themes; a new format belongs in a separately registered schema, not a retrofit of v1 or v2.

## Image-backed themes

Wox 2.4.4 adds optional image surfaces to schema 2. Existing v1/v2 JSON files,
color defaults, window materials, and Action Panel placement are unchanged.
Use `MinWoxVersion: "2.4.4"` when publishing a theme using these fields.

### Package layout

A `.wox-theme` file is a ZIP archive with `theme.json` at its root, not inside
an enclosing folder:

```text
ming.wox-theme
  theme.json
  assets/lacquer.png
  assets/palace-frame.png
  assets/crest.png
```

Open the package with Wox, or select the file and invoke Wox's selection query.
The installer shows its name and an Install action. Installation validates all
entries before replacing `<theme-directory>/<ThemeId>/`; failed staging leaves
the previous package intact. Plain `<ThemeId>.json` themes remain supported.
Resource-backed themes are excluded from Cloud Sync.
Theme editor color changes and Save As retain the image declarations and assets.

The package accepts PNG and JPEG images. Paths are case-sensitive, slash-separated,
relative to the package root. Absolute paths, traversal, symlinks, Windows device
names, and case-colliding archive entries are rejected. Limits are 128 archive
entries, 32 MiB uncompressed data, and 16 megapixels across all images. Nine-slice
source cuts must leave a nonempty center. Assets for inactive platforms must also
be included. No network image URLs or executable theme code are supported.

### Example theme.json

```json
{
  "SchemaVersion": 2,
  "MinWoxVersion": "2.4.4",
  "ThemeId": "6cf090bd-ef04-44e9-aa61-cbe0e1dc2275",
  "ThemeName": "明",
  "BaseBackgroundColor": "#721D16",
  "BaseTextColor": "#FFF0CA",
  "BaseAccentColor": "#F4C453",
  "Surfaces": {
    "App": {
      "ContentInsets": {"Top": 110, "Right": 48, "Bottom": 32, "Left": 48},
      "Background": {
        "Source": "assets/lacquer.png",
        "Mode": "tile",
        "Size": {"Width": 128, "Height": 128}
      },
      "Frame": {
        "Source": "assets/palace-frame.png",
        "Mode": "nineSlice",
        "Slice": {"Top": 160, "Right": 96, "Bottom": 96, "Left": 96},
        "Insets": {"Top": 80, "Right": 48, "Bottom": 48, "Left": 48}
      },
      "Decorations": [{
        "Source": "assets/crest.png",
        "Anchor": "topCenter",
        "Offset": {"X": 0, "Y": 0},
        "Size": {"Width": 120, "Height": 110}
      }]
    },
    "ActionContainer": {
      "Background": {"Source": "assets/lacquer.png", "Mode": "tile"}
    }
  }
}
```

### Surface contract

The supported regions are `App`, `QueryBox`, `ResultItemActive`,
`ActionContainer`, `Preview`, and `Toolbar`. Each accepts the same optional
`Background`, `Frame`, and `Decorations` declarations. The generic preview shell
is supported; embedded web/native content and specialized previews still own
their inner rendering.

Layers draw in this order: existing color/material, Background, Frame,
Decorations, existing borders, interactive contents. Images do not introduce
hit targets. Transparent image pixels reveal the underlying fill; they do not
erase it. Author a transparent App background and custom chrome explicitly when
needed. AppBorder fields still disable the OS material as before. Native
arbitrary-shape input regions are not part of this extension.

`Background` and `Frame` accept these modes:

| Mode        | Geometry                                                                                             |
| ----------- | ---------------------------------------------------------------------------------------------------- |
| `stretch`   | Scale the image to the surface bounds.                                                               |
| `tile`      | Repeat at `Size` logical units; if absent, use the image's pixel dimensions as logical units.        |
| `nineSlice` | Cut at `Slice` source pixels, draw borders at `Insets` logical units, stretch the remaining regions. |

Stretch and tiled images follow the surface corner radius. Nine-slice images
retain their authored alpha silhouette; put rounded corners in the asset.
Four corners retain their destination size unless the surface is too small,
in which case opposing borders shrink proportionally. A frame's center is
also drawn, so use a transparent center when only an outline is wanted.
Tiled backgrounds retain only their latest raster size, at the active display's
physical pixel density, capped by the asset's authored density. Moving between
displays rebuilds this cache; `Size` remains in logical units. The limit is 16
megapixels per background; larger requests use the existing color fallback.
Reuse the same `Source` for shared textures: decoded pixels are shared across
surfaces, while each surface keeps its own size cache. PNG file size is not its
memory cost: decoded RGBA uses roughly width × height × 4 bytes, plus raster
and native renderer caches. Use reasonably sized texture tiles.

Decorations require `Source`, `Anchor`, and a positive logical `Size`; `Offset`
defaults to zero. Anchors are `topLeft`, `topCenter`, `topRight`, `centerLeft`,
`center`, `centerRight`, `bottomLeft`, `bottomCenter`, and `bottomRight`.
Offsets are in logical units and decorations stay clipped to their owner.
Decorations are static and do not change control layout.

Only `App` accepts `ContentInsets`. These add to the existing uniform
`AppContentInset`, outside the inner content panel and existing `AppPadding`.
They reserve space for the entire launcher body, including the Toolbar and
Action Panel. Frame insets do not implicitly add layout padding.

Platform and variant `Surfaces` objects merge **by region**. A supplied region
replaces that region's whole declaration; omitted regions inherit. Set a region
to `null` to remove it, or `Surfaces: null` to remove every inherited surface.
Existing scalar platform override semantics are unchanged.

```json
{
  "windows": {"Surfaces": {"Toolbar": null}},
  "linux": {"Surfaces": null}
}
```

There is no separate package format version; the schema version belongs to
`theme.json`.

## Author the document

### Localized name and description

Schema v2 supports optional inline `I18n`, using the same locale-to-key map as
plugin.json. Set `ThemeName` and `Description` to `i18n:` keys:

```json
{
  "ThemeName": "i18n:theme_name",
  "Description": "i18n:theme_description",
  "I18n": {
    "en_US": {"theme_name": "Ming", "theme_description": "An imperial red and gold theme."},
    "zh_CN": {"theme_name": "明", "theme_description": "朱红与金黄的皇家主题。"}
  }
}
```

Provide `en_US` as fallback, plus the intended languages (`zh_CN`, `ru_RU`,
`pt_BR`, `ko_KR`, `ja_JP`). Missing translations fall back to English, then
Wox's built-in translations, then the original key. Literal text stays literal.
Translations are resolved for display and search; saved JSON retains the keys
and translation map. `I18n` belongs at the root, not in platform overrides.
This extension uses schema 2 and requires Wox 2.4.4. Theme translations currently
use inline `I18n`; separate `lang/` files are not loaded.

A minimal v2 document looks like this; replace the sample ID and name:

```json
{
  "SchemaVersion": 2,
  "MinWoxVersion": "2.4.3",
  "ThemeId": "66288aad-d3fa-493b-bc46-7e8e7280c40d",
  "ThemeName": "Jade",
  "BaseBackgroundColor": "rgba(28, 35, 37, 0.72)",
  "BaseTextColor": "#E5ECE9",
  "BaseAccentColor": "#70D6A6"
}
```

The three base colors, ID, and name are required. Explicitly declare schema 2. Set `MinWoxVersion` to the release supporting the features used; check current built-ins rather than lowering the floor to make installation succeed. `Version` is the theme's own version, separate from schema and minimum Wox versions. Include truthful author, description, and URL metadata when available. Do not mark a custom theme as system-installed. Automatic appearance themes use `IsAutoAppearance`, `DarkThemeId`, and `LightThemeId`; their required base colors still provide a fallback.

Group overrides by surface: window, query/Glance/Attention, result container/items, Action Panel/items/query, preview/tags, toolbar/keycaps. Keep the document sparse; add overrides for intentional design differences.

| Authored value                            | Meaning                                                  |
| ----------------------------------------- | -------------------------------------------------------- |
| Missing optional style                    | Inherit parent, or schema default at the root            |
| `null` at the root                        | Same as omitted: schema default                          |
| `null` on a platform or variant           | Clear the inherited value and restore the schema default |
| Integer `0`                               | Explicit zero; never substitute a default                |
| `transparent` or alpha zero               | Explicit transparency                                    |
| Empty color, negative/fractional geometry | Invalid                                                  |

Colors accept `#RRGGBB`, `#RRGGBBAA` (alpha last), `rgb(r,g,b)`, `rgba(r,g,b,a)`, and `transparent`. RGB channels are 0–255; alpha is 0–1. Geometry uses logical units, not physical screen pixels.

Font sizes are controlled by the application Interface size setting; do not add theme font-size overrides.

## Default window shape and transparency

Unless the user explicitly requests a custom app outline or window corners, omit `AppBorderColor`, `AppBorderWidth`, and `AppBorderRadius` at the root and all platform/variant levels. Do not infer a custom window outline or radius from a reference screenshot or a general request for a rounded visual style. Keep the system/default window shape and use a slightly translucent `AppBackgroundColor` by default (for example, alpha 0.90).

If the user explicitly requests a custom app outline or window corners, author the requested `AppBorder*` fields and default `AppBackgroundColor` to fully opaque (alpha 1). An explicit request for transparency or opacity overrides that default. QueryBox, result, preview, and Action Panel corner settings do not count as a request for custom window chrome. Toolbar and Action Panel may retain modest in-app translucency in either case.

These are theme-authoring defaults, not changes to schema parsing. Apply them when creating themes or when the user asks to restyle an existing theme; do not silently rewrite other existing themes. Keep `BaseBackgroundColor` independent when an explicit app background override is sufficient, so changing window alpha does not unintentionally change every derived surface.

## Design the surfaces together

Start with background, text, and accent roles. Optional colors derive independently from these roles; changing one optional token does not change another. Base alpha also affects derived colors. Consult the resolver for exact defaults instead of duplicating its entire catalog here.

- For translucent themes, consider app, query, Action Panel, action query, preview, and toolbar backgrounds together. An opaque surface can hide translucency beneath it; layered alpha and native materials affect the final appearance.
- V2 `AppContentInset` reserves a uniform logical inset around the entire launcher content, including the toolbar, previews, and floating panels. `AppContentBackgroundColor` paints the inner panel and `AppContentBorderRadius` rounds its background. Defaults are 0, transparent, and 0, preserving existing themes. These fields do not select custom window chrome: use a translucent `AppBackgroundColor` and omit `AppBorder*` to expose a native-material rim. Existing `AppPadding*` remains spacing inside the content panel. Child surfaces retain their own corner styles; the toolbar follows the panel's bottom corners.
- V2 uses `ResultItemActiveIndicatorColor`, `ResultItemActiveIndicatorWidth`, `ResultItemActiveIndicatorInsetLeft`, `ResultItemActiveIndicatorInsetTop`, `ResultItemActiveIndicatorInsetBottom`, and `ResultItemActiveIndicatorBorderRadius` for the selected-result marker. Width defaults to 0, color to the base accent, and insets/radius to 0. Zero insets/radius reproduce the edge strip; use positive insets and radius for a short rounded marker. Reserve icon space with result-item padding; marker geometry does not shift content. The unreleased v2 `ResultItemActiveBorderLeftWidth/Color` fields were removed; v1 keeps its original fields. Check normal, selected, and hovered rows separately; `ResultItemHoverBackgroundColor` controls hover.
- `QueryBoxBorderBottomColor` and `QueryBoxBorderBottomWidth` draw an inside bottom edge without changing query layout. Width defaults to 0; color defaults to the base accent. Zero disables it and explicit transparency is preserved.
- Action Panel border fields are `ActionContainerBorderColor`, `ActionContainerBorderWidth`, and `ActionContainerBorderRadius`. `ActionContainerDividerColor` controls internal separators independently of `PreviewSplitLineColor`.
- Toolbar primary actions (default/bare Enter) support `ToolbarPrimaryFontColor` and `ToolbarPrimaryHotkey{Font,Background,Border}Color`. Omitted values inherit the corresponding effective toolbar colors; explicit transparency is preserved. Configure emphasis here rather than relying on automatic dimming of other actions.
- Keycaps have three independent v2 groups: `ToolbarHotkey{Font,Background,Border}Color`, `ActionItemHotkey{Font,Background,Border}Color`, and `ActionItemActiveHotkey{Font,Background,Border}Color`. All accept explicit transparency. Omitted values derive from base colors, not another surface's overrides: normal text uses secondary text and border uses divider; active text/border use base text; backgrounds default transparent. Set active colors explicitly for contrasting selection backgrounds (for example, white keycaps on blue). The unreleased shared `Hotkey*` fields were removed. Keep keycaps readable without competing with labels.
- `PreviewBorderRadius` and `PreviewTagBorderRadius` control generic preview and metadata-tag corners in logical units. Omitted values retain 8; zero is square.
- Preview has `PreviewBackgroundColor`, `PreviewBorderColor`, existing text/property/selection colors, and `PreviewTagFontColor`, `PreviewTagBackgroundColor`, `PreviewTagBorderColor`. Generic preview tokens do not necessarily control specialized chat, terminal, media, or native/web surfaces.
- Glance has `GlanceFontColor`, `GlanceIconColor`, `GlanceBackgroundColor`, and `GlanceHoverBackgroundColor`. Raster images retain their own colors.
- Attention has `AttentionFontColor`, `AttentionIconColor`, `AttentionBackgroundColor`, `AttentionBorderColor`, `AttentionHoverBackgroundColor`, and `AttentionHoverBorderColor`. Omitted values match Glance: transparent idle fill/border and a query-text hover wash. Raster images retain their own colors.
- Shared scrollbars expose `ScrollbarThumbColor`, `ScrollbarThumbHoverColor`, `ScrollbarThumbActiveColor`, `ScrollbarWidth`, `ScrollbarHoverWidth`, and `ScrollbarBorderRadius`. Widths default to 3/7 logical units; omitted radius follows half the animated thickness. Zero width/radius and transparent colors are explicit. Horizontal bars use width as thickness. Native/web-owned scrollbars remain outside this contract.
- Filters use `RefinementButton{Font,Icon,Background,Border,HoverBackground}Color` and their `RefinementButtonActive...Color` counterparts. Active means expanded or a non-default filter is applied. The expanded strip uses `Refinement{Background,Border,Title,Divider,Hotkey}Color`; options use `RefinementItem{Font,Background,HoverBackground}Color` and `RefinementItemActive...Color`. These are JSON overrides; preserve explicit alpha for every state, including selected-hovered options. Control dimensions remain launcher-density geometry.
- `AppBorderColor`, `AppBorderWidth`, and `AppBorderRadius` control launcher outer chrome. Width/radius use logical units; zero disables the outline or makes corners square. Omitted fields keep the previous platform behavior. Set width explicitly when designing an outline. If any one of these three fields is authored, every platform disables system window material (Windows Acrylic, macOS Liquid Glass/vibrancy, Linux compositor blur) so Go UI can paint the outline. Windows still applies a DPI-scaled region plus DirectComposition clip only when a radius is authored; color or width alone leaves DWM's default corners. macOS detaches the glass/vibrancy wrapper and clips the renderer. Linux turns compositor blur off and can then paint rounded corners. Theme-color alpha still controls ordinary window transparency. Verify shadow and transparency on each target OS before promising identical results.

## Explain the custom-chrome tradeoff

When creating, editing, or recommending a theme that authors any of `AppBorderColor`, `AppBorderWidth`, or `AppBorderRadius`, explicitly tell the user that this disables system window material on every platform (Acrylic, Liquid Glass, Linux compositor blur). The native window switches to transparent composition; theme-color alpha controls ordinary window transparency. This does not disable Wox's own in-app frosted-glass surfaces. Include this limitation in the delivery message, not only in JSON or internal notes.

Toolbar and Action Panel retain their application-rendered frosted-glass transparency independently of the native window material. When designing custom-chrome themes, consider modest transparency in `ToolbarBackgroundColor` and `ActionContainerBackgroundColor` (for example, alpha around 0.88–0.90), while keeping text readable. Do not make these surfaces opaque merely because custom window chrome is enabled. An opaque `AppBackgroundColor` can remain intentional: these panels reveal or blur underlying app content, not the desktop through that opaque background.

An explicit zero (`AppBorderWidth: 0` or `AppBorderRadius: 0`) also selects this path. To restore system material, omit all three fields. A child platform or variant can restore material after a parent outline by setting those fields to `null`; that clears the inherited chrome instead of painting a square window. Assigning a default-looking number does not restore the material. Background colors still need alpha below 1 to reveal anything underneath.

Suggested user-facing wording: “主题只要设置了 AppBorderColor、AppBorderWidth 或 AppBorderRadius 中的任意一项，所有平台都不会再使用系统窗口材质（Windows Acrylic、macOS Liquid Glass、Linux 合成器模糊）。窗口改用普通透明合成，透明程度由主题颜色的 Alpha 决定。Toolbar 和 Action Panel 的应用内磨玻璃效果仍保留，可以适度保留透明度。若要恢复系统材质，需要移除这三项配置。子级平台或变体可以把这三项设为 null，用来取消父级圆角/描边并重新打开系统材质。”

## Platform overrides and compatibility

Resolution order is root authored values, then `windows`/`macos`/`linux`, then that platform's matching `variants` entry, then defaults. Read `theme_platform.go` for supported variant names. Omitted platform fields inherit the parent. JSON `null` on a platform or variant clears the inherited value so the schema default applies; this is how Hyprland can keep compositor blur after a shared Linux `AppBorderRadius`. Base colors may be overridden so dependent defaults derive from the effective palette. All platform variants are validated, including inactive ones.

Preserve authored values and platform nodes when editing or saving. Never flatten a resolved platform theme into the source document. When explicitly converting v1, retain its effective appearance with explicit v2 overrides where defaults differ; verify zero values, aliases, and legacy contextual color behavior rather than only changing `SchemaVersion`.

## Deliver and check

For local debugging, use the user's requested path; the normal user theme directory is `~/.wox/wox-user/themes/`. Use `<ThemeId>.json` for a plain color theme. For image-backed themes, deliver a `.wox-theme` package containing root `theme.json` and its assets; installed packages live under `<ThemeId>/`. Do not overwrite an unrelated theme or publish to the store as part of local authoring.

Validate JSON and use Wox's actual parser/resolver to check fields, colors, platform nodes, and version compatibility. Existing checks live in `wox.core/common/theme_v2_test.go` and `theme_document_test.go`; for changes to parsing run `go test -tags 'sqlite_fts5,wox_automation' ./common` from `wox.core`. That suite validates the implementation, not an arbitrary new file: load the authored file through Wox or a focused parser check as well.

When a running preview is available, inspect selected/hovered results, the Action Panel, keycaps, toolbar, preview/tags, and Glance. For transparency, compare against light and dark desktop content. Report which platforms were actually observed; do not claim native material behavior from a static JSON check.

Resource-backed themes are local-only for now: Cloud Sync skips their install, update, deletion, and snapshot payloads. Ordinary JSON themes continue to sync. Share image themes using the `.wox-theme` file.

## Resizable frames: stretch versus repeat

Schema v2 nine-slice images accept optional `"Repeat": {"X": "tile", "Y": "stretch"}`.
Each omitted axis defaults to `stretch`; accepted values are `stretch` and `tile`.
`Repeat` is invalid on whole-image `stretch` or `tile` modes. Existing declarations are unchanged.

Corners retain their authored logical Insets. X controls the top/bottom middle strips;
Y controls the left/right middle strips. The center follows both axes. Tiling starts
at the left/top of each slice; the final partial tile is clipped, never squeezed.
Top/bottom tile width equals source strip width multiplied by actual logical strip
height divided by source strip height. Left/right tile height uses the corresponding
width ratio. The center uses the first positive Insets/Slice ratio in Top, Bottom,
Left, Right order (or 1 if all are zero). Keep these ratios consistent for a uniform
material density. DPI is applied by the renderer; do not multiply Insets by display scale.
Very small windows shrink opposing borders proportionally. A pathological slice that
would require more than 4096 tile draws falls back to stretching that slice; avoid tiny
repeat units and verify the maximum supported window size.

Use stretching for flat fills and simple straight rules, tiling for roof tiles, woven
patterns, rivets or bamboo, and fixed-size Decorations for crests, lettering and figures.
Nine-slice does not turn an arbitrary illustration into a seamless material. The full
middle strip is the repeat unit: its two ends must match in height, color, lighting,
alpha and motif phase. Do not include isolated statues in it. Keep curved-to-straight
transitions entirely within fixed corner slices; put cut lines only in straight,
regular sections and preserve tangent continuity. Corners and adjacent strips need
matching scale as well as matching pixels. Inspect the join at both ends, not only
repeat-to-repeat seams. A clipped final motif can meet the far corner at a different
phase; prefer a neutral join or a fixed decorative cap where this would be visible.

For an irregular Action/About panel, set ActionContainerBackgroundColor and its border
color to transparent, remove the separate rectangular Background image, and provide
an opaque interior inside a transparent-exterior Frame. This also skips the rectangular
floating material. Leave padding for the ornaments. App ContentInsets reserve draggable
outer chrome; controls remain inside the reserved content bounds.

Verify minimum, default and wide launcher widths, short and tall result lists, fractional
DPI (125%, 150%), fixed crest proportions, partial tiles, alpha silhouette, and Action/
About content clearance. Check the actual rendered UI before claiming seamless joins;
image generation and parser validation alone cannot establish that.
