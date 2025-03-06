import Cocoa
import FlutterMacOS

@main
class AppDelegate: FlutterAppDelegate {
  // Store the previous active application
  private var previousActiveApp: NSRunningApplication?
  
  private func log(_ message: String) {
    //NSLog("WoxApp: \(message)")
  }
  
  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return false
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }
  
  /// Apply acrylic effect to window
  private func applyAcrylicEffect(to window: NSWindow) {
    // Set window base properties
    window.backgroundColor = .clear
    window.isOpaque = false
    window.hasShadow = true
    
    // Ensure light theme is used
    window.appearance = NSAppearance(named: .aqua)
    
    // Remove window title bar to ensure the entire window is transparent
    window.titlebarAppearsTransparent = true
    window.titleVisibility = .hidden
    window.styleMask.insert(.fullSizeContentView)
    
    if let contentView = window.contentView {
      // Clear any existing effect views
      for subview in contentView.subviews {
        if subview is NSVisualEffectView {
          subview.removeFromSuperview()
        }
      }
      
      // Create visual effect view
      let effectView = NSVisualEffectView(frame: contentView.bounds)
      
      // Try .hudWindow material, which is one of the closest materials to Windows acrylic effect in macOS
      effectView.material = .popover
      
      // Always keep active state
      effectView.state = .active
      
      // Use behindWindow to ensure the effect applies to content behind the window
      effectView.blendingMode = .behindWindow
      
      // Ensure the effect view resizes with the window
      effectView.autoresizingMask = [.width, .height]
      
      // Add the effect view to the bottom layer of the content view
      contentView.addSubview(effectView, positioned: .below, relativeTo: nil)
      
      // Try to make all Flutter-related views transparent
      for subview in contentView.subviews where !(subview is NSVisualEffectView) {
        subview.wantsLayer = true
        subview.layer?.backgroundColor = NSColor.clear.cgColor
        subview.layer?.opacity = 1.0 // Ensure views are visible but with transparent background
        
        // Recursively set all subviews to transparent
        setViewsTransparent(subview)
      }
      
      // Force refresh view hierarchy
      contentView.needsDisplay = true
      contentView.displayIfNeeded()
      window.contentViewController?.view.needsDisplay = true
      window.contentViewController?.view.displayIfNeeded()
    }
  }
  
  /// Recursively set view and its subviews to transparent
  private func setViewsTransparent(_ view: NSView) {
    for subview in view.subviews {
      subview.wantsLayer = true
      subview.layer?.backgroundColor = NSColor.clear.cgColor
      subview.layer?.opacity = 1.0 // Ensure views are visible but with transparent background
      
      // If it's a FlutterView or related view, try to set transparency more deeply
      if subview.className.contains("Flutter") {
        if let layer = subview.layer {
          // Try more settings to ensure Flutter view is transparent
          layer.isOpaque = false
          
          // Iterate through sublayers and set them to transparent
          if let sublayers = layer.sublayers {
            for sublayer in sublayers {
              sublayer.backgroundColor = CGColor.clear
              sublayer.isOpaque = false
            }
          }
        }
        
        // Check if there are any custom properties that can be set to transparent
        // This is to handle potential Flutter-specific implementations
        if let mainFlutterView = subview as? NSView, mainFlutterView.responds(to: Selector(("isOpaque"))) {
          mainFlutterView.setValue(false, forKey: "isOpaque")
        }
      }
      
      // Recursively process subviews
      setViewsTransparent(subview)
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
             let height = args["height"] as? Double {
            let size = NSSize(width: width, height: height)
            window.setContentSize(size)
            result(nil)
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid arguments for setSize", details: nil))
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
             let y = args["y"] as? Double {
            let screenFrame = window.screen?.frame ?? NSScreen.main?.frame ?? NSRect.zero
            // Convert from bottom-left to top-left origin coordinate system
            let flippedY = screenFrame.height - y - window.frame.height
            window.setFrameOrigin(NSPoint(x: x, y: flippedY))
            result(nil)
          } else {
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid arguments for setPosition", details: nil))
          }
          
        case "show":
          self?.log("Showing Wox window")
          // Save the current frontmost application before activating Wox
          if let frontApp = NSWorkspace.shared.frontmostApplication, frontApp != NSRunningApplication.current {
            self?.log("Saving previous active app: \(frontApp.localizedName ?? "Unknown") (bundleID: \(frontApp.bundleIdentifier ?? "Unknown"))")
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
            self?.log("Activating previous app: \(prevApp.localizedName ?? "Unknown") (bundleID: \(prevApp.bundleIdentifier ?? "Unknown"))")
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
            result(FlutterError(code: "INVALID_ARGS", message: "Invalid arguments for setAlwaysOnTop", details: nil))
          }
          
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