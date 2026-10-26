# Plugin management, queries and result processing

Manager coordinates the public entry points, query planning and business
processing. Internal owners manage plugin lifetimes, UI callbacks, query timing
and result retention. These responsibilities remain in the plugin package.

## Plugin and host lifecycle

`pluginLifecycle` owns the installed instance list, snapshot lock, per-plugin
load locks, offline transition lock, file watchers, reload timers, and host
watchdog/restart state. It has no Manager reference. `manager_lifecycle.go`
preserves the public entry points and keeps UI refreshes in the coordinator.

- `plugin_loader.go` discovers plugins and performs host loading and Init.
- `plugin_lifecycle.go` owns registration, enable/disable, unload and shutdown.
- `host_lifecycle.go` and `host_watchdog.go` share the same restart lock and
  recovery state, including cooldowns and metadata restoration.
- `script_plugin_watcher.go` and `single_file_watcher.go` own their distinct
  watch policies and reload flows. `plugin_metadata.go`, `inline_metadata.go`
  and `single_file.go` retain metadata validation and source identity rules.
- `offline.go` suspends/resumes runtimes without changing saved enablement.

Startup still initializes system plugins on the calling UI thread, starts
third-party hosts asynchronously, and serializes loads by plugin ID. Shutdown
stops the watchdog before closing watchers, cancelling pending reload timers
and stopping hosts. This refactor does not add cancellation or waits for already
executing reload/Init callbacks, or change plugin/SDK contracts.

`plugin_presentation.go` owns Glance/Toolbar callback stores and active query-session
routing, using instance lookup and a live UI getter rather than a Manager pointer
or a duplicate UI reference.
Manager wires its release callback into the lifecycle owner. Unload, disable,
offline suspension, failed Init, successful host replacement and shutdown discard
the affected plugin's cached UI actions and clear its visible Toolbar message.
Other plugins and query-session routing are preserved. Existing Glance refresh,
Toolbar replacement and session-leave cleanup remain in this owner; result-query
callbacks remain in the result store.

Lifecycle, watchdog and presentation tests use fake hosts and UI capabilities;
no real plugin process or native window is needed for these regressions.

## Query scheduling

`query_scheduler.go` owns the plugin-wide debounce timers, per-query executions,
fallback/final completion counters, result delivery latency and first-flush
deadline calculation. Manager selects eligible plugins and supplies three
callbacks: execute a plugin query, observe its response for history, and restore
result aliases. Neither execution nor scheduler holds a Manager reference.
Alias matching, restoration and polishing stay in `result_binding.go`; their
goroutine, delivery and completion are owned by the scheduler.

All jobs are counted before any starts. Debounced jobs count toward `Done` but
do not block fallback; stopping a replaced timer completes its old job. Timer
keys remain plugin IDs across sessions. Ordinary plugin responses are delivered
before completion even when their context was canceled; alias restoration keeps
its cancellation checks and cancellable send. The public `QueryExecution`
channels, buffering, completion ordering and 12-20 ms first-flush policy are
unchanged. This extraction adds no cancellation on window hide, session teardown
or Manager stop, and does not change timer synchronization or panic recovery.

The existing alias-job deadline test lives in `query_scheduler_test.go`.
Query planning, fallback, result binding and UI smoke coverage continue to
exercise the Manager entry point.

## Result processing

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
results and tokens. Silent-query and temporary result-hotkey sessions also release
both together after the action finishes or the query exits.

External host callbacks share this retention lifetime. Query and MRU execution
contexts carry a small cleanup registry without retaining the result set itself.
Hosts register cleanup after their RPC finishes (including cancellation), so
eviction during an in-flight request triggers cleanup on completion. Pruning,
hide trimming and session destruction release the corresponding host query
callbacks. Registration and release are synchronized; transport cleanup runs
outside store locks and does not block the UI.

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
