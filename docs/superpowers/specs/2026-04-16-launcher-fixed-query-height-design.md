# Launcher Fixed Query Height Design

## Problem
The launcher currently resizes its window height as query results arrive, disappear, or transition through partial snapshots. That makes the window grow and shrink repeatedly while typing, which is especially noticeable on Windows and causes visible flicker.

Recent mitigations preserve the old height for a short grace window, but they still keep the window on a result-count-driven resize path. The visual instability remains because the height target still changes during normal query flow.

## Goal
Change query-time window sizing to a two-state model:

- `expanded`: when the query box has input, or when the query is empty and the configured start page is `MRU`
- `compact`: when the query is empty and the configured start page is not `MRU`

This should remove result-count-driven height changes during normal launcher use and minimize flicker.

## Non-Goals
- No backend protocol changes
- No redesign of launcher layout widgets
- No new user setting
- No attempt to remove all existing resize calls; only the target height policy changes

## Current Behavior Summary
- `calculateWindowHeight(...)` derives height from result count, grid/list mode, toolbar visibility, preview/action panels, and query box line count.
- `resizeHeight(...)` is triggered from many paths such as query updates, result updates, preview visibility, toolbar changes, and query box line count changes.
- Recent code preserves the previous visible height during partial query updates to reduce flicker, but the launcher still eventually resizes to multiple intermediate heights.

## Approved Approach
Use the existing resize pipeline, but replace the height calculation policy with a fixed two-mode decision.

Why this approach:
- It keeps the native window-management path intact.
- It localizes the behavioral change in `WoxLauncherController`.
- It minimizes risk to show/hide positioning, Windows DWM workarounds, and existing launcher lifecycle logic.

## Height Policy
Add a small controller-level decision that answers whether the launcher should be in `compact` or `expanded` mode.

Rules:
- `expanded` when `currentQuery` is non-empty
- `expanded` when `currentQuery` is empty and `lastStartPage == MRU`
- `compact` otherwise

In this design:
- result count does not affect height
- partial/final query snapshots do not affect height
- empty result responses do not affect height
- query box line count does not affect height

## Height Definition
### Expanded height
Expanded height should represent the launcher at its maximum normal query height:

- query box height
- maximum result container height for the configured max result count
- toolbar height only when the toolbar is actually shown
- existing bottom padding rules that are still needed for the current layout shell

Expanded height should not depend on the current number of results.

### Compact height
Compact height should represent the launcher with only the input area visible:

- query box height
- no result container height
- no toolbar height when there are no results and no persistent toolbar message
- preserve existing platform rounding behavior

This is the blank-start-page case the user explicitly requested.

## Layout Consequences
With a fixed expanded height:
- result lists will scroll within the existing viewport instead of growing the native window
- preview and action panels will reuse the expanded viewport instead of requesting extra native height
- toolbar visibility may still affect height if it is shown in compact mode without results, but the main query flow will remain in one of the two approved heights

The key product requirement is that the launcher no longer changes height repeatedly during query updates. Small residual differences outside that path should be avoided where practical.

## Code Changes
Primary file:
- `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`

Key changes:
- add a helper that determines whether the current launcher state is `compact` or `expanded`
- simplify `calculateWindowHeight(...)` to return one of those two heights
- stop using live result count, grid height, preview/action panel height, and query-box line count as native height drivers
- remove or neutralize now-redundant visible-height-preservation logic when it no longer changes behavior

Secondary file:
- `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart`

Key test changes:
- replace result-count-based resize expectations with two-state expectations
- add coverage for:
  - query text toggling `compact -> expanded`
  - clearing query on blank start page toggling `expanded -> compact`
  - clearing query on MRU start page remaining `expanded`
  - partial query updates not changing expanded height

## Risks
- Some non-query flows, such as toolbar-only states or query-box-hidden sessions, may still rely on older assumptions about height calculation.
- Multi-line query input will no longer increase native window height, so the input area must remain usable inside the fixed shell.
- The existing partial-height-preservation code may become dead weight if not cleaned up carefully.

## Validation
Manual validation:
- Blank start page: open launcher, verify compact height
- Type any query: verify immediate switch to expanded height
- Keep typing while results change: verify height remains stable
- Clear query with blank start page: verify return to compact height
- MRU start page: open launcher empty, verify expanded height
- Clear a non-empty query with MRU start page: verify height stays expanded

Smoke validation:
- update resize smoke tests to assert two-state height behavior instead of result-count-driven growth/shrink cycles
- keep a partial-results test to verify no intermediate shrink occurs inside expanded mode

## Expected Outcome
The launcher will only use two heights during standard query interactions:
- compact blank state
- expanded query/MRU state

That removes the repeated native resize churn caused by changing result counts and should significantly reduce visible flicker.
