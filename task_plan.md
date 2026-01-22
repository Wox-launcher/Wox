# Task Plan: WPF Launcher Reimplementation Plan

## Goal
Create a concrete migration plan to reimplement the Wox launcher UI in `wox.ui.windows` (WPF) based on `wox.ui.flutter`, excluding all settings-related UI.

## Current Phase
Phase 3

## Phases

### Phase 1: Requirements & Discovery
- [x] Confirm scope: launcher UI only, settings excluded
- [x] Inventory Flutter launcher views/controllers and key behaviors
- [x] Review current WPF UI status and existing TODOs
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [x] Build a feature matrix (Flutter launcher -> WPF mapping)
- [x] Identify gaps vs existing WPF implementation (todo + code)
- [x] Define WPF components/views/viewmodels to add or extend
- [x] Outline integration points with WoxApi/WebSocket and services
- **Status:** complete

### Phase 3: Implementation Roadmap
- [ ] Query box + input handling (IME, hotkeys, quick select, drag move)
- [ ] Results list/grid + selection + action panel/form actions
- [ ] Preview panel parity (types, scroll, properties, chat, update)
- [ ] Toolbar parity (left message + actions/hotkeys)
- [ ] Window behaviors (hide/show, positioning, focus, drag/drop)
- [ ] AI chat UI XAML + model selector + tool call views
- [ ] Theme/resource parity (brushes, padding, radii, shadows)
- **Status:** in_progress

## Phase 3 Detailed Checklist (File-Level)
### 3.1 Window Shell & Behaviors
- [ ] Add window-level drag/drop handling (AllowDrop + Drop event) and forward to core API (`wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/MainWindow.xaml.cs`, `wox.ui.windows/Services/WoxApiService.cs`).
- [ ] Persist window position on hide or move (`wox.ui.windows/MainWindow.xaml.cs`, `wox.ui.windows/Services/WoxApiService.cs`).
- [ ] Align hide/show focus callbacks with core expectations (`wox.ui.windows/MainWindow.xaml.cs`).
- [ ] Wire always-on-top and transparency from theme/setting (`wox.ui.windows/App.xaml`, `wox.ui.windows/Services/ThemeService.cs`).

### 3.2 Query Box Input & Hotkeys
- [x] IME composition complete detection (trigger query when composition ends) (`wox.ui.windows/MainWindow.xaml.cs`, `wox.ui.windows/ViewModels/MainViewModel.cs`).
- [x] Add Home/End key handling and action hotkey (Alt+J) / update hotkey (Ctrl+U via action hotkey match) (`wox.ui.windows/MainWindow.xaml.cs`).
- [ ] Quick select modifier-only behavior (Alt/Cmd) + update overlay on scroll (`wox.ui.windows/MainWindow.xaml.cs`, list/grid item templates).
- [x] Query icon click action + loading indicator state (`wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs`, `wox.ui.windows/Services/WoxApiService.cs`).
- [ ] Replace query placeholder text with i18n key (`wox.ui.windows/MainWindow.xaml`).
- [x] Update preview width ratio from query metadata (`wox.ui.windows/ViewModels/MainViewModel.cs`, `wox.ui.windows/Services/WoxApiService.cs`).
- [x] Delay clear results to reduce flicker (port Flutter clear delay + flicker detector) (`wox.ui.windows/ViewModels/MainViewModel.cs`, `wox.ui.windows/Services/WindowFlickerDetector.cs`).

### 3.2 Detailed Steps (Query Box & Hotkeys)
1. **IME 组合输入处理**
   - 在 `wox.ui.windows/MainWindow.xaml.cs` 为 QueryTextBox 监听 `TextCompositionManager.PreviewTextInputStart/PreviewTextInputUpdate/PreviewTextInput`，维护 `IsImeComposing` 状态。
   - 在 `wox.ui.windows/ViewModels/MainViewModel.cs` 的 `OnQueryTextChanged` 中：若 `IsImeComposing` 为 true，暂不发送查询；当组合结束时触发一次 `SendQueryAsync(QueryText)`。
2. **热键对齐 Flutter**
   - 在 `wox.ui.windows/MainWindow.xaml.cs` 增加 Home/End 支持（光标移动到首/尾）。
   - 增加 `Alt+J` 触发动作面板切换（与 Flutter “More actions”一致）。
   - 增加 `Ctrl+U`（若工具栏存在更新入口）触发更新动作。
   - 增加“动作热键”解析：根据 `ResultItem.Actions[].Hotkey` 判断键盘组合并执行对应动作。
3. **快速选择模式**
   - 实现“仅按下 Alt”触发快速选择模式（带定时阈值），松开 Alt 退出。
   - 在结果列表/网格项模板上叠加序号（1-9/0），滚动时同步更新。
   - `Alt+数字` 直接执行对应结果默认动作并退出快速选择。
4. **查询图标 + Loading**
   - 为 `MainViewModel` 增加 `IsLoading` 状态与 QueryIcon 点击回调；在 `MainWindow.xaml` 中显示加载旋转图或 QueryIcon。
   - 新增元数据请求或消息通道（对齐 Flutter `GetQueryMetadata` 逻辑）以更新 QueryIcon 与结果预览比例/网格布局。
5. **i18n 替换**
   - 替换 QueryTextBox placeholder 的硬编码文案，绑定到 `wox.core/resource/lang/*.json` 的 i18n key。

### 3.2 验收要点
- IME 输入时不会反复触发查询，组合结束后会触发一次查询。
- Home/End、Alt+J、Ctrl+U、动作热键均有效且不干扰文本输入。
- Alt 进入快速选择后显示编号，Alt+数字执行正确结果。
- 查询图标随查询元数据变化，加载状态可视化。

### 3.3 Results List/Grid + Action Panel
- [ ] Ensure list item template supports quick-select numbers and action hotkeys (`wox.ui.windows/MainWindow.xaml`).
- [x] Complete grid layout metadata wiring (columns/padding/showTitle) and preview suppression in grid mode (`wox.ui.windows/ViewModels/MainViewModel.cs`, `wox.ui.windows/Views/GridView.xaml`).
- [ ] Implement Alt+J toggle for action panel and "More actions" hotkey entry (`wox.ui.windows/MainWindow.xaml.cs`, `wox.ui.windows/ViewModels/MainViewModel.cs`).
- [ ] Validate action form focus/escape behavior (`wox.ui.windows/Views/FormView.xaml`, `wox.ui.windows/ViewModels/FormViewModel.cs`).

### 3.4 Preview Panel Parity
- [ ] Extend preview types: pdf, code, svg, plugin detail, update view (`wox.ui.windows/Views/PreviewView.xaml` or `wox.ui.windows/MainWindow.xaml` + converters).
- [ ] Add preview properties list (key/value with tooltip) (`wox.ui.windows/Views/PreviewView.xaml`).
- [ ] Add AI chat preview rendering or delegation to AI chat view (`wox.ui.windows/Views/PreviewView.xaml`, `wox.ui.windows/ViewModels/AIChatViewModel.cs`).
- [ ] Enforce file size limits for code preview (`wox.ui.windows/Services/*` or viewmodel).

### 3.5 Toolbar Parity
- [x] Bind toolbar message/icon to viewmodel, remove hardcoded text (`wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs`).
- [x] Render dynamic action list with hotkeys (`wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs`).
- [ ] Implement copy/snooze actions and i18n (`wox.ui.windows/Services/WoxApiService.cs`, XAML).
- [ ] Add doctor check toolbar message/actions (`wox.ui.windows/ViewModels/MainViewModel.cs`, `wox.ui.windows/MainWindow.xaml`).

### 3.6 Drag/Drop Files
- [ ] Map dropped files to query selection and send to core (`wox.ui.windows/MainWindow.xaml.cs`, `wox.ui.windows/Services/WoxApiService.cs`).

### 3.7 AI Chat UI (XAML)
- [ ] Build AI chat view layout + conversation list + tool call display (`wox.ui.windows/Views/AIChatView.xaml` new).
- [ ] Add model selector UI (`wox.ui.windows/Views/AIModelSelectorView.xaml` new).
- [ ] Bind to `AIChatViewModel` and ensure focus routing (`wox.ui.windows/ViewModels/AIChatViewModel.cs`).

### 3.8 Theme Resources + i18n
- [ ] Align padding/radius/brushes with theme JSON and Flutter values (`wox.ui.windows/App.xaml`, `wox.ui.windows/Services/ThemeService.cs`).
- [ ] Replace all user-visible strings with i18n keys (`wox.core/resource/lang/*.json`).

### Phase 4: Verification & Handoff
- [ ] Define acceptance checks for each feature group
- [ ] Identify any manual test steps (no automated builds)
- [ ] Deliver final plan + checklist for implementation
- **Status:** pending

## Key Questions
1. Which Flutter launcher behaviors are missing or only partial in current WPF?
2. What WPF components/services must be added to match Flutter preview types and toolbar behaviors?
3. How should i18n keys be surfaced in WPF for all user-facing strings?
4. What are the acceptance criteria for parity (keyboard, focus, window, drag/drop, AI chat)?

## Scope & Feature Map (Launcher Only)
- Window shell and behaviors (Flutter: `WoxLauncherView` + controller; WPF: `MainWindow.xaml` + code-behind)
- Query box input and hotkeys (Flutter: `wox_query_box_view.dart`; WPF: query TextBox handlers + viewmodel)
- Results list/grid (Flutter: `WoxQueryResultView`, `WoxListView`/`WoxGridView`; WPF: ListView + `Views/GridView.xaml`)
- Preview panel (Flutter: `WoxPreviewView`; WPF: preview content panel + converters)
- Action panel + form actions (Flutter: `WoxFormActionView`; WPF: `Views/FormView.xaml`)
- Toolbar (Flutter: `WoxQueryToolbarView`; WPF: toolbar strip in MainWindow)
- Drag/drop files (Flutter: `DropTarget`; WPF: window-level drag/drop handling)
- AI chat UI (Flutter: `WoxAIChatView`; WPF: `AIChatViewModel` + new XAML view)

## Feature Matrix (Flutter -> WPF)
| Feature | Flutter Reference | WPF Reference | Status | Gap / Planned Work |
|---|---|---|---|---|
| Window shell + drag/move + hide/show | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_launcher_view.dart`, `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/MainWindow.xaml.cs` | Partial | Add drag/drop files, persist position, always-on-top/transparent configurable, align focus/hide behavior with core API callbacks. |
| Query box input + IME + hotkeys | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_query_box_view.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/MainWindow.xaml.cs` | Partial | IME composition completion logic, Home/End, action hotkey (Alt+J), update hotkey (Ctrl+U), quick-select modifier-only behavior, dynamic height per theme. |
| Query icon + loading indicator | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_query_box_view.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/Models/Query.cs` | Partial | Wire query icon click action, loading spinner state, update icon from query metadata. |
| Results list view | `wox.ui.flutter/wox/lib/components/wox_list_view.dart`, `wox.ui.flutter/wox/lib/modules/launcher/views/wox_query_result_view.dart` | `wox.ui.windows/MainWindow.xaml` | Mostly done | Verify selection visuals, quick-select overlay numbers, and action hotkey execution parity. |
| Grid layout results | `wox.ui.flutter/wox/lib/components/wox_grid_view.dart` | `wox.ui.windows/Views/GridView.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs` | Partial | Map query metadata to `GridLayoutParams` (columns/padding/showTitle), hide preview in grid mode, ensure selection/row height handling. |
| Action panel (list actions) | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_query_result_view.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs` | Mostly done | Add Alt+J toggle, toolbar "More actions" parity, ensure filter box hotkeys if used. |
| Form action panel | `wox.ui.flutter/wox/lib/components/wox_form_action_view.dart` | `wox.ui.windows/Views/FormView.xaml`, `wox.ui.windows/ViewModels/FormViewModel.cs` | Done | Verify focus/escape behavior and i18n labels. |
| Preview panel (text/markdown/image) | `wox.ui.flutter/wox/lib/components/wox_preview_view.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs` | Partial | Expand beyond text/markdown/image; support file previews (pdf/code/svg) and size limits. |
| Preview panel (plugin detail/update/chat/properties) | `wox.ui.flutter/wox/lib/components/wox_preview_view.dart` | N/A | Missing | Implement preview types: plugin detail, update view, AI chat preview, preview properties list. |
| Toolbar message + actions | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_query_toolbar_view.dart` | `wox.ui.windows/MainWindow.xaml`, `wox.ui.windows/ViewModels/MainViewModel.cs` | Missing | Bind toolbar message/icon, render actions with hotkeys, implement copy/snooze, dynamic width calculation, and i18n. |
| Quick select mode (numbers overlay) | `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart` | `wox.ui.windows/MainWindow.xaml.cs` | Partial | Show number overlays in list/grid, handle modifier-only detection and update on scroll. |
| Drag/drop files | `wox.ui.flutter/wox/lib/modules/launcher/views/wox_launcher_view.dart` | N/A | Missing | Implement window-level drop handling and send selection to core. |
| Doctor check toolbar | `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart` | `wox.ui.windows/Models/Query.cs` | Missing | Render doctor toolbar message/icon + update action; integrate with toolbar actions. |
| AI chat UI (view + model selector + tool calls) | `wox.ui.flutter/wox/lib/components/wox_ai_chat_view.dart` | `wox.ui.windows/ViewModels/AIChatViewModel.cs` | Missing | Build XAML view, model selector, conversation list, tool call display; bind to viewmodel. |
| Theme + effects (acrylic/blur) | `wox.ui.flutter/wox/lib/utils/wox_theme_util.dart` | `wox.ui.windows/App.xaml`, `wox.ui.windows/Services/ThemeService.cs` | Partial | Acrylic/blur background, theme switching parity, and padding/radius values aligned with theme JSON. |
| i18n of user strings | Flutter i18n keys | WPF XAML | Missing | Replace hardcoded strings ("输入查询...", "打开", "个结果") with i18n keys from `wox.core/resource/lang/*.json`. |

## Integration Points (WPF)
- `wox.ui.windows/Services/WoxApiService.cs`: ensure query metadata, toolbar messages/actions, update actions, and drop-file selection are forwarded to the viewmodels.
- `wox.ui.windows/ViewModels/MainViewModel.cs`: consume metadata for grid layout + preview ratio, update query icon, and drive toolbar/action lists.
- `wox.ui.windows/MainWindow.xaml.cs`: align key handling with Flutter (action hotkeys, update hotkey, IME triggers).

## Acceptance Criteria (Launcher Parity)
- Query box: IME input triggers query correctly; Enter executes, Tab autocompletes, Alt+J toggles actions; Home/End supported.
- Results: list + grid navigation parity; quick-select numbers visible and usable.
- Preview: supports markdown/text/image plus file/pdf/code; chat/update previews render.
- Toolbar: left message with icon + copy/snooze; right actions with hotkeys and "More actions".
- Window: drag/move, hide on blur, drop files -> query selection; always-on-top and transparency match theme.

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Use existing wox.ui.windows MVVM structure (MainWindow + ViewModels/Views) | Aligns with current implementation and minimizes churn |
| Exclude settings UI from the plan | Matches user scope request |
| Map features against existing wox.ui.windows/todo.md | Provides a ready gap checklist |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
|       | 1       |            |

## Notes
- Do not compile or run builds; provide plan and checklist only.
- All user-facing text must map to existing i18n keys in wox.core.
