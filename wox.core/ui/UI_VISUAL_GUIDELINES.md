# Wox Go UI Visual Guidelines

This document is the source-level visual contract for Wox Go UI. It records the
product language already expressed by the launcher theme, shared Wox controls,
and Settings window. New UI should extend this language instead of introducing
page-local styles.

The guidelines cover portable Go UI under `wox.core/ui`. Native title-bar
controls, renderer implementation details, and user-authored launcher themes
remain platform or theme contracts.

## Principles

1. **Content first.** Keep decoration quiet so query results, settings, and
   actions remain the strongest visual elements.
2. **One semantic role, one treatment.** Equivalent controls and states should
   use the same shared component and theme role across every page.
3. **Compact, not crowded.** Wox is keyboard-first, but pointer targets and text
   must remain comfortable and readable.
4. **State must not depend on color alone.** Selection, focus, disabled, error,
   and progress states also need shape, text, semantics, or motion cues.
5. **Platform-native where it matters.** Window controls, fonts, IME, and native
   dialogs follow platform behavior; product layout and component hierarchy stay
   portable.
6. **Prefer existing contracts.** Do not add a token, component variant, or
   setting until the current theme and shared components cannot express the
   required role.

## Sources of truth

Use these layers in order:

1. `launcher/theme.go` resolves user theme data into the portable palette.
2. `launcher/component.Theme` exposes semantic colors to shared controls.
3. `launcher/component/wox_*.go` owns reusable control geometry, visuals,
   interaction, focus, and accessibility behavior.
4. `launcher/view/` composes pages from shared controls and immutable props.

Views should not define a second button, field, switch, panel, dialog, or list
item treatment. A page-specific value is acceptable only when it describes the
page's layout rather than a reusable control.

## Color

Always select color by semantic role. Do not copy RGBA values into views.

| Role | `component.Theme` value | Use |
| --- | --- | --- |
| Window canvas | `Background` | Launcher and Settings window background |
| Control or card surface | `QueryBackground` | Fields, quiet panels, default list rows |
| Elevated surface | `ActionBackground` | Popovers, dialogs, action surfaces |
| Primary text | `QueryText`, `ResultTitle`, `ActionText` | Titles, values, primary labels |
| Secondary text | `ResultSubtitle`, `ActionHeader`, `ToolbarText` | Descriptions, metadata, section labels |
| Selection | `SelectedBackground`, `SelectedTitle`, `SelectedSubtitle` | Current row, option, or destination |
| Focus and caret | `Cursor` | Focus rings and text caret |
| Divider | `PreviewSplit` | Hairlines, borders, table separators |
| Error | `ErrorText` | Validation and operation failures |

Translucent surfaces should derive alpha from a semantic theme color. Raw
colors are reserved for platform-defined visuals such as macOS traffic lights
or Windows destructive window controls.

Text and interactive elements must retain sufficient contrast in both light and
dark themes. When a theme value is missing or invalid, use the existing palette
fallback instead of inventing a page-local fallback.

## Typography

Use the configured application font and the following hierarchy:

| Role | Size | Weight | Example |
| --- | ---: | --- | --- |
| Page title | 22 | Semibold | Settings page title |
| Primary control/body | 13 | Regular | Buttons, navigation, field values |
| Emphasized body | 13 | Semibold | Important row or title |
| Dense label | 12 | Regular or semibold | Table header, compact result title |
| Section label | 11 | Semibold, uppercase | Settings section divider |
| Supporting metadata | 10-11 | Regular | Secondary dense metadata |

Use weight before introducing another size. Supporting text may be smaller than
11 only when space is inherently constrained and the content is non-essential.
Do not vertically position text with guessed font baselines; rely on measured
text and alignment containers.

## Spacing and alignment

Start with the 4 px rhythm: `4`, `8`, `12`, `16`, `20`, and `24`. Existing
optical values `6`, `10`, and `14` are valid for tightly related text, compact
control padding, and rail alignment. Do not introduce another spacing value
without a concrete layout constraint.

- Settings pages use a 20 px outer inset.
- The Settings navigation rail uses a 14 px inset and 4 px between destinations.
- Keep label and description text left-aligned within a section.
- Align repeated controls to a common leading edge and width.
- Keep related label/value/action content in one row; separate different tasks
  into sections instead of adding decorative containers.
- Preserve space for loading, validation, and empty states so asynchronous
  updates do not cause avoidable layout jumps.

## Shape and geometry

Use the existing component defaults:

| Element | Geometry |
| --- | --- |
| Normal button | 38 px high, 4 px radius |
| Compact button | 30 px high, 4 px radius |
| Navigation row | 46 px high, 6 px radius |
| General list item | 7 px radius |
| Panel, query surface, dialog | 8 px radius unless the owning component specifies otherwise |
| Switch | 42 x 22 px track, 18 px thumb |
| Settings title bar | 40 px high |
| Settings page header | 72 px high |
| Shared table header and row | 36 px high |

Use a 1 px divider or border for structure. Avoid shadows and blur as basic
hierarchy tools because their rendering differs across platforms; prefer
surface color, spacing, and borders.

Launcher density scales launcher-owned query, result, refinement, and toolbar
geometry. Settings controls keep their shared component geometry unless a
separate cross-page density contract is introduced.

## Surfaces and hierarchy

Use no more surface levels than the interaction requires:

1. Window canvas: `Background`.
2. Inline controls and quiet cards: `QueryBackground`.
3. Temporary or elevated content: `ActionBackground`.
4. Current selection: `SelectedBackground`.

Do not wrap every settings group in a card. Section headers, spacing, and a
divider are enough for ordinary groups. Use `WoxPanel` when content needs a
bounded surface, such as a preview, table, or temporary interaction.

## Components and interaction states

Use shared `Wox*` components before primitive widgets. Shared components own the
complete state contract:

- **Default:** semantic foreground and surface colors.
- **Hover:** a subtle surface change; it must not replace selection.
- **Selected:** selected semantic colors and `Selected` accessibility state.
- **Focused:** a visible `Cursor` focus ring and keyboard activation.
- **Disabled:** reduced emphasis, no activation action, and disabled semantics.
- **Error:** `ErrorText`, concise recovery text, and no loss of the user's value.
- **Loading:** retain the control's position and disable conflicting actions.
- **Empty:** explain what is absent and, when useful, offer the next valid action.

Animation should explain a state transition, not decorate it. Keep it short,
interruptible, and local. Respect an eventual reduced-motion platform capability
before adding non-essential motion.

## Icons and imagery

- Prefer the existing icon pipeline and structured `woxui.Image` values.
- Use 16 px icons in controls, 18 px icons in navigation, and 24 px icons where
  an item needs stronger identity.
- Pair unfamiliar icons with text. Icon-only controls require an accessible
  label and a clear hover or focus treatment.
- Preserve image aspect ratio and use renderer pixel snapping where the platform
  implementation provides it.
- Emoji or text glyphs are fallbacks, not replacements for an available product
  icon.

## Layout behavior

- Build responsive layouts from available width; do not branch on a named
  platform when the real constraint is size.
- Truncate secondary text before primary labels or values. Show the full value
  through an existing detail, preview, or tooltip path when available.
- Scroll long collections inside their owning region and keep keyboard selection
  visible.
- Modal content must trap focus, choose an intentional initial focus target, and
  restore focus when dismissed.
- Loading, empty, populated, error, disabled, and long-content states are part of
  the visual contract, not follow-up polish.

## Platform differences

Portable views must render from the same widget tree on macOS, Windows, and
Linux. Platform-specific treatment is limited to behavior users expect from the
operating system:

- native window controls and draggable title areas;
- system font measurement and application font resolution;
- native dialogs, clipboard, IME, accessibility, and WebView ownership;
- renderer-required opacity or compositing differences.

Keep those differences behind runtime capabilities or thin platform files.
Document any deliberate visual divergence next to the platform implementation.

## Accessibility

- Every actionable control needs a semantic role, label, enabled state, and
  keyboard action.
- Keyboard focus order follows reading order. Launcher result rows remain out of
  the focus chain when launcher selection already owns keyboard navigation.
- Enter and Space activate focused buttons, list items, and switches where
  appropriate; Escape dismisses temporary surfaces.
- Focus must remain visible in every theme.
- Do not communicate status only through hue, translucency, or animation.
- Pointer targets must not be smaller than the owning shared component's
  standard geometry.

## Adding or changing UI

Before implementing a visual change:

1. Identify the semantic role and the nearest existing `Wox*` component.
2. Check the default, hover, selected, focused, disabled, loading, empty, error,
   and long-content states that apply.
3. Put reusable appearance and interaction in `launcher/component`; keep page
   composition in `launcher/view`.
4. Add a shared token only after the same stable value is needed by multiple
   production surfaces.
5. Format and run the narrowest relevant source-level tests.
6. Use a serial runtime comparison only for effects, native lifecycle, font/DPI
   measurement, or other behavior that source inspection cannot establish.