# Wox Go UI Visual Style Guide

This guide defines the top-level visual policy for Wox's portable Go UI: product character, shared control conventions, and ownership boundaries.

Add a rule here only when it applies across unrelated features. Keep individual page layouts, icon colors, theme-field semantics, platform workarounds, and bug-fix explanations beside their owning code, tests, or domain documentation. This guide is not a change log or an inventory of implementation details.

## Product character

- Put content before decoration. Use quiet surfaces, spacing, and typography to establish hierarchy.
- Keep the interface compact without crowding text or reducing interaction targets.
- Give equivalent controls the same treatment across pages. Prefer shared components over local variants.
- Support keyboard and pointer use equally, with visible focus and clear action feedback.

## Scope and ownership

Classify the surface before applying ordinary control metrics:

- Settings, catalogs, dialogs, forms, tables, and onboarding management use the shared ordinary control system.
- Launcher query, results, accessories, and previews follow launcher density and theme geometry.
- The Action Panel has its own density and geometry contract.
- Native window controls and dialogs follow platform conventions.

Do not normalize special surfaces to Settings dimensions or change shared defaults to solve one page's layout problem.

Keep responsibilities in these layers:

- `launcher/component` owns reusable control appearance, geometry, interaction states, and accessibility.
- `launcher/view` owns page composition and responsive layout through immutable props and callbacks.
- Launcher controllers prepare data and callbacks; runtime adapters own native rendering and window behavior.
- `launcher/theme.go` resolves launcher appearance. `launcher/settings_theme.go` owns the independent Settings palette through `component.ControlTheme`.

Settings chrome uses its fixed dark palette, independent of the active launcher theme. Theme previews retain the appearance being previewed. Keep mutable application state out of cached widget builds.

## Control size system

All authored dimensions are logical UI units.

| Tier | Height | Scope |
| --- | --- | --- |
| Compact | 28 | Dense table actions and genuinely constrained utility controls |
| Standard | 32 | Ordinary buttons, single-line fields, dropdowns, and icon buttons |
| Search | 40 | Page-level Settings and catalog search |

Default to Standard. Keep adjacent actions the same height; distinguish importance through style, not size. Center checkbox and switch visuals within the interaction frame instead of stretching them.

Use shared component metrics for padding, radius, borders, and internal spacing. Expand rows for wrapped help or validation rather than shrinking their controls. A page-specific exception must have a concrete layout reason and remain local to that page.

## Color and surfaces

- Use semantic `ControlTheme` roles for text, surfaces, selection, focus, status, and errors. Avoid page-local palette copies.
- Use secondary text for metadata and help. Use status colors to communicate meaning, not decoration.
- Preserve authored brand and semantic icon colors. Use theme-adaptive colors where appearance is intended to follow the launcher theme.
- Express hierarchy with spacing and restrained surfaces before adding cards, outlines, shadows, or blur.
- Ordinary Settings actions use secondary buttons; reserve primary styling for the primary action and text styling for metadata links.
- Keep selection, hover, focus, and disabled treatments distinct. Check contrast on each supported surface; color alone must not carry state.

## Typography

Use the configured application font and shared constants in `launcher/component/typography.go`.

| Role | Size | Treatment |
| --- | --- | --- |
| Settings page title | 22 | Semibold |
| Body, label, value | 13 | Regular; semibold for emphasis |
| Help and secondary text | 12 | Regular |
| Section label | 11 | Semibold; 13 for scripts without case |
| Dense metadata | 10–11 | Regular or medium |
| Compact table status tag | 9 | Shared compact tag only |

Use weight before introducing another size. Keep translated labels readable and allow secondary text to wrap. Measure text instead of guessing baselines or platform-specific offsets; set line height explicitly for wrapping text.

## Spacing, alignment, and shape

- Prefer a 4-unit spacing rhythm. Preserve established optical spacing when a shared component already defines it.
- Align repeated labels, controls, and actions to common edges and centerlines.
- Express alignment through `Align`, Flex alignment, `Expanded`, and constraints. Padding is for content insets, not manual centering.
- Use thin borders and restrained rounding consistently through shared components.
- Author whole logical units for dimensions, padding, gaps, radii, borders, and font sizes. Alignment factors, ratios, animation values, measured geometry, and DPI conversions may be fractional.
- Keep related content together and reserve space for loading or validation feedback so controls do not jump.

## Interaction states and accessibility

Shared controls own default, hover, pressed, selected, focused, disabled, loading, empty, and error behavior where applicable.

- Preserve selected styling during hover and keep focus visible over both.
- Disabled controls must not activate or retain active hover behavior.
- Loading should keep a stable footprint and prevent duplicate actions.
- Empty and error states should explain the situation and expose an appropriate recovery action.
- Give actions semantic roles, accessible labels, and keyboard equivalents. Keep focus order consistent with reading order.
- Trap focus in modal content and restore it on dismissal. Keep keyboard selection visible in scrollable collections.
- Preserve user input through validation failures. Communicate state through text, shape, or semantics as well as color.

## Components, icons, and content

- Reuse shared `Wox*` controls. Page-specific composition must not recreate a reusable control's styling or interaction contract.
- Use `wox/common/icons` via `icons.Get`; introduce a semantic catalog name before adding a local asset.
- For theme-adaptive SVG paints, explicitly use `var(--wox-theme-icon-color)`. Preserve fixed colors and gradients; do not infer theme behavior from black or white paint or recolor an entire mixed-color icon.
- Pair unfamiliar icons with text and label icon-only actions accessibly.
- Preserve image aspect ratio. Keep pixel snapping at the renderer boundary.
- Keep full content reachable when truncating text, and ensure translated labels do not push neighboring controls outside their bounds.

## Layout and platform behavior

- Build responsive layouts from available space, not named operating systems.
- Keep one portable widget tree unless behavior genuinely belongs to the platform.
- Keep native windows, font measurement, IME, clipboard, dialogs, and renderer details behind runtime capabilities.
- Distinguish logical units from physical pixels and convert at platform boundaries. Coordinate-sensitive work must account for display scaling, mixed-DPI transitions, and negative desktop origins.
- Treat native blur and compositing as platform capabilities. Document their limitations in the owning implementation rather than encoding workarounds in page composition.

## Validation and maintenance

Validate the affected surface at narrow widths, with long or translated content, and in its applicable interaction states. Check Settings against its fixed palette and launcher controls against light and dark appearances.

Format changed code and run focused component or layout tests. Use native runtime comparison when font metrics, DPI, clipping, focus, or compositing cannot be established from portable tests.

Update this guide when a cross-feature design principle or shared control convention changes. Keep feature-specific values and regression cases in their owning components, tests, or documentation; do not append each visual adjustment here.
