# Query and Preview UI Design QA

> Status: blocked while the preview surface, metadata pills, split layout, and refinement controls are being revalidated against the Flutter references.

- source visual truth: `/Users/qianlifeng/Projects/Wox/screenshots/theme_glass_dark.png`
- implementation screenshot: `/private/tmp/wox-go-ui-query-full.png`
- viewport: Query window normalized to `1600 x 1142` physical pixels (`800 x 571` points at 2x)
- state: macOS, glass-dark theme, query `WOX`, list results selected, toolbar visible
- full-view comparison evidence: `/private/tmp/wox-query-material-comparison.png`
- focused region comparison evidence: `/private/tmp/wox-query-focused-comparison.png` (query header, selected result rows, toolbar)
- transition evidence: `/private/tmp/wox-query-transition-final.mp4`, `/private/tmp/wox-transition-sheet-01.png`, `/private/tmp/wox-transition-sheet-02.png`
- VS Code compound launch evidence: `/private/tmp/wox-vscode-launch.png`

## Findings

No actionable P0, P1, or P2 visual differences remain for the scoped Query page.

- Fonts and typography: Query text, result title/subtitle hierarchy, toolbar labels, truncation, and optical weight match the Flutter reference closely. Both implementations use the configured system/app font path.
- Spacing and layout rhythm: Header, result rows, selected-row radius, list inset, and toolbar height align at the normalized viewport. The current result text varies because results are live core data rather than fixture data.
- Colors and visual tokens: Foreground and selected-state contrast match the glass-dark intent. The Go window tint is more olive in the evidence because it is sampling the live forest wallpaper; this is expected behavior for the requested native frosted material.
- Image quality and asset fidelity: Visible result icons come from Wox core/native application assets. No placeholder, CSS-drawn, inline-SVG, or emoji replacement is used.
- Copy and content: Static toolbar actions preserve the Flutter semantics and are localized by the live core. Dynamic result names and locale differ from the reference and are not design drift.
- Icons: Query/result/action icons preserve the source size and alignment; live result icon subjects differ with query data.
- Interaction states: Query editing, result selection, More Actions click, `Cmd+J`, and `Escape` were exercised in the native window through Computer Use. The final window was also launched from VS Code's `Run Go UI` compound through Computer Use.
- Accessibility: Keyboard operation covers the scoped query flow. Screen-reader semantics for the custom GPU surface were not separately audited and remain a test gap outside this visual target.

## Open Questions

- None for the scoped Query page visual, material, and transition behavior.

## Comparison History

1. Initial comparison found three blocking fidelity issues:
   - P1: the clear transparent window exposed background detail instead of producing the Flutter-like frosted material.
   - P1: the Go header/footer still contained demo identity and shortcut content, and result density differed from Flutter.
   - P1: a query change could clear results and resize the window before replacement results arrived, producing a visible flash.
2. Fixes made:
   - Replaced the clear macOS content view with an active behind-window `NSVisualEffectView` using popover material.
   - Matched Flutter Query metrics and rendered live query/accessory/action data instead of demo labels.
   - Retained prior query results for the Flutter-equivalent 80 ms transition interval, then preserved window height while awaiting current-query results.
3. Post-fix evidence:
   - `/private/tmp/wox-query-material-comparison.png` shows the reference and normalized live implementation together.
   - `/private/tmp/wox-query-focused-comparison.png` confirms header, row, selection, and toolbar geometry.
   - The 60 fps transition recording and contact sheets show no blank frame or post-expansion height collapse during `a -> ap -> app -> ap -> app`.

## Implementation Checklist

- [x] Match Query header, result rows, selection, and toolbar geometry.
- [x] Use live Wox data and localized actions.
- [x] Use native frosted material instead of clear transparency.
- [x] Prevent query-result transition flashing and window collapse.
- [x] Verify mouse and keyboard action-panel interaction in the real native window.
- [x] Build with the same debug flags used by Delve.

## Follow-up Polish

- The keycap fill is marginally stronger than the Flutter reference at 2x zoom. This is a P3 renderer-level refinement and does not affect readability or layout fidelity.

final result: blocked
