# Window API validation

Validated on Windows on 2026-11-02.

| Check | Result |
| --- | --- |
| `make test` | Passed |
| `make test-go-ui-unit` | Passed |
| `make smoke` | Passed: 138 cases, 3 conditional skips |
| `go test ./plugin ./plugin/host ./ui/pluginwindow ./ui/previewcontent ./ui/runtime/internal/webview ./ui/runtime/internal/window ./util/overlay` | Passed |
| Native `TestNativeWindowMessages` with `WOX_WINDOW_NATIVE_TEST=1` and repository WebView2 loader | Passed |
| Affected system-plugin API test doubles: system, browser_bookmark, chat, emoji, file_search, app, Raycast extension | Passed |
| Node host `npx tsc --noEmit`, `npm test`, `npm run build` | Passed; 29 tests |
| Python host `python -m unittest discover -s tests` | Passed; 42 tests |
| `uv build` for Python host and SDK | Passed; source distributions and wheels |
| `git diff --check` | Passed |

Native coverage includes two separate WebView2 windows, messages queued before
ready, the 64-message queue limit, JSON fidelity (Unicode, false, `__proto__`),
hide/show retaining DOM state, HTML replacement resetting state, stale document
rejection, and switching to the shared text/Markdown renderers. Other tests cover
plugin ownership, unload admission, callback cleanup, early ready events, real
WebSocket replies/reverse callbacks, SDK serialization, iframe bridge exclusion,
UTF-8 limits and listener exceptions.

The first full smoke attempt timed out while copying text from external Notepad.
The isolated case passed on retry, followed by the successful complete smoke run.

## Wider-suite baseline failures

An additional `go test ./plugin/...` run found the following failures outside this
feature. All three were reproduced using a Go build overlay restoring the tracked
Go/native files to `HEAD` and excluding new Go files, without modifying the working
tree:

- `TestClipboardImageCacheMissLoadsOriginalOnDemand`: partial thumbnail-cache assertion.
- `TestTraversalContextFallbackHandlesTrailingSpaceDirectory`: Windows cannot open
  the test's trailing-space directory path.
- `TestDevCommandsIncludeOpenOnboarding`: the command's hide-after-action assertion.

`TestFlowScriptQueryRoundTrip` also timed out in the broad concurrent run; its
isolated rerun passed. The new API initially required additions to several test
doubles; those were fixed and their affected packages passed on rerun.

macOS/Linux adapters have not been built or exercised on their native platforms
in this Windows session. Shared overlay geometry tests pass; physical mixed-DPI
multi-monitor transitions were not exercised here. Timer and Dictation examples
demonstrate HTML presentation/messages and do not migrate their business logic.
