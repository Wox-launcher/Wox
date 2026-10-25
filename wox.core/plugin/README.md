# Plugin result processing

The manager retains plugin lifecycle and query scheduling. Its internal
`queryResultStore` owns session/query result maps and deferred image tokens.
Result presentation is organized within the same package:

- `result_polish.go` owns initial result processing, ranking, remote preview
  wrapping, cache publication and the existing per-stage timing diagnostics.
- `result_update.go` owns incremental patches and exporting cached results through
  `GetUpdatableResult`, including restoring system actions and callback merging.
- `result_normalize.go` contains rules used by both paths: action identity/type
  defaults, action icon and tail-icon conversion, default action selection,
  platform hotkeys, action/form translation, tail translation and query-pin tails.
  It also owns action proxies and drag-payload normalization.
- `query_result_store.go` owns result storage, scoped and legacy lookups, retention
  and cached diagnostic fields. The per-session limit remains 32 queries.
- `query_result_icons.go` owns lazy-token registration, expiry and hydration,
  including the existing plugin-image lock through publication.
- `query_result_access.go` keeps the public Manager methods and plugin lifecycle
  callbacks separate from store operations. `result_icon.go` chooses image sizing
  and delegates deferred ownership to the store.

Explicit session/query lookups never fall back to another query's result. Calls
without that scope retain the legacy lookup behavior. Starting a query preserves
other retained result sets so responses can finish out of order; storing a result
does not recreate an evicted set. Hiding keeps the newest set and its tokens.
Destroying a session invokes the existing plugin-leave flow before releasing the
results and tokens. Temporary result-hotkey sessions also release both together.

The store intentionally retains the existing shared result pointers, map locks
and alias-precedence lock. This extraction does not introduce immutable snapshots,
new scheduling/cancellation rules, or synchronization around incremental fields.
Sorting and UI snapshot construction remain presentation responsibilities.

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

Existing cache lifecycle and scope tests live in `query_result_store_test.go`;
lazy-image tests also remain in `image_cache_test.go` and `manager_test.go`.
Other result coverage lives in `query_polish_test.go`, `manager_test.go`,
`result_binding_test.go`, `query_fallback_test.go` and `webview_update_test.go`.
Run `go test ./plugin` from `wox.core`, then the repository's core, Go UI unit and
native smoke suites for end-to-end validation.
