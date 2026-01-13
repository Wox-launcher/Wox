# Wox macOS UI Sync Plan

## Overview

This document outlines the plan to sync the Flutter query interface implementation to the macOS SwiftUI implementation, achieving 1:1 feature parity for the query functionality.

## Current State Analysis

### Flutter Implementation (Source of Truth)

#### Controllers
1. **WoxLauncherController** (1,314 lines)
   - Query management (input, MRU, history)
   - Result management (list & grid)
   - Keyboard navigation (arrows, enter, shortcuts)
   - Action panel management (Cmd+J)
   - Preview panel management
   - Toolbar management
   - Quick select mode (Cmd+number)
   - Grid layout support
   - Loading animation
   - Window resize management

2. **WoxBaseListController** (210 lines)
   - Item management with original/filtered lists
   - Active/hovered index management
   - Fuzzy matching filter
   - Scroll controller sync
   - Group handling

3. **WoxListController** (134 lines)
   - Direction-based navigation (up/down/left/right)
   - Scroll position sync with active index
   - Group item skipping

4. **WoxGridController** (260 lines)
   - Grid layout navigation
   - Row-based movement
   - Dynamic row height calculation
   - Scroll sync for grid rows

#### Views/Components

1. **WoxQueryBoxView** (340 lines)
   - Multi-line TextField with ExtendedTextField
   - IME composition handling (critical for Chinese input)
   - Keyboard event handling
   - Quick select mode (Cmd key detection)
   - Auto-complete (Tab)
   - Icon/loading indicator

2. **WoxQueryResultView** (210 lines)
   - Result container with List/Grid
   - Preview panel (flexible width)
   - Action panel (floating, 320px max)
   - Form action panel (360x400 max)

3. **WoxListView** (286 lines)
   - ScrollView with custom scrollbar
   - Mouse region for hover detection
   - Filter box at bottom
   - Keyboard navigation in filter box
   - Action hotkey matching

4. **WoxGridView** (See separate file)
   - GridLayoutParams configuration
   - Responsive column calculation
   - Icon grid display

5. **WoxListItemView** (263 lines)
   - Icon (30x30)
   - Title (16px, single line)
   - SubTitle (13px, optional)
   - Tails (text, hotkey, image)
   - Quick select number badge
   - Active border indicator (left)
   - Group header styling
   - Hover/active background colors

6. **WoxQueryToolbarView** (306 lines)
   - Left: Icon + text message with copy/snooze
   - Right: Action hotkeys
   - Dynamic width calculation
   - Text overflow handling

### macOS Current Implementation

#### Files
- **ViewModel.swift** (1,048 lines) - Combined controller
- **ContentView.swift** (864 lines) - Main UI
- **ResultRow.swift** (188 lines) - List item
- **Models.swift** (812 lines) - Data models
- **WebSocketManager.swift** - Connection handling

#### Gaps Identified

| Feature | Flutter | macOS | Priority |
|---------|---------|-------|----------|
| Controller Architecture | Separate controllers | Single ViewModel | HIGH |
| Grid Layout | WoxGridController + View | Basic GridView | HIGH |
| Mouse Hover State | Full support | None | MEDIUM |
| List Filtering | WoxBaseListController | None | MEDIUM |
| Toolbar Features | Full (copy, snooze) | Basic | MEDIUM |
| Action Panel | Filter + keyboard nav | Basic | MEDIUM |
| Hotkey Display | WoxHotkeyView (detailed) | HotkeyBadge (basic) | LOW |

## Implementation Plan

### Phase 1: Controller Refactoring (HIGH)

#### 1.1 Create BaseListController

```swift
// Controllers/BaseListController.swift
class BaseListController<T>: ObservableObject {
    // MARK: - Published Properties
    @Published var items: [T] = []
    @Published var filteredItems: [T] = []
    @Published var originalItems: [T] = []
    @Published var activeIndex: Int = 0
    @Published var hoveredIndex: Int = -1
    
    // Filter
    @Published var filterText: String = ""
    let filterBoxController = NSTextField()
    let filterBoxFocusNode = FocusNode()
    
    // Scroll
    let scrollController = ScrollController()
    
    // Callbacks
    var onItemExecuted: ((T) -> Void)?
    var onItemActive: ((T) -> Void)?
    var onFilterBoxEscPressed: (() -> Void)?
    var onItemsEmpty: (() -> Void)?
    
    // Methods
    func updateItems(_ newItems: [T])
    func filterItems(_ filterText: String)
    func updateActiveIndex(_ index: Int)
    func clearHoveredResult()
    func isFuzzyMatch(text: String, pattern: String) -> Bool
}
```

#### 1.2 Create ResultListController

```swift
// Controllers/ResultListController.swift
class ResultListController: BaseListController<WoxQueryResult> {
    func updateActiveIndexByDirection(_ direction: Direction)
    func syncScrollPositionWithActiveIndex()
    private func findPrevNonGroupIndex(_ current: Int) -> Int
    private func findNextNonGroupIndex(_ current: Int) -> Int
}
```

#### 1.3 Create ResultGridController

```swift
// Controllers/ResultGridController.swift
class ResultGridController: BaseListController<WoxQueryResult> {
    @Published var gridLayoutParams: GridLayoutParams = .empty()
    @Published var rowHeight: CGFloat = 0
    
    func calculateGridHeight() -> CGFloat
    func updateGridParams(_ params: GridLayoutParams)
    func updateActiveIndexByDirection(_ direction: Direction)
}
```

#### 1.4 Create ActionListController

```swift
// Controllers/ActionListController.swift
class ActionListController: BaseListController<WoxResultAction> {
    // Similar to ResultListController but for actions
}
```

### Phase 2: Grid View Implementation (HIGH)

#### 2.1 GridView Component

```swift
// Views/GridView.swift
struct GridView: View {
    @ObservedObject var controller: ResultGridController
    let maxHeight: CGFloat
    let onItemTapped: () -> Void
    let onRowHeightChanged: () -> Void
    
    var body: some View {
        ScrollViewReader { proxy in
            ScrollView(.vertical, showsIndicators: false) {
                LazyVGrid(columns: columns, spacing: gridLayoutParams.itemMargin) {
                    ForEach(Array(filteredItems.enumerated()), id: \.element.id) { index, item in
                        GridItemView(
                            item: item,
                            isActive: controller.activeIndex == index,
                            isHovered: controller.hoveredIndex == index,
                            theme: theme,
                            quickSelectNumber: quickSelectNumber(for: item, at: index)
                        )
                        .id(item.id)
                        .onTapGesture {
                            if !item.isGroup {
                                controller.activeIndex = index
                                onItemTapped()
                            }
                        }
                    }
                }
                .padding(.horizontal, gridLayoutParams.itemMargin)
            }
            .onChange(of: controller.activeIndex) { newIndex in
                withAnimation(.easeInOut(duration: 0.1)) {
                    proxy.scrollTo(filteredItems[newIndex].id, anchor: .center)
                }
            }
        }
    }
}
```

### Phase 3: Enhanced List View (MEDIUM)

#### 3.1 Enhanced ResultRow

```swift
// Views/ResultRow.swift (Enhanced)
struct ResultRow: View {
    let result: WoxQueryResult
    let isSelected: Bool
    let isHovered: Bool  // NEW
    let theme: WoxTheme
    let quickSelectNumber: Int?
    
    var body: some View {
        if result.isGroup {
            GroupHeader(title: result.title, theme: theme)
        } else {
            HStack(alignment: .center, spacing: 0) {
                // Left border indicator
                if isSelected && theme.resultItemActiveBorderLeftWidth > 0 {
                    Rectangle()
                        .fill(Color(hex: theme.resultItemActiveBackgroundColor))
                        .frame(width: theme.resultItemActiveBorderLeftWidth)
                }
                
                // Icon with quick select badge
                ZStack(alignment: .topLeading) {
                    WoxIconView(icon: result.icon, size: 30)
                    if let number = quickSelectNumber {
                        QuickSelectBadge(number: number, theme: theme, isSelected: isSelected)
                    }
                }
                
                // Title and subtitle
                VStack(alignment: .leading, spacing: 2) {
                    Text(result.title)
                        .font(.system(size: 16, weight: .medium))
                        .foregroundColor(titleColor)
                        .lineLimit(1)
                    
                    if let subTitle = result.subTitle, !subTitle.isEmpty {
                        Text(subTitle)
                            .font(.system(size: 13))
                            .foregroundColor(subtitleColor)
                            .lineLimit(1)
                    }
                }
                
                Spacer()
                
                // Tails (text, hotkey, image)
                if let tails = result.tails {
                    TailsView(tails: tails, isSelected: isSelected, theme: theme)
                }
            }
            .padding(.horizontal, theme.resultItemPaddingLeft)
            .padding(.vertical, theme.resultItemPaddingTop)
            .background(backgroundColor)
            .cornerRadius(theme.resultItemBorderRadius)
        }
    }
    
    private var backgroundColor: Color {
        if isSelected {
            return Color(hex: theme.resultItemActiveBackgroundColor)
        } else if isHovered {
            return Color(hex: theme.resultItemActiveBackgroundColor).opacity(0.3)
        }
        return .clear
    }
}
```

#### 3.2 Add Mouse Hover Detection

```swift
// Add to ContentView.swift - Results List
ScrollViewReader { proxy in
    ScrollView(.vertical, showsIndicators: false) {
        ForEach(Array(results.enumerated()), id: \.element.id) { index, result in
            ResultRow(
                result: result,
                isSelected: viewModel.selectedResultId == result.id,
                isHovered: viewModel.hoveredIndex == index,  // NEW
                theme: viewModel.theme,
                quickSelectNumber: quickSelectNumber(for: result, at: index)
            )
            .id(result.id)
            .onHover { hovering in
                if hovering && !result.isGroup {
                    viewModel.hoveredIndex = index
                } else if viewModel.hoveredIndex == index {
                    viewModel.hoveredIndex = -1
                }
            }
        }
    }
}
```

### Phase 4: Toolbar Enhancement (MEDIUM)

#### 4.1 Enhanced ToolbarView

```swift
// Views/ToolbarView.swift (Enhanced)
struct ToolbarView: View {
    let info: ToolbarInfo
    let theme: WoxTheme
    let hasResultItems: Bool
    
    var body: some View {
        HStack(spacing: 0) {
            // Left: Icon + Message
            leftSection
                .frame(maxWidth: .infinity, alignment: .leading)
            
            // Right: Actions with hotkeys
            rightSection
                .frame(maxWidth: .infinity, alignment: .trailing)
        }
        .padding(.horizontal, theme.toolbarPaddingLeft)
        .background(backgroundColor)
    }
    
    private var leftSection: some View {
        HStack(spacing: 8) {
            if let icon = info.icon {
                WoxIconView(icon: icon, size: 14)
            }
            if let text = info.text {
                Text(text)
                    .font(.system(size: 12))
                    .foregroundColor(Color(hex: theme.toolbarFontColor))
                    .lineLimit(1)
                    .truncationMode(.tail)
                
                // Copy button on hover
                CopyButton(text: text)
            }
        }
    }
    
    private var rightSection: some View {
        HStack(spacing: 16) {
            ForEach(info.actions ?? [], id: \.name) { action in
                HStack(spacing: 4) {
                    Text(action.name)
                        .font(.system(size: 11))
                        .foregroundColor(Color(hex: theme.toolbarFontColor))
                    
                    HotkeyBadge(hotkey: action.hotkey, theme: theme)
                }
            }
        }
    }
}
```

### Phase 5: Action Panel Enhancement (MEDIUM)

#### 5.1 Enhanced ActionPanelView

```swift
// Views/ActionPanelView.swift (Enhanced)
struct ActionPanelView: View {
    let actions: [WoxResultAction]
    let selectedActionId: String?
    let theme: WoxTheme
    let onActionTap: (WoxResultAction) -> Void
    
    @StateObject private var filterController = ActionFilterController()
    
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            // Header with filter
            HStack {
                Text("操作")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundColor(Color(hex: theme.actionContainerHeaderFontColor))
                
                Spacer()
                
                // Filter box
                HStack(spacing: 4) {
                    Image(systemName: "magnifyingglass")
                        .font(.system(size: 12))
                        .foregroundColor(.secondary)
                    
                    TextField("过滤", text: $filterController.filterText)
                        .font(.system(size: 12))
                        .frame(width: 120)
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(Color(hex: theme.actionQueryBoxBackgroundColor))
                .cornerRadius(4)
            }
            
            Divider()
                .background(Color(hex: theme.previewSplitLineColor))
            
            // Filtered actions list
            ScrollViewReader { proxy in
                ScrollView(.vertical, showsIndicators: false) {
                    LazyVStack(spacing: 2) {
                        ForEach(filteredActions) { action in
                            ActionRow(
                                action: action,
                                isSelected: selectedActionId == action.id,
                                theme: theme
                            )
                            .id(action.id)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                onActionTap(action)
                            }
                            .onAppear {
                                if selectedActionId == action.id {
                                    withAnimation {
                                        proxy.scrollTo(action.id, anchor: .center)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
        .padding(12)
        .frame(width: 280, maxHeight: 320)
        .background(Color(hex: theme.actionContainerBackgroundColor))
        .cornerRadius(theme.actionQueryBoxBorderRadius)
    }
    
    private var filteredActions: [WoxResultAction] {
        if filterController.filterText.isEmpty {
            return actions
        }
        return actions.filter {
            $0.name.localizedCaseInsensitiveContains(filterController.filterText)
        }
    }
}
```

### Phase 6: Hotkey Display Enhancement (LOW)

#### 6.1 Enhanced HotkeyView

```swift
// Views/HotkeyView.swift
struct HotkeyView: View {
    let hotkey: String
    let theme: WoxTheme
    let backgroundColor: Color
    let borderColor: Color
    let textColor: Color
    
    var body: some View {
        HStack(spacing: 4) {
            ForEach(parseHotkeyParts(), id: \.self) { part in
                KeyBox(key: part, theme: theme)
            }
        }
    }
    
    private func parseHotkeyParts() -> [String] {
        let parts = hotkey.lowercased().split(separator: "+").map { String($0) }
        return parts.compactMap { part in
            switch part {
            case "cmd", "command": return "⌘"
            case "ctrl", "control": return "⌃"
            case "alt", "option": return "⌥"
            case "shift": return "⇧"
            case "enter", "return": return "⏎"
            case "tab": return "⇥"
            case "esc", "escape": return "⎋"
            case "space": return "␣"
            case "backspace": return "⌫"
            case "delete": return "⌦"
            case "up": return "↑"
            case "down": return "↓"
            case "left": return "←"
            case "right": return "→"
            default: return part.uppercased()
            }
        }
    }
}

struct KeyBox: View {
    let key: String
    let theme: WoxTheme
    
    var body: some View {
        Text(key)
            .font(.system(size: 11, weight: .medium))
            .frame(minWidth: 22, minHeight: 20)
            .padding(.horizontal, 4)
            .foregroundColor(Color(hex: theme.toolbarFontColor))
            .background(backgroundColor)
            .cornerRadius(4)
            .overlay(
                RoundedRectangle(cornerRadius: 4)
                    .stroke(borderColor, lineWidth: 1)
            )
    }
}
```

## Implementation Order

1. **Week 1**: Phase 1 (Controller Refactoring)
   - Create BaseListController
   - Create ResultListController
   - Create ResultGridController
   - Create ActionListController
   - Refactor ViewModel to use new controllers

2. **Week 2**: Phase 2 & 3 (Grid View + Enhanced List)
   - Implement GridView component
   - Add mouse hover detection
   - Enhance ResultRow with all Flutter features

3. **Week 3**: Phase 4 & 5 (Toolbar + Action Panel)
   - Enhance toolbar with copy/snooze
   - Implement action panel filtering
   - Add keyboard navigation to action panel

4. **Week 4**: Phase 6 (Hotkey Display + Testing)
   - Enhance hotkey display
   - Comprehensive testing
   - Bug fixes and polish

## Testing Checklist

- [ ] Query input and MRU display
- [ ] Result filtering (local and remote)
- [ ] Keyboard navigation (arrows, enter)
- [ ] Quick select mode (Cmd+1-9)
- [ ] Mouse hover selection
- [ ] Grid layout display and navigation
- [ ] Action panel (Cmd+J) with filtering
- [ ] Preview panel display
- [ ] Toolbar message and actions
- [ ] Theme switching
- [ ] Window resize behavior
- [ ] WebSocket reconnection

## Files to Create/Modify

### New Files
- `wox.ui.macos/Sources/wox.ui.macos/Controllers/BaseListController.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Controllers/ResultListController.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Controllers/ResultGridController.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Controllers/ActionListController.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Views/GridView.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Views/ToolbarView.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Views/ActionPanelView.swift`
- `wox.ui.macos/Sources/wox.ui.macos/Views/HotkeyView.swift`

### Modified Files
- `wox.ui.macos/Sources/wox.ui.macos/ViewModel.swift` - Refactor to use controllers
- `wox.ui.macos/Sources/wox.ui.macos/Views/ContentView.swift` - Use new components
- `wox.ui.macos/Sources/wox.ui.macos/Views/ResultRow.swift` - Add hover support
- `wox.ui.macos/Sources/wox.ui.macos/Models.swift` - Add missing models
