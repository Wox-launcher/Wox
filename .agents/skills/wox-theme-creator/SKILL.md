---
name: wox-theme-creator
description: Design, create, and refine Wox theme JSON files, including schema v2 base colors, optional styles, transparency, platform overrides, and local debugging. Use for authoring themes or converting an existing theme while preserving its appearance.
---

# Wox Theme Creator

Create a deliberately designed theme with a coherent palette, readable states, and a valid authored JSON document. Prefer schema v2 for new themes. Preserve the requested output location and existing theme identity when editing; generate a fresh UUID for a new theme.

## Source of truth

Paths below are relative to the Wox repository root. Read the current implementation before choosing fields; do not invent theme tokens or copy resolved runtime values into a new theme.

- `wox.core/common/theme_schema_v2.go`: complete v2 document, accepted fields, color/geometry defaults, validation, and platform resolution.
- `wox.core/resource/themes/`: built-in themes as working examples.
- `wox.core/common/theme_schema_v1.go`: legacy wire format and behavior, needed when converting an old theme.
- `wox.core/common/theme_runtime.go` and `theme.go`: independent runtime representation, schema dispatch, and Wox version checks. These are not the authored JSON schema.

Each schema owns its complete document and parser. Missing `SchemaVersion` or historical zero means v1. Loading must not rewrite old files. Changing v2 default semantics would change sparse themes; a new format belongs in a separately registered schema, not a retrofit of v1 or v2.

## Author the document

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

Group overrides by surface: window, query/Glance, result container/items, Action Panel/items/query, preview/tags, toolbar/keycaps. Keep the document sparse; add overrides for intentional design differences.

| Authored value | Meaning |
| --- | --- |
| Missing or `null` optional style | Inherit/default |
| Integer `0` | Explicit zero; never substitute a default |
| `transparent` or alpha zero | Explicit transparency |
| Empty color, negative/fractional geometry | Invalid |

Colors accept `#RRGGBB`, `#RRGGBBAA` (alpha last), `rgb(r,g,b)`, `rgba(r,g,b,a)`, and `transparent`. RGB channels are 0–255; alpha is 0–1. Geometry uses logical units, not physical screen pixels.

Font sizes are controlled by the application Interface size setting; do not add theme font-size overrides.

## Default window shape and transparency

Unless the user explicitly requests a custom app outline or window corners, omit `AppBorderColor`, `AppBorderWidth`, and `AppBorderRadius` at the root and all platform/variant levels. Do not infer a custom window outline or radius from a reference screenshot or a general request for a rounded visual style. Keep the system/default window shape and use a slightly translucent `AppBackgroundColor` by default (for example, alpha 0.90).

If the user explicitly requests a custom app outline or window corners, author the requested `AppBorder*` fields and default `AppBackgroundColor` to fully opaque (alpha 1). An explicit request for transparency or opacity overrides that default. QueryBox, result, preview, and Action Panel corner settings do not count as a request for custom window chrome. Toolbar and Action Panel may retain modest in-app translucency in either case.

These are theme-authoring defaults, not changes to schema parsing. Apply them when creating themes or when the user asks to restyle an existing theme; do not silently rewrite other existing themes. Keep `BaseBackgroundColor` independent when an explicit app background override is sufficient, so changing window alpha does not unintentionally change every derived surface.

## Design the surfaces together

Start with background, text, and accent roles. Optional colors derive independently from these roles; changing one optional token does not change another. Base alpha also affects derived colors. Consult the resolver for exact defaults instead of duplicating its entire catalog here.

- For translucent themes, consider app, query, Action Panel, action query, preview, and toolbar backgrounds together. An opaque surface can hide translucency beneath it; layered alpha and native materials affect the final appearance.
- V2 uses `ResultItemActiveIndicatorColor`, `ResultItemActiveIndicatorWidth`, `ResultItemActiveIndicatorInsetLeft`, `ResultItemActiveIndicatorInsetTop`, `ResultItemActiveIndicatorInsetBottom`, and `ResultItemActiveIndicatorBorderRadius` for the selected-result marker. Width defaults to 0, color to the base accent, and insets/radius to 0. Zero insets/radius reproduce the edge strip; use positive insets and radius for a short rounded marker. Reserve icon space with result-item padding; marker geometry does not shift content. The unreleased v2 `ResultItemActiveBorderLeftWidth/Color` fields were removed; v1 keeps its original fields. Check normal, selected, and hovered rows separately; `ResultItemHoverBackgroundColor` controls hover.
- `QueryBoxBorderBottomColor` and `QueryBoxBorderBottomWidth` draw an inside bottom edge without changing query layout. Width defaults to 0; color defaults to the base accent. Zero disables it and explicit transparency is preserved.
- Action Panel border fields are `ActionContainerBorderColor`, `ActionContainerBorderWidth`, and `ActionContainerBorderRadius`. `ActionContainerDividerColor` controls internal separators independently of `PreviewSplitLineColor`.
- Keycaps have three independent v2 groups: `ToolbarHotkey{Font,Background,Border}Color`, `ActionItemHotkey{Font,Background,Border}Color`, and `ActionItemActiveHotkey{Font,Background,Border}Color`. All accept explicit transparency. Omitted values derive from base colors, not another surface's overrides: normal text uses secondary text and border uses divider; active text/border use base text; backgrounds default transparent. Set active colors explicitly for contrasting selection backgrounds (for example, white keycaps on blue). The unreleased shared `Hotkey*` fields were removed. Keep keycaps readable without competing with labels.
- `PreviewBorderRadius` and `PreviewTagBorderRadius` control generic preview and metadata-tag corners in logical units. Omitted values retain 8; zero is square.
- Preview has `PreviewBackgroundColor`, `PreviewBorderColor`, existing text/property/selection colors, and `PreviewTagFontColor`, `PreviewTagBackgroundColor`, `PreviewTagBorderColor`. Generic preview tokens do not necessarily control specialized chat, terminal, media, or native/web surfaces.
- Glance has `GlanceFontColor`, `GlanceIconColor`, `GlanceBackgroundColor`, and `GlanceHoverBackgroundColor`. Raster images retain their own colors.
- Shared scrollbars expose `ScrollbarThumbColor`, `ScrollbarThumbHoverColor`, `ScrollbarThumbActiveColor`, `ScrollbarWidth`, `ScrollbarHoverWidth`, and `ScrollbarBorderRadius`. Widths default to 3/7 logical units; omitted radius follows half the animated thickness. Zero width/radius and transparent colors are explicit. Horizontal bars use width as thickness. Native/web-owned scrollbars remain outside this contract.
- Filters use `RefinementButton{Font,Icon,Background,Border,HoverBackground}Color` and their `RefinementButtonActive...Color` counterparts. Active means expanded or a non-default filter is applied. The expanded strip uses `Refinement{Background,Border,Title,Divider,Hotkey}Color`; options use `RefinementItem{Font,Background,HoverBackground}Color` and `RefinementItemActive...Color`. These are JSON overrides; preserve explicit alpha for every state, including selected-hovered options. Control dimensions remain launcher-density geometry.
- `AppBorderColor`, `AppBorderWidth`, and `AppBorderRadius` control launcher outer chrome. Width/radius use logical units; zero disables the outline or makes corners square. Omitted fields keep the previous platform behavior. Set width explicitly when designing an outline. If any one of these three fields is authored, every platform disables system window material (Windows Acrylic, macOS Liquid Glass/vibrancy, Linux compositor blur) so Go UI can paint the outline. Windows still applies a DPI-scaled region plus DirectComposition clip only when a radius is authored; color or width alone leaves DWM's default corners. macOS detaches the glass/vibrancy wrapper and clips the renderer. Linux turns compositor blur off and can then paint rounded corners. Theme-color alpha still controls ordinary window transparency. Verify shadow and transparency on each target OS before promising identical results.

## Explain the custom-chrome tradeoff

When creating, editing, or recommending a theme that authors any of `AppBorderColor`, `AppBorderWidth`, or `AppBorderRadius`, explicitly tell the user that this disables system window material on every platform (Acrylic, Liquid Glass, Linux compositor blur). The native window switches to transparent composition; theme-color alpha controls ordinary window transparency. This does not disable Wox's own in-app frosted-glass surfaces. Include this limitation in the delivery message, not only in JSON or internal notes.

Toolbar and Action Panel retain their application-rendered frosted-glass transparency independently of the native window material. When designing custom-chrome themes, consider modest transparency in `ToolbarBackgroundColor` and `ActionContainerBackgroundColor` (for example, alpha around 0.88–0.90), while keeping text readable. Do not make these surfaces opaque merely because custom window chrome is enabled. An opaque `AppBackgroundColor` can remain intentional: these panels reveal or blur underlying app content, not the desktop through that opaque background.

An explicit zero (`AppBorderWidth: 0` or `AppBorderRadius: 0`) also selects this path. To restore system material, omit all three fields or set them to `null` at every applicable root/platform/variant level. Assigning a default-looking number does not restore the material. Background colors still need alpha below 1 to reveal anything underneath.

Suggested user-facing wording: “主题只要设置了 AppBorderColor、AppBorderWidth 或 AppBorderRadius 中的任意一项，所有平台都不会再使用系统窗口材质（Windows Acrylic、macOS Liquid Glass、Linux 合成器模糊）。窗口改用普通透明合成，透明程度由主题颜色的 Alpha 决定。Toolbar 和 Action Panel 的应用内磨玻璃效果仍保留，可以适度保留透明度。若要恢复系统材质，需要移除这三项配置。”

## Platform overrides and compatibility

Resolution order is root authored values, then `windows`/`macos`/`linux`, then that platform's matching `variants` entry, then defaults. Read `theme_platform.go` for supported variant names. Omitted or null platform fields inherit the parent; base colors may be overridden so dependent defaults derive from the effective palette. All platform variants are validated, including inactive ones.

Preserve authored values and platform nodes when editing or saving. Never flatten a resolved platform theme into the source document. When explicitly converting v1, retain its effective appearance with explicit v2 overrides where defaults differ; verify zero values, aliases, and legacy contextual color behavior rather than only changing `SchemaVersion`.

## Deliver and check

For local debugging, use the user's requested path; the normal user theme directory is `~/.wox/wox-user/themes/`. Use `<ThemeId>.json` for a new file. Do not overwrite an unrelated theme or publish to the store as part of local authoring.

Validate JSON and use Wox's actual parser/resolver to check fields, colors, platform nodes, and version compatibility. Existing checks live in `wox.core/common/theme_v2_test.go` and `theme_document_test.go`; for changes to parsing run `go test -tags 'sqlite_fts5,wox_automation' ./common` from `wox.core`. That suite validates the implementation, not an arbitrary new file: load the authored file through Wox or a focused parser check as well.

When a running preview is available, inspect selected/hovered results, the Action Panel, keycaps, toolbar, preview/tags, and Glance. For transparency, compare against light and dark desktop content. Report which platforms were actually observed; do not claim native material behavior from a static JSON check.
