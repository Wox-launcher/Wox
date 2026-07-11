# Window Manager Layouts Demo — Sequential Animation Redesign

## Goal

Replace the current side-by-side preview in `wox_window_manager_layouts_demo.dart`
with a sequential animation that narrates the real user flow:

1. Summon Wox and type a workspace name (`code`)
2. Select the result and press Enter — Wox hides
3. A three-pane workspace layout expands into the container

## Current State

`wox.ui.flutter/wox/lib/components/demo/wox_window_manager_layouts_demo.dart`
renders a `Row` with two columns that are always visible simultaneously:

- Left: `WoxDemoWindow` typing `工作`/`Work` with three results
- Right: `_DemoMonitor` containing three `_DemoWindowTile`s that expand from
  center to their target rects on a loop

There is no temporal relationship between searching and the layout appearing.

## Target Behavior

Single animated area below the always-visible `WoxDemoHintCard`. The area
shows either the Wox search window or the three-pane layout, cross-faded by
animation phase.

### Animation Timeline (~6500ms loop)

| Phase | Range | Duration | What happens |
|-------|-------|----------|--------------|
| 1. Type query | 0.00 → 0.45 | ~2925ms | Wox centered, query types `code` char-by-char from 0.10; results visible with `Code` selected |
| 2. Confirm pause | 0.45 → 0.55 | ~650ms | Query complete, selected row holds — simulates Enter press |
| 3. Wox hides | 0.55 → 0.66 | ~715ms | Wox scales 1 → 0.85 → 0 and opacity 1 → 0 |
| 4. Layout expands | 0.66 → 0.88 | ~1430ms | Three tiles lerp from center small rect to target rects, opacity 0.40 → 1 |
| 5. Layout holds | 0.88 → 0.96 | ~520ms | Full layout visible |
| 6. Layout fades | 0.96 → 1.00 | ~260ms | Layout opacity → 0, loop restarts |

### Wox Search Phase

- Wox window is centered in the `Expanded` area (not the full container — the
  `WoxDemoHintCard` stays on top)
- `WoxDemoWindow` with `showToolbar: false`, `opaqueBackground: true` so the
  mica surface stays readable over the desktop background
- Query string: hard-coded `code` (no i18n needed)
- Results:
  1. `Code` — `view_quilt_outlined` accent icon, `selected: true`,
     tail = i18n `plugin_window_manager_setting_groups`
  2. `Browser` — `language_rounded` green icon, subtitle "Right display"
  3. `Terminal` — `terminal_rounded` yellow icon, subtitle "Bottom-right"
- Query typing uses the existing `_queryText` pattern: substring grows
  proportionally to phase progress

### Wox Hide Animation

- `Transform.scale(alignment: Alignment.center, scale: 1 → 0.85 → 0.0)` with
  `Curves.easeInCubic`
- `Opacity(opacity: 1 → 0)` on the same interval
- Both driven by `_interval(0.55, 0.66, Curves.easeInCubic)`
- After 0.66 the Wox widget is kept in the tree at opacity 0 so the layout
  widget can take its place via the same `Stack`

### Layout Expand Phase

- Three `_DemoWindowTile` widgets fill the `Expanded` area directly — no
  `_DemoMonitor` frame border
- Each tile's `target` rect is relative to the `Expanded` area constraints:
  - Code: `Rect.fromLTWH(0.02, 0.06, 0.46, 0.88)` — left half, tall
  - Browser: `Rect.fromLTWH(0.52, 0.06, 0.46, 0.42)` — right top
  - Terminal: `Rect.fromLTWH(0.52, 0.52, 0.46, 0.42)` — right bottom
- Each tile's `start` rect is the same center small rect:
  `Rect.fromLTWH(width * 0.30, height * 0.40, width * 0.40, height * 0.24)`
- `Rect.lerp(start, target, progress)` with `progress` from
  `_interval(0.66, 0.88, Curves.easeOutCubic)`
- Tile opacity: `0.40 + 0.60 * progress` (unchanged from current)
- Colors and icons unchanged: Code `0xFF60A5FA`, Browser `0xFF34D399`,
  Terminal `0xFFFACC15`

### Layout Fade-out Phase

- Whole layout subtree wrapped in `Opacity`:
  `1 - _interval(0.96, 1.00, Curves.easeInCubic)`
- At 1.00 the controller loops back to 0.00, Wox reappears at full opacity

## Container Layout Change

Before:

```
ClipRRect
  Stack
    WoxDemoDesktopBackground
    Padding(_demoDesktopHintContentPadding)
      Column
        WoxDemoHintCard
        Expanded
          Row
            Expanded(flex 9)  → WoxDemoWindow
            Expanded(flex 11) → _WindowManagerLayoutPreview
```

After:

```
ClipRRect
  Stack
    WoxDemoDesktopBackground
    Padding(_demoDesktopHintContentPadding)
      Column
        WoxDemoHintCard  (always visible, from="code", to=i18n three_pane)
        Expanded
          Stack
            Opacity + Transform.scale  → WoxDemoWindow   (phase 1-3)
            _WindowManagerLayout        (phase 4-6, fills Expanded)
```

`_WindowManagerLayoutPreview` and `_DemoMonitor` are removed. `_DemoWindowTile`
stays but its `rect` semantics change from "inside the monitor preview" to
"inside the Expanded area".

## Files Changed

- `wox.ui.flutter/wox/lib/components/demo/wox_window_manager_layouts_demo.dart`
  — full rewrite of `WoxWindowManagerLayoutsDemo` and removal of
  `_WindowManagerLayoutPreview` + `_DemoMonitor`. `_DemoWindowTile` kept with
  adjusted start rect.

No other files change. The call site
`wox_window_manager_groups_setting.dart:177` stays identical. The popover size
`700 × 460` stays identical.

## i18n

Two of the three existing keys are reused unchanged:

- `plugin_window_manager_layouts_demo_title` — hint card title
- `plugin_window_manager_layouts_demo_three_pane` — hint card `to` value and
  no longer used as a monitor label (the `_DemoMonitor` widget is removed)

`plugin_window_manager_layouts_demo_workspace_name` is **no longer used** by
the demo because the query text is now the literal `code`, and the hint card
`from` is also `code` to match the typed query. The key stays in the lang
files (no removal in this change) so other potential consumers are unaffected;
a separate i18n cleanup can drop it later.

## Verification

Per `AGENTS.md`: do not run Flutter build. Only check syntax/static errors
by reading the code carefully and matching `part of` / import patterns used by
sibling demo files. No unit tests, no smoke tests.