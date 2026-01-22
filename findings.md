# Findings & Decisions
<!-- 
  WHAT: Your knowledge base for the task. Stores everything you discover and decide.
  WHY: Context windows are limited. This file is your "external memory" - persistent and unlimited.
  WHEN: Update after ANY discovery, especially after 2 view/browser/search operations (2-Action Rule).
-->

## Requirements
<!-- 
  WHAT: What the user asked for, broken down into specific requirements.
  WHY: Keeps requirements visible so you don't forget what you're building.
  WHEN: Fill this in during Phase 1 (Requirements & Discovery).
  EXAMPLE:
    - Command-line interface
    - Add tasks
    - List all tasks
    - Delete tasks
    - Python implementation
-->
<!-- Captured from user request -->
- Provide a migration plan from `wox.ui.flutter` to `wox.ui.windows` for the launcher UI.
- Exclude all settings-related UI/features.
- Use current repo structure and existing WPF scaffolding.

## Research Findings
<!-- 
  WHAT: Key discoveries from web searches, documentation reading, or exploration.
  WHY: Multimodal content (images, browser results) doesn't persist. Write it down immediately.
  WHEN: After EVERY 2 view/browser/search operations, update this section (2-Action Rule).
  EXAMPLE:
    - Python's argparse module supports subcommands for clean CLI design
    - JSON module handles file persistence easily
    - Standard pattern: python script.py <command> [args]
-->
<!-- Key discoveries during exploration -->
- Flutter launcher views/controllers and behaviors are identified (query box, results, preview, toolbar).
- WPF project already implements several core features; remaining gaps are listed in `wox.ui.windows/todo.md`.
- Reviewed planning files; preparing detailed Flutter -> WPF feature matrix for launcher-only scope.
- MainWindow toolbar area uses hardcoded Chinese strings ("个结果", "打开", "Enter") and a static layout; likely needs i18n + dynamic actions to match Flutter toolbar behaviors.
- MainWindow.xaml.cs handles quick select (Alt + digits), action panel navigation, hide-on-ESC, window drag, hide-on-deactivate, and query box key handling (arrows, Enter, Tab, grid left/right); aligns with parts of Flutter hotkey behavior but lacks some keys (Home/End, action hotkey toggle, update hotkey, IME composition handling).
- MainViewModel includes grid layout selection navigation (up/down/left/right) and uses GridLayoutParams columns; grid layout exists but needs full metadata wiring and UI parity with Flutter grid mode.
- MainViewModel parses toolbar messages (Text/Icon) from `ShowToolbarMsg`, but `MainWindow.xaml` currently renders static toolbar text/actions, so dynamic message/actions/hotkey list is missing.
- WPF models already define QueryIconInfo and DoctorCheckInfo in `wox.ui.windows/Models/Query.cs`, but the UI wiring for query icon actions and doctor toolbar display is not implemented.
- WoxApiService handles Query/ChangeQuery/ShowHistory/RefreshQuery/ShowToolbarMsg/UpdateResult/GetCurrentQuery; no obvious query-metadata or query-icon update message found yet.
- WPF models include `QueryMetadata` (icon, result preview ratio, grid layout params), but `QueryResult` does not include metadata; wiring for metadata updates appears missing.
- `QueryMetadata` is only defined in `wox.ui.windows/Models/Query.cs` and not referenced elsewhere; `QueryIcon` is not used in viewmodels yet.
- Core `/query/metadata` endpoint expects `query` payload with `QueryId/QueryType/QueryText/QuerySelection` and returns a RestResponse wrapper with `Data` containing icon/widthRatio/grid layout info.

## Technical Decisions
<!-- 
  WHAT: Architecture and implementation choices you've made, with reasoning.
  WHY: You'll forget why you chose a technology or approach. This table preserves that knowledge.
  WHEN: Update whenever you make a significant technical choice.
  EXAMPLE:
    | Use JSON for storage | Simple, human-readable, built-in Python support |
    | argparse with subcommands | Clean CLI: python todo.py add "task" |
-->
<!-- Decisions made with rationale -->
| Decision | Rationale |
|----------|-----------|
|          |           |

## Issues Encountered
<!-- 
  WHAT: Problems you ran into and how you solved them.
  WHY: Similar to errors in task_plan.md, but focused on broader issues (not just code errors).
  WHEN: Document when you encounter blockers or unexpected challenges.
  EXAMPLE:
    | Empty file causes JSONDecodeError | Added explicit empty file check before json.load() |
-->
<!-- Errors and how they were resolved -->
| Issue | Resolution |
|-------|------------|
|       |            |

## Resources
<!-- 
  WHAT: URLs, file paths, API references, documentation links you've found useful.
  WHY: Easy reference for later. Don't lose important links in context.
  WHEN: Add as you discover useful resources.
  EXAMPLE:
    - Python argparse docs: https://docs.python.org/3/library/argparse.html
    - Project structure: src/main.py, src/utils.py
-->
<!-- URLs, file paths, API references -->
- wox.ui.flutter/wox/lib/modules/launcher
- wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart
- wox.ui.windows/MainWindow.xaml
- wox.ui.windows/ViewModels/MainViewModel.cs
- wox.ui.windows/todo.md

## Visual/Browser Findings
<!-- 
  WHAT: Information you learned from viewing images, PDFs, or browser results.
  WHY: CRITICAL - Visual/multimodal content doesn't persist in context. Must be captured as text.
  WHEN: IMMEDIATELY after viewing images or browser results. Don't wait!
  EXAMPLE:
    - Screenshot shows login form has email and password fields
    - Browser shows API returns JSON with "status" and "data" keys
-->
<!-- CRITICAL: Update after every 2 view/browser operations -->
<!-- Multimodal content must be captured as text immediately -->
-

---
<!-- 
  REMINDER: The 2-Action Rule
  After every 2 view/browser/search operations, you MUST update this file.
  This prevents visual information from being lost when context resets.
-->
*Update this file after every 2 view/browser/search operations*
*This prevents visual information from being lost*
- Repo contains UI targets: wox.ui.flutter and wox.ui.windows alongside wox.ui.macos (existing Windows UI scaffold likely exists).
- wox.ui.windows already contains WPF project structure (Views/ViewModels/Services/etc.) plus MainWindow/App files and a csproj; wox.ui.flutter contains a single 'wox' subfolder.
- Flutter UI lives under wox.ui.flutter/wox with lib/ containing api, components, controllers, entity, enums, models, modules, utils, and main.dart.
- Flutter modules include launcher and setting; launcher module has a views/ directory (settings excluded per request).
- Launcher views: wox_launcher_view.dart, wox_query_box_view.dart, wox_query_result_view.dart, wox_query_toolbar_view.dart. Controllers include wox_launcher_controller.dart and wox_ai_chat_controller.dart plus list/grid/base, query_box controller, and settings controller.
- WoxLauncherView builds a DropTarget around the app, shows query box + results, optional toolbar, uses theme padding/colors. QueryBoxView handles IME composition, keyboard navigation (arrows, enter, tab, home/end), quick-select (Alt/Cmd + digits), action hotkey, update hotkey, drag-move area, loading icon, and uses WoxPlatformFocus for key events.
- QueryResultView: supports list or grid result views, preview panel (local/remote previews), action panel list with hotkeys, and action form panel with form inputs; hides panels on item tap and resizes via theme max heights. ToolbarView: shows left message (optional icon, copy/snooze actions), right-side action hotkeys sized to fit, background/border depends on results.
- Launcher controller manages query lifecycle, result list/grid controllers, preview panel, action panel/form actions, toolbar state (including update/doctor messages), quick select, drag-drop files, window show/hide/position, query metadata (grid layout + preview ratio), and action hotkeys; integrates WoxApi/websocket and AI chat preview handling.
- wox.ui.windows/todo.md tracks WPF parity work: Phases 1-3 done; pending Phase 4 (grid layout, doctor toolbar, drag-drop, query icon, multiline query box), Phase 5 (transparent/topmost/acrylic, theme switch), Phase 6 AI chat UI XAML components.\n- MainWindow.xaml already defines a transparent, topmost, no-taskbar window with query TextBox (AcceptsReturn, MaxLines=3) and query icon slot; results list/grid views are wired with styles and bindings.
- wox.ui.windows Views include FormView.xaml and GridView.xaml; ViewModels include MainViewModel.cs, FormViewModel.cs, and AIChatViewModel.cs.
- MainViewModel manages query text, results, selection, preview content/type/image path, toolbar visibility, window size, action panel state, quick-select, grid layout params, query icon, and doctor check info; handles settings (width/max results/preview ratio) and preview type detection for file/image/markdown.\n- GridView.xaml uses ItemsControl + UniformGrid bound to GridLayoutParams columns, shows icon/title, and highlights selected/hover; click binds to ExecuteSelectedAsync on MainViewModel.
- Flutter WoxPreviewView supports markdown, text, file preview (pdf, md, images, code with syntax highlighting and size limit), image data via WoxImage, plugin detail, AI chat preview, update preview, preview properties list, and scroll positioning.
