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
2. `launcher/theme.go` resolves user theme values into the portable palette.
3. `launcher/component.Theme` and shared typography expose semantic visual roles.
4. `launcher/component/wox_*.go` owns reusable control geometry, state visuals, focus, and accessibility.
5. `launcher/view/` composes pages and owns responsive, page-specific layout.

Do not create a second button, text field, dropdown, checkbox, switch, list item, panel, or dialog treatment in a view.

### Special product surfaces

Do not apply the ordinary control-size system to:

- the Launcher query and its accessories;
- Launcher results, toolbar, refinements, and Glance;
- the complete Action Panel, including its filter and action rows;
- native title-bar controls and platform-owned dialogs.

These surfaces keep purpose-built density and theme geometry. Check their internal rhythm and states, but never normalize them to the ordinary 32-unit control height.

Preserve the current special contracts unless a task explicitly targets them:

| Surface | Contract |
| --- | --- |
| Launcher query | 50 compact, 55 normal, 61 comfortable; add measured line height for each extra line |
| Action Panel header | 18 optically centered line; do not use a 16 Text slot |
| Action Panel filter | 40 input inside a 46-high slot |
| Action Panel row | 40 |

If a shared primitive serves both an ordinary page and a special surface, provide an explicit context-specific composition or semantic size instead of changing one default and relying on call-site overrides.

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
| Button | 32 high, 4 radius, 12 horizontal padding, 11 semibold label, 16 leading icon, 8 icon gap |
| Single-line Settings text field | 32 high, 4 radius, 1 border, 13 regular text |
| Dropdown | 32 high, 4 radius, 1 border, 13 regular value |
| Ordinary icon button | 32 by 32; center the icon and provide an accessible label |
| Compact icon action | 28 by 28; use only in an approved compact context |
| Settings/catalog search | 40 high; keep internal icon actions at least 28 by 28 |
| Checkbox | 18 by 18 visible mark inside at least a 32 by 32 interaction frame |
| Switch | Preserve approximately 36 by 24 visible geometry inside a 32-high alignment slot |
| Settings row | 64 high for a label plus one description line and one ordinary control |

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

Select color by semantic role. Do not copy RGBA values into ordinary views.

| Role | `component.Theme` value | Use |
| --- | --- | --- |
| Window canvas | `Background` | Launcher and Settings window background |
| Inline control or quiet card | `QueryBackground` | Fields, default rows, bounded quiet regions |
| Elevated surface | `ActionBackground` | Dialogs, popovers, temporary action surfaces |
| Primary text | `QueryText`, `ResultTitle`, `ActionText` | Titles, values, labels |
| Secondary text | `ResultSubtitle`, `ActionHeader`, `ToolbarText` | Help, metadata, section labels |
| Selection | `SelectedBackground`, `SelectedTitle`, `SelectedSubtitle` | Current destination, row, or option |
| Focus and caret | `Cursor` | Focus rings and text caret |
| Divider | `PreviewSplit` | Hairlines, table separators, structural borders |
| Error | `ErrorText` | Validation and operation failures |

Derive translucent overlays from the relevant semantic foreground or surface. Keep raw colors for platform-defined visuals or genuinely semantic fixed brand/status colors, and document those exceptions locally.

Use no more hierarchy than the interaction needs:

1. Window canvas.
2. Inline control or quiet bounded region.
3. Temporary elevated content.
4. Current selection.

Do not wrap every Settings group in a card. Use section spacing and a divider for ordinary groups. Avoid shadows and blur as basic hierarchy because their rendering differs across platforms.

Verify contrast in light and dark themes. Status must never depend on hue, alpha, or animation alone.

Read-only Markdown links use the shared document blue accent and an underline, with a native hand cursor on hover. Keep keyboard focus rings on the theme's `Cursor` color; the caret color must not determine link text color.

## Typography

Use the configured application font and shared constants in `launcher/component/typography.go`.

| Role | Size | Weight |
| --- | ---: | --- |
| Settings page title | 22 | Semibold |
| Primary body, label, value | 13 | Regular or semibold by emphasis |
| Help and secondary control text | 12 | Regular |
| Section label | 11 | Semibold, uppercase when already established |
| Supporting dense metadata | 10-11 | Regular or medium |

Use weight before adding another size. Keep ordinary body text at 13 and avoid text below 10. Apple uses 13 points as the default macOS body size and 10 points as the recommended minimum for custom type.

Use measured text and alignment containers. Do not position text with guessed baselines or platform-specific offsets. Define line height explicitly for wrapping text and test configured fonts with taller native metrics.

## Spacing, alignment, and shape

Use the 4-unit rhythm: 4, 8, 12, 16, 20, and 24. Allow 6, 10, and 14 only for established optical relationships such as icon gaps, dense text, or navigation alignment.

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

- Prefer categorized SVG icons from `wox.core/common/icons.go`.
- Place Settings help tooltips above their trigger, including table header/cell info icons and choice-picker options. If the top side overflows, flip below the trigger.
- Use 16-unit icons in ordinary controls, 18 in navigation, and 24 where an item needs stronger identity.
- Pair unfamiliar icons with text. Give icon-only controls an accessible label and visible hover/focus treatment.
- Preserve image aspect ratio and use physical-pixel snapping only in the renderer or platform boundary.
- Treat emoji and text glyphs as fallbacks, not substitutes for an existing product icon.
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
