# WoxView completion checklist

This tracks the full user-supplied "WoxView JSON completion plan". A checked item
means its implementation and focused verification exist; it does not imply the
entire phase is complete. All changes currently remain in the working tree.

Current status (2026-11-06): all implementation work and the delegated local
verification are complete. The user explicitly took ownership of macOS native
verification; it has not been run or claimed as passing in this Windows/WSL
workspace. The records below retain the earlier checkpoints for traceability.

## Foundation and contract

- [x] 0.1 View-owned field values; cached-form submission regression.
- [x] 0.2 Revision reset for every existing field; stale echo and caret tests.
- [x] 0.3 Handler/field capability interfaces and immutable handler index.
- [x] 0.4 Implicit form submission, disabled/submitting, required/error display.
- [x] 1.1 Catalog-derived JSON Schema with golden comparison.
- [x] 1.2 Structured validation errors with preserved diagnostic text.
- [x] 1.3 Additive version-1 and future breaking-version rules documented.
- [x] 1.4 Display-string translation, including tooltip, supplied by plugin owner.

## Interface

- [x] 2.1 Style: alignSelf, aspectRatio, hidden overflow, opacity, shadow tokens,
  floating, side borders, and text features supported by runtime.TextStyle.
- [x] 2.1 alignSelf overrides the parent Flex cross-axis alignment, including
  weighted items, bounded stretch, and intrinsic sizing on unbounded axes.
- [x] 2.1 Preferred aspectRatio, hidden overflow, floating material, and all four
  side border widths, including explicit zero overrides on shared controls.
- [x] 2.1 Group opacity through generic widget/runtime layers, preserving layout,
  input, clipped/overflow paint, nested groups, and partial-frame composition.
- [x] 2.1 Fixed sm/md/lg outer shadows through widget.BoxShadow, with density/DPI,
  clipping, opacity, cache reuse, and old/new paint damage coverage.
- [x] 2.1 Text capability audit: runtime.TextStyle has size/weight/family/italic;
  unsupported general fields remain rejected. Span underline and maxLines retain
  their existing shared-component behavior; the distinction is documented.
- [x] 2.2 Row wrapping, grid, horizontal scroll, tail following, end notification.
- [x] 2.2 Fixed-extent keyed lazy-list with visible-item construction.
  Field echo state survives unmount and reconciles offscreen producer updates;
  image eviction waits for deferred layout builders.
- [x] 2.3 Text/span inline rich text, restricted nesting, link handlers.
  Shared measurement keeps selection and link fragments aligned; inline styles
  inherit from the parent and translated display strings preserve whitespace.
- [x] 2.4 radio-group, multi-select/tags, tabs, tag/badge.
  Shared choice groups provide keyboard navigation and density-scaled chrome;
  array values support stale echoes, revision reset, and offscreen retention.
- [x] 2.4 number-field, hotkey, table, copyable code, link.
  Numeric drafts retain valid form values; hotkeys record locally; stable table rows
  retain action identity; code copies literal bytes; links use handler/host precedence.
- [x] 2.4 Common tooltip: translated, floating, viewport clamping, no input theft.
- [x] 2.4 Select floating overlay, arrow/Enter/Escape/typeahead, list navigation.

## Theme and animation

- [x] 3 Dual light/dark colors and expanded semantic color tokens.
- [x] 3 Size type, dimension tokens, and custom JSON decoding.
- [x] 3 Hover/pressed/focus/disabled styles restricted to non-layout properties,
  including inline spans with selection-preserving fragment observation/painting.
- [x] 3 Overlay theme status colors; remove pluginwindow hardcoded colors.
- [x] 3 theme_changed window event and SDK/host consistency.
- [x] 4 Generic widget opacity, drawing transform, and outer shadow capabilities
  implemented and tested before protocol mapping.
  Opacity and outer shadow are complete. Drawing transforms now have public
  recording, widget/damage integration, and JSON style mapping. Windows/Linux
  native paths pass offscreen tests; macOS native validation is handed to the user.
- [x] 4 Implicit transitions with keyed state and supported curves.
  Seven channels and three curves now map to AnimatedFloat, including local
  state styles and inline opacity/background fragments. Shared controls resolve
  and interpolate their own native default fills through AnimatedColor. Natural
  dimensions snap because zero is the existing auto-size sentinel.
- [x] 4 Pulse/spin/shimmer loop presets.
  Core LoopAnimation drives keyed block and inline presets. Tests cover phase,
  period/type/key changes, removal, transition composition, cached boundaries,
  selection, opacity, clipping, input geometry and density/display scaling.
- [x] 4 Enter presets for block nodes and inline spans, with mount identity,
  cancellation, transition/loop composition and injected-clock tests.
- [x] 4 Exit presets, inert removed snapshots, handler invalidation.
  View retains only last-painted snapshots, revokes removed node capabilities
  immediately, and releases exits on completion, reinsertion or document disposal.
  Inline selection and glyph fragments composite together with shared ancestor
  opacity, using one keyed clock and one paint transform per removed node.
- [x] 4 Injected-clock tests for intermediate values, key changes, and removal.
  Transition, loop, enter and exit lifecycles cover cancellation, reinsertion,
  cached boundaries, inert removed controls and fractional display scaling.

## Events and commands

- [x] 5.1 Hover, double-tap, context-menu, focus, end-reached, Markdown link events.
  Shared widgets observe descendants without taking over their interactions;
  focus notifications track transitions immediately and ignore sibling switches.
- [x] 5.2 Subtree/window shortcuts with platform-aware mod parsing.
- [x] 5.3 Declarative context menu with native Wox overlay controls.
- [x] 5.4 WindowContentCommand focus/scrollTo/selectAll, option/result contracts,
  driver, host routes, both SDKs, document guard, and integration tests.
- [x] 5.5 Coalescing marked change events without plugin-to-UI dependency.
- [x] 5.5 Optional text-field changeDebounce (included in this implementation goal).
- [x] 5.6 Additive event Data.Key and window-adapter regression.

## Verification records

2026-11-05 continuation:

- `go build ./...` passed in wox.core.
- `go test ./ui/woxview ./ui/pluginwindow ./plugin ./plugin/host ./ui/launcher/component ./ui/widget` passed.
- `go test -tags wox_automation ./appcontrol ./ui/automation ./ui/runtime ./ui/widget ./ui/launcher ./test/automationdriver ./test/smoke ./test/smokerunner -count=1` passed.
  The Makefile wrapper could not run its resource prerequisite because this
  Windows PATH lacks sed/rm; its exact Go test command was run directly instead.
- New focused tests: lazy_list_test.go, interaction_test.go, anchored_overlay_test.go,
  gesture_observer_test.go, focus_observer_test.go, and the pluginwindow event adapter.
- Layout/interaction tests cover 1x, 1.5x, 2x and scale transitions; no native
  interactive smoke session was launched during this continuation.
- Schema golden regenerated and normal comparison passed. README and SDK payload
  documentation updated alongside the new props and events.

Keep adding invalid-input cases, schema updates, documentation, and meaningful
render/input coverage for each new protocol field. At completion audit every
unchecked item against actual implementation and test evidence before marking
the overall goal complete.

2026-11-05 menu/shortcut continuation:

- Shared WoxMenuList/WoxPopupMenu provides disabled-row skipping, initial-letter
  navigation, bounded scrolling, floating chrome, modal isolation, and dismissal.
- Select uses a Host overlay; inline list reuses the same navigation. Context-menu
  actions and labels update while open. Node removal and disabling close popups.
- Fixed Host modal focus restoration to use stable keys across overlay-root changes.
- Added generic widget.ShortcutScope with no extra Tab target. WoxView parses normal
  shortcuts through the shared parser and expands the platform-specific mod alias.
- Focused tests are in menu_test.go, shortcuts_test.go, wox_menu_test.go, and
  shortcut_scope_test.go; layout tests cover 1x, 1.5x, and 2x logical/physical scales.
- `go build ./...`, the six required package tests, and the broad `wox_automation`
  UI package command above all passed after the menu/shortcut changes.

2026-11-05 event-queue continuation:

- Replaced the bounded event channel with a mutex-protected bounded slice and
  wakeup channel. No map-lock pair was introduced. Producer-supplied opaque keys
  coalesce pending changes; discrete events remain ordering barriers.
- WoxView marks field changes; pluginwindow supplies opaque handler/node keys;
  plugin infrastructure remains independent of WoxView. Internal metadata has
  json:"-" and is verified absent from public payloads.
- Queue tests cover full-capacity replacement, document isolation, ordering
  barriers, concurrent producers, close cleanup, and a real blocked callback
  followed by a 1000-event burst. Existing overflow reporting remains tested.

- Final `go build ./...` and all six required package tests passed after queue changes.
- `go test -race ./plugin -run 'TestWindowEventQueue|TestSlowWindowCallbackReceivesLatestChange'` passed.
- Draft 2020-12 schema validation passed with jsonschema; valid menu/shortcut examples
  passed and missing/null shortcut handlers were rejected. Golden comparison passed
  in the normal package test run.

The unchecked scope at the top is authoritative. Do not mark the goal complete
until all requirements and their associated verification are complete.

2026-11-05 choice-component continuation:

- Added radio-group and tabs using shared segmented buttons; added multi-select/tags
  using wrapping selectable Wox tags, plus tag/badge metadata nodes. Selection remains
  local until activation and keyboard navigation skips disabled choices.
- Expanded retained echo state to immutable array snapshots. Tests cover stale echoes,
  explicit empty-array reset, option removal, form snapshots, and virtualized unmount/remount.
- Fixed field normalization to retain the producer's last prop so repeated builds do not
  erase surviving uncontrolled selections after an option is removed.
- Fixed submit buttons bypassing form-value collection when an embedder uses OnNodeEvent.
  Button actions now receive the form callback directly; tests verify full values and node Key.
- Corrected shared segmented-button density scaling for height/radius alongside text, with
  bounded single-line labels. Existing usage-settings controls share the corrected geometry.
- Updated schema golden and component/event documentation. Invalid-input tests cover array
  value types, duplicate or missing options, handler validation, and the 500-choice budget.
- `go build ./...`, all six required package tests, and the broad wox_automation package
  command above passed. Draft 2020-12 schema validation and six new-node examples passed.
- Interaction/layout tests cover 1x, 1.5x, 2x, density 0.9/1/1.1, translations, disabled
  groups, required/error labels, and narrow wrapping. No native interactive session was launched.

2026-11-06 settings-component continuation:

- Completed the remaining 2.4 catalog nodes: number-field, hotkey, table, code, and link.
  Numeric fields retain incomplete editor drafts, emit finite numeric values, support
  range/step commit, stale echoes, offscreen range changes, and revision reset.
- Hotkey entry reuses the native recorder, adds disabled support, and owns recording
  before ancestor shortcuts, including held-key repeats. Formatting moved to the shared
  component layer; native global recording policies remain in the launcher controller.
- Shared WoxDataTable uses collapsed grid borders, fixed headers, two-axis scrolling,
  stable row/action identities, translated text, disabled actions, and tooltips.
- Shared WoxCodeBlock uses a monospace read-only editor and exact-byte copy. Extracted
  the existing Markdown copy/feedback control into wox_code_copy.go for both consumers.
  Link actions expose link semantics and give producer handlers precedence over host opening.
- Fixed Host focus reveal for non-editor controls through nested scrollers. Successive
  layout passes reveal outer ancestors; pointer actions cancel pending reveals and
  clicking the already-focused control preserves manual scrolling.
- Added setting_components_test.go, wox_setting_components_test.go, and
  focus_scroll_test.go. Coverage includes 1x/1.5x/2x, density-scaled code typography,
  malformed input, exact clipboard content, numeric drafts, local shortcut capture,
  both table scroll axes, and refreshed row handlers after reorder.
- Final `go build ./...`, all six required package tests, and the full wox_automation
  package command recorded above passed. Schema golden comparison and external Draft
  2020-12 validation passed for all five new node contracts and invalid examples.
- gofmt reported no unformatted files; git diff --check reported no whitespace errors.
  Validation was headless/source-based; no native interactive session was launched.

2026-11-06 commands and debounce continuation:

- Added WindowContentCommand with named option/result and command enum across Go,
  WebSocket routing, Node and Python SDKs, and both runtime hosts. Ownership,
  plugin unload, closed windows, content type, focusability, and DocumentID guards
  remain enforced at the appropriate capability boundary.
- Shared widget.Target preserves descendant identity; Host focus, select-all,
  and reveal operations use logical geometry and respect disabled/modal controls.
  WoxView owns unique-key resolution, bounded FIFO delivery after layout, and
  materialization of nested lazy rows. Commands survive row reorder and cached
  Boundaries, but cannot redirect to a replaced target or new document. Deferred
  errors include Command/Key in the existing document-scoped error event.
- Added changeDebounce (0-60000 ms, default immediate) with View-owned timers.
  Local values are immediate; blur and submit flush changes first. Timer identity
  guards suppress already-posted callbacks after reset, removal, handler/delay
  changes, disabling, authoritative values, or document disposal. Stale echoes
  preserve newer pending edits; virtualized fields do not lose notification.
- Tests cover actual editor selection with Unicode, nested virtual lists,
  reordering, two-axis scrolling, 1x/1.5x/2x and scale transitions, modal isolation,
  first-frame requests, reentrant cancellation, Boundary caching, queue limits,
  WebSocket ownership and payload routing, and deterministic debounce timing.
- `go build ./...`, all six required Go package tests, and the broad
  wox_automation package command above passed. Schema golden comparison and
  external Draft 2020-12 positive/negative debounce validation passed.
- Node SDK compilation, host `tsc --noEmit`, and all 37 Node tests passed. The
  installed SDK was stale; the host's local dependency was linked to the in-repo
  SDK and its runtime output rebuilt. No package manifests or lockfile changes
  were retained. All 50 Python tests and Ruff checks on touched files passed.
- gofmt and git diff --check passed. Validation remains headless/source-based;
  no native interactive session was launched.

2026-11-06 inline-rich-text and alignment continuation:

- Added text/span contracts, strict inline style subsets, parent inheritance,
  translated display strings, common span interactions, keyed link handlers, and
  schema placement restrictions. A span is only a direct leaf child of text.
- Shared WoxRichText uses the Host's font-measurement capability through generic
  widget.TextLayoutBuilder. Grapheme-safe wrapping, mixed fonts, underlines,
  transparent colors, centering, selection, and link fragments share geometry.
  Each wrapped link has one keyboard target; clipped links cannot retain focus.
- Read-only paragraphs retain exact copy text and keep their viewport fixed when
  select-all includes clipped text. Corrected horizontal scrolling of wrapped
  one-line editor viewports and preserved headless measurement behavior after
  the broad Notes regression exposed a nil-interface hover path.
- Added widget.AlignSelf and WoxView style.alignSelf. Per-item alignment composes
  with Expanded/Flexible without extra layout boxes or loss of child identity.
- Tests cover rejected span placement and style fields, inheritance, translation,
  stale handler revocation, focus commands, wrapped link activation, drag/copy
  across different font advances, density 0.9/1/1.1, and 1x/1.5x/2x DPI changes.
  Alignment tests cover both axes, all parent/child alignment combinations,
  weighted wrappers, pointer bounds, and unbounded/nil children.
- Final `go build ./...`, all six required Go package tests, and the broad
  wox_automation package command recorded above passed. The previously failing
  Notes typing-below-table case also passed independently after its fix.
- Updated both READMEs and schema golden. Normal golden comparison and external
  Draft 2020-12 positive/negative span and alignSelf validation passed. gofmt and
  git diff --check passed. Validation was headless/source-based; no native
  interactive session was launched.

2026-11-06 box-style continuation:

- Added preferred aspectRatio using LayoutBuilder and Constrained. Explicit
  dimensions, padding/margin, min/max bounds, and bounded/unbounded axes keep
  their documented precedence. Scroll and lazy-list measure their viewports from
  the ratio's outer constraints; shared leaf controls also receive those bounds.
- Extended generic widget.Clip with intrinsic-size clipping, then mapped
  overflow:hidden before margin. Default fixed-size clipping remains unchanged.
  Visible overflow retains existing ancestor pointer-bound rules.
- Completed Container's four side borders with top-edge support and optional
  foreground edge painting. Horizontal edges own shared corners, rounded edges
  stay inside the silhouette, and top/bottom widths clamp without overlapping.
  WoxView side overrides replace the uniform stroke so an explicit zero works;
  opaque child controls cannot obscure the new edge decorations.
- Mapped floating to Container's existing native material treatment without
  changing sibling layout/order or creating another window/overlay layer.
- Tests compare actual display lists for clipping, material and border output;
  cover negative origins, rounded/translucent corners, oversized top edges,
  controls with opaque paint, shared-control dimensions, scroll/lazy viewports,
  pointer bounds, invalid JSON/Go values, and 1x/1.5x/2x layout transitions.
- Updated protocol/architecture READMEs and schema golden. Normal golden
  comparison and external Draft 2020-12 validation passed (five valid and nine
  invalid box-style examples).
- `go build ./...`, all six required Go package tests, and the broad
  wox_automation package command recorded above passed. gofmt and git diff
  --check passed. Verification was headless/source-based; no native interactive
  session was launched.

2026-11-06 opacity continuation:

- Added generic widget.Opacity and BeginOpacity/EndOpacity display-list commands,
  then mapped the optional 0-1 style property. Omission defaults to one; zero
  skips subtree painting while preserving layout, focus, and input. Visible
  overflow and focus chrome contribute to layer bounds instead of being clipped
  to the node's layout box.
- Implemented D2D group alpha on Windows, CoreGraphics transparency layers on
  macOS, and premultiplied before/after snapshot composition on Linux. Existing
  edge fades and opacity reuse the native composite machinery. Layer entry and
  exit restore enclosing clips; scene transforms do not scale alpha.
- Extended the software reference renderer and comparison logic. Composite
  layers disable standalone caret patches; malformed software layer streams
  return an error and restore the original target instead of panicking.
- Pixel tests cover overlapping opaque siblings, nested groups, translucent
  backdrops, following drawing, clipping, partial-damage replay, and display
  scales 1x/1.25x/1.5x/2x. Widget/protocol tests cover actual composited pixels,
  visible overflow, keyed updates, focus/input preservation, and invalid values.
- Windows offscreen WARP and software pixel tests passed without opening Wox.
  Equivalent macOS bitmap and Linux GTK/GL fixtures were added but cannot be
  executed on this Windows host; native validation there remains platform CI work.
- `go build ./...`, the six required Go packages plus ui/runtime/internal/window,
  and the broad wox_automation package command above passed. Normal schema golden
  comparison and external Draft 2020-12 validation passed (five valid and four
  invalid opacity examples). Protocol and architecture documentation was updated.
- gofmt and git diff --check passed. No native interactive session was launched.

2026-11-06 outer-shadow continuation:

- Completed generic widget.BoxShadow with rounded silhouette knockout, Gaussian
  distance-field falloff, immutable nine-slice images, and display-scale rasterization.
  Layout and pointer bounds remain unchanged; transparent interiors remain clear.
- The concurrent LRU uses a bounded slice, at most 64 entries / 8 MiB; textures
  are capped at 512 pixels per axis. Long edges share raster textures while panels
  resize, and concurrent misses converge on the same retained immutable image.
- Paint extents propagate through unclipped ancestors, cached Boundaries, keyed
  invalidations, hover/scroll repaints, and opacity group allocation. Old extents
  are retained for moves, shrink, removal, and shadow removal without a Boundary.
- Added shadow sm/md/lg protocol tokens with density-scaled softness/offset. The
  shadow surrounds the clipped border box before margin and respects ancestor clips.
  Updated schema golden, protocol README, and UI architecture documentation.
- Pixel/input tests cover transparent silhouettes, rounded corners, falloff,
  slice joins, group opacity, ancestor clipping, leaf controls, negative origins,
  display transitions at 1x/1.5x/2x, and density 0.9/1/1.1. Retained partial-frame
  pixels match full renders after cached panel move/shrink/removal.
- go build ./..., all six required package tests, and the broad wox_automation
  command above passed. All shadow tests also passed under the race detector.
  Schema golden and external Draft 2020-12 validation passed (four valid and four
  invalid token cases). gofmt and git diff --check passed.
- Verified runtime.TextStyle's actual fields and documented the supported-text
  boundary. The requested conditional audit is complete; unsupported font/style
  names were not added to the protocol. No native interactive session was launched.

The remaining unchecked requirements are stage 3 theme capabilities and stage 4
transforms/animation. Stage 2 and stage 5 are complete.

2026-11-06 theme-color continuation:

- Color is now an immutable scalar literal/token or complete light/dark pair.
  Both branches validate at parse time; selection and token resolution use the
  embedding ControlTheme.Dark on every build. Existing scalar/null behavior and
  explicit transparent values remain supported; pairs cannot nest.
- Added hover, selected, selectedText, focus, disabled, link, overlay, and
  warningText tokens. Shared ControlTheme carries the surface appearance and
  semantic roles. Plugin windows resolve the current overlay palette; launcher
  and fixed-dark Settings adapters provide their own surface roles.
- Added optional SuccessColor, WarningColor, WarningTextColor, and ErrorColor
  theme fields in both schema versions. Shared common.Theme.StatusColors owns
  appearance-aware fallbacks; pluginwindow no longer hardcodes the palette.
  V2 fields support validation, platform inheritance, and null clearing. Legacy
  and V2 roundtrips preserve authored values without writing fallback defaults.
- Tests cover actual rendered dual-color/token pixels, transparent branches,
  live appearance/token changes without replacing the tree, field-value retention,
  1x/1.5x/2x display transitions, malformed/nested/partial pairs, programmatic
  validation, surface adapter roles, theme roundtrips and inactive overrides.
- go build ./..., all six required Go package tests plus common/util/overlay,
  and the broad wox_automation command above passed. JSON Schema golden and
  external Draft 2020-12 validation passed (13 valid and 10 invalid color cases).
  gofmt and git diff --check passed. Protocol and architecture documentation
  updated; no native interactive session was launched.

Stage 3 still requires Size/dimension tokens, local interaction-state styles,
and the theme_changed event with SDK/host consistency. Stage 4 remains pending.

2026-11-06 theme-event continuation:

- Added document-scoped theme_changed events with Data {Dark: bool}, Go/Node/Python
  payload types, and the Python enum. Existing generic host routes preserve opaque
  data; new end-to-end RPC tests cover false/true values and released callbacks.
- Added generic overlay theme subscriptions backed by util.HashMap, with callbacks
  outside the map lock. Separately managed plugin windows now refresh native
  appearance and paint on theme updates, including while hidden. Session teardown
  cancels subscriptions; shared infrastructure has no plugin-window dependency.
- No creation-time event is added. HTML changes wait for ready and collapse to the
  latest appearance; document replacement clears pending changes and delivery
  guards reject old queued events. Theme notifications include same-appearance
  palette updates and use the receiving overlay's appearance. Pending events use
  the existing coalescing mechanism without crossing discrete-event barriers.
- Tests cover snapshot-safe callback replacement, concurrent cancellation, HTML
  readiness ordering, closed sessions, Go WebSocket routing, both SDK hosts, and
  a real hidden Windows window's subscription, replacement, and close lifecycle.
- The opt-in TestNativeHiddenWindowThemeChanges passed with
  WOX_WINDOW_THEME_NATIVE_TEST=1. It did not show or activate any window and did
  not launch the Wox application. Other native interactive fixtures were not run.
- go build ./..., all six required Go package tests plus util/overlay, the broad
  wox_automation command, and overlay subscription race tests passed. Node SDK
  and host tsc --noEmit passed; all 37 Node and 50 Python tests passed. Ruff,
  gofmt, and git diff --check passed. No dependency manifests or locks changed.
- Updated protocol, window, and architecture documentation. The node JSON Schema
  is unchanged because this is a window event rather than a tree field; normal
  golden comparison passed in the WoxView package tests.

Stage 3 now only requires Size/dimension tokens and local interaction-state
styles. Stage 4 transforms/animations remains pending.

2026-11-06 dimension-token continuation:

- Added immutable Size values, all ten dimension tokens, strict JSON decoding,
  numeric compatibility and token-preserving serialization. Dimensions, insets,
  anchors, borders, grid gaps, spacers, scroll thresholds, virtual row extents,
  table column widths and typography resolve through their owning layout paths.
- Geometry tokens follow Interface size on each build; numeric geometry retains
  logical-unit semantics. Font owners scale normal-density bases once. Fixed
  selectable-text line-height tokens, which otherwise missed density scaling.
  Virtual-row commands resolve current extents after queued density changes.
- Fixed programmatic anchor validation: a valid later anchor could hide an invalid
  earlier anchor because they shared one validation entry.
- Focused tests cover all dimensional Style fields, malformed and nonfinite sizes,
  dimensionless-field rejection, serialization, explicit zero edges, actual layout,
  grid/table/stack geometry, border paint, input, immutable trees, font measurement,
  selectable line boxes, end notifications, virtual commands and offscreen edits.
  Fixtures cover density 0.9/1/1.1, DPI 1x/1.5x/2x and display transitions;
  border command comparison also applies a negative scene origin.
- go build ./..., all six required Go packages, and the broad wox_automation
  command passed. Schema golden comparison and external Draft 2020-12 validation
  passed (15 valid and 12 invalid size documents). gofmt and git diff --check
  passed. Updated protocol documentation; no native interactive session launched.

Stage 3 still requires local interaction-state styles. Stage 4 transforms,
transitions, loop presets, and enter/exit animations remain pending.

2026-11-06 interaction-state prerequisite:

- Extended generic Gesture.ObserveDescendants to observe primary press state,
  including raw-input descendants, without taking over child activation. The
  Host resolves current callback identities after rebuilds and cancels observed
  presses on release, pointer leave, blur, drag handoff, or target removal.
  Removed observers cannot receive stale release callbacks. Ordinary gestures
  retain their existing direct-press behavior.
- Independent widget tests cover own-surface and descendant input, raw capture,
  current callback replacement, cancellation/removal, secondary-button exclusion,
  and 1x/1.5x/2x DPI. go build ./..., the six required package tests and broad
  wox_automation suite passed. gofmt and git diff --check passed.
- This is a prerequisite only. The four protocol state-style props and their
  WoxView mapping/validation/render tests are still outstanding; the stage 3
  interaction-style checkbox remains unchecked.

2026-11-06 block interaction-style continuation:

- Added StateStyle and the four paint-only patch props to Styled block nodes,
  with strict parser/programmatic validation and a shared JSON Schema definition.
  Color pairs, semantic tokens, dimension tokens, explicit zero and transparency
  retain their meaning. Layout and typography fields are rejected in patches.
- Keyed node State widgets retain hover/press/focus locally and apply immutable
  style snapshots, ordered hover -> press -> focus, or disabled alone. Existing
  field/form/submit/link disabled policy controls which patch is active. Observer
  removal clears local flags so later reattachment cannot resurrect stale focus.
  Local updates invalidate cached boundaries without producer event traffic.
- Added shared ControlPaint overrides to buttons, text fields and dropdowns.
  WoxView maps button/link/icon-button, text/number-field and select paint paths;
  authored state fills supersede native hover defaults. Controls own their fill
  while the outer wrapper owns side borders, preventing double alpha composition.
- Tests cover card precedence, actual control pixels, disabled activation,
  inherited form disable, style removal/readdition, immutable trees, edited field
  retention, cached boundaries, current theme tokens, zero radius/borders, explicit
  transparency, and 0.9/1/1.1 density with 1x/1.5x/2x display transitions. Independent
  shared-control tests check transparent paint against fallback/hover colors.
- go build ./..., the six required Go packages, and the broad wox_automation suite
  passed. After the observer-removal fix, WoxView/pluginwindow tests passed again.
  Schema golden and external Draft 2020-12 validation passed (12 valid and 40 invalid
  state-style documents). gofmt and git diff --check passed. No native interactive
  application session was launched.
- Still incomplete: inline span state styles (including selectable text without
  interception), remaining component paint mappings, and broader coverage of node
  types and lifecycle. The four props currently live in Styled, not CommonProps,
  so spans reject them rather than silently ignoring them. Stage 3 stays unchecked.
  Stage 4 transforms and all animation requirements are still pending.

2026-11-06 remaining block-control paint continuation:

- Mapped shared ControlPaint through switch, checkbox, slider, hotkey recorder,
  list/choice groups, code blocks, data tables and tags/badges. Progress honors
  explicit square corners. Selectable text preserves explicit transparent text
  colors instead of invoking the shared editor's default-color fallback.
- Group fills are composited once, including nested unselected tags; selected
  rows retain their selection surface and check marks. Table frames paint their
  background behind cells and one border above them, with independent internal
  separators. Slider radius overrides apply to both rail segments.
- A new pixel regression reproduced native hotkey hover overriding the authored
  color. Applying explicit paint after native feedback fixes both transparency
  and border overrides.
- Tests verify composited fill/opacity and explicit transparency on ten node
  types; group selection surfaces on five choice types; retained focus and
  keyboard actions on ten controls across observer removal/readdition; and
  unchanged sibling geometry in both flex axes. Fixtures cover density
  0.9/1/1.1 and DPI 1x/1.5x/2x. Existing Host key remapping already preserves
  focus across added paint wrappers; no Host change was needed.
- Shared table pixel tests cover removed borders, theme-color fallback for wider
  borders, a single translucent stroke, and unchanged internal separators.
- go build ./..., all six required packages, and the broad wox_automation command
  passed. The subsequently added shared table pixel regression also passed.
  No protocol fields changed; the package run includes the schema golden test.
  Verification was headless; no interactive Wox application was launched.
- Stage 3 remains incomplete because inline span state styles and their
  selection-preserving interaction mapping are outstanding. Stage 4 transforms,
  transitions, loop presets, enter/exit snapshots and clock tests remain pending.

2026-11-06 inline state-style continuation:

- Added the shared state-style contract to SpanProps and regenerated the catalog
  schema. Every catalog node now declares the four paint-only state patches.
  Inline feedback is owned by keyed WoxView paragraph state, with the same
  hover/press/focus/disabled precedence as block nodes and no producer traffic.
- Generic Gesture.OnPointerObserve reports local logical coordinates along the
  visible hit-test branch, before descendant consumption, and refreshes after
  layout under a stationary pointer. It leaves raw input, capture, drag selection
  and normal activation unchanged. Existing hover/press cancellation owns exit
  and window-blur cleanup.
- WoxRichText maps observations against actual measured fragments instead of a
  wrapped run's bounding rectangle. All fragments share the node's state; gaps,
  clipping and occluding overlays cannot activate that state. The existing
  paragraph editor continues to own selection and copy.
- Shared text range decorators paint each fragment's glyphs and box chrome in
  one opacity layer. Container background/foreground paint methods reuse the
  widget implementation without creating layout/input nodes. Native selection
  paint takes priority over span colors and decorations, keeping selection
  visible; span opacity applies to selected glyphs, not selection chrome.
- Tests cover actual composited text/background/border pixels, explicit transparent
  glyphs, cached boundaries, live light/dark changes, multi-fragment press state,
  unchanged sibling geometry, selected-text retention/copy, keyed link reorder,
  keyboard activation, unavailable callbacks, removed observers, raw descendants,
  negative origins, clipping, modal occlusion and stationary-pointer reflow.
  Fixtures cover density 0.9/1/1.1 and display scales 1x/1.5x/2x.
- go build ./..., all six required Go packages, and the broad wox_automation
  command passed. Schema golden comparison and external Draft 2020-12 validation
  passed (12 valid and 24 invalid span-state documents). gofmt and git diff --check
  passed. Verification was headless; no interactive Wox session was launched.
- Stage 3 is complete. Stage 4 drawing transforms, implicit transitions, loop
  presets, enter/exit snapshots and injected-clock tests remain outstanding.

2026-11-06 affine-transform foundation continuation:

- Added shared AffineTransform geometry with explicit application-order
  composition, point mapping, exact rectangle silhouettes, conservative bounds,
  and guarded inversion. Logical translation and physical display conversion
  remain separate. Added convex clip intersection with either winding and no
  arbitrary small vertex cap; inputs remain immutable.
- Added internal command matrix data and software-reference interpretation.
  Vector shapes, raster images and reference glyph geometry share inverse pixel
  sampling. Exact clip polygons remain in target coordinates. Scene conversion
  composes onto recorded transforms instead of scaling local geometry twice.
  Collapsed transforms paint nothing; nonfinite maps return an error.
- Reference tests cover rotated clip corners, overlapping children inside alpha
  groups, nested transformed/untransformed layers, local-direction edge fades,
  text/image orientation, retained partial damage, following drawing, next-frame
  state reset, matrix-aware scene comparison and conservative caret-patch fallback.
  Geometry tests cover skew/reflections, negative origins, display scales
  1x/1.25x/1.5x/2x, tiny valid scales, overflow and 64 nested rotated clips.
- go build ./..., the six required package tests plus runtime/internal/graphics
  and runtime/internal/window, and the broad wox_automation command passed.
  Subsequently added scene-composition/fade/nested-layer regressions also passed.
  No WoxView fields changed, so the schema remains unchanged.
- This is a prerequisite, not a completed drawing-transform feature. Native
  encoders still need matrix and exact-clip support before adding the recording
  API, widget.Transform, paint-damage integration and WoxView mapping. Internal
  affine command fields are currently exercised only by tests; production
  recording retains its existing behavior. Stage 4 remains unchecked, including
  transitions, loop presets, enter/exit snapshots and injected-clock tests.

2026-11-06 Windows drawing-transform continuation:

- Windows now consumes internal command matrices before display scaling and
  restores identity between transformed and ordinary commands. Exact convex
  target-space clip paths use native geometry masks without a small vertex cap;
  replacement, clearing, layer boundaries and teardown release the correct mask.
- Floating materials sample the transformed physical envelope and replace pixels
  through the exact rounded silhouette, preserving pixels outside rotated corners.
  An offscreen regression exposed pending clip/composite layers hiding backdrop
  content. Clip sampling now flushes/restores the clip; groups containing materials
  use readable targets and premultiplied before/after interpolation. Ordinary
  groups retain native D2D layers. Nested materials see preceding group content,
  preserve translucent backdrop alpha and survive partial replay.
- Readable targets are reused only after EndDraw, bounded to a 16 MiB retained
  pool, included in resident-byte accounting, and released on resize, trim,
  hidden cache clearing and destruction.
- Added an offscreen WARP target that reuses the production Go encoder, frame
  lifecycle, DirectWrite drawing, image drawing and material graph. Tests cover
  exact rotated clips, opacity/fade composition, text/image orientation, backdrop
  sampling, singular/nonfinite matrices, following drawing, error recovery,
  retained damage, clip replacement with 64 vertices, and pool reuse/release.
  Fixtures include negative local origins and display scales 1x/1.25x/1.5x/2x,
  including changes on a retained target. Readable and native group paths have
  matching ordinary pixels at opacity 0/0.5/1 and with nested fades.
- go build ./..., runtime/internal/window tests, the six required package tests
  plus runtime/internal/graphics, and the broad wox_automation command passed.
  gofmt and git diff --check passed. Validation was offscreen; no interactive Wox
  application was launched. The WoxView schema is unchanged.
- Stage 4 is still incomplete. macOS/Linux matrix/clip encoding, the public
  recording API, widget.Transform with damage integration, WoxView transform
  mapping, implicit transitions, loop presets, enter/exit snapshots and clock
  tests remain outstanding. No stage-4 completion checkbox has been changed.

2026-11-06 macOS transform source and shared native fixtures:

- Added macOS command-matrix encoding, target-space convex clip paths, and a
  separate top graphics state for paint transforms. Clip replacement and group
  transitions preserve the requested matrix. Singular maps skip painting without
  concatenating a singular CTM. Overlay handoff retains the exact clip and matrix;
  frame/error teardown unwinds clip, paint and composite scopes.
- Floating-material sampling now uses the transformed physical envelope and
  restores target axes when drawing the sampled image through its exact local
  rounded mask. Material-containing groups retain before pixels over their
  physical envelope and interpolate before/after premultiplied channels at exit,
  allowing pending group content to remain readable without duplicating alpha.
  Ordinary groups retain CoreGraphics transparency layers; snapshots are freed
  at exit and teardown. The material-group classifier is shared with Windows.
- Extracted encodeDarwinCommands from the production frame wrapper so an
  offscreen bitmap fixture uses the same encoder, including text/images and
  error handling. Moved geometry/material/composite regressions into a shared
  Windows/macOS native suite, with platform-specific target adapters. Windows
  resource-pool tests remain Windows-specific.
- Native PR compilation now includes backend tests and graphics. The scheduled
  macOS/Windows smoke jobs run backend tests after plugin-host resources exist.
  Workflow YAML parsed successfully; no remote workflow was dispatched.
- Windows go build ./..., runtime/internal/window and graphics, the six required
  packages, and the broad wox_automation command passed. Shared native transform
  and backdrop regressions passed against WARP after the test extraction and
  classifier move. gofmt and git diff --check passed.
- Located a usable Ubuntu 26.04 WSL environment with GTK/epoxy and Xvfb. Its
  window package baseline passed; explicit GDK_BACKEND=x11/Xvfb opacity and
  edge-fade pixel tests ran and passed without skips. Linux transforms are not
  implemented by this continuation.
- macOS Objective-C compilation and actual CoreGraphics fixture execution have
  NOT run on this Windows host. Its new source must not be counted as verified
  native completion. The WoxView schema and stage-4 checkboxes remain unchanged.
  Linux encoding, native macOS validation, recording/widget/damage integration,
  WoxView mapping and all animation requirements remain outstanding.

2026-11-06 Linux native-transform continuation:

- Added logical command matrices to shape, image and text shaders, with identity
  restoration between commands. Exact convex clips use texture-buffer half-planes
  alongside coarse scissor/damage bounds. The actual GL buffer limit is checked;
  clip paths are not restricted to the fill primitive's 16-vertex limit.
- Composite scopes retain their entry matrix and transformed bounds. Children
  with independent matrices remain clipped to the parent silhouette; local fade
  direction survives rotation. Material snapshots use transformed physical bounds
  and target-coordinate sampling through local rounded geometry. Presentation
  corner masks bypass command state; overlay handoff preserves matrix and clip.
- Added clip-resource cleanup and resident-byte accounting for clip buffers and
  composite snapshots. Frame setup resets unfinished drawing scopes after errors.
- Extracted encodeLinuxCommands for an unmapped retained GTK/GL fixture. Shared
  native transform/backdrop regressions now run on Windows, macOS and Linux.
  The fixture releases current GL contexts between calls to support Go subtests;
  only tests enable actual material shaders on CPU GL rasterizers.
- Fixed the shared blur fixture to use a sharp image edge: magnifying its former
  two-pixel image produced a linear gradient that a correct Gaussian could leave
  unchanged. The sharper test passes on both Linux GL and Windows WARP.
- Linux X11/Xvfb window and graphics package tests passed, including all nine
  shared native transform/backdrop tests and existing opacity/edge-fade tests,
  without skips. The standalone EGL material shader test also passed. Scheduled
  Linux backend tests explicitly select X11 to use their Xvfb display.
- Windows go build ./..., all six required packages plus native window/graphics,
  and the broad wox_automation command passed. Shared native tests passed against
  WARP. gofmt and git diff --check passed. No interactive Wox session launched.
- This completes Linux internal encoding, not stage 4. Native macOS validation,
  public recording API, widget.Transform and damage integration, WoxView JSON
  transforms, implicit transitions, loop presets, enter/exit snapshots and clock
  tests remain outstanding. Schema fields and completion checkboxes are unchanged.

2026-11-06 public transform and protocol continuation:

- Added DisplayList.PushTransform/PopTransform with local-inside-parent
  composition. Clip pushes capture target geometry and intersect exact convex
  silhouettes; pops restore immutable previous clips. ClipRect exposes a
  conservative current-coordinate envelope for existing text/caret culling.
  Damage culling and floating-material metadata use target bounds, and scene
  conversion scales material regions and sample margins. Collapsed diagonal
  clips remain empty even after returning to ordinary drawing axes.
- Added independent widget.Transform with relative pivots and paint-only
  semantics. Layout, input, scroll extents and accessibility stay unchanged.
  Nested transforms include shadow, focus chrome, carets and opacity overflow.
  Cached boundary/key invalidation covers old and new target bounds. Boundaries
  affected by transforms retain Build caching but repaint conservatively because
  equal envelopes can contain different rotated pixels or ancestor clip shapes.
- Added WoxView style translateX/Y, scale and rotate with backward-compatible
  defaults. Signed Offset values accept numeric logical lengths and existing
  dimension tokens; Size still rejects negative layout dimensions. Scale zero
  collapses paint; rotation uses clockwise degrees around the border-box center,
  excluding margin, followed by translation. Shared widget/runtime layers own
  rendering; WoxView only maps validated values.
- Focused tests cover public nested stacks, clip restoration, 64 intersected
  rotated rectangles, collapsed clips, invalid transforms, material metadata,
  cached reflections with equal envelopes, transform removal, subtree removal,
  damage-culling/full-repaint pixel equality, negative-origin clipped shadows,
  opacity overflow, caret damage, unchanged input geometry, token density and
  focused draft preservation across document updates. Display scales include
  1x/1.25x/1.5x/2x and retained-target transitions.
- Windows go build ./..., the six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb window, graphics,
  widget and WoxView package suites passed. Subsequent focused collapsed-clip,
  material-bound and negative-origin regressions passed on both platforms.
- Regenerated Schema golden and updated protocol/native/UI architecture docs.
  Independent Draft 2020-12 validation passed for 24 valid and 28 invalid
  transform documents. The check caught and corrected rotate:null disagreeing
  with Go decoding. gofmt and git diff --check passed. No interactive Wox session
  was launched; macOS native compilation/execution has still not run.
- Remaining: the macOS native gate, implicit transitions, loop presets, enter/exit
  snapshots and injected-clock lifecycle tests. Stage 4 is not marked complete.

2026-11-06 implicit-transition continuation:

- Added Host.SetAnimationClock for caller-driven frames: one timestamp per frame,
  no timer/vsync scheduling with an injected clock, and timeline reset when the
  clock changes. Deterministic cached-boundary tests cover intermediate values,
  retargeting, completion caching, deletion and fresh remounts.
- Added the transition JSON contract with named property/curve enums, strict
  validation, shared Schema definitions and regenerated golden. Seven properties
  map to AnimatedFloat; RGBA channels interpolate independently. Default duration
  is 200ms, explicit zero cancels, and supported curves map to existing widget
  curves. Keys include the owning View and node identity.
- Local state patches are resolved before interpolation. Builders consume copied
  props and immutable per-frame style samples. Inline spans share opacity/color
  timelines across all fragments, including selectable paragraphs; existing
  selection paint and input behavior remain intact.
- Protocol/render tests cover all seven channels, three curves, retargeting,
  new keys, removed handlers/input, fresh View ownership, zero duration, cached
  hover return, token density, natural-size resets and source-tree immutability.
  Frames cover DPI 1x/1.25x/1.5x/2x. No wall-clock sleeps drive new tests.
- Windows go build ./..., the six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb widget and WoxView
  package suites passed. Normal Schema golden comparison passed; independent
  Draft 2020-12 validation passed for 22 valid and 42 invalid transition documents.
  gofmt and git diff --check passed. No interactive Wox application launched.
- Stage 4 remains incomplete. Returning an authored control paint override to
  its omitted native default still snaps that native fill; this needs a shared
  control capability rather than duplicating default colors in WoxView. Explicit
  positive dimensions interpolate; natural/auto dimensions retain their existing
  snap semantics. Loop presets, enter/exit snapshots and their clock-driven tests,
  plus actual macOS compilation/native execution, remain outstanding.

2026-11-06 native-control transition continuation:

- Added generic widget.AnimatedColor using four AnimatedFloat channels and the
  existing Host clock. Independent tests cover RGBA, alpha, retargeting, clamped
  overshoot, zero duration and empty keys.
- Shared ControlPaint now carries optional background timing. Each control
  resolves its actual native fill before interpolation, including theme, hover,
  selection and disabled policy. WoxView sends authored targets rather than
  duplicating those policies or feeding already-interpolated samples back in.
- Buttons, icon buttons, text/number fields, dropdowns, checkbox/switch tracks,
  sliders, tags, code, hotkeys, lists/segments and tables share this capability.
  Composite parts have distinct stable keys, including nested choice tags. Table
  header/body regions retain different defaults and composite translucent fills
  once. Controls without transition timing keep their ordinary widget path.
- Outer WoxView control decoration no longer duplicates the transitioning fill.
  Group backgrounds remain independently animated behind distinct selected rows.
  Preserved text-field transparent-default semantics; a full JSON regression
  caught and fixed passing the outer sampled color as the native field target.
- Added 14 shared-control pixel cases plus JSON hover-return regressions for
  button, icon button, text field, tag and table. Coverage includes default/clear
  restoration, midpoint symmetry, alpha, border wrappers, theme-token changes
  during focused editing, source immutability and DPI 1x/1.5x/2x.
- Windows go build ./..., the six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb widget, WoxView and
  shared-component suites passed. The unchanged Schema golden comparison passed;
  gofmt and git diff --check passed. No interactive Wox application was launched.
- The implicit-transition item is complete. Stage 4 still requires pulse/spin/
  shimmer loops, enter/exit snapshots with inert removed nodes, their injected-
  clock tests, and actual macOS native compilation/execution. The overall goal
  remains active and is not complete.

2026-11-06 loop-preset continuation:

- Added widget.PaintOverlay for noninteractive foreground paint after one child
  measurement. Independent tests cover geometry, clipping, opacity, transforms,
  display scale, focus and input. Uncached LoopAnimation now schedules local
  damage as well as cached-boundary loops, preserving transformed paint bounds.
- Added the animation JSON contract with required pulse/spin/shimmer type and
  optional 1..60000ms period (default 1200ms). Schema, parser, Go-tree validation
  and README agree. Loops are keyed by View owner, node identity and preset;
  period changes restart through the shared LoopAnimation implementation.
- Pulse multiplies effective opacity between 1 and 0.65; spin adds clockwise
  rotation; shimmer sweeps a clipped neutral band inside the border box. Two
  immutable 256x1 premultiplied textures cover light/dark appearance without
  per-frame image allocation. Presets compose after implicit transitions.
- Inline spans share phase across wrapped fragments in selectable and ordinary
  text. Fragment spins remain inside the paragraph clip. Shimmer respects state
  opacity and skips selected text so native selection colors remain intact.
- Clock-driven tests cover loop phase/pause/period changes, key/type changes,
  deletion/remount, immediate cancellation, layout/input invariance, cache reuse,
  transition composition, seam-free repeat, source immutability, theme appearance,
  asymmetric margins and DPI 1x/1.25x/1.5x/2x. A scheduler test also verifies actual
  local invalidation for an uncached loop.
- Windows go build ./..., all six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb widget, WoxView and
  shared-component suites passed. Regenerated Schema golden and normal comparison
  passed; independent Draft 2020-12 validation passed for 39 valid and 33 invalid
  loop documents. No interactive Wox application was launched.
- Loop presets are complete. Enter/exit snapshots, inert removed nodes and their
  lifecycle tests remain, as does actual macOS native compilation/execution.
  The overall goal remains active and is not complete.

2026-11-06 entry-animation continuation:

- Added AnimatedFloat.Initial for an optional first-mount value, preserving
  retarget continuity and the existing nil-initial behavior. Independent clock
  tests cover initial changes, new keys, removal/remount and zero duration.
- Added enter shorthand and strict object contracts for fade/slideUp/scale,
  with a default 200ms cubic curve and an optional 0..60000ms duration. Entry
  multiplies effective opacity/scale and adds density-scaled logical travel.
  It composes after transitions and before loops without mutating source props.
- Keyed visual state captures the mount declaration once. Producer echoes,
  timing changes and later additions cannot replay the mount; removal cancels
  immediately. Completed entries retire their clocks. A fresh View, new key or
  remounted node gets fresh entry state. Inline fragments share this lifecycle
  while preserving paragraph clipping, selection and observer state.
- Clock-driven tests cover intermediate pixels, cancellation, remount, source
  immutability, cached boundaries, transition/loop composition, focus/editing,
  selectable and ordinary spans, logical density and DPI 1x/1.25x/1.5x/2x.
- Windows go build ./..., all six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb widget, WoxView and
  shared-component tests passed. Schema golden regenerated and normal comparison
  passed; independent Draft 2020-12 validation passed for 44 valid and 50 invalid
  entry documents. gofmt and git diff --check passed. No interactive Wox launched.
- Exit snapshots, inert removed nodes and their lifecycle tests remain, together
  with actual macOS native compilation/execution. The full goal is not complete.

2026-11-06 paint-snapshot foundation:

- Added runtime snapshot scopes and balanced subtree capture/replay. Snapshots
  retain immutable commands/images, frozen caret phases, exact clips and internal
  ancestor groups, with logical-coordinate rebasing. Outer groups remain live.
  Capture suspends recording culls but preserves native damage scissoring. Replay
  intersects the current parent clip and restores material damage metadata.
- Added widget.CapturePaint and SnapshotScope/SnapshotLayer. Replay contains only
  paint data, consumes no layout space, cannot retain old focus/semantics/gestures,
  and reports transformed overflow for damage and enclosing opacity groups.
  Snapshot storage/lifetime remains the feature owner's responsibility.
- Native pixel comparison exposed local image-edge rounding before affine maps
  on Windows. Axis-aligned image edges now snap in physical target coordinates;
  rotated/skewed images remain unsnapped. Equivalent rebased draws now agree.
- Tests cover software and actual Windows/Linux native replay of text, images,
  caret phases, rotated clips, inherited/nested opacity and fades, floating
  material groups, changed origins and DPI 1x/1.25x/1.5x/2x. Widget tests prove
  old callbacks/focus are gone, input reaches live content, natural layout can
  collapse, and cached partial-frame removal leaves no stale snapshot pixels.
- Windows go build ./..., all six required packages plus window/graphics, and
  the broad wox_automation command passed. Linux X11/Xvfb window, graphics,
  widget, WoxView and shared-component package tests passed. gofmt and
  git diff --check passed. No interactive Wox application was launched.
- The JSON contract is unchanged: exit is still rejected. WoxView snapshot
  retention, exit presets/timing, immediate event invalidation and lifecycle
  tests remain, as does actual macOS compilation/native execution. The overall
  goal remains active and is not complete.

2026-11-06 exit-animation continuation:

- Added exit fade/slideUp/scale with the same validated shorthand/object contract
  as entry, including zero-duration removal. View retains only the last actually
  painted nodes; hidden and virtualized rows cannot leave stale ghosts.
- Removed nodes lose layout, focus, input and event capabilities immediately.
  Reusing a handler on another node cannot revive old callbacks. Parent snapshots
  suppress duplicate child exits; reinsertion, namespace changes and document
  disposal cancel and release retained paint without restarting unrelated exits.
- Each exiting node uses one keyed clock and one group opacity/transform. Inline
  snapshots include editor selection paint, and shared captured ancestor groups
  composite once. Distinct sibling groups keep separate identities. Whole-line
  selection avoids an extra clip that would multiply native antialias coverage.
- Clock-driven tests cover all presets, midpoint pixels, cancellation, key reuse,
  nil roots, cached damage, inherited opacity, selected and ordinary inline spans,
  non-interactive reflow, and DPI 1x/1.25x/1.5x/2x. Runtime tests separately cover
  fragment composition and clipped shared paint in software and native renderers.
- Windows go build ./... and all six required packages plus graphics/window
  passed. Linux X11/Xvfb window, graphics, widget, WoxView and shared-component
  suites passed. Schema golden comparison, gofmt and git diff --check passed.
  The broad wox_automation package command recorded above also passed.
- All requested feature implementations now exist. Actual macOS compilation and
  native execution remain unverified; the platform validation item stays open.
  No interactive Wox application was launched. The full goal is not complete.

2026-11-06 final audit and macOS handoff:

- The user explicitly requested that macOS testing be left to them. This closes
  the agent's remaining environment dependency without claiming macOS passed.
- PR native compilation now includes WoxView, pluginwindow, shared components,
  and plugin/host contracts. Corrected the workflow path filter to include both
  existing UI workflows. Scheduled native tests include the same feature packages
  on Windows/macOS; the Linux unit job explicitly covers those contracts too.
- Both edited workflows parsed successfully as YAML. The expanded PR compile
  command passed on Windows. The exact Linux contract package set passed under
  X11/Xvfb with TMPDIR in the user's home filesystem. WSL's tmpfs-backed /tmp
  rejects gio trash, so the first run failed an existing single-file-plugin
  uninstall test; changing only TMPDIR made that full suite pass. No production
  uninstall behavior or test assertion was changed.
- Re-ran the SDK/host suites: Node.js 37/37 and Python 50/50 passed. The current
  Windows build, required Go packages, broad automation command, Linux native
  renderer suites and Schema golden comparison passed in the preceding exit
  verification. Workflow changes do not alter those production sources.
- All review phases are implemented, documented and covered by their focused
  tests. Formatting and git diff --check passed. Work remains uncommitted and
  unpublished; macOS native execution is the explicit user-owned follow-up.
