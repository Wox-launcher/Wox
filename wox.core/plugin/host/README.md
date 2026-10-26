# Plugin host RPC

`host_websocket.go` owns the host connection, incoming envelopes and outgoing
Core-to-plugin calls. `plugin_request.go` resolves plugin callers, propagates
launcher/query context, dispatches plugin-to-Core calls and writes one response
per completed request. Unknown methods, missing callers, invalid required
arguments and API panics produce error responses rather than leaving host
promises unresolved. A result that cannot be serialized also receives an error.
Disconnected transports can only log a failed send; this is not a delivery guarantee.

Operation handlers live with related protocol behavior:

- `plugin_request_app.go`: app visibility, clipboard, screenshot, appearance,
  notifications, logging and toolbar messages.
- `plugin_request_query.go`: query changes, query commands, trigger keywords and
  incremental results.
- `plugin_request_settings.go`: setting reads and both setting-write contracts.
- `plugin_request_callbacks.go`: callback registrations and callback payloads.
- `plugin_request_tools.go`: plugin tool options, structured errors and callback
  invocation ownership.
- `plugin_request_ai.go`: AI stream startup and stream status callbacks.

Handlers decode the existing string-valued parameters into plugin API types and
return `(result, error)`. They never write a response directly. Required string
parameters distinguish absence from an empty value. Keep method names, parameter
keys, response field casing and the following compatibility behavior unchanged:

- Void calls acknowledge with `""`; missing updatable results return `null`.
  Present updatable results remain JSON encoded inside a string.
- Screenshot/theme options retain their existing defaults. Optional query hints
  and clipboard image decoration retain their existing fallback behavior.
- `SaveSetting` remains supported, including its boolean string conversion.
  `RefreshQuery` accepts both selected-result key spellings.
- Trigger registration and plugin tools retain errors inside their named result
  structs. AI stream startup failures retain an empty acknowledgement and report
  the failure through `onLLMStream` when a callback ID is available.
- Callback registration replies and later callback RPCs are separate messages.

Both Node and Python SDKs keep their published signatures and wire format.

Core-to-host query and MRU requests carry the optional internal `ManagedQuery`
flag. Core releases those callbacks with `releaseQueryResults`, scoped by the
envelope's `SessionId` and `QueryId`, after evicting the result set. This message
applies to all plugins in that host and does not invoke plugin code. Hosts retain
callbacks by session, query, result and action IDs; publishing a later query
never clears another query's callbacks. An explicit scope miss never falls back.
Unscoped legacy access requires an unambiguous result, and hosts retain at most
32 queries per session for older Core builds without managed lifetimes.

Query, push, update, form and MRU paths use the same host-owned callback store.
Released buckets reject late responses, while plugin unload invalidates the
store held by that plugin's API. Update patches preserve omitted callbacks of
the same type and remove omitted actions when the action list is replaced.

This boundary does not impose a new deadline on plugin-to-Core calls; operations
such as screenshot selection may wait for user interaction. It prevents the
dispatcher's silent exits from stranding requests while the connection is alive.

Run `go test ./plugin/host` from `wox.core` for protocol tests. The WebSocket tests
cover error delivery, correlation, legacy defaults and callback behavior without
launching plugin processes or changing user settings.
