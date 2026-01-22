# Task Plan: WPF Launcher Reimplementation Plan

## Goal
Create a concrete migration plan to reimplement the Wox launcher UI in `wox.ui.windows` (WPF) based on `wox.ui.flutter`, excluding all settings-related UI.

## Current Phase
Phase 2

## Phases

### Phase 1: Requirements & Discovery
- [x] Confirm scope: launcher UI only, settings excluded
- [x] Inventory Flutter launcher views/controllers and key behaviors
- [x] Review current WPF UI status and existing TODOs
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [ ] Build a feature matrix (Flutter launcher -> WPF mapping)
- [ ] Identify gaps vs existing WPF implementation (todo + code)
- [ ] Define WPF components/views/viewmodels to add or extend
- [ ] Outline integration points with WoxApi/WebSocket and services
- **Status:** in_progress

### Phase 3: Implementation Roadmap
- [ ] Query box + input handling (IME, hotkeys, quick select, drag move)
- [ ] Results list/grid + selection + action panel/form actions
- [ ] Preview panel parity (types, scroll, properties, chat, update)
- [ ] Toolbar parity (left message + actions/hotkeys)
- [ ] Window behaviors (hide/show, positioning, focus, drag/drop)
- [ ] AI chat UI XAML + model selector + tool call views
- [ ] Theme/resource parity (brushes, padding, radii, shadows)
- **Status:** pending

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
