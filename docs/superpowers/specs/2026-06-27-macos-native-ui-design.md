# macOS Native UI 实现设计

## 目标

为 Wox 的 native launcher 实现 macOS 后端，与现有 Windows (Direct2D) 后端对称。复用全部平台无关的 Go 层（widget tree、layout engine、theme convert、icon rasterization、gpuUIImpl），仅替换最底层的 native renderer。

## 背景

当前状态：
- Windows native UI 已完成：`ui_windows.go` (CGO bindings) + `ui_windows.cpp` (Direct2D/DirectWrite/Win32)，`gpu_ui_impl.go` 实现了 `common.UI` 接口的 native 路径。
- macOS 当前回退到 WebSocket + Flutter UI（`ui_other.go` 提供 stub，`NewWindowsRenderer` 返回 error，`gpuUIImpl` 创建失败，`manager.go` 回退到 `uiImpl`）。
- `RESULT.md` 明确标记 macOS: CoreGraphics + CoreText (TODO)。

架构复用关系：
```
Go Widget Tree → LayoutEngine → DrawCommand List → Native Renderer
                                                      ├─ Windows: Direct2D + DirectWrite (已完成)
                                                      └─ macOS:  CoreGraphics + CoreText (本次)
```

以下文件平台无关，无需修改：
- `commands.go` (DrawCommand / CommandList)
- `widget.go` (Widget interface + VBox/HBox/Text/TextBox/ListBox/Image/Separator/Spacer/PreviewPanel)
- `theme.go` (Color + Theme)
- `event.go` (EventType + Key + Modifiers + Event + EventCallback)
- `layout.go` (LayoutEngine，含 preview/markdown 渲染)
- `theme_convert.go` (Wox JSON → ui.Theme)
- `icon.go` (WoxImage → PNG 缓存)
- `markdown.go` (ParseMarkdown)
- `stub_measurer.go` (测试用 fallback)

## 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 窗口背景 | NSVisualEffectView (vibrancy) | 与 Windows Mica 对齐，获得原生材质感 |
| 窗口类型 | NSPanel (borderless) | 适配 launcher 行为，不显示在 Dock/Expose，可成 key window |
| IME | 完整 NSTextInputClient 协议 | macOS 中文/日文输入法常用，需显示 composition 文本和候选词跟随 |
| 渲染 API | CoreGraphics (CGBitmapContext + CGContext) | 不引入 Metal/OpenGL 依赖，与 Direct2D 的命令式绘制对等 |
| 文本 | CoreText (CTFont + CTLine) | 与 DirectWrite 对等，支持 CJK fallback |
| 函数命名 | 重命名 `NewWindowsRenderer` → `NewNativeRenderer` | 平台无关命名，避免误导 |
| Composition 显示 | `gpu_ui_impl.go` 新增 `EventIMECompose` 处理 | 平台无关逻辑，Windows 未来可复用 |

## 架构

### 组件分层

```
┌─────────────────────────────────────────────────────────┐
│ gpu_ui_impl.go (平台无关)                               │
│   - gpuUIImpl struct (renderer: NativeRenderer 接口)    │
│   - handleEvent (含 EventIMECompose)                    │
│   - buildAndRender / triggerQuery / ...                 │
├─────────────────────────────────────────────────────────┤
│ renderer.go (平台无关接口)                              │
│   - NativeRenderer 接口                                 │
│   - TextMeasurer 接口                                   │
├──────────────────┬──────────────────┬───────────────────┤
│ ui_windows.go    │ ui_darwin.go     │ ui_other.go       │
│ (WindowsRenderer)│ (MacRenderer)    │ (stub)            │
│ + ui_windows.cpp │ + ui_darwin.m    │                   │
└──────────────────┴──────────────────┴───────────────────┘
```

### NativeRenderer 接口（renderer.go 改造）

合并现有 `Renderer` + `WindowLifecycle` + gpuUIImpl 需要的额外方法：

```go
type NativeRenderer interface {
    Render(commands *CommandList) error
    TextMeasurer() TextMeasurer
    Show() error
    Hide() error
    SetPosition(x, y int) error
    SetSize(w, h int) error
    Close() error
    IsVisible() bool
    GetSize() (int, int)
    SetDarkMode(dark bool)
    ReleaseMemory()
    RequestRepaint()
    RunMessageLoop(onRender func() *CommandList)
}
```

`TextMeasurer` 接口保持不变。

### Windows 侧适配

`ui_windows.go`:
- `NewWindowsRenderer` → 重命名为 `NewNativeRenderer`
- `*WindowsRenderer` 添加方法满足 `NativeRenderer` 接口（已有方法补齐 `GetSize`、`SetDarkMode`、`ReleaseMemory`、`RequestRepaint`、`RunMessageLoop`、`Close`、`IsVisible` —— 大部分已存在）
- 编译期接口断言：`var _ NativeRenderer = (*WindowsRenderer)(nil)`

### macOS 侧实现（ui_darwin.go + ui_darwin.m）

#### ui_darwin.go (CGO Go bindings)

- build tag: `darwin && cgo`
- `MacRenderer` struct 持有 native window ID（与 Windows 对称的整数句柄）
- `MacTextMeasurer` struct
- `NewNativeRenderer(width, height int, theme Theme) (*MacRenderer, error)`
- `SetEventHandler(cb EventCallback)` —— 与 Windows 相同的全局回调注册
- `//export uiEventCallback` —— C→Go 事件回调，构造 `Event` 并调用 `eventHandler`
- `//export uiGetRenderCommands` —— C→Go 渲染回调，调用存储的 `onRender` 函数，返回扁平化的 `CDrawCommand` 数组给 C 侧
- 包级变量 `renderCallback func() *CommandList` —— 由 `RunMessageLoop(onRender)` 存储，供 `uiGetRenderCommands` 调用
- 方法实现：`Show`/`Hide`/`SetPosition`/`SetSize`/`Close`/`IsVisible`/`GetSize`/`SetDarkMode`/`ReleaseMemory`/`RequestRepaint`/`RunMessageLoop`/`TextMeasurer`
- `Render` 方法为 no-op（macOS 渲染由 `drawRect:` 驱动，不经 Go 主动调用），保留以满足 `NativeRenderer` 接口
- CGO LDFLAGS: `-framework Cocoa -framework QuartzCore -framework CoreText -framework ApplicationServices`

#### ui_darwin.m (Objective-C 实现)

**窗口创建：**
- `NSPanel` + `styleMask = NSWindowStyleMaskBorderless`
- `setFloatingPanel:YES` —— 始终浮于普通窗口之上
- `setBecomesKeyOnlyIfNeeded:NO` —— 激活时成为 key window
- `setHidesOnDeactivate:NO` —— 自行管理 hide（由 `gpuUIImpl.HideApp` 控制）
- `setHasShadow:YES` / `setOpaque:NO` / `setBackgroundColor:[NSColor clearColor]`
- `setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorMoveToActiveSpace | NSWindowCollectionBehaviorFullScreenAuxiliary`
- `setLevel:NSPopUpMenuWindowLevel`（与 Windows `WS_EX_TOPMOST` 对齐）
- `setShowsInActivator:NO` / `setExcludedFromWindowsMenu:YES`（不出现在 App Switcher / Window 菜单）

**视图层级：**
```
NSPanel
  └── contentView = NSVisualEffectView (vibrancy 背景)
        material = NSVisualEffectMaterialMenu (或 HUD, 按 dark 切换)
        blendingMode = NSVisualEffectBlendingModeBehindWindow
        state = NSVisualEffectStateActive
        └── WoxRenderView (NSView 子视图，fill visualEffectView)
              └── drawRect: 执行 DrawCommand list
```

主题色叠加不使用额外 CALayer，而是通过 `CmdClear` 在 `drawRect:` 第一步用半透明 WindowBg 填充全窗（alpha < 1 时 vibrancy 透过），与 Windows 对称。

**WoxRenderView（核心自定义视图）：**
- 继承 `NSView`，实现 `NSTextInputClient` 协议
- `drawRect:` 中获取 `NSGraphicsContext` 的 `CGContextRef`，翻转坐标系（Y 轴向下），遍历 `DrawCommand` 数组执行绘制
- `acceptsFirstResponder` → YES
- `keyDown:` → 映射 keyCode 到 Key enum，调用 `uiEventCallback(EventKeyPress, ...)`
- `insertText:replacementRange:` → 调用 `uiEventCallback(EventTextInput, text, ...)`
- `setMarkedText:replacementRange:selection:` → 调用 `uiEventCallback(EventIMECompose, composeText, cursor, ...)`
- `unmarkText` → 调用 `uiEventCallback(EventIMECompose, "", 0, ...)` 清除
- `hasMarkedText` → 返回当前是否有 composition
- `markedRange` / `selectedRange` → 返回 composition 范围
- `validAttributesForMarkedText` → 返回空数组
- `scrollWheel:` → 计算 deltaY，调用 `uiEventCallback(EventScroll, ..., deltaY, ...)`
- `resignFirstResponder` → 调用 `uiEventCallback(EventFocusLost, ...)`
- `mouseDown:` → 调用 `uiEventCallback(EventClick, x, y, ...)`

**坐标系处理：**
macOS NSView 坐标原点在左下角，Y 轴向上。Wox 的 DrawCommand 用左上角原点 Y 轴向下。在 `drawRect:` 中：
```objc
CGContextRef ctx = [[NSGraphicsContext currentContext] CGContext];
CGContextSaveGState(ctx);
CGContextTranslateCTM(ctx, 0, viewHeight);
CGContextScaleCTM(ctx, 1.0, -1.0);
// 现在坐标原点在左上角，Y 轴向下，可直接执行 draw commands
ExecuteCommands(ctx, cmds, count);
CGContextRestoreGState(ctx);
```

**CoreGraphics 命令执行（对应 Windows 的 ExecuteCommands）：**
- `CmdClear` → `CGContextClearRect` 清空（透明时让 vibrancy 透出）+ `CGContextSetRGBFillColor` + `CGContextFillRect` 全窗填充主题背景色（alpha < 1 时半透明）
- `CmdDrawRect` → `CGContextSetRGBFillColor` + `CGContextFillRect`
- `CmdDrawRoundedRect` → `CGPathCreateMutableByAddingRoundedRect`（或 `CGPathCreateWithRoundedRect`）+ `CGContextAddPath` + `CGContextFillPath`
- `CmdDrawText` → `CTFontCreateWithName` + `CFAttributedStringCreate` + `CTLineCreateWithAttributedString` + `CTLineDraw`（注意 CoreText 坐标原点在左下，配合上面的 CTM 翻转后正常）
- `CmdDrawImage` → `CGImageCreateWithPNGDataProvider` + `CGContextDrawImage`（已翻转 CTM，图片正立）
- `CmdDrawLine` → `CGContextSetLineWidth` + `CGContextMoveToPoint` + `CGContextAddLineToPoint` + `CGContextStrokePath`
- `CmdPushClip` → `CGContextSaveGState` + `CGContextClipToRect`
- `CmdPopClip` → `CGContextRestoreGState`

**图片缓存：**
- 与 Windows 对称，用链表/字典按 `imageKey` 缓存 `CGImageRef`
- `ReleaseMemory` → 清空缓存（对应 Windows `ClearBitmapCache`）

**CoreText 文本测量：**
- `CTFontCreateWithName(family, size, NULL)`
- `CFAttributedStringCreate` + `CTLineCreateWithAttributedString`
- `CTLineGetTypographicBounds` 取 width；height ≈ fontSize × 1.2
- 默认 family = `"PingFang SC"`（覆盖中文 + 拉丁，与 Windows "Microsoft YaHei" 对等）
- `fontFamily` 为空时用默认

**DPI / Retina：**
- macOS 自动处理 backing scale，`[view backingScaleFactor]` 返回 2.0（Retina）或 1.0
- 逻辑坐标用 DIP（points），`drawRect:` 的 `NSGraphicsContext` 已配置好 backing store
- 与 Windows 的 `scale = dpi / 96` 对等，但 macOS 由系统处理无需手动缩放

**SetDarkMode(dark bool)：**
- 切换 `NSVisualEffectView.appearance`：`NSAppearanceNameVibrantDark` / `NSAppearanceNameVibrantLight`
- 不改变主题色叠加层（由主题 JSON 的 alpha 决定）

**消息循环（关键差异）：**
- Windows: `gpuUIImpl.Run` 调用 `renderer.RunMessageLoop(onRender)`，后者运行 Win32 消息循环并在每轮迭代调用 `onRender` 获取 commands 再绘制，阻塞主线程
- macOS: `mainthread_darwin.m` 的 `os_main()` 已调用 `[NSApp run]` 驻留事件循环
- **macOS 的 `RunMessageLoop(onRender)` 实现为：存储 `onRender` 回调到包级变量，立即返回（no-op，不阻塞）**
- 后续 `RequestRepaint` → `[view setNeedsDisplay:YES]` → 系统调度 `drawRect:` → 通过 `//export` Go 函数取回 commands 绘制
- `gpu_ui_impl.go` 的 `Run` 无需 build tag 分支，因为 `RunMessageLoop` 行为封装在 renderer 内

### gpu_ui_impl.go 改动

1. **`renderer` 字段类型**：`*ui.WindowsRenderer` → `ui.NativeRenderer`
2. **`NewWindowsRenderer` 调用** → `NewNativeRenderer`
3. **新增 `composeValue` 字段**：存储当前 IME composition 文本
4. **`handleEvent` 新增 `EventIMECompose` 分支**：
   ```go
   case ui.EventIMECompose:
       g.mu.Lock()
       g.composeValue = ev.ComposeText
       g.dirty = true
       g.mu.Unlock()
       // 不触发新 query，只更新显示
       g.requestRepaint()
   ```
5. **`buildAndRender` 中 TextBox 渲染调整**：当 `composeValue` 非空时，显示 `queryValue + composeValue`（composition 文本以不同颜色或下划线区分——需要 layout.go 支持，或简单拼接显示）。**简化方案：composition 期间 TextBox.Value 显示 `queryValue + composeValue`，未提交文本用下划线样式。由于 layout.go 的 `layoutTextBox` 不支持下划线，第一版直接拼接显示，后续增强。**
6. **`EventTextInput` 处理调整**：收到提交文本时，先清除 `composeValue`，再追加到 `queryValue`：
   ```go
   case ui.EventTextInput:
       g.mu.Lock()
       g.composeValue = "" // 清除 composition
       g.queryValue += ev.Text
       g.dirty = true
       g.mu.Unlock()
       g.triggerQuery(ctx)
   ```
   （Windows 版之前没有 `composeValue`，`EventTextInput` 直接追加；macOS 版需要先清 composition 再追加。此改动对 Windows 语义不变——Windows 不发送 `EventIMECompose`，`composeValue` 始终为空。）

### ui_other.go 改动

- build tag 从 `!windows` 改为 `!windows && !darwin`
- `NewWindowsRenderer` → `NewNativeRenderer`（stub 返回 error）
- 其它 stub 保持不变

## 数据流

### 输入事件流（macOS）
```
NSPanel key event → WoxRenderView keyDown:/insertText:/setMarkedText:
  → uiEventCallback (C→Go)
  → EventCallback (Go)
  → gpuUIImpl.handleEvent
    ├── EventKeyPress → 上下选择 / Enter 执行 / Escape 隐藏
    ├── EventTextInput → 清 composeValue，追加 queryValue，triggerQuery
    ├── EventIMECompose → 更新 composeValue，requestRepaint
    ├── EventScroll → 更新 scrollOffset / previewScrollOffset
    └── EventFocusLost → HideApp
```

### 渲染流（macOS）

`drawRect:` 由 macOS 系统在主线程调用。由于 Go 侧的 `buildAndRender` 持有布局状态，`drawRect:` 需要通过 CGO 回调 Go 获取 commands。

```
gpuUIImpl.Run(ctx) 调用 renderer.RunMessageLoop(onRender)
  → macOS RunMessageLoop 把 onRender 回调存为全局变量（不阻塞，立即返回）
  → 后续：任意时刻 RequestRepaint → [view setNeedsDisplay:YES]
  → 系统在主线程调度 drawRect:
  → drawRect: 调用 uiGetRenderCommands() (C→Go //export)
  → 该 Go 函数调用存储的 onRender 回调 (= gpuUIImpl.buildAndRender)
  → 返回 CommandList → 扁平化为 CDrawCommand 数组
  → ExecuteCommands (CoreGraphics 绘制到 CGContext)
```

关键点：
- `RunMessageLoop(onRender)` 在 macOS 上不阻塞，只是把 `onRender` 回调存储到 `ui_darwin.go` 的包级变量
- `drawRect:` 通过 `//export` 的 Go 函数 `uiGetRenderCommands()` 取回 commands
- 不走 `mainthread.Call`（会死锁：mainthread.Call 等主线程，但 drawRect 已在主线程）；直接调用 Go 的 `onRender`（在 Cocoa 主线程上执行 Go 代码，`buildAndRender` 用 `g.mu` 保证线程安全）
- Go 回调返回的 `*CommandList` 需要扁平化为 C 侧的 `CDrawCommand` 数组（与 Windows `ui_windows.go` 的 `Render` 方法的转换逻辑相同）

## 与 main.go 的集成

`main.go` 第 319-332 行已有分支：
```go
if gpuUI := ui.GetUIManager().GpuUI(); gpuUI != nil {
    util.Go(ctx, "start websocket server", func() {
        ui.GetUIManager().StartWebsocketAndWait(ctx)
    })
    mainthread.Call(func() {
        gpuUI.Run(ctx)
    })
    return
}
```

- **Windows**: `mainthread.Call` 将 `gpuUI.Run` 调度到主线程，`Run` 内部调用 `RunMessageLoop` 阻塞
- **macOS**: `mainthread.Call` 将 `gpuUI.Run` 调度到 Cocoa 主线程，`Run` 创建窗口后 `RunMessageLoop` 立即返回，`mainthread.Call` 返回后 `os_main` 的 `[NSApp run]` 继续驱动事件循环

`mainthread.Call` 是阻塞调用（等待 `f()` 返回），所以 macOS 上 `Run` 必须立即返回而不是阻塞——这与 `RunMessageLoop` 的 no-op 实现一致。

## 改动文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `wox.core/util/ui/renderer.go` | 修改 | 新增 `NativeRenderer` 接口，保留原有 `Renderer`/`TextMeasurer` 接口 |
| `wox.core/util/ui/ui_windows.go` | 修改 | `NewWindowsRenderer` → `NewNativeRenderer`；添加接口断言 |
| `wox.core/util/ui/ui_darwin.go` | 新增 | macOS CGO Go bindings + `MacRenderer` |
| `wox.core/util/ui/ui_darwin.m` | 新增 | NSPanel + CoreGraphics + CoreText + NSTextInputClient |
| `wox.core/util/ui/ui_other.go` | 修改 | build tag → `!windows && !darwin`；函数名重命名 |
| `wox.core/ui/gpu_ui_impl.go` | 修改 | renderer 类型改为接口；`NewNativeRenderer`；新增 `composeValue` + `EventIMECompose` 处理 |
| `wox.core/Makefile` | 无需改动 | macOS 构建命令已用 `CGO_ENABLED=1`，新文件自动包含 |

## 验证

- `go build -tags sqlite_fts5 ./...` 在 macOS 上通过
- `go vet ./...` 通过
- 手动验证项（用户验证）：
  - 启动后窗口可见，vibrancy 背景透出桌面
  - 键盘输入触发 query，结果列表正确显示
  - 上下键选择，Enter 执行
  - 中文输入法 composition 文本显示，候选词窗口跟随光标
  - 鼠标滚轮滚动结果列表 / 预览面板
  - 主题切换时明暗 vibrancy 正确
  - 切换 app（cmd+tab）后窗口不意外隐藏（因为 `setHidesOnDeactivate:NO`）
  - Escape / 点击外部隐藏窗口
  - 预览面板（text/markdown/image）正确渲染

## 不在本次范围

- Linux X11 实现（未来单独项目）
- AI chat view native 实现（Windows 版也是 TODO）
- Glance items native 实现（Windows 版也是 TODO）
- 截图/文件选择 native 实现（委托给 Flutter wsUI，与 Windows 一致）
- Settings/Onboarding native 实现（委托给 Flutter wsUI，与 Windows 一致）
- Composition 文本的下划线/高亮样式增强（第一版简单拼接显示，后续迭代）