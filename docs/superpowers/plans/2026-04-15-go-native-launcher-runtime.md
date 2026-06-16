# Go Native Launcher Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Flutter launcher window with a single-process Go-native launcher runtime while keeping the Flutter settings window as a separate frontend.

**Architecture:** Keep `wox.core` as the product root, but split startup into an explicit core bootstrap plus a native launcher runtime that lives on the locked UI thread. Keep launcher state, layout, and preview orchestration in Go, but draw fixed launcher chrome through platform-native drawing backends on each OS, host IME and WebView with platform-native child hosts, and preserve existing launcher semantics by moving current WebSocket/HTTP launcher behavior behind in-process `LauncherCoreAPI` and `LauncherEventBus` adapters. Land the work Windows-first with a real `wox.core` query spike before broad cross-platform rollout.

**Tech Stack:** Go, cgo, Win32 + Direct2D/DirectWrite + WebView2, AppKit/CoreText + WKWebView, GTK + Cairo/Pango + WebKitGTK, existing `wox.core` services, Flutter settings frontend

**Operator rule:** Do not create commits while executing this plan unless the operator explicitly asks for one in that session.

---

## Planned Repository Layout

**Shared Go runtime**
- Create: `wox.core/app/bootstrap.go`
- Create: `wox.core/app/core_services.go`
- Create: `wox.core/launcher/runtime.go`
- Create: `wox.core/launcher/contracts.go`
- Create: `wox.core/launcher/events.go`
- Create: `wox.core/launcher/store/state.go`
- Create: `wox.core/launcher/store/store.go`
- Create: `wox.core/launcher/store/reducer.go`
- Create: `wox.core/launcher/layout/layout.go`
- Create: `wox.core/launcher/layout/query_box.go`
- Create: `wox.core/launcher/layout/result_list.go`
- Create: `wox.core/launcher/scene/frame.go`
- Create: `wox.core/launcher/scene/diff.go`
- Create: `wox.core/launcher/theme/paint_theme.go`
- Create: `wox.core/launcher/theme/mapper.go`
- Create: `wox.core/launcher/input/router.go`
- Create: `wox.core/launcher/input/query_editor.go`
- Create: `wox.core/launcher/input/shortcuts.go`
- Create: `wox.core/launcher/result/reconciler.go`
- Create: `wox.core/launcher/result/selection.go`
- Create: `wox.core/launcher/preview/models.go`
- Create: `wox.core/launcher/preview/resolver.go`
- Create: `wox.core/launcher/preview/host.go`
- Create: `wox.core/launcher/preview/plain_text_renderer.go`
- Create: `wox.core/launcher/preview/markdown_renderer.go`
- Create: `wox.core/launcher/preview/image_renderer.go`
- Create: `wox.core/launcher/preview/file_renderer.go`
- Create: `wox.core/launcher/preview/webview_renderer.go`
- Create: `wox.core/launcher/webview/session.go`
- Create: `wox.core/launcher/webview/pool.go`
- Create: `wox.core/launcher/webview/profile.go`
- Create: `wox.core/launcher/debug/automation.go`

**Platform-native host layer**
- Create: `wox.core/launcher/platform/host.go`
- Create: `wox.core/launcher/platform/windows/dispatcher_windows.go`
- Create: `wox.core/launcher/platform/windows/host_windows.go`
- Create: `wox.core/launcher/platform/windows/window_windows.h`
- Create: `wox.core/launcher/platform/windows/window_windows.cc`
- Create: `wox.core/launcher/platform/windows/text_input_host_windows.h`
- Create: `wox.core/launcher/platform/windows/text_input_host_windows.cc`
- Create: `wox.core/launcher/platform/windows/webview_host_windows.h`
- Create: `wox.core/launcher/platform/windows/webview_host_windows.cc`
- Create: `wox.core/launcher/platform/darwin/dispatcher_darwin.go`
- Create: `wox.core/launcher/platform/darwin/host_darwin.go`
- Create: `wox.core/launcher/platform/darwin/app_host_darwin.mm`
- Create: `wox.core/launcher/platform/darwin/text_input_host_darwin.mm`
- Create: `wox.core/launcher/platform/darwin/webview_host_darwin.mm`
- Create: `wox.core/launcher/platform/linux/dispatcher_linux.go`
- Create: `wox.core/launcher/platform/linux/host_linux.go`
- Create: `wox.core/launcher/platform/linux/app_host_linux.c`
- Create: `wox.core/launcher/platform/linux/text_input_host_linux.c`
- Create: `wox.core/launcher/platform/linux/webview_host_linux.c`

**Renderer bridge and assets**
- Create: `wox.core/launcher/render/contracts.go`
- Create: `wox.core/launcher/render/text_metrics.go`
- Create: `wox.core/launcher/platform/windows/render_host_windows.h`
- Create: `wox.core/launcher/platform/windows/render_host_windows.cc`
- Create: `wox.core/launcher/platform/darwin/render_host_darwin.mm`
- Create: `wox.core/launcher/platform/linux/render_host_linux.c`
- Create: `wox.core/resource/ui/native_launcher/markdown/index.html`
- Create: `wox.core/resource/ui/native_launcher/markdown/main.css`
- Create: `wox.core/resource/ui/native_launcher/markdown/main.js`

**Bridges, settings, diagnostics, and smoke coverage**
- Create: `wox.core/ui/launcher_bridge.go`
- Create: `wox.core/ui/settings_transport.go`
- Create: `wox.core/test/native_launcher_test_helper.go`
- Create: `wox.core/test/native_launcher_startup_smoke_test.go`
- Create: `wox.core/test/native_launcher_query_preview_smoke_test.go`
- Create: `wox.core/test/native_launcher_webview_smoke_test.go`
- Create: `docs/superpowers/audits/2026-04-15-launcher-thread-audit.md`
- Create: `wox.ui.flutter/wox/lib/settings_main.dart`

**Existing files that must be refactored instead of replaced wholesale**
- Modify: `wox.core/main.go`
- Modify: `wox.core/Makefile`
- Modify: `wox.core/common/ui.go`
- Modify: `wox.core/common/theme.go`
- Modify: `wox.core/common/image.go`
- Modify: `wox.core/resource/resource.go`
- Modify: `wox.core/ui/manager.go`
- Modify: `wox.core/ui/ui_impl.go`
- Modify: `wox.core/ui/http.go`
- Modify: `wox.core/ui/router.go`
- Modify: `wox.core/util/mainthread/mainthread.go`
- Modify: `wox.core/util/mainthread/mainthread_darwin.go`
- Modify: `wox.ui.flutter/wox/lib/main.dart`

## Incremental Delivery Strategy

Execute this plan as **minimum working vertical slices**, not as a pure infrastructure-first rewrite. Every slice must end in a launcher build that can run and be smoke-tested on the target platform.

### Slice A: Native Window Shell

Target:
- native launcher process starts
- one native window exists
- show, hide, resize, move, transparency or acrylic effect, and rounded corners work
- no query box, result list, or preview yet

Primary tasks:
- Task 1
- Task 2, but only the parts needed for:
  - UI-thread dispatcher
  - native host window
  - window chrome and effects
  - one hard-coded frame

Exit signal:
- operator can launch the native window and verify show/hide/resize/effect behavior manually
- one startup smoke test passes

### Slice B: Query Box

Target:
- launcher window contains a working query box
- text input, caret, focus, and IME composition work
- query changes can be observed in the runtime even if results are still stubbed or empty

Primary tasks:
- Task 4, but only the store, theme, and layout parts needed for the query box
- Task 5, but only the query editor and input routing parts needed for typing

Exit signal:
- typing works
- `Esc`, focus return, and query-box show/hide behavior work
- query-box smoke test passes

### Slice C: Result List And Navigation

Target:
- real queries go through `wox.core`
- result list renders with selection
- arrow keys, enter, quick-select, and list reconciliation work
- preview pane can stay empty or placeholder-only

Primary tasks:
- remaining `LauncherCoreAPI` and `LauncherEventBus` work from Task 3
- remaining store and layout work from Task 4
- remaining result-list and selection work from Task 5

Exit signal:
- one real `wox.core` query path works end to end
- list selection and navigation are stable under incremental result flush
- query-plus-results smoke test passes

### Slice D: Basic Preview

Target:
- selected results can show `text`, `markdown`, `image`, and `file` preview
- text and simple file preview work without embedded browser state
- markdown works through the document webview template

Primary tasks:
- Task 6

Exit signal:
- fast selection changes do not show stale preview
- markdown and plain-text preview are both usable
- preview smoke test passes

### Slice E: Embedded WebView

Target:
- `webview` preview works with native embedded browser
- back, forward, refresh, focus handoff, session reuse, and `Esc` fallback work

Primary tasks:
- Task 7

Exit signal:
- cached webview session survives hide/show
- embedded browser crash does not kill the process
- webview smoke test passes

### Slice F: Settings Separation

Target:
- Flutter no longer provides the launcher window
- Flutter remains only for settings
- native launcher and Flutter settings can coexist safely

Primary tasks:
- Task 8

Exit signal:
- launcher starts without Flutter launcher startup
- settings still open and function

### Slice G: macOS Host

Target:
- slices A through F run on macOS with native backend implementations

Primary tasks:
- Task 9

### Slice H: Linux Host

Target:
- slices A through F run on Linux with GTK and `WebKitGTK`

Primary tasks:
- Task 10

### Slice I: Stress, Recovery, And Ship Readiness

Target:
- cross-platform smoke, recovery, and packaging checks are stable enough to ship

Primary tasks:
- Task 11

Planning rule:
- do not finish every sub-item in Tasks 1 through 11 before producing the first runnable shell
- for execution, always prefer “smallest runnable launcher state” over “maximum architectural completeness”

### Task 1: Carve out the bootstrap seam and launcher runtime contracts

**Files:**
- Create: `wox.core/app/bootstrap.go`
- Create: `wox.core/app/core_services.go`
- Create: `wox.core/launcher/runtime.go`
- Create: `wox.core/launcher/contracts.go`
- Create: `wox.core/launcher/events.go`
- Create: `wox.core/ui/launcher_bridge.go`
- Modify: `wox.core/main.go`
- Modify: `wox.core/common/ui.go`
- Modify: `wox.core/ui/manager.go`

- [ ] **Step 1: Extract the current startup path into a reusable core bootstrap package**

Create `wox.core/app/core_services.go` and `wox.core/app/bootstrap.go` so `main.go` no longer owns the full startup sequence directly.

```go
package app

type CoreServices struct {
	UIManager      *ui.Manager
	SettingManager *setting.Manager
	PluginManager  *plugin.Manager
}

func StartCoreServices(ctx context.Context, serverPort int) (*CoreServices, error) {
	return &CoreServices{
		UIManager:      ui.GetUIManager(),
		SettingManager: setting.GetSettingManager(),
		PluginManager:  plugin.GetPluginManager(),
	}, nil
}
```

Rules for this extraction:
- keep the current initialization order from `wox.core/main.go`
- do not change deeplink, telemetry, tray, updater, or hotkey behavior yet
- return an explicit `CoreServices` struct so the launcher runtime can depend on stable handles instead of package globals

- [ ] **Step 2: Define the launcher runtime contracts before adding any host code**

Create `wox.core/launcher/contracts.go`, `wox.core/launcher/events.go`, and `wox.core/launcher/runtime.go` with the minimum contract surface the native launcher will implement.

```go
package launcher

type CoreAPI interface {
	Query(ctx context.Context, req QueryRequest) error
	QueryMRU(ctx context.Context, req QueryMRURequest) ([]plugin.QueryResultUI, error)
	Action(ctx context.Context, req ActionRequest) error
	FormAction(ctx context.Context, req FormActionRequest) error
	ToolbarMsgAction(ctx context.Context, req ToolbarMsgActionRequest) error
	ResolvePreview(ctx context.Context, req ResolvePreviewRequest) (plugin.WoxPreview, error)
	GetQueryMetadata(ctx context.Context, req QueryMetadataRequest) (QueryMetadata, error)
	GetPluginDetail(ctx context.Context, pluginID string) (PluginDetailPayload, error)
	TerminalSubscribe(ctx context.Context, req TerminalSubscribeRequest) error
	TerminalUnsubscribe(ctx context.Context, req TerminalUnsubscribeRequest) error
	TerminalSearch(ctx context.Context, req TerminalSearchRequest) error
}

type EventBus interface {
	Publish(event Event)
	Subscribe(handler func(Event))
}

type UIThreadDispatcher interface {
	Invoke(fn func()) error
}
```

Rules:
- keep names aligned with the approved spec: `LauncherCoreAPI`, `LauncherEventBus`, `UIThreadDispatcher`
- keep request and event structs in Go, not C headers
- do not add paint or node-level methods to these contracts

- [ ] **Step 3: Introduce a launcher bridge instead of rewriting `common.UI` call sites**

Create `wox.core/ui/launcher_bridge.go` and keep `common.UI` as the stable backend-facing façade.

```go
type launcherBridge struct {
	runtime launcher.Runtime
}

func (b *launcherBridge) ShowApp(ctx context.Context, showContext common.ShowContext) {
	b.runtime.Show(ctx, showContext)
}

func (b *launcherBridge) HideApp(ctx context.Context) {
	b.runtime.Hide(ctx)
}
```

Rules:
- move launcher-specific behavior behind the bridge
- keep `common.UI` call sites in plugins and `ui.Manager` unchanged in this task
- do not route settings-only operations through the native launcher

- [ ] **Step 4: Shrink `main.go` to orchestration only**

Update `wox.core/main.go` so it becomes:

```go
func main() {
	mainthread.Init(run)
}

func run() {
	ctx := util.NewTraceContext()
	serverPort, err := resolveServerPort(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, err.Error())
		return
	}

	coreServices, err := app.StartCoreServices(ctx, serverPort)
	if err != nil {
		util.GetLogger().Error(ctx, err.Error())
		return
	}

	_ = coreServices
}
```

Rules:
- keep `mainthread.Init(run)` for now
- do not start the native launcher yet
- make this compile with a no-op runtime stub so later tasks can land incrementally

- [ ] **Step 5: Build-check the seam extraction before touching native code**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Expected:
- the binary still builds on the current platform
- no behavior changes are introduced yet

### Task 2: Build the Windows integrated spike on the real `wox.core` query path

**Files:**
- Create: `wox.core/launcher/platform/host.go`
- Create: `wox.core/launcher/platform/windows/dispatcher_windows.go`
- Create: `wox.core/launcher/platform/windows/host_windows.go`
- Create: `wox.core/launcher/platform/windows/window_windows.h`
- Create: `wox.core/launcher/platform/windows/window_windows.cc`
- Create: `wox.core/launcher/render/contracts.go`
- Create: `wox.core/launcher/render/text_metrics.go`
- Create: `wox.core/launcher/platform/windows/render_host_windows.h`
- Create: `wox.core/launcher/platform/windows/render_host_windows.cc`
- Create: `wox.core/launcher/debug/automation.go`
- Modify: `wox.core/Makefile`
- Modify: `wox.core/util/mainthread/mainthread.go`
- Modify: `wox.core/main.go`

- [ ] **Step 1: Define the native render contract for the Windows spike**

Create `wox.core/launcher/render/contracts.go`, `wox.core/launcher/render/text_metrics.go`, and `wox.core/launcher/platform/windows/render_host_windows.*`.

```go
type Renderer interface {
	MeasureText(ctx context.Context, req []TextMeasureRequest) ([]TextMeasureResult, error)
	PresentFrame(ctx context.Context, frame scene.Frame) error
}
```

Rules:
- keep the render contract platform-neutral from the Go side
- use Windows-native drawing primitives behind `render_host_windows.*`
- do not introduce a shared third-party rendering runtime in this task

- [ ] **Step 2: Implement a Windows UI-thread dispatcher around `PostMessage(...)`**

Create `wox.core/launcher/platform/host.go` and `wox.core/launcher/platform/windows/dispatcher_windows.go`.

```go
type Host interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Dispatcher() launcher.UIThreadDispatcher
}

type windowsDispatcher struct {
	hwnd   windows.HWND
	queue  chan func()
	msgID  uint32
}
```

Rules:
- the dispatcher must queue callbacks from goroutines and execute them only on the Win32 UI thread
- panic recovery must happen around callback execution
- do not make `launcher.Runtime` call Win32 APIs directly from background goroutines

- [ ] **Step 3: Create a transparent Win32 host window and a Direct2D/DirectWrite-backed test frame**

Create `wox.core/launcher/platform/windows/host_windows.go`, `window_windows.h`, `window_windows.cc`, and `render_host_windows.*`.

```go
type WindowsHost struct {
	dispatcher *windowsDispatcher
	renderer   render.Renderer
}

func (h *WindowsHost) Start(ctx context.Context) error {
	if err := h.createWindow(ctx); err != nil {
		return err
	}
	if err := h.renderer.Initialize(ctx, h.hwnd); err != nil {
		return err
	}
	return h.renderer.Present(ctx, h.bootstrapFrame)
}
```

Rules:
- create one top-level launcher window with transparency and rounded corner support
- draw launcher chrome through `Direct2D` and text through `DirectWrite`
- present one hard-coded frame containing query box, result list area, and preview area
- reserve one child-host rectangle for future WebView attachment

- [ ] **Step 4: Feed the spike from the real backend query path, not mock data**

Create `wox.core/launcher/debug/automation.go` and use it from `main.go` on a development-only path.

```go
type Automation struct {
	Runtime launcher.Runtime
}

func (a *Automation) SubmitQuery(ctx context.Context, text string) error {
	return a.Runtime.CoreAPI().Query(ctx, launcher.QueryRequest{
		QueryID:   uuid.NewString(),
		QueryType: plugin.QueryTypeInput,
		QueryText: text,
	})
}
```

Rules:
- the spike must submit a real launcher query through the new `LauncherCoreAPI`
- results must come from `plugin.GetPluginManager()` and existing backend lifecycle code
- do not hard-code fake result rows in the final spike verification path

- [ ] **Step 5: Verify the Windows spike before continuing**

Run on Windows:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Manual verification:
- launcher window opens on the native host
- one real query from `wox.core` populates the native result list
- selection changes cause visible preview placeholder changes
- no launcher-path WebSocket is required for this spike

### Task 3: Extract `LauncherCoreAPI` and `LauncherEventBus` from the current WebSocket handlers

**Files:**
- Create: `wox.core/launcher/coreapi/service.go`
- Create: `wox.core/launcher/coreapi/query_service.go`
- Create: `wox.core/launcher/coreapi/action_service.go`
- Create: `wox.core/launcher/coreapi/preview_service.go`
- Create: `wox.core/launcher/eventbus/eventbus.go`
- Create: `wox.core/launcher/eventbus/coalescer.go`
- Create: `docs/superpowers/audits/2026-04-15-launcher-thread-audit.md`
- Modify: `wox.core/ui/ui_impl.go`
- Modify: `wox.core/ui/http.go`
- Modify: `wox.core/ui/router.go`

- [ ] **Step 1: Move launcher request handling out of `ui_impl.go` into explicit services**

Move the business logic currently sitting in `handleWebsocketQuery`, `handleWebsocketAction`, `handleWebsocketFormAction`, `handleWebsocketToolbarMsgAction`, `handleWebsocketTerminalSubscribe`, `handleWebsocketTerminalUnsubscribe`, and `handleWebsocketTerminalSearch` into `wox.core/launcher/coreapi/*.go`.

```go
type Service struct {
	plugins *plugin.Manager
	ui      *ui.Manager
}

func (s *Service) Query(ctx context.Context, req launcher.QueryRequest) error {
	query, queryPlugin, err := plugin.GetPluginManager().NewQuery(ctx, common.PlainQuery{
		QueryId:   req.QueryID,
		QueryType: req.QueryType,
		QueryText: req.QueryText,
	})
	if err != nil {
		return err
	}
	plugin.GetPluginManager().HandleQueryLifecycle(ctx, query, queryPlugin)
	return nil
}
```

Rules:
- copy behavior first, refactor later
- keep existing `plugin.GetPluginManager()` semantics intact
- return typed Go values instead of WebSocket payload maps

- [ ] **Step 2: Implement the event bus with explicit backpressure and revision coalescing**

Create `wox.core/launcher/eventbus/eventbus.go` and `coalescer.go`.

```go
type Bus struct {
	subscribers []func(launcher.Event)
	results     map[string]launcher.ResultSnapshotEvent
}

func (b *Bus) Publish(event launcher.Event) {
	if snapshot, ok := event.(launcher.ResultSnapshotEvent); ok {
		b.results[snapshot.QueryRevision] = snapshot
		return
	}
	for _, subscriber := range b.subscribers {
		subscriber(event)
	}
}
```

Rules:
- coalesce result snapshots by query revision
- drop stale incremental updates once a new revision supersedes them
- bound terminal-chunk queues instead of blocking the core

- [ ] **Step 3: Audit every launcher-facing backend path for UI-thread safety**

Create `docs/superpowers/audits/2026-04-15-launcher-thread-audit.md` and list each current launcher-facing backend entrypoint with one explicit conclusion.

Required rows:
- `ShowApp`
- `HideApp`
- `ToggleApp`
- `ChangeQuery`
- `RefreshQuery`
- `UpdateResult`
- `PushResults`
- `ShowToolbarMsg`
- `ClearToolbarMsg`
- `OpenSettingWindow`
- `PickFiles`

Rules:
- call out whether the path currently holds locks
- record whether it waits for a response
- record which paths must become async-only

- [ ] **Step 4: Leave settings transport alive while cutting launcher transport over**

Refactor `wox.core/ui/http.go` and `wox.core/ui/router.go` so settings HTTP/WS routes remain available, but launcher semantics are now served by `LauncherCoreAPI` and `LauncherEventBus`.

```go
var keepSettingsRoutes = []string{
	"/setting/wox",
	"/setting/plugin/update",
	"/on/setting",
}

var removeLauncherTransport = []string{
	"Query",
	"Action",
	"FormAction",
	"ShowApp",
	"HideApp",
}
```

Rules:
- do not break settings window startup in this task
- keep any still-needed diagnostics routes behind dev-only guards if necessary
- stop adding new launcher behavior to the WebSocket path

- [ ] **Step 5: Build-check and verify the thread audit is complete**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Expected:
- the bridge compiles
- the audit document exists and covers all launcher-facing backend entrypoints

### Task 4: Implement the shared launcher store, scene graph, layout, and theme mapping

**Files:**
- Create: `wox.core/launcher/store/state.go`
- Create: `wox.core/launcher/store/store.go`
- Create: `wox.core/launcher/store/reducer.go`
- Create: `wox.core/launcher/layout/layout.go`
- Create: `wox.core/launcher/layout/query_box.go`
- Create: `wox.core/launcher/layout/result_list.go`
- Create: `wox.core/launcher/scene/frame.go`
- Create: `wox.core/launcher/scene/diff.go`
- Create: `wox.core/launcher/theme/paint_theme.go`
- Create: `wox.core/launcher/theme/mapper.go`
- Modify: `wox.core/common/theme.go`

- [ ] **Step 1: Define the launcher store state exactly once**

Create `wox.core/launcher/store/state.go`.

```go
type State struct {
	Visibility     VisibilityState
	Mode           ModeState
	FocusTarget    FocusTarget
	QuerySession   QuerySession
	PreviewSession PreviewSession
	Results        []plugin.QueryResultUI
	Theme          theme.PaintTheme
}
```

Rules:
- include the versioned `QuerySession` and `PreviewSession` fields from the spec
- keep current selection and preview identity in state
- do not hide focus or mode flags inside renderer-private structs

- [ ] **Step 2: Implement a reducer instead of ad-hoc store mutation**

Create `wox.core/launcher/store/reducer.go`.

```go
func Reduce(state State, action Action) State {
	switch action := action.(type) {
	case ShowAppAction:
	case QueryChangedAction:
	case ResultsFlushedAction:
	case SelectionChangedAction:
	case PreviewResolvedAction:
	}
	return next
}
```

Rules:
- every legal transition from the spec must have one explicit action
- illegal transitions must be rejected or normalized in one place
- freeze preview churn when overlays are open

- [ ] **Step 3: Create the scene graph and diff layer**

Create `wox.core/launcher/scene/frame.go` and `diff.go`.

```go
type Frame struct {
	Nodes        []Node
	Interactive  []Region
	ChildHosts   []ChildHostReservation
}
```

Rules:
- stable node IDs are mandatory
- the diff layer must emit dirty-node or dirty-region updates
- text blocks must be measurable without one cgo call per visual node

- [ ] **Step 4: Map existing Wox themes into renderer-facing paint tokens**

Create `wox.core/launcher/theme/paint_theme.go` and `mapper.go`.

```go
type PaintTheme struct {
	AppBackground color.NRGBA
	QueryBox      QueryBoxTheme
	ResultRow     ResultRowTheme
	Preview       PreviewTheme
	Toolbar       ToolbarTheme
}
```

Rules:
- load from current backend theme JSON, not a new schema
- parse once and cache
- cover all launcher-visible fields before any renderer tries to read raw theme JSON

- [ ] **Step 5: Render a real query box and result list from store state**

Update the Windows spike so the frame is no longer hard-coded.

Run on Windows:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Manual verification:
- changing the theme updates the native launcher colors
- result rows resize and diff cleanly under rapid result flushes

### Task 5: Implement query editing, input routing, and result-list reconciliation

**Files:**
- Create: `wox.core/launcher/input/router.go`
- Create: `wox.core/launcher/input/query_editor.go`
- Create: `wox.core/launcher/input/shortcuts.go`
- Create: `wox.core/launcher/result/reconciler.go`
- Create: `wox.core/launcher/result/selection.go`
- Create: `wox.core/launcher/platform/windows/text_input_host_windows.h`
- Create: `wox.core/launcher/platform/windows/text_input_host_windows.cc`
- Modify: `wox.core/launcher/platform/windows/host_windows.go`
- Create: `wox.core/test/native_launcher_test_helper.go`
- Create: `wox.core/test/native_launcher_startup_smoke_test.go`
- Create: `wox.core/test/native_launcher_query_preview_smoke_test.go`

- [ ] **Step 1: Reconcile results by identity instead of replacing the whole list**

Create `wox.core/launcher/result/reconciler.go`.

```go
func ReconcileResults(current []plugin.QueryResultUI, next []plugin.QueryResultUI, selectedID string) ([]plugin.QueryResultUI, string) {
	nextSelectedID := ""
	if selectedID != "" {
		for _, item := range next {
			if item.Id == selectedID {
				nextSelectedID = selectedID
				break
			}
		}
	}
	if nextSelectedID == "" && len(next) > 0 {
		nextSelectedID = next[0].Id
	}
	return next, nextSelectedID
}
```

Rules:
- preserve selection by `resultId`
- fall back to the first row only when the previous selection disappears
- keep quick-select numbering derived from visible order

- [ ] **Step 2: Implement the input-priority chain in one router**

Create `wox.core/launcher/input/router.go`.

```go
func (r *Router) HandleKey(event KeyEvent) bool {
	return r.overlay.Handle(event) ||
		r.webview.Handle(event) ||
		r.preview.Handle(event) ||
		r.resultList.Handle(event) ||
		r.queryEditor.Handle(event) ||
		r.window.Handle(event)
}
```

Rules:
- follow the exact priority chain from the spec
- keep `Esc`, `Tab`, arrows, and `Enter` behavior centralized
- do not let preview-specific code reach back into `QueryEditor` directly

- [ ] **Step 3: Add a native text-input host instead of reimplementing IME in Go**

Create `wox.core/launcher/platform/windows/text_input_host_windows.*` and `wox.core/launcher/input/query_editor.go`.

```go
type QueryEditor struct {
	Text        string
	Selection   selection.Selection
	Composition CompositionRange
	CaretRect   image.Rectangle
}
```

Rules:
- native text input owns composition and candidate windows
- the Go side owns visible query state and caret geometry
- expose caret rect lookup for candidate-window positioning

- [ ] **Step 4: Add native launcher smoke coverage for startup and query flows**

Create `wox.core/test/native_launcher_test_helper.go`, `native_launcher_startup_smoke_test.go`, and `native_launcher_query_preview_smoke_test.go`.

```go
func launchNativeLauncherForTest(t *testing.T) *launcherdebug.Automation {
	ctx := util.NewTraceContext()
	services, err := app.StartCoreServices(ctx, 0)
	require.NoError(t, err)
	runtime := launcher.NewForTest(services)
	require.NoError(t, runtime.Start(ctx))
	return launcherdebug.NewAutomation(runtime)
}
```

Smoke assertions:
- launcher shows and hides cleanly
- one query updates result rows
- rapid selection changes do not show stale preview state

- [ ] **Step 5: Run the first native smoke suite**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run 'TestNativeLauncher(Startup|QueryPreview)' -count=1
make build
```

Expected:
- both smoke tests pass
- the app still builds for the current platform

### Task 6: Implement preview resolution and the v1 text/markdown/image/file renderer set

**Files:**
- Create: `wox.core/launcher/preview/models.go`
- Create: `wox.core/launcher/preview/resolver.go`
- Create: `wox.core/launcher/preview/host.go`
- Create: `wox.core/launcher/preview/plain_text_renderer.go`
- Create: `wox.core/launcher/preview/markdown_renderer.go`
- Create: `wox.core/launcher/preview/image_renderer.go`
- Create: `wox.core/launcher/preview/file_renderer.go`
- Create: `wox.core/resource/ui/native_launcher/markdown/index.html`
- Create: `wox.core/resource/ui/native_launcher/markdown/main.css`
- Create: `wox.core/resource/ui/native_launcher/markdown/main.js`
- Modify: `wox.core/resource/resource.go`
- Modify: `wox.core/common/image.go`
- Create: `wox.core/test/native_launcher_query_preview_smoke_test.go`

- [ ] **Step 1: Normalize `WoxPreview` into typed runtime preview models**

Create `wox.core/launcher/preview/models.go` and `resolver.go`.

```go
type ResolvedPreview interface {
	Identity() string
	Type() PreviewType
}

type MarkdownPreview struct {
	HTML string
}
```

Rules:
- resolve `remote` previews before renderer selection
- keep cancellation tokens on every async resolve path
- derive preview identity from query ID, result ID, preview type, and preview hash

- [ ] **Step 2: Implement plain text, image, and file routing without WebView reuse yet**

Create `plain_text_renderer.go`, `image_renderer.go`, and `file_renderer.go`.

```go
func SelectFileRenderer(path string) RendererKind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return RendererMarkdown
	case ".pdf":
		return RendererPDF
	}
	return RendererText
}
```

Rules:
- implement the exact v1 file-extension matrix from the spec
- degrade unsupported file types to an explicit unsupported-preview state
- normalize relative and file-icon image references before painting

- [ ] **Step 3: Render markdown through a sandboxed document template**

Create `wox.core/resource/ui/native_launcher/markdown/index.html`, `main.css`, `main.js`, and `markdown_renderer.go`.

```go
func RenderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
```

Rules:
- use `goldmark` for Markdown-to-HTML conversion
- keep links external
- use a transient document profile, not a browsing session profile

- [ ] **Step 4: Extend resource extraction to include native launcher assets**

Update `wox.core/resource/resource.go` so the markdown template assets are extracted alongside the existing resources.

Rules:
- keep resource paths deterministic
- do not overwrite user data
- log extraction failures with the current logging pattern

- [ ] **Step 5: Re-run the preview smoke test against real preview content**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run TestNativeLauncherQueryPreview -count=1
make build
```

Expected:
- text, markdown, image, and file previews resolve without stale swaps
- markdown renders through the document template rather than a custom rich-text painter

### Task 7: Implement Windows `WebViewHost`, session pooling, and crash recovery

**Files:**
- Create: `wox.core/launcher/webview/session.go`
- Create: `wox.core/launcher/webview/pool.go`
- Create: `wox.core/launcher/webview/profile.go`
- Create: `wox.core/launcher/preview/webview_renderer.go`
- Create: `wox.core/launcher/platform/windows/webview_host_windows.h`
- Create: `wox.core/launcher/platform/windows/webview_host_windows.cc`
- Modify: `wox.core/launcher/input/router.go`
- Modify: `wox.core/launcher/platform/windows/host_windows.go`
- Create: `wox.core/test/native_launcher_webview_smoke_test.go`

- [ ] **Step 1: Implement explicit session objects and pooling rules**

Create `wox.core/launcher/webview/session.go`, `pool.go`, and `profile.go`.

```go
type Session struct {
	ID            string
	CacheKey      string
	Profile       ProfileKind
	Navigation    NavigationState
	AttachedHost  string
	CacheDisabled bool
}
```

Rules:
- cached and transient sessions must be explicit types
- `cacheDisabled=true` must always bypass reuse
- markdown document sessions must not share the general browsing profile

- [ ] **Step 2: Implement attach, activate, detach, and release on Windows**

Create `wox.core/launcher/platform/windows/webview_host_windows.*` and `preview/webview_renderer.go`.

```go
func (h *Host) Attach(sessionID string, rect image.Rectangle) error
func (h *Host) Activate(sessionID string) error
func (h *Host) Detach(sessionID string) error
func (h *Host) Release(sessionID string) error
```

Rules:
- use `WebView2`
- keep `Esc`, refresh, back, and forward routed through the active session
- preserve cached sessions across hide and show

- [ ] **Step 3: Add runtime fault recovery for WebView and renderer failures**

Extend the Windows host and renderer bridge so failure paths stay local.

Required handling:
- WebView child-process exit attempts session recreation
- renderer device or drawing-context loss attempts local renderer reinitialization
- dispatcher callback panic is recovered and logged

- [ ] **Step 4: Add smoke coverage for warm-cache webview reuse**

Create `wox.core/test/native_launcher_webview_smoke_test.go`.

```go
func TestNativeLauncherWebViewWarmCache(t *testing.T) {
	automation := launchNativeLauncherForTest(t)
	automation.SelectResultWithWebViewPreview(t)
	firstSessionID := automation.ActiveWebViewSessionID(t)
	automation.HideLauncher(t)
	automation.ShowLauncher(t)
	automation.SelectResultWithWebViewPreview(t)
	require.Equal(t, firstSessionID, automation.ActiveWebViewSessionID(t))
}
```

Smoke assertions:
- cached session survives hide/show
- `Esc` returns focus to the launcher
- a simulated WebView crash leaves the process alive

- [ ] **Step 5: Run the Windows native smoke suite**

Run on Windows:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run 'TestNativeLauncher(WebView|Startup|QueryPreview)' -count=1
make build
```

Expected:
- smoke tests pass
- cached and transient webview sessions both work

### Task 8: Convert the Flutter frontend to settings-only mode and remove launcher transport dependencies

**Files:**
- Create: `wox.ui.flutter/wox/lib/settings_main.dart`
- Modify: `wox.ui.flutter/wox/lib/main.dart`
- Modify: `wox.core/ui/manager.go`
- Modify: `wox.core/ui/ui_impl.go`
- Modify: `wox.core/ui/http.go`
- Modify: `wox.core/ui/router.go`

- [ ] **Step 1: Add a settings-only Flutter entrypoint**

Create `wox.ui.flutter/wox/lib/settings_main.dart`.

```dart
Future<void> main(List<String> arguments) async {
  await initialServices(arguments);
  runApp(const WoxSettingsApp());
}
```

Rules:
- do not register `WoxLauncherController` in the settings-only entrypoint
- keep `WoxSettingController` and language/theme loading
- settings startup must not depend on launcher WebSocket methods

- [ ] **Step 2: Stop launching the Flutter app as the main launcher window**

Update `wox.core/ui/manager.go` so `StartUIApp` is no longer part of launcher startup.

```go
func (m *Manager) OpenSettingWindow(ctx context.Context, windowContext common.SettingWindowContext) error {
	appPath := util.GetLocation().GetUIAppPath()
	_, err := shell.Run(appPath, "--settings", fmt.Sprintf("%d", m.serverPort), windowContext.Path, windowContext.Param)
	return err
}
```

Rules:
- the packaged product must not start the Flutter launcher at boot
- `OpenSettingWindow` must launch or focus the settings app
- settings process exit must not terminate the main `wox` process

- [ ] **Step 3: Remove launcher WebSocket request-response dependence from `ui_impl.go`**

Refactor `wox.core/ui/ui_impl.go` so launcher-facing methods delegate to the native launcher bridge, while settings transport lives in `settings_transport.go`.

Rules:
- `ShowApp`, `HideApp`, `ToggleApp`, `ChangeQuery`, `RefreshQuery`, `PushResults`, `UpdateResult`, `PickFiles`, and launcher chat hooks must no longer depend on `invokeWebsocketMethod`
- keep settings-specific transport behavior isolated

- [ ] **Step 4: Verify settings still open and launcher no longer depends on Flutter**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Manual verification:
- launcher starts without any Flutter launcher window
- opening settings starts the Flutter settings app
- closing settings does not close the main process

### Task 9: Add the macOS native host, text input bridge, and `WKWebView`

**Files:**
- Create: `wox.core/launcher/platform/darwin/dispatcher_darwin.go`
- Create: `wox.core/launcher/platform/darwin/host_darwin.go`
- Create: `wox.core/launcher/platform/darwin/app_host_darwin.mm`
- Create: `wox.core/launcher/platform/darwin/text_input_host_darwin.mm`
- Create: `wox.core/launcher/platform/darwin/webview_host_darwin.mm`
- Create: `wox.core/launcher/platform/darwin/render_host_darwin.mm`
- Create: `wox.core/test/native_launcher_platform_darwin_test.go`
- Modify: `wox.core/Makefile`
- Modify: `wox.core/util/mainthread/mainthread_darwin.go`

- [ ] **Step 1: Wire the macOS native drawing backend into the existing cgo build**

Create `wox.core/launcher/platform/darwin/render_host_darwin.mm` and extend `wox.core/Makefile` only as needed for the AppKit/CoreText build flags already required by the native host.

- [ ] **Step 2: Implement the AppKit host on the locked main thread**

Create `dispatcher_darwin.go`, `host_darwin.go`, and `app_host_darwin.mm`.

```go
func (h *DarwinHost) Start(ctx context.Context) error {
	if err := h.createWindow(ctx); err != nil {
		return err
	}
	return h.renderer.Initialize(ctx, h.window)
}
```

Rules:
- AppKit and `WKWebView` must remain on the main thread
- reuse the same store, scene, layout, and preview code from Windows

- [ ] **Step 3: Add native text input and `WKWebView` integration**

Create `text_input_host_darwin.mm` and `webview_host_darwin.mm`.

Rules:
- implement `NSTextInputClient`-compatible query input bridging
- use `WKUserContentController` for bridge messages
- use `evaluateJavaScript` only for imperative commands like refresh and navigation

- [ ] **Step 4: Add a macOS platform smoke test**

Create `wox.core/test/native_launcher_platform_darwin_test.go` with build tags as needed.

Assertions:
- launcher shows
- one query returns results
- markdown and webview preview both attach
- `Esc` returns focus to the query box

- [ ] **Step 5: Build and run the macOS smoke path**

Run on macOS:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run TestNativeLauncherDarwin -count=1
make build
```

Expected:
- the macOS host builds and passes the platform smoke

### Task 10: Add the Linux GTK host, IME bridge, and `WebKitGTK`

**Files:**
- Create: `wox.core/launcher/platform/linux/dispatcher_linux.go`
- Create: `wox.core/launcher/platform/linux/host_linux.go`
- Create: `wox.core/launcher/platform/linux/app_host_linux.c`
- Create: `wox.core/launcher/platform/linux/text_input_host_linux.c`
- Create: `wox.core/launcher/platform/linux/webview_host_linux.c`
- Create: `wox.core/launcher/platform/linux/render_host_linux.c`
- Create: `wox.core/test/native_launcher_platform_linux_test.go`
- Modify: `wox.core/Makefile`

- [ ] **Step 1: Wire the Linux native drawing backend into the GTK build**

Create `wox.core/launcher/platform/linux/render_host_linux.c` and document the required `GTK`, `Pango`, `Cairo`, and `WebKitGTK` development packages in the build notes or Makefile comments.

- [ ] **Step 2: Implement the GTK host and `g_idle_add()` dispatcher**

Create `dispatcher_linux.go`, `host_linux.go`, and `app_host_linux.c`.

```go
func (h *LinuxHost) Start(ctx context.Context) error {
	if err := h.createWindow(ctx); err != nil {
		return err
	}
	return h.renderer.Initialize(ctx, h.window)
}
```

Rules:
- use `g_idle_add()` for cross-thread UI dispatch
- keep a CPU fallback for unsupported GPU environments

- [ ] **Step 3: Add GTK IME and `WebKitGTK` bridges**

Create `text_input_host_linux.c` and `webview_host_linux.c`.

Rules:
- use `gtk_im_context` for query composition
- keep compositor differences visible in logs
- degrade a failing webview instance to “open in browser” without freezing the launcher

- [ ] **Step 4: Add Linux smoke coverage for X11 and Wayland targets**

Create `wox.core/test/native_launcher_platform_linux_test.go`.

Assertions:
- launcher starts
- query and preview work
- a failed embedded webview does not kill the process

- [ ] **Step 5: Build and run the Linux smoke path**

Run on Linux:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run TestNativeLauncherLinux -count=1
make build
```

Expected:
- Linux host builds
- smoke tests pass or the test explicitly validates degraded webview fallback

### Task 11: Finalize native smoke coverage, diagnostics, and ship-readiness checks

**Files:**
- Modify: `wox.core/test/native_launcher_test_helper.go`
- Modify: `wox.core/test/native_launcher_startup_smoke_test.go`
- Modify: `wox.core/test/native_launcher_query_preview_smoke_test.go`
- Modify: `wox.core/test/native_launcher_webview_smoke_test.go`
- Modify: `wox.core/Makefile`
- Modify: `docs/superpowers/specs/2026-04-15-launcher-runtime-design.md`

- [ ] **Step 1: Add smoke helpers for repeated query, hide/show, and crash-recovery loops**

Extend `wox.core/test/native_launcher_test_helper.go` with helpers that:
- submit repeated queries
- switch selection across mixed preview types
- hide and re-show the launcher
- simulate renderer or webview fault conditions

- [ ] **Step 2: Cover the spec’s stress scenarios in the smoke suite**

Update the smoke tests so they validate:
- 100 selection changes across mixed preview types
- 50 hide/show cycles with a cached webview preview
- 20 rapid query revisions while results flush incrementally

- [ ] **Step 3: Make `make build` fail when native smoke prerequisites are missing**

Update `wox.core/Makefile` so missing native dependencies or missing packaged assets fail fast.

```make
build:
	@test -n "$(PLATFORM)"
	# existing platform build commands
```

- [ ] **Step 4: Reconcile the implementation against the approved spec**

Update `docs/superpowers/specs/2026-04-15-launcher-runtime-design.md` only if the implementation forced a contract change.

Rules:
- do not change the spec for convenience
- only record real implementation-driven deltas

- [ ] **Step 5: Run the final verification matrix**

Run on each target platform:

```bash
cd /mnt/c/dev/Wox/wox.core
go test ./test -run TestNativeLauncher -count=1
make build
```

Expected:
- smoke suite passes
- launcher builds without Flutter launcher startup
- settings still open through the Flutter settings frontend

## Plan Self-Review

### Spec coverage
- Single-process startup and UI-thread discipline: Tasks 1, 2, 3
- Windows real-query spike: Task 2
- `LauncherCoreAPI`, `LauncherEventBus`, and backpressure: Task 3
- Shared scene, theme, query, result list, and IME rules: Tasks 4 and 5
- v1 preview matrix and markdown document webview: Task 6
- WebView sessions and recovery: Task 7
- Flutter settings-only retention: Task 8
- macOS and Linux hosts: Tasks 9 and 10
- smoke, diagnostics, and ship-readiness: Task 11

### Placeholder scan
- No `TODO`, `TBD`, or “similar to” placeholders remain.
- Every task names exact file paths and verification commands.

### Type consistency
- `LauncherCoreAPI`, `LauncherEventBus`, and `UIThreadDispatcher` are used consistently across tasks.
- The plan keeps `common.UI` as the stable backend-facing façade while moving launcher behavior behind a new bridge.

Plan complete and saved to `docs/superpowers/plans/2026-04-15-go-native-launcher-runtime.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
