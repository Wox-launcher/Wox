# WoxView window API validation

Validated on Windows on 2026-11-06 after restricting plugin windows to WoxView.

| Check | Result |
| --- | --- |
| `go test ./plugin ./plugin/host ./ui/pluginwindow` | Passed |
| `go build -tags sqlite_fts5 ./...` | Passed |
| `WOX_WINDOW_NATIVE_TEST=1 WOX_WINDOW_THEME_NATIVE_TEST=1 go test ./ui/pluginwindow -count=1` | Passed |
| Node SDK `pnpm test` | Passed; 5 tests and TypeScript contract checks |
| Node host `pnpm test`, `pnpm exec tsc --noEmit` | Passed; 39 tests |
| Python SDK `uv run python -m unittest discover -s tests` | Passed; 33 tests |
| Python host `uv run python -m unittest discover -s tests` | Passed; 52 tests |

Native coverage includes WoxView button interaction, in-place updates, stale
sequence/document rejection, removed-handler filtering, hidden-window theme
notifications, document replacement, and subscription cleanup. Unit tests cover
rejection of raw text, Markdown and HTML at creation, replacement and update;
plugin ownership, callback lifecycle, and matching SDK/RPC serialization.

The HTML bridge, examples, bridge tests, and macOS HTML-window smoke fixture were
removed with the unsupported content path. Query-preview WebViews are unchanged.
macOS and Linux native platforms and physical mixed-DPI monitor transitions were
not exercised in this session. The complete smoke suite was not run.
