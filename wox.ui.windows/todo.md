# Complete Windows UI Features

## Overview

Complete missing features in `wox.ui.windows` by referencing `wox.ui.flutter` implementation.

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
- [ ] Form action panel (handle form input actions)
- [ ] MRU (Most Recently Used) query history

### Phase 3: Preview Panel Improvements ✅

- [x] Support multiple preview types (text, markdown, image)
- [x] Preview type detection from file extension
- [ ] Preview scroll position handling
- [ ] Preview width ratio customization

### Phase 4: AI Chat UI ✅

- [x] AI chat models (AIModel, AIChatData, AIChatConversation, ToolCallInfo)
- [x] AI chat ViewModel with chat response handling
- [ ] AI chat view component (XAML - requires additional UI work)
- [ ] AI model selector
- [ ] Chat conversation management UI
- [ ] Tool call display

### Phase 5: Additional Features

- [ ] Grid layout mode for results
- [ ] Doctor check toolbar
- [ ] Drag and drop file support
- [ ] Query icon based on plugin
- [ ] Multi-line query box support

## Summary

**Completed:**

- Phase 1: All 8 WebSocket handlers implemented
- Phase 2: Core UI features (action panel, auto-complete, quick select)
- Phase 3: Multi-type preview (text, markdown, image)
- Phase 4: AI chat backend (models and ViewModel)

**Pending:**

- AI Chat UI XAML components
- Form action panel
- MRU query history
- Phase 5 additional features
