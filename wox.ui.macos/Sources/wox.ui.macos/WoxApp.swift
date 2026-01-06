import SwiftUI
import AppKit

class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory) // Hide dock icon
    }
}

@main
struct WoxApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @StateObject var viewModel: WoxViewModel
    
    init() {
        let args = CommandLine.arguments
        var port = 34987
        if args.count >= 2, let p = Int(args[1]) {
            port = p
        }
        _viewModel = StateObject(wrappedValue: WoxViewModel(port: port))
    }
    
    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
                .onAppear {
                    viewModel.connect()
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
                        viewModel.onUIReady()
                    }
                }
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
    }
}
