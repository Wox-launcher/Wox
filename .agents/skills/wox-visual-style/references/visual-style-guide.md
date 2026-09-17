# Wox Go UI Visual Style Guide

This guide is the sole visual policy for portable Go UI under `wox.core/ui`. It defines the target contract; existing code can contain migration debt. Update this guide in the same change whenever a shared visual contract intentionally changes.

## Contents

- [Product character](#product-character)
- [Scope and ownership](#scope-and-ownership)
- [Control size system](#control-size-system)
- [Color and surfaces](#color-and-surfaces)
- [Typography](#typography)
- [Spacing, alignment, and shape](#spacing-alignment-and-shape)
- [Integer logical units](#integer-logical-units)
- [Interaction state matrix](#interaction-state-matrix)
- [Components, icons, and content](#components-icons-and-content)
- [Layout and platform behavior](#layout-and-platform-behavior)
- [Accessibility](#accessibility)
- [Review checklist](#review-checklist)

## Product character

Design Wox as a focused utility that stays out of the user's way:

1. Put content before decoration. Keep results, settings, values, and actions visually stronger than their containers.
2. Keep the interface compact, not crowded. Preserve readable type, clear grouping, and comfortable pointer targets.
3. Give one semantic role one treatment. Equivalent controls and states must look and behave alike across pages.
4. Use quiet hierarchy. Prefer spacing, type weight, semantic surfaces, and thin borders over shadows or ornamental cards.
5. Support keyboard and pointer use equally. Every pointer affordance needs an equivalent focus and activation path.
6. Keep product structure portable while retaining platform-native fonts, IME, dialogs, accessibility, and window controls.

Use Apple's macOS guidance as a principle reference, not as a pixel-for-pixel skin. Apple recommends consistent sizes within control groups, style rather than size to distinguish preferred actions, and a comfortable macOS control target of 28 by 28 points. See:

- https://developer.apple.com/design/human-interface-guidelines/buttons
- https://developer.apple.com/design/human-interface-guidelines/accessibility
- https://developer.apple.com/design/human-interface-guidelines/toggles
- https://developer.apple.com/design/human-interface-guidelines/text-fields
- https://developer.apple.com/design/human-interface-guidelines/typography

## Scope and ownership

Apply this guide to Settings, dialogs, forms, tables, catalogs, onboarding management controls, and other ordinary management pages.

Use these ownership layers in order:

1. This guide defines visual policy.
2. `launcher/theme.go` resolves launcher theme values into the portable palette. `launcher/settings_theme.go` independently owns the single Glass-derived dark Settings palette, with a translucent window tint, translucent popup tints over floating blur materials, and native desktop material enabled. Settings does not inherit launcher colors, geometry overrides, or system light/dark changes.
3. `launcher/component.ControlTheme` defines the semantic colors consumed directly by shared controls and Settings views. `component.Theme` retains launcher-only appearance and supplies its `Controls` field at shared-control call sites.
4. `launcher/component/wox_*.go` owns reusable control geometry, state visuals, focus, and accessibility.
5. `launcher/view/` composes pages and owns responsive, page-specific layout.

Settings and onboarding management surfaces construct `ControlTheme` directly in `settings_theme.go` and passes it to controls, dialogs, tooltips, and navigation. Do not convert Settings appearance into `uiPalette` or `component.Theme`, or add launcher-token fallbacks to Settings. Untinted SVGs resolve explicit theme variables against their owning surface; fixed brand colors stay unchanged. Onboarding passes a separate `PreviewTheme` to illustrative launcher demos; its management surface keeps the fixed dark tint and system material even while hidden. Linux Settings rails add no duplicate tint over the root background; they do not query compositor capabilities.

Theme catalog previews and the editor draft preview retain the theme being previewed while their surrounding Settings chrome uses the fixed palette.

The theme editor places save actions above a persistent preview and a 340-unit property inspector. Below 760 logical units of content width, stack the preview above the inspector. Properties use one vertical scroll region, searchable collapsible surface groups, standard-height controls, and per-property reset. Equal four-sided padding may be linked; asymmetric authored values remain independent until the user links them. Effective preview values must never replace omitted or null authored fields on save. Editor demos honor resolved logical spacing and explicit zero corners; native backdrop and shadow behavior still require platform validation.

Do not create a second button, text field, dropdown, checkbox, switch, list item, panel, or dialog treatment in a view.

### Special product surfaces

Do not apply the ordinary control-size system to:

- the Launcher query and its accessories;
- Launcher results, toolbar, refinements, Glance, and Attention;
- the complete Action Panel, including its filter and action rows;
- native title-bar controls and platform-owned dialogs.

These surfaces keep purpose-built density and theme geometry. Check their internal rhythm and states, but never normalize them to the ordinary 32-unit control height.

Preserve the current special contracts unless a task explicitly targets them:

| Surface | Contract |
| --- | --- |
| Launcher query | 50 compact, 55 normal, 61 comfortable; add measured line height for each extra line |
| Launcher structured argument | One continuous query editor; same font and baseline as command text, no border or layout padding. Every empty argument uses the same quiet ghost chip, including a lone variable such as Volume (0–100), so the slot reads as a hole to fill rather than Tab completion. Multiple filled arguments use the same quiet mark as blocks; a single filled argument stays unmarked |
| Launcher query Tab mark | One 14-square `control.keyboard-tab` glyph after the next successful Tab target, painted at query-text color with alpha 96/255 and a 4-unit gap. Center it on the query letter ink (baseline minus a quarter em), not the input box or the font line box. No keycap fill or border. Omit it on the current sole argument, an empty required argument, the last slot, or when Tab would only shake the caret |
| Launcher structured block | Semantic text-color background with alpha 10/255 (18/255 when active), extending 3 logical units horizontally without moving text, clipped to the editor and shared gaps between adjacent blocks |
| Launcher toolbar | Theme-tinted floating material with a top divider. With the query above results, list and grid content extend behind the footer; selection visibility and scrollbars use the unobscured viewport. Preview panes and bottom query chrome retain their usable height. The footer blocks interaction with covered results. |
| Launcher selected list result | `ResultItemActiveBorderLeftWidth` paints a full-height left marker in logical units; zero disables it. `ResultItemActiveBorderLeftColor` sets its independent color, falling back to `QueryBoxCursorColor` when omitted or empty. The marker follows the result background rounded silhouette, including its left corners; square themes retain a full rectangular marker. It belongs to the background, never shifts content, and does not appear for hover-only rows or group headings. Theme previews use the same treatment. |
| Launcher toolbar divider | `ToolbarBorderColor` sets the top divider independently from text; omitted or empty uses `ToolbarFontColor` with alpha capped at 26/255. `ToolbarBorderWidth` defaults to 1 logical unit; zero disables it without changing toolbar layout. |
| Action Panel header | 18 optically centered line; do not use a 16 Text slot |
| Action Panel filter | 40 input inside a 46-high slot |
| Action Panel outer border | `ActionContainerBorderColor` sets the independent edge color; omitted or empty follows `PreviewSplitLineColor`. `ActionContainerBorderWidth` is in logical units; omitted defaults to 1, zero disables it. V1 internal dividers continue using `PreviewSplitLineColor`; v2 exposes `ActionContainerDividerColor`. Theme previews use the same edge. |
| Action Panel corner geometry | `ActionContainerBorderRadius` controls panel corners, falling back to `ActionQueryBoxBorderRadius`. `ActionItemBorderRadius` controls action rows, falling back to `ResultItemBorderRadius`. Explicit zero produces square corners; all values are logical units and also apply to previews. |
| Action Panel row | 40; optional 18 plugin identity tail or usage-score text such as `+55` in the trailing gutter, same 10/5 inset as hotkeys |
| Action Panel group divider | 16-high slot with a 1-unit hairline; v2 uses `ActionContainerDividerColor`, v1 uses `PreviewSplit` |
| Action Panel verb icons | Monochrome `action.*` SVGs using `var(--wox-theme-icon-color)` as the untinted fallback. On the Action Panel, tint only those theme-adaptive SVGs with `ActionText` / `ActionSelectedText` so they match the row label. Do not source-in tint plugin, brand, or status identity icons; a filled brand SVG would collapse into a solid blob. The SVG variable itself stays appearance black/white and is not a per-row text color. Execute actions use `action.execute` (lightning), not settings or play. |

If a shared primitive serves both an ordinary page and a special surface, provide an explicit context-specific composition or semantic size instead of changing one default and relying on call-site overrides.

### Structured query continuity

`QueryHint` is a hinting, background layer, not a form layout. Command text
and arguments must remain one continuous editing surface. Preserve ordinary
caret movement, cross-element selection, clipboard operations, deletion, undo and
IME; Tab navigation is an optional convenience. Use quiet backgrounds and ghost
hints without separate input borders, font changes, baseline shifts or focus traps.
Never constrain a user's edit merely to retain a structure annotation. Explicit
atomic blocks do not make ordinary arguments atomic or justify separate editors.
This principle applies to future element kinds and plugin integrations as well.
When forward Tab has neither a completion nor another semantic value, give the
painted caret a 220 ms damped horizontal shake of at most 3 logical units. Keep
text, selection, and the IME anchor fixed; suppress the feedback during composition.
See [the structured query design](../../../../wox.core/ui/launcher/QUERY_HINT.md#design-principle-continuous-input-comes-first).

Arguments with `Suggestions` show their empty choices in one quiet ghost chip,
joined with ` / `. Empty previews show at most three complete candidates and
` / …` when truncated, reducing the count to fit available logical width.
Core-generated command previews prefer shorter names; this must not reorder
the candidate list used for matching. Once a prefix matches, render its suffix as
plain ghost text and place the shared Tab glyph after it. This completion also
shows Tab on the sole or last argument, since accepting it changes the value.
Exact matches and unmatched input have no completion glyph; ordinary navigation
may still advertise its next target. Paint-only space may separate a completion
from later argument text, but native values, caret coordinates and clipboard
content remain unchanged. Map pointer positions back across that space and clip
long suggestions to the query viewport. Do not add a popup, border or new font.

A sole command candidate is immediately actionable after the trigger separator:
paint its complete name as plain ghost text with the Tab glyph, even before a
prefix is typed. Do not paint the empty-argument chip for this case. Ordinary
arguments and multiple command candidates retain the empty preview treatment.

Position the command completion Tab glyph after the trailing context space that
acceptance appends, matching ordinary query completion. Include that space in
paint insertion width and pointer mapping; ordinary argument candidates do not
reserve an extra space unless it is part of their actual completion suffix.

## Control size system

Treat every value in this section as a logical UI unit. Convert to physical pixels only at platform or renderer boundaries.

### Size tiers

| Tier | Height | Use |
| --- | ---: | --- |
| Compact | 28 | Dense table-cell actions and tightly constrained utility toolbars only |
| Standard | 32 | Ordinary buttons, single-line text fields, dropdowns, and icon buttons |
| Search | 40 | Page-level Settings or catalog search composites |

Default to Standard. Require a concrete layout reason for Compact. Use Search only when the control searches the page or catalog, not for an ordinary form value.

Do not use size to distinguish primary and secondary actions. Keep adjacent actions in one group at the same height and express hierarchy through variant, fill, border, foreground, and weight.

### Control geometry

| Control | Target geometry |
| --- | --- |
| Button | 32 high, 4 radius, 12 horizontal padding, 11 regular label, 16 leading icon, 8 icon gap |
| Single-line Settings text field | 32 high, 4 radius, 1 border, 13 regular text |
| Dropdown | 32 high, 4 radius, 1 border, 13 regular value |
| Ordinary icon button | 32 by 32; center the icon and provide an accessible label |
| Compact icon action | 28 by 28; use only in an approved compact context |
| Settings/catalog search | 40 high; keep internal icon actions at least 28 by 28 |
| Checkbox | 18 by 18 visible mark inside at least a 32 by 32 interaction frame |
| Switch | Preserve approximately 36 by 24 visible geometry inside a 32-high alignment slot. Off tracks use a stronger text-alpha wash than disabled so they stay visible on glass |
| Settings row | 64 minimum; descriptions wrap and increase height with content |
| Settings choice or switch slot | 200 wide; right-align the control inside the slot |
| Settings choice menu | At least the trigger width; grow up to 360 to fit labels, trailers, and info icons |
| Settings form column | Fill the available width after the shared 40-unit page insets, including ordinary form pages |
| Settings catalog list | 220–250 wide; 30% of the inset catalog content width. Plugin and theme catalogs share this column |
| Settings runtime status cards | Three columns at the default 800-wide Settings content row |
| Settings window | 1100 by 760 default to leave room for plugin list and detail panes |
| Settings choice group label | 28 high; section-label type; not selectable |
| Settings navigation destination | 40 high; 13 regular |
| Settings navigation group | 28 high; section-label type; 8 lead before later groups; not selectable |

Visible checkbox and switch shapes do not stretch to the frame height. Align their interaction frames and visual centers with neighboring fields.

Expand a Settings row in 4-unit increments when it contains multiline help, validation feedback, progress, or a composite control. Do not shrink ordinary controls to make an undersized row fit.

### Migration awareness

The target system intentionally differs from parts of the current implementation. Known examples include 34-high dropdowns, 40-high Settings text fields, a 42-high shared search field, 62/66-high Settings rows, and toggles without a full interaction frame. Treat these as audit findings, not additional approved tiers.

When implementing migration work:

- centralize the tier values in shared component metrics or a semantic size API;
- remove redundant page-level height literals;
- update control tests and affected layout tests together;
- verify that Launcher and Action Panel geometry remains unchanged.

## Color and surfaces

Themes explicitly declaring `SchemaVersion: 2` derive optional styles from required `BaseBackgroundColor`, `BaseTextColor`, and `BaseAccentColor` through the shared core resolver. For theme authoring, follow [Wox Theme Creator](../../wox-theme-creator/SKILL.md). Legacy themes retain their existing fallbacks. Preserve missing authored values separately from effective swatches: an inherited editor color must stay omitted on save, and explicit zero or transparency must not trigger fallback. Platform overrides are merged before defaults are derived. V2 editor base colors are required; optional color dialogs provide Use default and inherited labels.

Select color by semantic role. Do not copy RGBA values into ordinary views.

| Role | `component.ControlTheme` value | Use |
| --- | --- | --- |
| Window canvas | `Background` | Window background |
| Inline control | `InputBackground`, `InputText` | Input surface and value |
| Elevated surface | `Surface` | Dialogs and popovers |
| Primary text | `Text` | Titles, values, labels |
| Control labels | `ControlText` | Button and menu labels |
| Rich content | `BodyText` | Markdown and documents |
| Window controls | `ChromeText` | Native title-bar glyphs |
| Secondary text | `TextSecondary` | Help, metadata, section labels |
| Selection | `SelectionBackground`, `SelectionText` | Current destination, row, or option |
| Primary action | `Accent`, `AccentText` | Primary buttons, checked controls; Settings uses opaque near-white with dark foreground, independently of row selection washes; active switch thumbs use `AccentText` |
| Status | `Info`, `Success`, `Warning` | Fixed Settings information and connection-state colors |
| Focus and caret | `Focus` | Focus rings and text caret |
| Text selection | `TextSelectionBackground`, `TextSelectionText` | Selected input text |
| Divider | `Border` | Hairlines and structural borders |
| Error | `Error` | Validation and operation failures |

Derive translucent overlays from the relevant semantic foreground or surface. Keep raw colors for platform-defined visuals or genuinely semantic fixed brand/status colors, and document those exceptions locally.

Use no more hierarchy than the interaction needs:

1. Window canvas.
2. Inline control or quiet bounded region.
3. Temporary elevated content.
4. Current selection.

Do not wrap every Settings group in a card. Use section spacing and a divider for ordinary groups. Avoid shadows and blur as basic hierarchy because their rendering differs across platforms.

Verify Settings contrast against its fixed dark palette and launcher controls against light and dark themes. Status must never depend on hue, alpha, or animation alone.

Read-only Markdown links use the shared document blue accent and an underline, with a native hand cursor on hover. Keep keyboard focus rings on the control theme's `Focus` color; the caret color must not determine link text color.

## Typography

Use the configured application font and shared constants in `launcher/component/typography.go`.

| Role | Size | Weight |
| --- | ---: | --- |
| Settings page title | 22 | Semibold |
| Primary body, label, value | 13 | Regular or semibold by emphasis |
| Help and secondary control text | 12 | Regular |
| Section label | 11 | Semibold, uppercase when the script has case; 13 semibold with no case transform for scripts without case, such as CJK |
| Settings table field title | 13 | Semibold on Wox-owned Settings pages; regular on plugin tables |
| Settings table column title | 13 | Regular |
| Settings navigation item | 13 | Regular for destinations; group headers use the section-label treatment |
| Settings plugin detail tab | 14 | Regular; selection uses the underline, not weight |
| Ordinary button label | 11 | Regular; opt into semibold only for a specific emphasis |
| Supporting dense metadata | 10-11 | Regular or medium |
| Compact table status tag | 9 | Regular; `WoxCompactTag` only |

Use weight before adding another size. Keep ordinary body text at 13 and avoid text below 10, except compact table status tags at 9. Apple uses 13 points as the default macOS body size and 10 points as the recommended minimum for custom type.

Use measured text and alignment containers. Do not position text with guessed baselines or platform-specific offsets. Define line height explicitly for wrapping text and test configured fonts with taller native metrics.

## Spacing, alignment, and shape

Use the 4-unit rhythm: 4, 8, 12, 16, 20, and 24. Allow 6, 10, and 14 only for established optical relationships such as icon gaps, dense text, or navigation alignment.

- Settings table Add controls use the shared secondary button fill instead of a bright outline. Selected Settings navigation uses its theme selection fill without a decorative border; retain the keyboard focus ring. Parent navigation rows are chrome, not destinations: no icon, hover, or selection. Table hotkeys use readable key labels separated by ` + `, preserving stored values.
- Settings pages fill the available width after the shared 40-unit insets. Keep trailing controls and section dividers aligned to the right content edge as the window resizes.
- Align repeated labels, controls, and actions to shared leading or trailing edges.
- Align controls in one row by interaction-frame centerline.
- Use built-in horizontal and vertical alignment primitives (`Align`, Flex alignment, `Expanded`, and `Constrained`) instead of manual offsets or calculated centering padding. Do not write formulas such as `(rowHeight-controlHeight)/2` to position a child; make the layout component express the relationship.
- Keep related label, value, and action content together; separate unrelated tasks into sections.
- Preserve room for validation and progress where it prevents avoidable layout jumps.
- Use a 1-unit border or divider for structure. Do not author hairlines such as `0.5` or `0.75`; they clip under coverage AA and CJK metrics, especially on the top edge.
- Use 4 radius for ordinary controls, 6-8 for rows and bounded panels, and the owning shared component's radius for dialogs or special surfaces.
- Do not introduce a new spacing, radius, or height value when an existing tier expresses the role.

## Integer logical units

Authored layout values must be whole logical units. Do not write fractional widths, heights, padding, gaps, radii, border or stroke widths, icon sizes, font sizes, or line heights such as `0.5`, `0.75`, `1.5`, `2.5`, `10.5`, or `12.5`.

This applies to Settings, Launcher, the Action Panel, and preview chrome. Convert to physical pixels only at the renderer or platform boundary.

Allowed to stay non-integer:

- alignment factors (`Horizontal: 0.5`, `Vertical: 0.5`)
- opacity and color-channel math
- animation progress and in-flight interpolated frames
- ratios that split available space
- measured text, image, parsed SVG content, and DPI conversions

When a formula would produce a half unit, express the relationship with `Align` or Flex alignment instead of fractional padding.

## Interaction state matrix

Implement every state that applies to a control in its shared component.

| State | Visual contract | Behavioral and semantic contract |
| --- | --- | --- |
| Default | Semantic foreground, surface, and border | Correct role, label, value, and action |
| Hover | Subtle overlay or border emphasis; preserve layout | Only actionable enabled controls react |
| Pressed | Immediate local feedback without geometry shift | Activation remains interruptible and fires once |
| Selected | Selection surface and selected foreground | Expose selected or checked semantics |
| Focused | Visible `Cursor` focus ring | Enter/Space activation where appropriate; logical focus order |
| Disabled | Reduced emphasis without losing legibility | No pointer or keyboard action; expose disabled semantics |
| Loading | Stable footprint and clear progress | Disable conflicting actions and prevent duplicate work |
| Empty | Quiet explanation with an optional next action | Do not present a disabled-looking blank control |
| Error | `ErrorText`, concise recovery copy, stable user value | Move or announce focus only when necessary for recovery |

Hover must not replace selected styling. Disabled controls must not retain hover or pressed feedback. Focus must remain visible over hover and selection. Loading indicators must not resize their controls.

If the widget runtime lacks a reusable pressed-state capability, improve the shared interaction layer or record the limitation. Do not simulate it independently on one page.

## Components, icons, and content

Use shared `Wox*` components before primitive widgets. A page may use a primitive `Gesture` for a page-specific region, drag target, or tooltip, but not to recreate a common control.

- Prefer categorized SVG icons from `wox/common/icons` via `icons.Get`. Add a new semantic name there before introducing a local asset. Action Panel verbs (`action.*`) are monochrome and must use `var(--wox-theme-icon-color)` because the panel does not tint plugin action images. Settings chrome (`control.*`, `settings.*`) stays a white mask for host tinting.
- For SVG paints that should adapt to Wox appearance, write `fill="var(--wox-theme-icon-color)"` or `stroke="var(--wox-theme-icon-color)"`. The shared launcher image pipeline resolves this explicit variable to white (`#ffffff`) in dark themes and black (`#000000`) in light themes, including inline, file, and Base64 SVGs. Preserve fixed brand colors and gradients; do not classify authored black/white paints to infer theme behavior or tint an entire mixed-color SVG. Standard SVG `currentColor` keeps its normal semantics and is not this Wox variable. Existing controls that intentionally tint an entire icon retain that behavior.
- Settings tables use an 8-unit rounded frame, 36-unit header, 40-unit rows, 12-unit horizontal text insets, and quiet horizontal separators without vertical grid lines. Header labels use secondary text; omit header info icons and their width reserve. Keep body dividers quieter than the header divider. Round the maximum viewport height down to whole rows at rest while retaining continuous scrolling. Standard row actions use secondary-color glyphs at rest and their normal glyphs on hover or keyboard focus; disabled and delete-confirmation states retain their own treatment. Keep field help in the row editor and preserve cell/action tooltips. Paint continuous rounded header/body surfaces behind transparent cells so cell fills do not square off the corners.
- Place Settings help tooltips above their trigger, including table cell info icons and choice-picker options. If the top side overflows, flip below the trigger.
- Use 16-unit icons in ordinary controls, 18 in navigation, and 24 where an item needs stronger identity.
- Pair unfamiliar icons with text. Give icon-only controls an accessible label and visible hover/focus treatment.
- Settings choice menus may group long catalogs with 28-high section labels. Labels are chrome, not options.
- Preserve image aspect ratio and use physical-pixel snapping only in the renderer or platform boundary.
- Treat emoji and text glyphs as fallbacks, not substitutes for an existing product icon.
- Markdown and editable Notes reserve 16 logical units for bullet markers and at least 24 for ordered markers, expanding for longer labels. Nested lists reuse the same compact gutter and hanging alignment.
- Keep translated labels visible. Size text buttons to content, constrain fields by expected input length, and truncate secondary content before primary values.

## Layout and platform behavior

- Build responsive layout from available width, not a named operating system.
- Keep keyboard selection visible inside the collection's own scroll region.
- Trap focus in modal content, choose an intentional initial focus target, and restore focus on dismissal.
- Cover loading, empty, populated, error, disabled, narrow, and long-content layouts during design.
- Keep one portable widget tree on macOS, Windows, and Linux unless the difference is genuinely platform-owned.
- Keep window controls, font resolution, IME, native dialogs, clipboard, accessibility bridges, and renderer compositing behind runtime capabilities.
- Custom-drawn macOS traffic lights follow the native key-window contract: red/yellow/green (or unavailable gray) while the window is key, and a uniform inactive gray when it is not. Hovering the group restores the colored glyphs, matching AppKit.
- Distinguish logical units from physical pixels and test non-100% scaling, mixed-DPI displays, display transitions, and negative desktop origins when coordinates or capture are involved.

Document deliberate visual divergence next to the platform implementation.

## Accessibility

- Give every actionable control a semantic role, label, enabled state, and keyboard action.
- Follow reading order for keyboard focus. Keep Launcher result rows out of the focus chain when launcher selection already owns navigation.
- Make ordinary pointer targets at least the owning size tier. Use at least 28 by 28 for Compact controls.
- Activate focused buttons, toggles, and list items with Enter and Space where appropriate; dismiss temporary surfaces with Escape.
- Keep focus visible in every theme.
- Communicate state with shape, text, iconography, or semantics in addition to color.
- Preserve user values when validation fails.

## Review checklist

Before completing a visual change, confirm:

- the surface is classified as ordinary UI, Launcher, Action Panel, or platform-owned;
- each ordinary control uses Standard, approved Compact, or Search geometry;
- controls in one group share height, centerline, spacing rhythm, and hierarchy;
- authored geometry uses whole logical units (no fractional pixels);
- reusable visuals and states live in `launcher/component`, not a view;
- colors and typography come from semantic shared contracts;
- default, hover, pressed, selected, focused, disabled, loading, empty, and error states are covered where applicable;
- icons use the shared SVG pipeline and icon-only actions have labels;
- long text, translation, narrow width, scrolling, light/dark themes, and DPI boundaries are considered;
- tests assert shared contracts without changing Launcher or Action Panel geometry accidentally;
- this guide is updated in the same change if the intended shared contract changed.

V2 keycaps expose independent `ToolbarHotkey{Font,Background,Border}Color`, `ActionItemHotkey{Font,Background,Border}Color`, and `ActionItemActiveHotkey{Font,Background,Border}Color` groups. Explicit transparency must survive fallback. `ResultItemHoverBackgroundColor` only paints unselected hovered list backgrounds and grid frames; selected styling retains precedence. V1 retains all contextual fallback colors.

Toolbar and Action Panel keycaps use regular weight, 20 logical-unit height and minimum width, 10 total horizontal inset, and 4-unit key gaps; their outer alignment slot remains 28 (22 compact). Unthemed hotkey recorders retain their existing geometry and semibold weight. Keycap colors remain theme-owned. Toolbar primary actions (default/bare Enter) use optional `ToolbarPrimaryFontColor` and `ToolbarPrimaryHotkey{Font,Background,Border}Color`. Omitted primary colors inherit the corresponding effective toolbar colors, including platform overrides. Explicit transparency survives fallback. Other actions use ordinary toolbar colors without automatic dimming; hover adds the existing surface overlay without changing authored text alpha. Theme editor and catalog previews use the same primary styling.

V2 generic preview surfaces expose independent background and border colors. Preview metadata tags expose independent text, background and border colors, with exact authored alpha. Glance exposes text, SVG icon tint, normal background and hover background independently from QueryBox. Attention uses the same idle/hover chrome as Glance: omitted fill and border stay transparent, hover wash uses query text at 10% alpha, and count text/icon follow query-text opacity unless overridden. Theme authors may still set Attention fill or border tokens; a zero-alpha border paints no stroke. Geometry stays with launcher density (30-high, radius 5, 16 icon) so the two query accessories read as one family.

V2 Filters buttons expose independent normal/active text, icon, background, border, and hover background colors. Expanded refinements expose group background, border, title, divider, hotkey, and normal/selected/hovered option colors. Explicit alpha is preserved, including selected hover. V1 keeps contextual opacity rules; launcher density still owns control geometry.

Shared scrollbars accept v2 normal, hover, and dragging thumb colors plus logical thickness, hovered thickness, and radius. Preserve authored alpha; the visibility animation multiplies it without the legacy 150/255 cap. Omitted radius follows animated thickness/2. Keep the drag target at least 12 logical units and large enough for the visual thumb on both axes. Theme values travel through ScrollViewProps; do not read mutable app theme state inside the shared widget.

Launcher outer chrome exposes `AppBorderColor`, `AppBorderWidth`, and `AppBorderRadius`. Draw the inset outline after content; never enlarge layout padding to accommodate it. Clamp painted radius to half the window's smaller dimension. If any one of those three fields is authored, every platform disables system window material so Go UI can paint the outline. Windows updates a DPI-scaled region on resize/DPI changes only when a radius is authored; macOS detaches Liquid Glass/vibrancy and clips the renderer; Linux turns compositor blur off and can then paint rounded corners. Nil geometry preserves previous behavior; explicit zero stays zero and still disables system material. Theme previews use authored outline and radius values.

V2 `AppContentInset`, `AppContentBackgroundColor`, and `AppContentBorderRadius` style an inner launcher panel without changing native window material. Defaults are zero inset, transparent fill, and zero radius. Reserve the inset around all sections and floating panels, include it in window height, and translate native occlusion rectangles into window space. The panel background and toolbar bottom corners use the content radius; child surfaces keep their own corner styles. Existing AppPadding values remain internal spacing. Theme demos use the same content bounds and background composition.

Windows custom chrome also clips the DirectComposition root, including overlay visuals, and disables System Backdrop, Accent blur, and extended glass frames. It retains transparent composition without native blur; returning to omitted AppBorder* restores the normal material. Preserve the clip through renderer resize and device recreation. A pure Go theme preview cannot verify native backdrop or shadow behavior.

When exposing or recommending custom chrome, explicitly explain the all-platform tradeoff to users: system-material translucent blur is unavailable, while theme-color alpha transparency remains available. Authoring any of the three outline fields, including an explicit zero, selects this path. Restoring system material requires all three overrides to be unset, not merely assigning a default-looking number. See the theme-creator skill for authoring and delivery wording.

V2 generic previews and metadata tags accept `PreviewBorderRadius` and `PreviewTagBorderRadius` in logical units. Omission retains radius 8; zero is square; radii clamp to half the surface size. Font sizing remains owned by Interface size, never by theme overrides.

V2 selected-result markers use `ResultItemActiveIndicatorColor/Width/InsetLeft/InsetTop/InsetBottom/BorderRadius`; v1 retains its edge-border fields. Insets and radius default to zero, width to zero, color to base accent. Render on the background layer without changing row layout; demo and launcher share the geometry. `QueryBoxBorderBottomColor/Width` paint the query bottom edge inside its bounds, defaulting to accent/zero and preserving explicit transparency.

V2 selected Action Panel keycaps use their own active tokens; normal action and Toolbar tokens cannot override them. V1 retains caller-provided surface colors. Preserve the selected white-on-blue contrast in light themes with explicit active keycap colors.

Settings navigation uses a quiet white selection wash (alpha 32/255). Default dropdown outlines use value-text alpha 80/255 and search outlines use secondary-text alpha 100/255; hover fill and keyboard focus remain distinct. Built-in settings scroll to measured row keys so wrapped descriptions do not invalidate navigation offsets.

Settings search results replace the navigation list while open and use the rail material instead of an opaque black popup. Settings pages and the navigation rail expose the shared fading scrollbar. Theme catalog actions use secondary buttons and explicitly label the active theme Applied. Keep theme previews top-aligned with a 12-unit gap; the Usage heatmap panel is 204 units high. Plugin and theme catalog lists share the 220–250 column. Their System tags keep secondary text on selected rows so the badge does not invert with the title. Runtime status cards use three columns at the default Settings content width so Node.js, Python, and Script stay on one row. AI MCP tables show an empty-state message and use a secondary Edit JSON action; skill descriptions flex to available width and expose their full text through the shared tooltip.

Settings page scrollbars occupy the right gutter: content retains 40-unit horizontal insets while the scroll viewport extends to 8 units from the right window edge. The 32-unit spare strip keeps the scrollbar and its hit target clear of controls.

### Settings action buttons

Ordinary actions across Settings, catalogs, onboarding management, and dialogs use `ButtonSecondary`: a quiet fill without a resting outline. Primary actions retain `ButtonPrimary`; metadata links retain `ButtonText`. Focus rings, input borders, selection controls, and information tags remain distinct from action-button chrome. Table secondary actions follow this default without page-specific overrides.
