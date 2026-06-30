import Cocoa
import FlutterMacOS
import QuickLookUI

class WoxQuickLookPreviewPlugin: NSObject {
  static func register(with registrar: FlutterPluginRegistrar) {
    let factory = WoxQuickLookPreviewFactory()
    registrar.register(factory, withId: "wox/quick_look_preview")
  }
}

class WoxQuickLookPreviewFactory: NSObject, FlutterPlatformViewFactory {
  func create(withViewIdentifier viewId: Int64, arguments args: Any?) -> NSView {
    return WoxQuickLookPreviewNativeView(frame: .zero, args: args)
  }

  func createArgsCodec() -> (FlutterMessageCodec & NSObjectProtocol)? {
    return FlutterStandardMessageCodec.sharedInstance()
  }
}

final class WoxQuickLookPreviewNativeView: NSView {
  private let previewView: QLPreviewView
  private var previewItemURL: NSURL?
  private var fallbackLabel: NSTextField?

  init(frame frameRect: NSRect, args: Any?) {
    previewView = Self.makePreviewView()
    super.init(frame: frameRect)

    wantsLayer = true
    layer?.backgroundColor = NSColor.clear.cgColor

    previewView.autoresizingMask = [.width, .height]
    previewView.frame = bounds
    previewView.autostarts = false
    // AppKitView disposal does not close the containing NSWindow, so close the
    // Quick Look view explicitly when Flutter removes this platform view.
    previewView.shouldCloseWithWindow = false
    addSubview(previewView)

    configure(args: args)
  }

  @available(*, unavailable)
  required init?(coder: NSCoder) {
    fatalError("init(coder:) has not been implemented")
  }

  deinit {
    previewView.close()
  }

  override func layout() {
    super.layout()
    previewView.frame = bounds
    fallbackLabel?.frame = bounds.insetBy(dx: 16, dy: 16)
  }

  private static func makePreviewView() -> QLPreviewView {
    guard let previewView = QLPreviewView(frame: .zero, style: .normal) else {
      fatalError("QLPreviewView initialization failed")
    }
    return previewView
  }

  private func configure(args: Any?) {
    let creationParams = args as? [String: Any] ?? [:]
    let filePath = (creationParams["filePath"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
    guard !filePath.isEmpty else {
      showFallback("Missing file path")
      return
    }

    guard FileManager.default.fileExists(atPath: filePath) else {
      showFallback("File not found")
      return
    }

    let itemURL = URL(fileURLWithPath: filePath) as NSURL
    previewItemURL = itemURL
    previewView.previewItem = itemURL
    previewView.refreshPreviewItem()
  }

  private func showFallback(_ message: String) {
    previewView.isHidden = true

    let label = NSTextField(labelWithString: message)
    label.alignment = .center
    label.lineBreakMode = .byWordWrapping
    label.maximumNumberOfLines = 0
    label.textColor = .secondaryLabelColor
    label.autoresizingMask = [.width, .height]
    label.frame = bounds.insetBy(dx: 16, dy: 16)
    addSubview(label)
    fallbackLabel = label
  }
}
