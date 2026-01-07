
import SwiftUI
import AppKit
import WebKit

struct ContentView: View {
    @ObservedObject var viewModel: WoxViewModel
    @State private var windowController: WoxWindowController?
    
    var body: some View {
        VStack(spacing: 0) {
            // Input Area
            HStack(alignment: .center, spacing: 12) {
                 Image(systemName: "magnifyingglass")
                    .font(.system(size: 20, weight: .light))
                    .foregroundColor(Color(hex: viewModel.theme.queryBoxFontColor).opacity(0.6))
                
                InputView(
                    text: $viewModel.query,
                    textColor: Color(hex: viewModel.theme.queryBoxFontColor),
                    cursorColor: Color(hex: viewModel.theme.queryBoxCursorColor),
                    selectionColor: Color(hex: viewModel.theme.queryBoxTextSelectionBackgroundColor),
                    onArrowUp: {
                        viewModel.moveSelection(offset: -1)
                    }, onArrowDown: {
                        viewModel.moveSelection(offset: 1)
                    }, onEnter: {
                        executeSelected()
                    }, onCmdJ: {
                        viewModel.toggleActionPanel()
                    }, onEsc: {
                        if viewModel.isShowActionPanel {
                            viewModel.isShowActionPanel = false
                        }
                    }, onTab: {
                        viewModel.autoCompleteQuery()
                    }, onHotkey: { modifiers, key in
                        viewModel.handleKeyboardEvent(modifiers: modifiers, key: key)
                    })
                .frame(height: 34.0 + CGFloat(viewModel.queryBoxLineCount - 1) * 34.0)
            }
            .padding(.leading, viewModel.theme.appPaddingLeft + 16)
            .padding(.trailing, viewModel.theme.appPaddingRight + 16)
            .padding(.top, viewModel.theme.appPaddingTop + QUERY_BOX_CONTENT_PADDING_TOP)
            .padding(.bottom, viewModel.theme.appPaddingBottom + QUERY_BOX_CONTENT_PADDING_BOTTOM)
            .background(Color(hex: viewModel.theme.queryBoxBackgroundColor))
            .cornerRadius(viewModel.theme.queryBoxBorderRadius)
            
            if !viewModel.results.isEmpty {
                Divider()
                    .background(Color(hex: viewModel.theme.previewSplitLineColor))
                
                ZStack(alignment: .bottomTrailing) {
                    HStack(alignment: .top, spacing: 0) {
                        // Left Column: Results
                        ScrollViewReader { proxy in
                            ScrollView(.vertical, showsIndicators: false) {
                                VStack(spacing: 0) {
                                    ForEach(viewModel.results) { result in
                                        ResultRow(
                                            result: result,
                                            isSelected: viewModel.selectedResultId == result.id,
                                            theme: viewModel.theme
                                        )
                                        .id(result.id)
                                        .contentShape(Rectangle())
                                        .onTapGesture {
                                            if !result.isGroup {
                                                viewModel.selectedResultId = result.id
                                                viewModel.executeAction(result: result)
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
                                    withAnimation(.easeInOut(duration: 0.1)) {
                                        proxy.scrollTo(id, anchor: .center)
                                    }
                                }
                            }
                        }
                        
                        // Right Column: Preview
                        if viewModel.isShowPreviewPanel, let preview = viewModel.currentPreview {
                            Divider()
                                .background(Color(hex: viewModel.theme.previewSplitLineColor))
                            
                            PreviewPanel(preview: preview, theme: viewModel.theme)
                                .frame(width: previewWidth, height: resultsHeight)
                        }
                    }
                    
                    // Action Panel
                    if viewModel.isShowActionPanel, let result = viewModel.selectedResult, let actions = result.actions, !actions.isEmpty {
                        ActionPanelView(
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
                    
                    // Form Action Panel
                    if viewModel.isShowFormActionPanel, let action = viewModel.activeFormAction {
                        FormActionPanelView(
                            action: action,
                            values: Binding(
                                get: { viewModel.formActionValues.compactMapValues { $0 as? String } },
                                set: { viewModel.formActionValues = $0 }
                            ),
                            theme: viewModel.theme,
                            onSave: { values in
                                viewModel.submitFormAction(values: values)
                            },
                            onCancel: {
                                viewModel.hideFormActionPanel()
                            }
                        )
                        .padding(.trailing, viewModel.theme.actionContainerPaddingRight + 10)
                        .padding(.bottom, viewModel.theme.actionContainerPaddingBottom + 10)
                    }
                }
            }
            
            // Bottom Toolbar
            if viewModel.isShowToolbar {
                Divider()
                    .background(Color(hex: viewModel.theme.previewSplitLineColor))
                
                ToolbarView(info: viewModel.toolbarInfo, theme: viewModel.theme)
                    .frame(height: TOOLBAR_HEIGHT)
            }
        }
        .frame(width: totalWidth, height: totalHeight)
        .background(VisualEffectView(material: .popover, blendingMode: .behindWindow))
        .background(Color(hex: viewModel.theme.appBackgroundColor).opacity(0.85))
        .cornerRadius(12)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Color.white.opacity(0.15), lineWidth: 0.5)
        )
        .background(WindowAccessor { window in
            self.windowController = WoxWindowController(window: window)
            self.windowController?.configureWindow()
            self.windowController?.resizeWindow(to: totalHeight, width: totalWidth)
        })
        .onChange(of: totalHeight) { _ in
            windowController?.resizeWindow(to: totalHeight, width: totalWidth)
        }
        .onChange(of: totalWidth) { _ in
            windowController?.resizeWindow(to: totalHeight, width: totalWidth)
        }
    }
    
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
    
    private var totalHeight: CGFloat {
        viewModel.calculateTotalHeight()
    }
    
    private var totalWidth: CGFloat {
        var width = resultWidth
        if viewModel.isShowPreviewPanel {
            width += previewWidth + 1
        }
        return width
    }
    
    private func executeSelected() {
        if viewModel.isShowActionPanel {
            if let result = viewModel.selectedResult, let actionId = viewModel.selectedActionId {
                viewModel.executeAction(result: result, actionId: actionId)
            }
        } else {
            if let id = viewModel.selectedResultId, let result = viewModel.results.first(where: { $0.id == id }) {
                viewModel.executeAction(result: result)
            }
        }
    }
}

// MARK: - Subviews

struct ToolbarView: View {
    let info: ToolbarInfo
    let theme: WoxTheme
    
    var body: some View {
        HStack(spacing: 12) {
            if let icon = info.icon {
                WoxIconView(icon: WoxIcon(imageType: "emoji", imageData: icon), size: 14)
            }
            if let text = info.text {
                Text(text)
                    .font(.system(size: 12))
                    .foregroundColor(Color(hex: theme.toolbarFontColor))
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
    let theme: WoxTheme
    
    var body: some View {
        HStack(spacing: 10) {
            // Icon
            if let icon = action.icon {
                WoxIconView(icon: icon, size: 20)
            }
            
            Text(action.name)
                .font(.system(size: 14))
                .foregroundColor(isSelected ? Color(hex: theme.actionItemActiveFontColor) : Color(hex: theme.actionItemFontColor))
            
            Spacer()
            
            if !action.hotkey.isEmpty {
                HotkeyBadge(hotkey: action.hotkey, theme: theme)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(
            RoundedRectangle(cornerRadius: 6)
                .fill(isSelected ? Color(hex: theme.actionItemActiveBackgroundColor) : Color.clear)
        )
    }
}

struct ActionPanelView: View {
    let actions: [WoxResultAction]
    let selectedActionId: String?
    let theme: WoxTheme
    let onActionTap: (WoxResultAction) -> Void
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("操作")
                .font(.system(size: 14, weight: .medium))
                .foregroundColor(Color(hex: theme.actionContainerHeaderFontColor))
            
            Divider()
                .background(Color(hex: theme.previewSplitLineColor))
            
            ScrollView(.vertical, showsIndicators: false) {
                VStack(spacing: 2) {
                    ForEach(actions) { action in
                        ActionRow(action: action, isSelected: selectedActionId == action.id, theme: theme)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                onActionTap(action)
                            }
                    }
                }
            }
            .frame(maxHeight: 300)
        }
        .padding(.leading, theme.actionContainerPaddingLeft)
        .padding(.top, theme.actionContainerPaddingTop)
        .padding(.trailing, theme.actionContainerPaddingRight)
        .padding(.bottom, theme.actionContainerPaddingBottom)
        .frame(width: 280)
        .background(Color(hex: theme.actionContainerBackgroundColor))
        .background(VisualEffectView(material: .hudWindow, blendingMode: .behindWindow))
        .cornerRadius(theme.actionQueryBoxBorderRadius)
        .shadow(color: Color.black.opacity(0.2), radius: 8, x: 0, y: 4)
        .overlay(
            RoundedRectangle(cornerRadius: theme.actionQueryBoxBorderRadius)
                .stroke(Color.white.opacity(0.15), lineWidth: 0.5)
        )
    }
}

// MARK: - Helper Views

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

// MARK: - Window Controller

class WoxWindowController {
    weak var window: NSWindow?
    
    init(window: NSWindow?) {
        self.window = window
    }
    
    func configureWindow() {
        guard let window = window else { return }
        
        window.styleMask = [.borderless, .fullSizeContentView]
        window.isOpaque = false
        window.backgroundColor = .clear
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.hasShadow = true
        window.isMovableByWindowBackground = true
        window.level = .floating
        window.standardWindowButton(.closeButton)?.isHidden = true
        window.standardWindowButton(.miniaturizeButton)?.isHidden = true
        window.standardWindowButton(.zoomButton)?.isHidden = true
        window.center()
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
        DispatchQueue.main.async {
            self.callback(view.window)
        }
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

// MARK: - Input View

struct InputView: NSViewRepresentable {
    @Binding var text: String
    var textColor: Color
    var cursorColor: Color
    var selectionColor: Color
    var onArrowUp: () -> Void
    var onArrowDown: () -> Void
    var onEnter: () -> Void
    var onCmdJ: () -> Void
    var onEsc: () -> Void
    var onTab: (() -> Void)?
    var onHotkey: ((NSEvent.ModifierFlags, String) -> Bool)?
    
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
        textField.onEnter = onEnter
        textField.onCmdJ = onCmdJ
        textField.onEsc = onEsc
        textField.onTab = onTab
        textField.onHotkey = onHotkey
        
        return textField
    }
    
    func updateNSView(_ nsView: NSTextField, context: Context) {
        if nsView.stringValue != text {
            nsView.stringValue = text
        }
        nsView.textColor = NSColor(textColor)
    }
    
    func makeCoordinator() -> Coordinator {
        Coordinator(text: $text)
    }
    
    class Coordinator: NSObject, NSTextFieldDelegate {
        @Binding var text: String
        
        init(text: Binding<String>) {
            _text = text
        }
        
        func controlTextDidChange(_ obj: Notification) {
            guard let textField = obj.object as? NSTextField else { return }
            text = textField.stringValue
        }
    }
}

class CustomNSTextField: NSTextField {
    var onArrowUp: (() -> Void)?
    var onArrowDown: (() -> Void)?
    var onEnter: (() -> Void)?
    var onCmdJ: (() -> Void)?
    var onEsc: (() -> Void)?
    var onTab: (() -> Void)?
    var onHotkey: ((NSEvent.ModifierFlags, String) -> Bool)?
    
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        // Handle Tab key for auto-complete
        if event.keyCode == 48 { // Tab
            onTab?()
            return true
        }
        
        if event.modifierFlags.contains(.command) {
            let chars = event.charactersIgnoringModifiers?.lowercased() ?? ""
            if chars == "a" {
                 return super.performKeyEquivalent(with: event)
            }
            if chars == "j" {
                onCmdJ?()
                return true
            }
            
            // Check for other command+key hotkeys
            if let onHotkey = onHotkey, !chars.isEmpty {
                if onHotkey(event.modifierFlags, chars) {
                    return true
                }
            }
        }
        
        // Check for other modifier combinations
        if event.modifierFlags.contains(.option) || event.modifierFlags.contains(.control) {
            let chars = event.charactersIgnoringModifiers?.lowercased() ?? ""
            if let onHotkey = onHotkey, !chars.isEmpty {
                if onHotkey(event.modifierFlags, chars) {
                    return true
                }
            }
        }
        
        if event.keyCode == 53 { // Escape
            onEsc?()
            return super.performKeyEquivalent(with: event)
        }
        
        switch event.specialKey {
        case .upArrow:
            onArrowUp?()
            return true
        case .downArrow:
            onArrowDown?()
            return true
        case .enter, .carriageReturn:
            onEnter?()
            return true
        default:
            return super.performKeyEquivalent(with: event)
        }
    }
}
