# Wox Architecture

This guide explains how Wox is split across the repository and how data moves through the app at runtime.

## The big picture

Wox is a desktop launcher whose Go core and native Go UI run in one process. Third-party plugins run inside dedicated language hosts and communicate with the core over WebSocket-based JSON-RPC.

At a high level:

```text
Go UI + wox.core  <->  plugin hosts  <->  plugins
```

## Main components

### `wox.core`

`wox.core` is the runtime center of Wox. It is responsible for:

- query routing
- built-in plugin execution
- third-party plugin lifecycle and metadata loading
- settings and data storage
- the in-process UI contract and the small primary-instance control endpoint
- packaging the final desktop runtime assets

Areas worth knowing early:

- `wox.core/plugin/`: plugin contracts, manager, query/result models, host bridge
- `wox.core/common/`: shared UI payloads and common runtime types
- `wox.core/setting/`: setting definitions and persistence
- `wox.core/appcontrol/`: the loopback-only `/ping`, `/show`, deeplink, and diagnostic restart controls
- `wox.core/ui/`: launcher, retained widgets, automation contract, and native platform runtime
- `wox.core/resource/`: embedded UI, host binaries, translations, other runtime resources

### `wox.core/ui`

This is the desktop UI that users see. It renders:

- the launcher window
- result list and action panel
- settings pages
- screenshot flows
- webview previews and related native bridges

The UI is compiled in the `wox.core` Go module and runs in the same process. It does not own plugin execution. Its job is to render state, send user intent back to the core, and host platform-specific presentation logic.

Lifecycle, query/action execution, terminal subscriptions, and every core-to-UI update cross typed `Services`/`View` interfaces. Settings and catalog screens still have a transitional in-process adapter over the old router while those domains are migrated; it opens no listener. The only production loopback HTTP surface is `appcontrol`, and test automation is compiled only with the `wox_automation` build tag.

### `wox.plugin.host.nodejs` and `wox.plugin.host.python`

These are long-lived host processes for full-featured plugins. They:

- start the correct runtime
- load plugin code from `~/.wox/plugins`
- expose the public plugin API to plugin authors
- relay plugin requests and callbacks back to `wox.core`

This host layer is where SDK/runtime compatibility matters. If a plugin API shape changes, the core, host, and SDK types need to stay aligned.

### `wox.plugin.nodejs` and `wox.plugin.python`

These are the SDKs used by third-party plugin authors. They provide:

- typed query/result models
- public API wrappers
- plugin bootstrap helpers

## Runtime flow

### 1. Query handling

When a user types in Wox:

1. the Go UI calls the typed query service in `wox.core`
2. `wox.core` decides which built-in plugins and third-party plugins should run
3. built-in plugins execute directly in Go
4. third-party plugins are invoked through the matching plugin host
5. results are aggregated and returned to the UI
6. the UI renders the result list, preview, tails, and actions

### 2. Action execution

When a user triggers a result action:

1. the UI sends the selected action context to `wox.core`
2. `wox.core` resolves whether the action belongs to a built-in plugin or a hosted plugin
3. the action runs in the correct runtime
4. follow-up UI updates can happen through APIs such as `UpdateResult`, `PushResults`, `RefreshQuery`, `Notify`, or `HideApp`

### 3. Plugin-initiated UI flows

Some flows start from a plugin instead of the launcher UI. For example:

- toolbar messages
- deep links
- screenshot capture
- clipboard copy
- AI streaming responses

The plugin calls the SDK API, the host forwards the request to `wox.core`, and the core coordinates the UI or native platform behavior.

## Why the boundaries matter

Understanding the ownership boundary saves a lot of debugging time:

- If the problem is about query routing, plugin metadata, settings persistence, or runtime contracts, start in `wox.core`.
- If the problem is visual or input-related, start in `wox.core/ui`.
- If a third-party plugin API works in one language but not another, inspect the host and SDK layers together.

## Repository workflow

The top-level `Makefile` is the entrypoint for cross-project development:

- `make dev`: prepare shared resources and build plugin hosts
- `make test`: run the Go test suite under `wox.core/test`
- `make test-go-ui-unit`: run retained-widget and automation contract tests
- `make test-go-ui-smoke`: run the authenticated native launcher smoke and visual golden
- `make build`: build the full application and packaging outputs

For changes that touch shared contracts, `make build` is the verification step that matters most.

## Runtime data and logs

Wox stores runtime data under the user's home directory:

- macOS / Linux: `~/.wox`
- Windows: `C:\Users\<username>\.wox`

Useful locations:

- `~/.wox/plugins/`: local third-party plugins
- `~/.wox/log/`: runtime logs

When debugging plugin or UI issues, start from the logs and the exact boundary where the failure happens rather than guessing which layer is wrong.
