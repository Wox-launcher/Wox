import SwiftUI
import AppKit
import WebKit

struct ContentView: View {
    @ObservedObject var viewModel: WoxViewModel
    @State private var windowController: WoxWindowController?
    
    private let inputHeight: CGFloat = 64
    private let resultRowHeight: CGFloat = 52
    private let toolbarHeight: CGFloat = 32
    private let maxVisibleResults = 8
    
    private let resultWidth: CGFloat = 700
    private let previewWidth: CGFloat = 500
    
    var body: some View {
        VStack(spacing: 0) {
            // Input Area
            HStack(alignment: .center, spacing: 12) {
                 Image(systemName: "magnifyingglass")
                    .font(.system(size: 20, weight: .light))
                    .foregroundColor(.secondary)
                
                InputView(text: $viewModel.query, onArrowUp: {
                    moveSelection(offset: -1)
                }, onArrowDown: {
                    moveSelection(offset: 1)
                }, onEnter: {
                    executeSelected()
                }, onCmdJ: {
                    viewModel.toggleActionPanel()
                }, onEsc: {
                    if viewModel.isShowActionPanel {
                        viewModel.isShowActionPanel = false
                    }
                })
                .frame(height: 32)
            }
            .padding(16)
            
            if !viewModel.results.isEmpty || (viewModel.isShowActionPanel && viewModel.selectedResult != nil) {
                Divider()
                    .background(Color.white.opacity(0.1))
                
                HStack(alignment: .top, spacing: 0) {
                    // Left Column: Results or Actions
                    ScrollViewReader { proxy in
                        ScrollView(.vertical, showsIndicators: false) {
                            VStack(spacing: 0) {
                                if viewModel.isShowActionPanel, let result = viewModel.selectedResult {
                                    ForEach(result.actions ?? []) { action in
                                        ActionRow(action: action, isSelected: viewModel.selectedActionId == action.id)
                                            .id(action.id)
                                            .contentShape(Rectangle())
                                            .onTapGesture {
                                                viewModel.selectedActionId = action.id
                                                viewModel.executeAction(result: result, actionId: action.id)
                                            }
                                    }
                                } else {
                                    ForEach(viewModel.results) { result in
                                        ResultRow(result: result, isSelected: viewModel.selectedResultId == result.id)
                                            .id(result.id)
                                            .contentShape(Rectangle())
                                            .onTapGesture {
                                                viewModel.selectedResultId = result.id
                                                viewModel.executeAction(result: result)
                                            }
                                    }
                                }
                            }
                        }
                        .frame(width: resultWidth, height: resultsHeight)
                        .onChange(of: viewModel.selectedResultId) { id in
                            if let id = id, !viewModel.isShowActionPanel {
                                withAnimation(.easeInOut(duration: 0.1)) {
                                    proxy.scrollTo(id, anchor: .center)
                                }
                            }
                        }
                        .onChange(of: viewModel.selectedActionId) { id in
                            if let id = id, viewModel.isShowActionPanel {
                                withAnimation(.easeInOut(duration: 0.1)) {
                                    proxy.scrollTo(id, anchor: .center)
                                }
                            }
                        }
                    }
                    
                    // Right Column: Preview
                    if viewModel.isShowPreviewPanel, let preview = viewModel.currentPreview {
                        Divider()
                            .background(Color.white.opacity(0.1))
                        
                        PreviewPanel(preview: preview)
                            .frame(width: previewWidth, height: resultsHeight)
                    }
                }
            }
            
            // Bottom Toolbar
            if showToolbar {
                Divider()
                    .background(Color.white.opacity(0.1))
                
                ToolbarView(info: viewModel.toolbarInfo)
                    .frame(height: toolbarHeight)
            }
        }
        .frame(width: totalWidth, height: totalHeight)
        .background(VisualEffectView(material: .hudWindow, blendingMode: .behindWindow))
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
    
    private var showToolbar: Bool {
        !viewModel.results.isEmpty || viewModel.toolbarInfo.text != nil
    }
    
    private var resultsHeight: CGFloat {
        if viewModel.isShowActionPanel {
            let count = min(viewModel.selectedResult?.actions?.count ?? 0, maxVisibleResults)
            return CGFloat(count) * resultRowHeight
        } else {
            let count = min(viewModel.results.count, maxVisibleResults)
            return CGFloat(count) * resultRowHeight
        }
    }
    
    private var totalHeight: CGFloat {
        var height = inputHeight
        if !viewModel.results.isEmpty || (viewModel.isShowActionPanel && viewModel.selectedResult != nil) {
            height += 1 + resultsHeight
        }
        if showToolbar {
            height += 1 + toolbarHeight
        }
        return height
    }
    
    private var totalWidth: CGFloat {
        var width = resultWidth
        if viewModel.isShowPreviewPanel {
            width += previewWidth + 1
        }
        return width
    }
    
    private func moveSelection(offset: Int) {
        if viewModel.isShowActionPanel {
            guard let actions = viewModel.selectedResult?.actions, !actions.isEmpty else { return }
            let ids = actions.map { $0.id }
            if let currentId = viewModel.selectedActionId, let index = ids.firstIndex(of: currentId) {
                let newIndex = max(0, min(ids.count - 1, index + offset))
                viewModel.selectedActionId = ids[newIndex]
            } else {
                viewModel.selectedActionId = actions.first?.id
            }
        } else {
            guard !viewModel.results.isEmpty else { return }
            let ids = viewModel.results.map { $0.id }
            if let currentId = viewModel.selectedResultId, let index = ids.firstIndex(of: currentId) {
                let newIndex = max(0, min(ids.count - 1, index + offset))
                viewModel.selectedResultId = ids[newIndex]
            } else {
                viewModel.selectedResultId = viewModel.results.first?.id
            }
        }
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
    
    var body: some View {
        HStack(spacing: 12) {
            if let icon = info.icon {
                WoxIconView(icon: WoxIcon(imageType: "emoji", imageData: icon), size: 14)
            }
            if let text = info.text {
                Text(text)
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
            }
            
            Spacer()
            
            if let actions = info.actions {
                HStack(spacing: 16) {
                    ForEach(actions, id: \.name) { action in
                        HStack(spacing: 4) {
                            Text(action.name)
                                .font(.system(size: 11))
                                .foregroundColor(.secondary)
                            
                            HotkeyBadge(hotkey: action.hotkey)
                        }
                    }
                }
            }
        }
        .padding(.horizontal, 16)
        .background(Color.black.opacity(0.05))
    }
}

struct HotkeyBadge: View {
    let hotkey: String
    
    var body: some View {
        Text(hotkey.uppercased())
            .font(.system(size: 9, weight: .bold, design: .monospaced))
            .padding(.horizontal, 4)
            .padding(.vertical, 2)
            .background(Color.secondary.opacity(0.15))
            .cornerRadius(4)
            .foregroundColor(.secondary)
            .overlay(
                RoundedRectangle(cornerRadius: 4)
                    .stroke(Color.white.opacity(0.1), lineWidth: 0.5)
            )
    }
}

struct PreviewPanel: View {
    let preview: WoxPreview
    
    var body: some View {
        Group {
            switch preview.previewType {
            case "markdown", "text":
                TextView(text: preview.previewData)
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
                TextView(text: preview.previewData)
                    .padding(16)
            }
        }
        .background(Color.black.opacity(0.1))
    }
}

struct TextView: NSViewRepresentable {
    let text: String
    
    func makeNSView(context: Context) -> NSScrollView {
        let scrollView = NSScrollView()
        let textView = NSTextView()
        textView.isEditable = false
        textView.isSelectable = true
        textView.backgroundColor = .clear
        textView.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        textView.textColor = .labelColor
        textView.autoresizingMask = [.width]
        
        scrollView.documentView = textView
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = false
        
        return scrollView
    }
    
    func updateNSView(_ nsView: NSScrollView, context: Context) {
        if let textView = nsView.documentView as? NSTextView {
            textView.string = text
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

struct ActionRow: View {
    let action: WoxResultAction
    let isSelected: Bool
    
    var body: some View {
        HStack {
            Text(action.name)
                .font(.system(size: 16))
                .foregroundColor(isSelected ? .white : .primary)
            Spacer()
            HotkeyBadge(hotkey: action.hotkey)
                .opacity(isSelected ? 0.8 : 1.0)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .background(isSelected ? Color.accentColor : Color.clear)
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

// MARK: - Window Accessor

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

// MARK: - Visual Effect View

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
    var onArrowUp: () -> Void
    var onArrowDown: () -> Void
    var onEnter: () -> Void
    var onCmdJ: () -> Void
    var onEsc: () -> Void
    
    func makeNSView(context: Context) -> NSTextField {
        let textField = CustomNSTextField()
        textField.delegate = context.coordinator
        textField.focusRingType = .none
        textField.isBordered = false
        textField.drawsBackground = false
        textField.font = NSFont.systemFont(ofSize: 26, weight: .light)
        textField.placeholderString = "Wox..."
        
        textField.onArrowUp = onArrowUp
        textField.onArrowDown = onArrowDown
        textField.onEnter = onEnter
        textField.onCmdJ = onCmdJ
        textField.onEsc = onEsc
        
        return textField
    }
    
    func updateNSView(_ nsView: NSTextField, context: Context) {
        if nsView.stringValue != text {
            nsView.stringValue = text
        }
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
    
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        if event.modifierFlags.contains(.command) {
            let chars = event.charactersIgnoringModifiers?.lowercased()
            if chars == "a" {
                 return super.performKeyEquivalent(with: event)
            }
            if chars == "j" {
                onCmdJ?()
                return true
            }
        }
        
        if event.keyCode == 53 { // Escape
            onEsc?()
            // We return false here to let the system handle escape (which might dismiss the app)
            // unless the active panel was closed. But for simple logic, we just return super.
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
