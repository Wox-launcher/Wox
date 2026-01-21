# Complete Windows UI Features

## Overview

Complete missing features in `wox.ui.windows` by referencing `wox.ui.flutter` implementation.

DON'T Implement Setting Related Features

## Current Progress

### Phase 1: API/WebSocket Missing Methods ✅

- [x] Add missing WebSocket request handlers:
  - [x] `RefreshQuery` - Re-execute current query
  - [x] `PickFiles` - File picker dialog
  - [x] `ShowToolbarMsg` - Display toolbar messages
  - [x] `GetCurrentQuery` - Return current query state
  - [x] `UpdateResult` - Update specific result item
  - [x] `FocusToChatInput` - Focus AI chat input
  - [x] `SendChatResponse` - Handle AI chat responses
  - [x] `ReloadChatResources` - Reload AI chat resources

### Phase 2: Core UI Features ✅

- [x] Action panel (show all actions for selected result) - Alt+Enter
- [x] Auto-complete (Tab key to auto-complete query)
- [x] Quick select mode (Alt+number keys for quick selection)
- [x] Improved toolbar with message display
- [x] Form action panel (handle form input actions)
- [x] MRU (Most Recently Used) query history

### Phase 3: Preview Panel Improvements ✅

- [x] Support multiple preview types (text, markdown, image)
- [x] Preview type detection from file extension
- [x] Preview scroll position handling
- [x] Preview width ratio customization

### Phase 4: Additional Features

- [ ] Grid layout mode for results
- [ ] Doctor check toolbar
- [ ] Drag and drop file support
- [ ] Query icon based on plugin
- [ ] Multi-line query box support

### Phase 5: Theme and Style Effects

- [ ] Transparent window support
- [ ] Always on Top (TopMost) support
- [ ] Acrylic/Blur background effects
- [ ] Dark/Light theme switching

### Phase 6: AI Chat UI

- [x] AI chat models (AIModel, AIChatData, AIChatConversation, ToolCallInfo)
- [x] AI chat ViewModel with chat response handling
- [ ] AI chat view component (XAML - requires additional UI work)
- [ ] AI model selector
- [ ] Chat conversation management UI
- [ ] Tool call display

## Summary

**Completed:**

- Phase 1: All 8 WebSocket handlers implemented
- Phase 2: Core UI features (action panel, auto-complete, quick select, form panel, MRU)
- Phase 3: Preview panel improvements (multi-type, scroll, ratio)

**Pending:**

- Phase 4 additional features
- Phase 5 theme and style effects
- Phase 6 AI Chat UI XAML components
