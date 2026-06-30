import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSPanel {
  var isReadyToShow: Bool = false
  private var webViewPreviewChannel: FlutterMethodChannel?

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
        result(WoxWebViewPreviewPlugin.openInspector())
      case "refresh":
        result(WoxWebViewPreviewPlugin.refresh())
      case "goBack":
        result(WoxWebViewPreviewPlugin.goBack())
      case "goForward":
        result(WoxWebViewPreviewPlugin.goForward())
      case "getCurrentUrl":
        result(WoxWebViewPreviewPlugin.getCurrentUrl())
      case "clearState":
        result(WoxWebViewPreviewPlugin.clearState())
      case "focusActiveSession":
        result(WoxWebViewPreviewPlugin.focusActiveSession())
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
