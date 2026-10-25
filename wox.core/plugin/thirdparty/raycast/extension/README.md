# Local Raycast extensions

This compatibility layer runs a subset of Raycast commands through existing Wox capabilities. It does not add a Raycast store, OAuth, background schedules, menu bar, or a separate UI toolkit.

## Import and run

1. Build Wox's hosts with `make host`. Development changes to this runtime can be rebuilt with `make -C wox.core/plugin/thirdparty/raycast/extension/runtime build`. Restart Wox to extract the new embedded bundle.
2. Keep the Raycast extensions installed locally, including `package.json`, `<command-name>.js` CommonJS entry points, assets, and any external dependencies the commands need. Import does not download packages, build extensions, or run package scripts.
3. Open the onboarding migration wizard, choose Raycast, unlock the `.rayconfig` backup, and select the available local plugins. This is the supported import entry point; pasting paths or selecting files does not offer Raycast installation.
4. Enter `<extension-name> ` to open a plugin with one available command and search directly with `triggerkeyword search`, such as `kill-process chrome`. Plugins with multiple available commands show command discovery; selecting one fills `triggerkeyword command search`, such as `port-manager open-ports `. The only default trigger keyword is the manifest name, without an added prefix or global `*` keyword. Raycast does not enter QueryScope. For multi-command plugins, delete the command to return to discovery; clear the input to return to global search. Clearing search preserves the trigger and, for multi-command plugins, the command. Extension and command preferences appear in Wox's plugin settings.
5. Disable/uninstall through Wox's existing plugin management. The migration wizard skips plugins that have already been imported.

Commands marked `disabledByDefault` are excluded from command discovery and Wox command hints. Commands without their own icon inherit the extension icon.

Raycast `fileIcon` objects use Wox's existing system file-icon loader. Emoji strings use Wox's emoji loader; asset paths containing emoji remain file paths.

Enter on a command discovery result completes the query prefix first. Single-command plugins omit the command from both that prefix and Wox command hints, so its internal name remains valid search text. A single text argument uses the query input instead of a form, whether optional or required; required text waits for nonblank input. For view commands, the initial search text seeds the argument and subsequent edits use the view's search callback. Headless commands run only after Enter on the execution result. Multiple and non-text arguments use the existing form flow at the command's query entry.

The onboarding migration wizard reads `nodeExtensions.extensions`, matches each UUID under `~/.config/raycast/extensions`, and checks the local manifest name, author, owner and platform. The backup contains references, not executable bundles. Missing, mismatched and unsupported extensions remain visible with an explanation. Selected extensions use the internal local installer; already imported extensions are shown as imported and are not overwritten on retry. Resetting the migration session cancels unfinished copies.

Explicit preferences from `settings.nodeExtensions[].meta` and `settings.commands[].meta` are joined by extension/command identity and filtered against the local manifest. Text, password, checkbox and dropdown values use Wox's existing plugin setting storage and sync path. Missing/null values keep defaults; empty strings and false remain explicit values. Passwords/API keys are restored only when included in the backup. Existing Wox overrides are preserved, and setting values are never included in import diagnostics. Runtime metadata, local storage, stored dropdown state and OAuth sessions are not imported. Settings and sync records commit in one transaction; a settings failure rolls back the installation through the existing file/host rollback path.

`testdata/example` is a self-contained, prebuilt local extension for development through the internal installer. It exercises asynchronous results, refinement, preview, grid, form submission, and a manually invoked no-view command without external services.

Installed files live under the reserved `plugins/raycast/<plugin-id>` collection. Import stages and validates files before replacement; activation failure restores the previous copy. Symlinks/special files, overlapping source/collection directories, more than 20,000 files, or more than 256 MiB are rejected. Declared platform restrictions are respected. Extensions execute with the same user-level Node access as other local Wox Node plugins; this adapter is not a sandbox.

## Capability mapping

| Raycast | Existing Wox capability |
| --- | --- |
| List and sections | Query results and groups |
| List.Item accessories | Ordered image/text result tails, with tooltips preserved |
| List/Grid search dropdown | Single-select Refinement |
| Grid | QueryResponse GridLayout |
| Detail / List.Item.Detail | Markdown Preview |
| Action / ActionPanel | Result actions and hotkeys |
| confirmAlert in a result action | Update the invoking result; Enter confirms, Ctrl+Enter cancels |
| Form + Action.SubmitForm | Form action, text/password/select/checkbox fields |
| React effects and asynchronous updates | UpdateResult for stable rows; RefreshQuery for membership/order/layout/refinement changes |
| Empty List/Grid with isLoading | Keep the query pending for Wox's existing loading animation; no placeholder row |
| Copy, open, reveal, notification, hide | Existing Wox/OS operations |
| Preferences / LocalStorage | Plugin settings; LocalStorage is local-only |
| Cache / supportPath | Per-plugin cache directory |
| Cache.subscribe | Namespace notifications update React; the existing UpdateResult / RefreshQuery bridge publishes changes |
| view / no-view | Explicit command activation; no scheduler |

The form is opened by its submit action in Wox's existing form UI. Live field callbacks, selection callbacks, and pagination are rejected because this first adapter does not map those interactions. Action sections/submenus flatten into the existing Action Panel. Navigation uses an in-process React stack and a Back action; returning remounts the previous page. Search matching is a simple case-insensitive word filter, not Raycast's fuzzy ranker. Symbolic icons use a small Wox catalog mapping with a fallback; icon tint/mask and some decorative props are not reproduced. Unsupported APIs report an error when invoked. Optional Paste/CreateQuicklink actions can render alongside usable actions, but invoking them reports unsupported. Frontmost-application queries, directory/app-picker preferences and OAuth are not implemented. `@raycast/utils` must be bundled or supplied with the extension and can only use the implemented API subset.

Cache instances share data and subscriptions by namespace within a command process. Set, remove and clear notify subscribers after persistence succeeds; `clear({notifySubscribers:false})` suppresses notification, and the returned unsubscribe function is safe to call repeatedly. The method can be passed unbound to React's `useSyncExternalStore`. Files persist across command runs; currently separate running command processes do not receive live cache notifications from each other. No polling task or new Wox UI capability is added.

Empty loading views wait on runtime render notifications in Go, so cancellation releases the query and a new search can still reach the same React process. Views that already have results remain usable during background loading. When loading completes with no results, the existing empty-view mapping applies.

`confirmAlert` replaces the invoking result's title, description and actions while its Promise waits. Cancellation restores the latest row; changing the query or leaving/unloading the command cancels the pending decision. Async refreshes do not overwrite the confirmation. Primary/dismiss labels and callbacks are honored; `rememberUserChoice` is not persisted, so each call still asks. Calls without an active result action return false. Confirmation waiting pauses the action timeout without changing other runtime timeouts.

## Ownership and lifecycle

The Go host owns imports, manifests, settings, processes and query sessions. A command gets a dedicated Node process per Wox window. The runtime owns React reconciliation and callbacks, then emits Wox query DTOs over JSON Lines; Go and the UI never receive a React tree. API requests and command replies use separate correlation IDs, so an action awaiting a host API does not deadlock its own reply.

Search/refinement changes reuse React state. Leaving the extension, returning to command discovery, changing commands, destroying the Wox window, disabling, or unloading closes the session. Hiding Wox retains the current command until its query is left. Closing allows a short React cleanup grace period before killing an unresponsive process. Unload waits for Node to exit before Windows can remove the working directory. Late actions are bound to the original process and cannot target a replacement session. This does not promise termination of arbitrary child processes spawned by extension code.

Runtime compatibility code, sources and tests stay in this feature directory. The parent Raycast migration package owns backup discovery and wizard orchestration and calls `extension.Import` directly, following the same feature-owned import structure as Flow Launcher. Raycast owns validation, staged copying, replacement rollback and runtime loading, plus serialization with its uninstall path. Successful imports refresh the existing plugin settings list. Store does not recognize or dispatch Raycast imports. Core delivers the existing leave-query event when a window is destroyed.

## Verification

From the repository root:

```sh
make -C wox.core/plugin/thirdparty/raycast/extension/runtime test
cd wox.core
go test -tags sqlite_fts5 ./plugin/thirdparty/raycast/... ./plugin
```

Runtime tests exercise real React/Node processes, initial search/refinement delivery, async updates, awaited host calls, grid filtering, typed form submission, unsupported APIs, navigation and effect cleanup. A TSX fixture uses the classic JSX factory configuration verified in the published `@raycast/api@2.8.2` CLI; automatic JSX-runtime imports are supported separately. Go tests cover import validation/rollback, command routing, UpdateResult versus RefreshQuery, multi-window isolation, stale callbacks and process release. The direct-import integration test imports, executes, replaces a running extension while retaining settings, and uses normal plugin management to uninstall using a temporary Wox database and host directory in an isolated process. Process tests require Node and the built host bundle; otherwise they explicitly skip. The symlink test may skip on Windows without link-creation privileges.

`TestExtensionWizardImport` exercises Unlock, Catalog, Import, command discovery and repeat-import behavior with a temporary Wox database. To repeat verification against an actual backup, provide `WOX_RAYCAST_VERIFY_BACKUP`, `WOX_RAYCAST_VERIFY_PASSWORD` and `WOX_RAYCAST_VERIFY_EXTENSIONS` in the process environment and run that test. Only extension rows are selected. The backup and installed Raycast directories are read-only inputs; decrypted private data is not saved as a fixture.

The runtime tests optionally use `WOX_RAYCAST_VERIFY_EXTENSIONS` to execute installed Lorem Ipsum (`words`), Random (`index`) and Change Case (`convert-upper-case`) bundles copied into temporary directories. Clipboard and other host calls use synthetic data, so these tests do not alter the desktop. Verification with the supplied October 17, 2026 backup found 11 local extensions: 10 imported and exposed commands; GitHub was unavailable because its directory/app-picker preferences are unsupported. The three commands above rendered/generated text and invoked copy successfully. Change Case's interactive view still requires `getFrontmostApplication`; successful import does not imply every command or API is supported.

API behavior references: [Raycast List](https://developers.raycast.com/api-reference/user-interface/list) and [extension manifest](https://developers.raycast.com/information/manifest). This adapter is implemented locally and does not include Tinycast or Vicinae runtime code.

With the same optional extension-root environment variable, the Port Manager test runs its installed `open-ports.js` against a temporary loopback listener. It verifies real port enumeration, cached detail toggling and exposure filtering through Wox refinements. It never invokes kill actions. A Go process test separately verifies cache subscriptions trigger UpdateResult for changed rows and RefreshQuery for membership changes, and deleting the command releases the React process.

Set `WOX_RAYCAST_VERIFY_TOOTHPICK` to a local Toothpick extension directory to run `TestLocalToothpickCommandDiscovery`. It copies the actual bundle to a temporary directory, verifies only the enabled Bluetooth management entry and its inherited icon are exposed, and checks activation produces a trigger-only Wox query. It does not invoke Bluetooth operations.

Set `WOX_RAYCAST_VERIFY_YOUTUBE` to a local YouTube extension directory to run `TestLocalYouTubeSearchActivation`. It checks the real video/channel/live search manifests, query completion and initial argument mapping without making YouTube requests. A React process test covers subsequent search edits and clearing without restarting the command.

Set `WOX_RAYCAST_VERIFY_KILL_PROCESS` to a local Kill Process extension directory to run `TestLocalKillProcessSingleCommand`. It copies the actual bundle to a temporary directory, opens the process list using only the trigger, searches process names, verifies empty matches and clearing reuse the view, and checks leaving releases the runtime. On Windows it also loads an ordinary process's emoji through Wox and, when Chrome is running, extracts and decodes Chrome's executable icon. Icon caches are isolated in a temporary directory; the emoji loader may download its existing Twemoji asset. It never invokes kill actions.

With that same variable on Windows, `TestLocalKillProcessConfirmation` starts uniquely named test-owned processes and requires an exact name/PID match in the real plugin before invoking Force Kill. It verifies confirmation terminates only the owned target, cancellation and leaving preserve it, and stale confirmation callbacks cannot accept a canceled operation. Cleanup terminates the remaining owned targets.
