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
  
  override func applicationDidFinishLaunching(_ notification: Notification) {
    let controller = self.mainFlutterWindow?.contentViewController as! FlutterViewController
    
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
          
        case "center":
          window.center()
          result(nil)
          
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
            // Fallback: try to find any other application to activate
            let runningApps = NSWorkspace.shared.runningApplications
            let activeApps = runningApps.filter { $0.activationPolicy == .regular && $0 != NSRunningApplication.current }
            if let anyApp = activeApps.first {
              self?.log("Activating fallback app: \(anyApp.localizedName ?? "Unknown") (bundleID: \(anyApp.bundleIdentifier ?? "Unknown"))")
              anyApp.activate(options: .activateIgnoringOtherApps)
            } else {
              self?.log("No suitable app found to activate")
            }
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
            // force appearance to light mode, otherwise borderless window will have a dark border line
            NSApp.appearance = NSAppearance(named: .aqua)

            window.level = .popUpMenu
            window.titlebarAppearsTransparent = true
            window.styleMask.insert(.fullSizeContentView)
            window.styleMask.insert(.nonactivatingPanel)
            
            // hide windows button
            window.titleVisibility = .hidden
            window.standardWindowButton(.closeButton)?.isHidden = true
            window.standardWindowButton(.miniaturizeButton)?.isHidden = true
            window.standardWindowButton(.zoomButton)?.isHidden = true

            // make window can join all spaces
            window.collectionBehavior.insert(.canJoinAllSpaces)
            window.collectionBehavior.insert(.fullScreenAuxiliary)
            window.styleMask.insert(.nonactivatingPanel)
            
            // make window ready to show
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