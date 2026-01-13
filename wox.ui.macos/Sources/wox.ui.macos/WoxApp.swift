import AppKit
import SwiftUI

class KeyablePanel: NSPanel {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }
    
    override init(contentRect: NSRect, styleMask style: NSWindow.StyleMask, backing backingStoreType: NSWindow.BackingStoreType, defer flag: Bool) {
        super.init(contentRect: contentRect, styleMask: style, backing: backingStoreType, defer: flag)
        self.becomesKeyOnlyIfNeeded = false
    }
}

@main
class AppDelegate: NSObject, NSApplicationDelegate {
    var panel: NSPanel!
    var viewModel: WoxViewModel!

    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.run()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)

        let args = CommandLine.arguments
        var port = 34987
        if args.count >= 2, let p = Int(args[1]) {
            port = p
        }
        viewModel = WoxViewModel(port: port)

        panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 700, height: 60),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )

        panel.level = .popUpMenu
        panel.collectionBehavior = [
            .canJoinAllSpaces,
            .fullScreenAuxiliary,
            .transient,
            .ignoresCycle
        ]
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = true
        panel.isMovableByWindowBackground = true
        panel.titlebarAppearsTransparent = true
        panel.titleVisibility = .hidden
        panel.appearance = NSAppearance(named: .aqua)

        let windowController = WoxWindowController(window: panel)
        viewModel.windowActionShow = { [weak windowController] in windowController?.show() }
        viewModel.windowActionHide = { [weak windowController] in windowController?.hide() }
        viewModel.windowActionToggle = { [weak windowController] in windowController?.toggle() }
        viewModel.windowActionSetSize = { [weak windowController] w, h in windowController?.resizeWindow(to: h, width: w) }
        viewModel.windowActionSetPosition = { [weak windowController] x, y in windowController?.setPosition(x: x, y: y) }
        viewModel.windowActionCenter = { [weak windowController] w, h in windowController?.center(width: w, height: h) }
        viewModel.windowActionSetAlwaysOnTop = { [weak windowController] val in windowController?.setAlwaysOnTop(val) }

        let contentView = ContentView(viewModel: viewModel, windowController: windowController)
        let hostingView = NSHostingView(rootView: contentView)
        panel.contentView = hostingView

        viewModel.connect()
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
            self.viewModel.onUIReady()
        }

        panel.center()
        panel.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}
