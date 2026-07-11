import Cocoa
import FlutterMacOS
import ObjectiveC.runtime

class MainFlutterWindow: NSPanel {
  var isReadyToShow: Bool = false
  private var webViewPreviewChannel: FlutterMethodChannel?

  // Temporary Flutter windowing compatibility hack.
  //
  // Wox still starts macOS through the standard storyboard-backed MainFlutterWindow, which creates
  // and attaches the implicit FlutterViewController before Dart can create any RegularWindowController.
  // Flutter's experimental macOS windowing API enables multiview lazily when the first
  // RegularWindowController is created, but the engine asserts if multiview is enabled after a view
  // controller has already been added. Flip the same internal flag here so secondary RegularWindow
  // creation does not crash while the existing primary-window startup path remains unchanged.
  //
  // This touches a private FlutterEngine ivar and must be treated as a short-term bridge. Remove it
  // once Wox migrates macOS startup to Flutter's official multiple_windows pattern: create only a
  // FlutterEngine in AppDelegate, register plugins against that engine, and let Dart create both the
  // primary and secondary windows through RegularWindowController.
  private func enableFlutterEngineMultiViewAfterImplicitView(_ engine: FlutterEngine) {
    guard let multiViewIvar = class_getInstanceVariable(FlutterEngine.self, "_multiViewEnabled") else {
      NSLog("FlutterEngine _multiViewEnabled ivar is unavailable; additional Flutter windows may fail")
      return
    }

    let offset = ivar_getOffset(multiViewIvar)
    let rawEngine = Unmanaged.passUnretained(engine).toOpaque()
    rawEngine.advanced(by: offset).assumingMemoryBound(to: ObjCBool.self).pointee = true
  }

  override var canBecomeKey: Bool {
    // Screenshot annotation reuses the main Flutter window as a temporary borderless panel. AppKit
    // does not reliably treat that shell as key-capable, which leaves keyboard shortcuts like Esc
    // outside the responder chain and produces the system alert beep instead of dismissing capture.
    // Keep the panel explicitly key-capable so Flutter continues receiving keyboard events in both
    // the normal launcher chrome and the temporary screenshot presentation.
    return true
  }

  override var canBecomeMain: Bool {
    // The screenshot workspace activates this window directly while the launcher chrome is removed.
    // Matching canBecomeMain with canBecomeKey avoids AppKit leaving the panel in a half-active
    // state where mouse works but keyboard focus never settles onto the Flutter content view.
    return true
  }

  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    enableFlutterEngineMultiViewAfterImplicitView(flutterViewController.engine)
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: false)

    RegisterGeneratedPlugins(registry: flutterViewController)
    WoxWebViewPreviewPlugin.register(with: flutterViewController.registrar(forPlugin: "WoxWebViewPreviewPlugin"))
    WoxQuickLookPreviewPlugin.register(with: flutterViewController.registrar(forPlugin: "WoxQuickLookPreviewPlugin"))

    let webViewPreviewChannel = FlutterMethodChannel(
      name: "com.wox.webview_preview",
      binaryMessenger: flutterViewController.engine.binaryMessenger
    )
    WoxWebViewPreviewPlugin.setMethodChannel(webViewPreviewChannel)
    webViewPreviewChannel.setMethodCallHandler { call, result in
      switch call.method {
      case "openInspector":
        result(WoxWebViewPreviewPlugin.openInspector(arguments: call.arguments))
      case "refresh":
        result(WoxWebViewPreviewPlugin.refresh(arguments: call.arguments))
      case "goBack":
        result(WoxWebViewPreviewPlugin.goBack(arguments: call.arguments))
      case "goForward":
        result(WoxWebViewPreviewPlugin.goForward(arguments: call.arguments))
      case "getCurrentUrl":
        result(WoxWebViewPreviewPlugin.getCurrentUrl(arguments: call.arguments))
      case "clearState":
        result(WoxWebViewPreviewPlugin.clearState(arguments: call.arguments))
      case "focusActiveSession":
        result(WoxWebViewPreviewPlugin.focusActiveSession(arguments: call.arguments))
      default:
        result(FlutterMethodNotImplemented)
      }
    }
    self.webViewPreviewChannel = webViewPreviewChannel

    super.awakeFromNib()
  }

  override public func order(_ place: NSWindow.OrderingMode, relativeTo otherWin: Int) {
    super.order(place, relativeTo: otherWin)

    if !isReadyToShow {
      setIsVisible(false)
    }
  }
}
