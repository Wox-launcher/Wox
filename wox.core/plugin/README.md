# Plugin result processing

The manager retains plugin lifecycle, query scheduling and session cache ownership.
Result presentation is organized within the same package:

- `result_polish.go` owns initial result processing, ranking, remote preview
  wrapping, cache publication and the existing per-stage timing diagnostics.
- `result_update.go` owns incremental patches and exporting cached results through
  `GetUpdatableResult`, including restoring system actions and callback merging.
- `result_normalize.go` contains rules used by both paths: action identity/type
  defaults, action icon and tail-icon conversion, default action selection,
  platform hotkeys, action/form translation, tail translation and query-pin tails.
  It also owns action proxies and drag-payload normalization.

The two entry points deliberately keep separate orchestration. An omitted patch
field retains its cached value; supplied empty strings or collections still pass
through the existing update rules. In particular, replacing actions regenerates
system actions, and replacing tails regenerates system decorations. MRU updates
reuse their restore-owned system callbacks. Host updates recover callbacks by
action ID, unchanged result icons remain no-ops, and WebView script updates do not
replace the cached HTML preview.

Initial polish retains batch icon caches and per-result lazy-icon ownership.
Incremental polish uses the same action image converter for both `Icon` and
`TailIcon`, including plugin-relative paths. Normalization does not add deadlines,
change published plugin API types, rerank incremental results, or write settings.

Existing coverage lives in `query_polish_test.go`, `manager_test.go`,
`result_binding_test.go`, `query_fallback_test.go` and `webview_update_test.go`.
Run `go test ./plugin` from `wox.core`, then the repository's core, Go UI unit and
native smoke suites for end-to-end validation.
