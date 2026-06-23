# Stage 0 — Feasibility Verification Results

## Architecture: Go struct DSL + Draw Command List + Native Renderer

```
Go Widget Tree → Layout Engine → Draw Command List → C/C++ Native Renderer
                                                      ├─ Windows: Direct2D + DirectWrite
                                                      ├─ macOS: CoreGraphics + CoreText (TODO)
                                                      └─ Linux: X11 + Xft (TODO)
```

## Verification Results

### Step 0.1 — Cleanup ✅
- Deleted gogpu prototype and all gogpu/go-webgpu dependencies from go.mod
- `go build ./...` passes clean

### Step 0.2 — DSL + Layout Engine ✅
- `util/ui/commands.go` — DrawCommand types (Clear/DrawRect/DrawRoundedRect/DrawText/DrawImage/DrawLine/PushClip/PopClip)
- `util/ui/widget.go` — Widget interface + VBox/HBox/Text/TextBox/ListBox/Image/Separator/Spacer
- `util/ui/theme.go` — Color + Theme structs
- `util/ui/event.go` — Event types (KeyPress/TextInput/IMECompose/Click/Scroll/FocusLost/Resize)
- `util/ui/renderer.go` — Renderer + TextMeasurer + WindowLifecycle interfaces
- `util/ui/layout.go` — LayoutEngine: measures text, computes positions, generates CommandList
- Unit tests pass: 12 draw commands generated from launcher widget tree

### Step 0.3 — Windows Direct2D Backend ✅
- `util/ui/ui_windows.go` — Go CGO bindings
- `util/ui/ui_windows.cpp` — Direct2D + DirectWrite + Win32 implementation
  - Frameless window with DWM rounded corners
  - Direct2D HWND render target
  - DirectWrite text format + layout
  - WIC image factory (for future PNG decoding)
  - PerMonitor DPI awareness (V2)
  - Win32 message loop (PeekMessage + DispatchMessage)
  - Command executor: Clear/DrawRect/DrawRoundedRect/DrawText/DrawImage/DrawLine/PushClip/PopClip
  - Event dispatch: KeyPress (VK→Key enum), TextInput (WM_CHAR), FocusLost (WM_KILLFOCUS)
  - HitTestCaption for drag from top 8px
- **Memory: 64.9MB PrivateWS** (empty window + Direct2D/DirectWrite init)

### Step 0.4 — DirectWrite Text Rendering ✅
- DrawText command uses IDWriteTextFormat + ID2D1RenderTarget::DrawText
- MeasureText uses IDWriteTextLayout with NO_WRAP and large maxWidth for consistent measurement
- CJK text (Microsoft YaHei) renders correctly
- Font: "Microsoft YaHei" as default (covers Latin + CJK)

### Step 0.5 — Text Input + IME ✅
- WM_CHAR dispatches final IME characters (including CJK surrogate pairs)
- Chinese IME (Microsoft Pinyin) confirmed working
- Cursor position calculated via MeasureText
- Cursor visible as vertical bar at end of text
- **Gap**: No WM_IME_COMPOSITION handling — composition string not shown in-window,
  candidate box doesn't follow cursor. Acceptable for MVP, needs IME enhancement later.

### Step 0.6 — Result List ✅
- 100-item list with title (CJK) + subtitle
- Scroll via keyboard (Up/Down auto-scrolls selected into view)
- Selected item highlighted with rounded rect
- Scrollbar rendered on right side
- PushClip/PopClip for viewport clipping
- Clear command at start of each frame prevents highlight residue

### Step 0.7 — Final Memory ✅

| Measurement | PrivateWS | WorkingSet |
|---|---|---|
| 3s after start | 65.4MB | 71.3MB |
| 8s after start | 65.8MB | 71.6MB |

**Target: 60-70MB → ACHIEVED (65MB stable)**

## Comparison

| Approach | PrivateWS | Notes |
|---|---|---|
| gogpu/WebGPU (DX12) | 345MB | WebGPU device overhead |
| gogpu/WebGPU (GLES) | 206MB | Still too high |
| Flutter wox-ui.exe | 80-120MB | Multi-process (core separate) |
| **Direct2D + DirectWrite (this)** | **65MB** | Single-process, shares Go runtime with core |

## Key Technical Decisions

| Decision | Rationale |
|---|---|
| Draw command list (Go→C) | Go controls layout, C layer is thin executor |
| Direct2D + DirectWrite | CJK font fallback, subpixel AA, native IME |
| C++ (.cpp) not C | COM vtable syntax, D2D1:: helpers |
| CString/CBytes for CGO | Complies with "no Go pointers to C" rule |
| WS_POPUP (no WS_EX_LAYERED) | Direct2D HWND render target incompatible with layered windows |
| Per-frame Clear | Prevents highlight residue from previous frame |

## Files

```
wox.core/util/ui/
├── commands.go         # DrawCommand types + CommandList helpers
├── widget.go           # Widget interface + concrete widget structs
├── theme.go            # Color + Theme
├── event.go            # Event types + Key constants
├── renderer.go         # Renderer/TextMeasurer/WindowLifecycle interfaces
├── layout.go           # LayoutEngine (widget tree → CommandList)
├── layout_test.go      # Unit tests
├── stub_measurer.go    # Fallback TextMeasurer for testing
├── ui_windows.go       # Windows CGO Go bindings
├── ui_windows.cpp      # Windows Direct2D/DirectWrite/Win32 C++ implementation
├── ui_other.go         # Non-Windows stubs
└── demo/
    └── main.go         # Launcher MVP demo
```

## Next Steps (Phase 1)

1. Integrate `util/ui` into wox.core launcher as `gpuUIImpl` implementing `common.UI` interface
2. Wire up query/result data flow (replace WebSocket with direct Go calls)
3. Add SVG icon rendering (oksvg → PNG → Direct2D bitmap)
4. Add global hotkey + focus-lost hide + window positioning
5. Theme system (Wox JSON → Theme struct)
6. Delete Flutter launcher code