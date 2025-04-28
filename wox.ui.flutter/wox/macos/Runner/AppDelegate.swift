import Cocoa
import FlutterMacOS
import flutter_inappwebview_macos

@main
class AppDelegate: FlutterAppDelegate {
  // Store the previous active application
  private var previousActiveApp: NSRunningApplication?
  // Flutter method channel for window events
  private var windowEventChannel: FlutterMethodChannel?

  private func log(_ message: String) {
    // NSLog("WoxApp: \(message)")
  }

  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return false
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }

  /// Apply acrylic effect to window
  private func applyAcrylicEffect(to window: NSWindow) {
    // Ensure light theme is used, otherwise the dark theme effect the theme color
    window.appearance = NSAppearance(named: .aqua)

    if let contentView = window.contentView {
      let effectView = NSVisualEffectView(frame: contentView.bounds)
      effectView.material = .popover
      effectView.state = .active
      effectView.blendingMode = .behindWindow
      // Ensure the effect view resizes with the window
      effectView.autoresizingMask = [.width, .height]
      contentView.addSubview(effectView, positioned: .below, relativeTo: nil)

      // Try to make all Flutter-related views transparent
      for subview in contentView.subviews where !(subview is NSVisualEffectView) {
        subview.wantsLayer = true
        subview.layer?.backgroundColor = NSColor.clear.cgColor
      }
    }
  }

  // Setup notification for window blur event
  private func setupWindowBlurNotification() {
    guard let window = self.mainFlutterWindow else { return }

    NotificationCenter.default.addObserver(
      self,
      selector: #selector(windowDidResignKey),
      name: NSWindow.didResignKeyNotification,
      object: window
    )
  }

  // Handle window loss of focus
  @objc private func windowDidResignKey(_ notification: Notification) {
    log("Window did resign key (blur)")
    // Notify Flutter about the window blur event
    DispatchQueue.main.async {
      self.windowEventChannel?.invokeMethod("onWindowBlur", arguments: nil)
    }
  }

  override func applicationDidFinishLaunching(_ notification: Notification) {
    let controller = self.mainFlutterWindow?.contentViewController as! FlutterViewController

    // Try to make Flutter view background transparent
    let flutterView = controller.view
    flutterView.wantsLayer = true
    flutterView.layer?.backgroundColor = NSColor.clear.cgColor

    let channel = FlutterMethodChannel(
      name: "com.wox.macos_window_manager",
      binaryMessenger: controller.engine.binaryMessenger)

    // Store window event channel for use in window events
    self.windowEventChannel = channel

    // Setup window blur notification
    setupWindowBlurNotification()

    channel.setMethodCallHandler { [weak self] (call, result) in
      guard let window = self?.mainFlutterWindow else {
        result(FlutterError(code: "NO_WINDOW", message: "No window found", details: nil))
        return
      }

      DispatchQueue.main.async {
        switch call.method {
        case "setSize":
          if let args = call.arguments as? [String: Any],
            let width = args["width"] as? Double,
            let height = args["height"] as? Double
          {
            let size = NSSize(width: width, height: height)
            window.setContentSize(size)
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setSize", details: nil))
          }

        case "getPosition":
          let frame = window.frame
          let screenFrame = window.screen?.frame ?? NSScreen.main?.frame ?? NSRect.zero
          // Convert to bottom-left origin coordinate system
          let x = frame.origin.x
          let y = screenFrame.height - frame.origin.y - frame.height
          result(["x": x, "y": y])

        case "setPosition":
          if let args = call.arguments as? [String: Any],
            let x = args["x"] as? Double,
            let y = args["y"] as? Double
          {
            let screenFrame = window.screen?.frame ?? NSScreen.main?.frame ?? NSRect.zero
            // Convert from bottom-left to top-left origin coordinate system
            let flippedY = screenFrame.height - y - window.frame.height
            window.setFrameOrigin(NSPoint(x: x, y: flippedY))
            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setPosition", details: nil))
          }

        case "center":
          let screenFrame = window.screen?.frame ?? NSScreen.main?.frame ?? NSRect.zero
          var windowWidth: CGFloat = window.frame.width
          var windowHeight: CGFloat = window.frame.height
          if let args = call.arguments as? [String: Any] {
            if let width = args["width"] as? Double {
              windowWidth = CGFloat(width)
            }
            if let height = args["height"] as? Double {
              windowHeight = CGFloat(height)
            }
          }

          let x = (screenFrame.width - windowWidth) / 2 + screenFrame.minX
          let y = (screenFrame.height - windowHeight) / 2 + screenFrame.minY

          let newFrame = NSRect(x: x, y: y, width: windowWidth, height: windowHeight)
          window.setFrame(newFrame, display: true)
          result(nil)

        case "show":
          self?.log("Showing Wox window")
          // Save the current frontmost application before activating Wox
          if let frontApp = NSWorkspace.shared.frontmostApplication,
            frontApp != NSRunningApplication.current
          {
            self?.log(
              "Saving previous active app: \(frontApp.localizedName ?? "Unknown") (bundleID: \(frontApp.bundleIdentifier ?? "Unknown"))"
            )
            self?.previousActiveApp = frontApp
          } else {
            self?.log("No suitable previous app to save")
          }

          window.makeKeyAndOrderFront(nil)
          NSApp.activate(ignoringOtherApps: true)
          result(nil)

        case "hide":
          self?.log("Hiding Wox window")
          window.orderOut(nil)
          // Activate the previous active application after hiding Wox
          if let prevApp = self?.previousActiveApp {
            self?.log(
              "Activating previous app: \(prevApp.localizedName ?? "Unknown") (bundleID: \(prevApp.bundleIdentifier ?? "Unknown"))"
            )
            prevApp.activate(options: .activateIgnoringOtherApps)
          } else {
            self?.log("No previous app saved, looking for any other app to activate")
          }
          result(nil)

        case "focus":
          window.makeKeyAndOrderFront(nil)
          NSApp.activate(ignoringOtherApps: true)
          result(nil)

        case "isVisible":
          result(window.isVisible)

        case "setAlwaysOnTop":
          if let alwaysOnTop = call.arguments as? Bool {
            if alwaysOnTop {
              window.level = .popUpMenu
            } else {
              window.level = .normal
            }

            result(nil)
          } else {
            result(
              FlutterError(
                code: "INVALID_ARGS", message: "Invalid arguments for setAlwaysOnTop", details: nil)
            )
          }

        case "startDragging":
          if let currentEvent = window.currentEvent {
            self?.log("Performing drag with event: \(currentEvent)")
            window.performDrag(with: currentEvent)
          }
          result(nil)

        case "waitUntilReadyToShow":
          // Force appearance to light mode, otherwise borderless window will have a dark border line
          NSApp.appearance = NSAppearance(named: .aqua)

          window.level = .popUpMenu
          window.titlebarAppearsTransparent = true
          window.styleMask.insert(.fullSizeContentView)
          window.styleMask.insert(.nonactivatingPanel)
          window.styleMask.remove(.resizable)

          // Hide windows buttons
          window.titleVisibility = .hidden
          window.standardWindowButton(.closeButton)?.isHidden = true
          window.standardWindowButton(.miniaturizeButton)?.isHidden = true
          window.standardWindowButton(.zoomButton)?.isHidden = true

          // Make window can join all spaces
          window.collectionBehavior.insert(.canJoinAllSpaces)
          window.collectionBehavior.insert(.fullScreenAuxiliary)
          window.styleMask.insert(.nonactivatingPanel)
          self?.applyAcrylicEffect(to: window)

          if let mainWindow = window as? MainFlutterWindow {
            mainWindow.isReadyToShow = true
          }

          result(nil)
        default:
          result(FlutterMethodNotImplemented)
        }
      }
    }

    super.applicationDidFinishLaunching(notification)
  }
}

// flutter_inappwebview has bugs to enable transparent background
// this is the workaround https://github.com/pichillilorenzo/flutter_inappwebview/issues/2475#issuecomment-2699794479
extension InAppWebView {
  @objc public override func viewDidMoveToWindow() {
    super.viewDidMoveToWindow()

    if window != nil {
      print("InAppWebView moved to window, enforcing transparency")
      self.setValue(false, forKey: "opaque")
      self.setValue(false, forKey: "drawsBackground")
      self.layer?.backgroundColor = NSColor.clear.cgColor
    }
  }
}
