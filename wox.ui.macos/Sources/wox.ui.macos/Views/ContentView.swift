import AppKit
import SwiftUI
import WebKit

struct ContentView: View {
    @ObservedObject var viewModel: WoxViewModel
    var windowController: WoxWindowController

    var body: some View {
        VStack(spacing: 0) {
            InputAreaView(viewModel: viewModel)
            
            if !viewModel.results.isEmpty {
                Divider()
                    .background(Color(hex: viewModel.theme.previewSplitLineColor))

                ResultAreaView(viewModel: viewModel)
            }

            if viewModel.isShowToolbar {
                Divider()
                    .background(Color(hex: viewModel.theme.previewSplitLineColor))

                ToolbarAreaView(viewModel: viewModel)
            }
        }
        .frame(width: totalWidth, height: totalHeight)
        .background(
            ZStack {
                VisualEffectView(material: .popover, blendingMode: .behindWindow)
                Color(hex: viewModel.theme.appBackgroundColor).opacity(0.85)
            }
        )
        .cornerRadius(12)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Color.white.opacity(0.15), lineWidth: 0.5)
        )
        .onChange(of: totalHeight) { newValue in
            windowController.resizeWindow(to: newValue, width: totalWidth)
        }
        .onChange(of: totalWidth) { newValue in
            windowController.resizeWindow(to: totalHeight, width: newValue)
        }
        .ignoresSafeArea(.container, edges: [.top, .bottom])
    }

    private var totalHeight: CGFloat {
        viewModel.calculateTotalHeight()
    }

    private var totalWidth: CGFloat {
        let resultWidth: CGFloat = 700
        let previewWidth: CGFloat = 500
        var width = resultWidth
        if viewModel.isShowPreviewPanel {
            width += previewWidth + 1
        }
        return width
    }
}

struct InputAreaView: View {
    @ObservedObject var viewModel: WoxViewModel
    
    var body: some View {
        HStack(alignment: .center, spacing: 12) {
            Group {
                if viewModel.isLoading {
                    ProgressView()
                        .scaleEffect(0.7)
                        .frame(width: 20, height: 20)
                } else if let icon = viewModel.queryIcon.icon {
                    WoxIconView(icon: icon, size: 20)
                        .onTapGesture {
                            viewModel.queryIcon.action?()
                        }
                }
            }
            .frame(width: 24, height: 24)

            InputView(
                text: $viewModel.query,
                textColor: Color(hex: viewModel.theme.queryBoxFontColor),
                cursorColor: Color(hex: viewModel.theme.queryBoxCursorColor),
                selectionColor: Color(hex: viewModel.theme.queryBoxTextSelectionBackgroundColor),
                onArrowUp: { viewModel.moveSelection(offset: -1) },
                onArrowDown: { viewModel.moveSelection(offset: 1) },
                onArrowLeft: {
                    if viewModel.isGridLayout {
                        viewModel.resultGridController.updateActiveIndexByDirection(.left)
                        viewModel.selectedResultId = viewModel.results[viewModel.resultGridController.activeIndex].id
                    }
                },
                onArrowRight: {
                    if viewModel.isGridLayout {
                        viewModel.resultGridController.updateActiveIndexByDirection(.right)
                        viewModel.selectedResultId = viewModel.results[viewModel.resultGridController.activeIndex].id
                    }
                },
                onEnter: { executeSelected() },
                onCmdJ: { viewModel.toggleActionPanel() },
                onEsc: { if viewModel.isShowActionPanel { viewModel.isShowActionPanel = false } },
                onTab: { viewModel.autoCompleteQuery() },
                onHotkey: { modifiers, key in viewModel.handleKeyboardEvent(modifiers: modifiers, key: key) },
                onCmdKeyDown: { viewModel.startQuickSelectTimer() },
                onCmdKeyUp: { viewModel.stopQuickSelectTimer() },
                onQuickSelectNumber: { number in viewModel.handleQuickSelectNumber(number) }
            )
            .frame(height: 34.0 + CGFloat(viewModel.queryBoxLineCount - 1) * 34.0)
        }
        .padding(.leading, viewModel.theme.appPaddingLeft + 16)
        .padding(.trailing, viewModel.theme.appPaddingRight + 16)
        .padding(.top, viewModel.theme.appPaddingTop + QUERY_BOX_CONTENT_PADDING_TOP)
        .padding(.bottom, viewModel.theme.appPaddingBottom + QUERY_BOX_CONTENT_PADDING_BOTTOM)
        .background(Color(hex: viewModel.theme.queryBoxBackgroundColor))
        .cornerRadius(viewModel.theme.queryBoxBorderRadius)
    }
    
    private func executeSelected() {
        if viewModel.isShowActionPanel {
            if let result = viewModel.selectedResult, let actionId = viewModel.selectedActionId {
                viewModel.executeAction(result: result, actionId: actionId)
            }
        } else {
            if let id = viewModel.selectedResultId,
                let result = viewModel.results.first(where: { $0.id == id })
            {
                viewModel.executeAction(result: result)
            }
        }
    }
}

struct ResultAreaView: View {
    @ObservedObject var viewModel: WoxViewModel
    
    private let resultWidth: CGFloat = 700
    private let previewWidth: CGFloat = 500
    
    private var resultsHeight: CGFloat {
        let maxResultCount = 10
        let itemCount = viewModel.results.filter { !$0.isGroup }.count
        let count = min(itemCount, maxResultCount)
        if count == 0 { return 0 }

        let itemHeight = RESULT_ITEM_BASE_HEIGHT + viewModel.theme.resultItemPaddingTop + viewModel.theme.resultItemPaddingBottom
        return CGFloat(count) * itemHeight
    }
    
    var body: some View {
        ZStack(alignment: .bottomTrailing) {
            HStack(alignment: .top, spacing: 0) {
                if viewModel.isGridLayout {
                    GridView(viewModel: viewModel, controller: viewModel.resultGridController, maxHeight: resultsHeight)
                        .padding(.leading, viewModel.theme.resultContainerPaddingLeft)
                        .padding(.trailing, viewModel.theme.resultContainerPaddingRight)
                        .padding(.top, viewModel.theme.resultContainerPaddingTop)
                        .padding(.bottom, viewModel.theme.resultContainerPaddingBottom)
                        .frame(width: resultWidth, height: resultsHeight)
                } else {
                    ResultListView(viewModel: viewModel, resultWidth: resultWidth, resultsHeight: resultsHeight)
                }

                if viewModel.isShowPreviewPanel, let preview = viewModel.currentPreview {
                    Divider()
                        .background(Color(hex: viewModel.theme.previewSplitLineColor))

                    PreviewPanel(preview: preview, theme: viewModel.theme)
                        .frame(width: previewWidth, height: resultsHeight)
                }
            }

            if viewModel.isShowActionPanel, let result = viewModel.selectedResult,
                let actions = result.actions, !actions.isEmpty
            {
                ActionPanelView(
                    viewModel: viewModel,
                    actions: actions,
                    selectedActionId: viewModel.selectedActionId,
                    theme: viewModel.theme,
                    onActionTap: { action in
                        viewModel.selectedActionId = action.id
                        viewModel.executeAction(result: result, actionId: action.id)
                    }
                )
                .padding(.trailing, viewModel.theme.actionContainerPaddingRight + 10)
                .padding(.bottom, viewModel.theme.actionContainerPaddingBottom + 10)
            }

            if viewModel.isShowFormActionPanel, let action = viewModel.activeFormAction {
                FormActionPanelView(
                    action: action,
                    values: Binding(
                        get: { viewModel.formActionValues.compactMapValues { $0 as? String } },
                        set: { viewModel.formActionValues = $0 }
                    ),
                    theme: viewModel.theme,
                    onSave: { values in viewModel.submitFormAction(values: values) },
                    onCancel: { viewModel.hideFormActionPanel() }
                )
                .padding(.trailing, viewModel.theme.actionContainerPaddingRight + 10)
                .padding(.bottom, viewModel.theme.actionContainerPaddingBottom + 10)
            }
        }
    }
}

struct ToolbarAreaView: View {
    @ObservedObject var viewModel: WoxViewModel
    
    var body: some View {
        ToolbarView(viewModel: viewModel, info: viewModel.toolbarInfo, theme: viewModel.theme)
            .frame(height: TOOLBAR_HEIGHT)
    }
}

struct ToolbarView: View {
    @ObservedObject var viewModel: WoxViewModel
    let info: ToolbarInfo
    let theme: WoxTheme
    @State private var isCopied = false

    var body: some View {
        HStack(spacing: 12) {
            if let icon = info.icon {
                WoxIconView(icon: WoxIcon(imageType: "emoji", imageData: icon), size: 14)
            }
            if let text = info.text {
                HStack(spacing: 8) {
                    Text(text)
                        .font(.system(size: 12))
                        .foregroundColor(Color(hex: theme.toolbarFontColor))
                        .lineLimit(1)
                    
                    HStack(spacing: 4) {
                        Button(action: {
                            copyToClipboard(text)
                        }) {
                            Text(isCopied ? "已复制" : "复制")
                                .font(.system(size: 10))
                                .foregroundColor(Color(hex: theme.toolbarFontColor).opacity(0.7))
                                .underline()
                        }
                        .buttonStyle(.plain)
                        
                        Menu {
                            Button("3天") { viewModel.toolbarSnooze(text: text, duration: "3d") }
                            Button("7天") { viewModel.toolbarSnooze(text: text, duration: "7d") }
                            Button("1个月") { viewModel.toolbarSnooze(text: text, duration: "1m") }
                            Button("永久") { viewModel.toolbarSnooze(text: text, duration: "forever") }
                        } label: {
                            Text("稍后")
                                .font(.system(size: 10))
                                .foregroundColor(Color(hex: theme.toolbarFontColor).opacity(0.7))
                                .underline()
                        }
                    }
                }
            }

            Spacer()
            
            if let actions = info.actions {
                HStack(spacing: 16) {
                    ForEach(actions, id: \.name) { action in
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
        .padding(.leading, theme.toolbarPaddingLeft)
        .padding(.trailing, theme.toolbarPaddingRight)
        .background(Color(hex: theme.toolbarBackgroundColor))
    }
    
    private func copyToClipboard(_ text: String) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(text, forType: .string)
        
        isCopied = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.0) {
            isCopied = false
        }
    }
}

struct ResultListView: View {
    @ObservedObject var viewModel: WoxViewModel
    let resultWidth: CGFloat
    let resultsHeight: CGFloat
    
    var body: some View {
        ScrollViewReader { proxy in
            ScrollView(.vertical, showsIndicators: false) {
                VStack(spacing: 0) {
                    ForEach(
                        Array(viewModel.results.enumerated()), id: \.element.id
                    ) { index, result in
                        ResultRow(
                            result: result,
                            isSelected: viewModel.selectedResultId == result.id,
                            isHovered: viewModel.resultListController.hoveredIndex == index,
                            theme: viewModel.theme,
                            quickSelectNumber: quickSelectNumber(for: result, at: index)
                        )
                        .id(result.id)
                        .contentShape(Rectangle())
                        .onTapGesture {
                            if !result.isGroup {
                                viewModel.selectedResultId = result.id
                                viewModel.executeAction(result: result)
                            }
                        }
                        .onHover { hovering in
                            if hovering && !result.isGroup {
                                viewModel.resultListController.hoveredIndex = index
                            } else if viewModel.resultListController.hoveredIndex == index {
                                viewModel.resultListController.hoveredIndex = -1
                            }
                        }
                    }
                }
            }
            .padding(.leading, viewModel.theme.resultContainerPaddingLeft)
            .padding(.trailing, viewModel.theme.resultContainerPaddingRight)
            .padding(.top, viewModel.theme.resultContainerPaddingTop)
            .padding(.bottom, viewModel.theme.resultContainerPaddingBottom)
            .frame(width: resultWidth, height: resultsHeight)
            .onChange(of: viewModel.selectedResultId) { id in
                if let id = id {
                    proxy.scrollTo(id, anchor: .center)
                }
            }
        }
    }
    
    private func quickSelectNumber(for result: WoxQueryResult, at index: Int) -> Int? {
        guard viewModel.isQuickSelectMode, !result.isGroup else { return nil }

        var visibleIndex = 0
        for i in 0..<index {
            if !viewModel.results[i].isGroup {
                visibleIndex += 1
            }
        }

        guard visibleIndex < 9 else { return nil }
        return visibleIndex + 1
    }
}

struct HotkeyBadge: View {
    let hotkey: String
    let theme: WoxTheme

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
            case "enter", "return": return "↩"
            case "tab": return "⇥"
            case "esc", "escape": return "⎋"
            case "space": return "␣"
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
            .background(Color(hex: theme.toolbarBackgroundColor))
            .overlay(
                RoundedRectangle(cornerRadius: 4)
                    .stroke(Color(hex: theme.toolbarFontColor).opacity(0.3), lineWidth: 1)
            )
            .cornerRadius(4)
    }
}

struct PreviewPanel: View {
    let preview: WoxPreview
    let theme: WoxTheme

    var body: some View {
        Group {
            switch preview.previewType {
            case "markdown", "text":
                TextView(text: preview.previewData, theme: theme)
                    .padding(16)

            case "image":
                AsyncImage(url: URL(string: preview.previewData)) { image in
                    image.resizable().aspectRatio(contentMode: .fit)
                } placeholder: {
                    ProgressView()
                }
                .padding(16)

            case "url":
                WebView(url: URL(string: preview.previewData))

            default:
                TextView(text: preview.previewData, theme: theme)
                    .padding(16)
            }
        }
        .background(Color(hex: theme.appBackgroundColor).opacity(0.1))
    }
}

struct ActionRow: View {
    let action: WoxResultAction
    let isSelected: Bool
    let isHovered: Bool
    let theme: WoxTheme

    var body: some View {
        HStack(spacing: 10) {
            if let icon = action.icon {
                WoxIconView(icon: icon, size: 20)
            }

            Text(action.name)
                .font(.system(size: 14))
                .foregroundColor(
                    isSelected
                        ? Color(hex: theme.actionItemActiveFontColor)
                        : Color(hex: theme.actionItemFontColor))

            Spacer()

            if !action.hotkey.isEmpty {
                HotkeyBadge(hotkey: action.hotkey, theme: theme)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(
            RoundedRectangle(cornerRadius: 6)
                .fill(backgroundColor)
        )
    }
    
    private var backgroundColor: Color {
        if isSelected {
            return Color(hex: theme.actionItemActiveBackgroundColor)
        } else if isHovered {
            return Color(hex: theme.actionItemActiveBackgroundColor).opacity(0.3)
        }
        return .clear
    }
}

struct ActionPanelView: View {
    @ObservedObject var viewModel: WoxViewModel
    let actions: [WoxResultAction]
    let selectedActionId: String?
    let theme: WoxTheme
    let onActionTap: (WoxResultAction) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("操作")
                .font(.system(size: 16))
                .foregroundColor(Color(hex: theme.actionContainerHeaderFontColor))

            Divider()
                .background(Color(hex: theme.previewSplitLineColor))

            ScrollViewReader { proxy in
                ScrollView(.vertical, showsIndicators: false) {
                    VStack(spacing: 2) {
                        let filteredItems = viewModel.actionListController.items
                        ForEach(Array(filteredItems.enumerated()), id: \.element.id) { index, item in
                            ActionRow(
                                action: item.data,
                                isSelected: viewModel.selectedActionId == item.id,
                                isHovered: viewModel.actionListController.hoveredIndex == index,
                                theme: theme
                            )
                            .id(item.id)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                onActionTap(item.data)
                            }
                            .onHover { hovering in
                                if hovering {
                                    viewModel.actionListController.hoveredIndex = index
                                } else if viewModel.actionListController.hoveredIndex == index {
                                    viewModel.actionListController.hoveredIndex = -1
                                }
                            }
                        }
                    }
                }
                .frame(maxHeight: 320)
                .onChange(of: viewModel.selectedActionId) { id in
                    if let id = id {
                        proxy.scrollTo(id, anchor: .center)
                    }
                }
            }
            
            TextField("", text: Binding(
                get: { viewModel.actionListController.filterText },
                set: { viewModel.actionListController.filterItems($0) }
            ))
            .textFieldStyle(.plain)
            .font(.system(size: 14))
            .foregroundColor(Color(hex: theme.actionQueryBoxFontColor))
            .padding(.horizontal, 8)
            .padding(.vertical, 10)
            .background(Color(hex: theme.actionQueryBoxBackgroundColor))
            .cornerRadius(theme.actionQueryBoxBorderRadius)
        }
        .padding(.leading, theme.actionContainerPaddingLeft)
        .padding(.top, theme.actionContainerPaddingTop)
        .padding(.trailing, theme.actionContainerPaddingRight)
        .padding(.bottom, theme.actionContainerPaddingBottom)
        .frame(width: 320)
        .background(Color(hex: theme.actionContainerBackgroundColor))
        .background(VisualEffectView(material: .hudWindow, blendingMode: .behindWindow))
        .cornerRadius(theme.actionQueryBoxBorderRadius)
        .shadow(color: Color.black.opacity(0.1), radius: 8, x: 0, y: 3)
        .overlay(
            RoundedRectangle(cornerRadius: theme.actionQueryBoxBorderRadius)
                .stroke(Color.white.opacity(0.15), lineWidth: 0.5)
        )
    }
}

struct TextView: NSViewRepresentable {
    let text: String
    let theme: WoxTheme

    func makeNSView(context: Context) -> NSScrollView {
        let scrollView = NSScrollView()
        let textView = NSTextView()
        textView.isEditable = false
        textView.isSelectable = true
        textView.backgroundColor = .clear
        textView.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        textView.textColor = NSColor(Color(hex: theme.previewFontColor))
        textView.autoresizingMask = [.width]

        scrollView.documentView = textView
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = false

        return scrollView
    }

    func updateNSView(_ nsView: NSScrollView, context: Context) {
        if let textView = nsView.documentView as? NSTextView {
            textView.string = text
            textView.textColor = NSColor(Color(hex: theme.previewFontColor))
        }
    }
}

struct WebView: NSViewRepresentable {
    let url: URL?

    func makeNSView(context: Context) -> WKWebView {
        return WKWebView()
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        if let url = url {
            let request = URLRequest(url: url)
            nsView.load(request)
        }
    }
}

class WoxWindowController {
    weak var window: NSWindow?

    init(window: NSWindow?) {
        self.window = window
    }

    func configureWindow() {
    }
    
    func show() {
        guard let window = window else { return }
        window.collectionBehavior.insert(.canJoinAllSpaces)
        window.collectionBehavior.insert(.fullScreenAuxiliary)
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
    
    func hide() {
        guard let window = window else { return }
        window.orderOut(nil)
        NSApp.hide(nil)
    }
    
    func toggle() {
        guard let window = window else { return }
        if window.isVisible {
            hide()
        } else {
            show()
        }
    }
    
    func setAlwaysOnTop(_ top: Bool) {
        guard let window = window else { return }
        window.level = top ? .popUpMenu : .normal
    }
    
    func center(width: CGFloat?, height: CGFloat?) {
        if let w = width, let h = height {
            resizeWindow(to: h, width: w)
        }
        centerWindow()
    }
    
    func centerWindow() {
        guard let window = window else { return }
        let mouseLocation = NSEvent.mouseLocation
        var targetScreen: NSScreen? = nil
        for screen in NSScreen.screens {
            if screen.frame.contains(mouseLocation) {
                targetScreen = screen
                break
            }
        }
        let screenFrame = targetScreen?.frame ?? NSScreen.main?.frame ?? NSRect.zero
        let windowWidth = window.frame.width
        let windowHeight = window.frame.height
        let x = (screenFrame.width - windowWidth) / 2 + screenFrame.minX
        let y = (screenFrame.height - windowHeight) / 2 + screenFrame.minY
        window.setFrame(NSRect(x: x, y: y, width: windowWidth, height: windowHeight), display: true)
    }
    
    func setPosition(x: CGFloat, y: CGFloat) {
        guard let window = window else { return }
        let targetScreen = NSScreen.screens.first { screen in
            let frame = screen.frame
            return x >= frame.origin.x && x < frame.origin.x + frame.width
        } ?? window.screen ?? NSScreen.main
        let frame = targetScreen?.frame ?? NSRect.zero
        let screenTopInAppKit = frame.origin.y + frame.height
        let windowTopInAppKit = screenTopInAppKit - y
        let flippedY = windowTopInAppKit - window.frame.height
        window.setFrameOrigin(NSPoint(x: x, y: flippedY))
    }

    func resizeWindow(to height: CGFloat, width: CGFloat) {
        guard let window = window else { return }
        var frame = window.frame
        let oldHeight = frame.height
        let oldWidth = frame.width
        let heightDiff = height - oldHeight
        let widthDiff = width - oldWidth
        if heightDiff == 0 && widthDiff == 0 { return }
        frame.size.height = height
        frame.size.width = width
        frame.origin.y -= heightDiff
        frame.origin.x -= widthDiff / 2
        window.setFrame(frame, display: true)
    }
}

struct WindowAccessor: NSViewRepresentable {
    var callback: (NSWindow?) -> Void
    func makeNSView(context: Context) -> NSView {
        let view = NSView()
        DispatchQueue.main.async { self.callback(view.window) }
        return view
    }
    func updateNSView(_ nsView: NSView, context: Context) {}
}

struct VisualEffectView: NSViewRepresentable {
    let material: NSVisualEffectView.Material
    let blendingMode: NSVisualEffectView.BlendingMode
    func makeNSView(context: Context) -> NSVisualEffectView {
        let visualEffectView = NSVisualEffectView()
        visualEffectView.material = material
        visualEffectView.blendingMode = blendingMode
        visualEffectView.state = .active
        return visualEffectView
    }
    func updateNSView(_ visualEffectView: NSVisualEffectView, context: Context) {
        visualEffectView.material = material
        visualEffectView.blendingMode = blendingMode
    }
}

struct InputView: NSViewRepresentable {
    @Binding var text: String
    var textColor: Color
    var cursorColor: Color
    var selectionColor: Color
    var onArrowUp: () -> Void
    var onArrowDown: () -> Void
    var onArrowLeft: () -> Void
    var onArrowRight: () -> Void
    var onEnter: () -> Void
    var onCmdJ: () -> Void
    var onEsc: () -> Void
    var onTab: (() -> Void)?
    var onHotkey: ((NSEvent.ModifierFlags, String) -> Bool)?
    var onCmdKeyDown: (() -> Void)?
    var onCmdKeyUp: (() -> Void)?
    var onQuickSelectNumber: ((Int) -> Bool)?

    func makeNSView(context: Context) -> NSTextField {
        let textField = CustomNSTextField()
        textField.delegate = context.coordinator
        textField.focusRingType = .none
        textField.isBordered = false
        textField.drawsBackground = false
        textField.font = NSFont.systemFont(ofSize: 28, weight: .light)
        textField.placeholderString = "Wox..."
        textField.textColor = NSColor(textColor)
        textField.onArrowUp = onArrowUp
        textField.onArrowDown = onArrowDown
        textField.onArrowLeft = onArrowLeft
        textField.onArrowRight = onArrowRight
        textField.onEnter = onEnter
        textField.onCmdJ = onCmdJ
        textField.onEsc = onEsc
        textField.onTab = onTab
        textField.onHotkey = onHotkey
        textField.onCmdKeyDown = onCmdKeyDown
        textField.onCmdKeyUp = onCmdKeyUp
        textField.onQuickSelectNumber = onQuickSelectNumber
        return textField
    }

    func updateNSView(_ nsView: NSTextField, context: Context) {
        if nsView.stringValue != text { nsView.stringValue = text }
        nsView.textColor = NSColor(textColor)
    }

    func makeCoordinator() -> Coordinator { Coordinator(text: $text) }

    class Coordinator: NSObject, NSTextFieldDelegate {
        @Binding var text: String
        init(text: Binding<String>) { _text = text }
        func controlTextDidChange(_ obj: Notification) {
            guard let textField = obj.object as? NSTextField else { return }
            text = textField.stringValue
        }
    }
}

class CustomNSTextField: NSTextField {
    var onArrowUp: (() -> Void)?
    var onArrowDown: (() -> Void)?
    var onArrowLeft: (() -> Void)?
    var onArrowRight: (() -> Void)?
    var onEnter: (() -> Void)?
    var onCmdJ: (() -> Void)?
    var onEsc: (() -> Void)?
    var onTab: (() -> Void)?
    var onHotkey: ((NSEvent.ModifierFlags, String) -> Bool)?
    var onCmdKeyDown: (() -> Void)?
    var onCmdKeyUp: (() -> Void)?
    var onQuickSelectNumber: ((Int) -> Bool)?

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if event.keyCode == 48 { onTab?(); return true }
        if event.modifierFlags.contains(.command) {
            let chars = event.charactersIgnoringModifiers?.lowercased() ?? ""
            if let number = Int(chars), number >= 1, number <= 9 {
                if let onQuickSelectNumber = onQuickSelectNumber, onQuickSelectNumber(number) { return true }
            }
            if chars == "a" { return super.performKeyEquivalent(with: event) }
            if chars == "j" { onCmdJ?(); return true }
            if let onHotkey = onHotkey, !chars.isEmpty { if onHotkey(event.modifierFlags, chars) { return true } }
        }
        if event.modifierFlags.contains(.option) || event.modifierFlags.contains(.control) {
            let chars = event.charactersIgnoringModifiers?.lowercased() ?? ""
            if let onHotkey = onHotkey, !chars.isEmpty { if onHotkey(event.modifierFlags, chars) { return true } }
        }
        if event.keyCode == 53 { onEsc?(); return true }
        switch event.specialKey {
        case .upArrow: onArrowUp?(); return true
        case .downArrow: onArrowDown?(); return true
        case .leftArrow: onArrowLeft?(); return true
        case .rightArrow: onArrowRight?(); return true
        case .enter, .carriageReturn: onEnter?(); return true
        default: return super.performKeyEquivalent(with: event)
        }
    }

    override func flagsChanged(with event: NSEvent) {
        if event.modifierFlags.contains(.command) { onCmdKeyDown?() } else { onCmdKeyUp?() }
        super.flagsChanged(with: event)
    }
}
