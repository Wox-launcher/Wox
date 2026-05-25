import Cocoa
import FlutterMacOS
import WebKit

private let mobileUserAgent =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1"
private let webViewPreviewMessageHandlerName = "woxWebViewPreview"
private let unhandledEscapeMessageType = "woxUnhandledEscape"
private let startDraggingMessageType = "woxStartDragging"

private enum WoxWebViewSessionPolicy {
  case persistent
}

private struct WoxWebViewPreviewRequest {
  let urlString: String
  let injectCss: String
  let cacheDisabled: Bool
  let cacheKey: String
  let toolbarTriggerWidth: Double
  let toolbarTriggerHeight: Double
  let toolbarTriggerBottom: Double

  init(args: [String: Any]) {
    urlString = args["url"] as? String ?? ""
    injectCss = args["injectCss"] as? String ?? ""
    cacheDisabled = args["cacheDisabled"] as? Bool ?? false
    cacheKey = args["cacheKey"] as? String ?? ""
    toolbarTriggerWidth = args["toolbarTriggerWidth"] as? Double ?? 288
    toolbarTriggerHeight = args["toolbarTriggerHeight"] as? Double ?? 72
    toolbarTriggerBottom = args["toolbarTriggerBottom"] as? Double ?? 42
  }

  var hasCache: Bool {
    !cacheDisabled && !cacheKey.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
  }

  var cacheSignature: String {
    "\(injectCss)|\(mobileUserAgent)"
  }
}

private final class WoxCachedWebViewEntry {
  let webView: WKWebView
  let signature: String
  var currentURL: String

  init(webView: WKWebView, signature: String, currentURL: String) {
    self.webView = webView
    self.signature = signature
    self.currentURL = currentURL
  }
}

private final class WoxWeakWebViewBox {
  weak var webView: WKWebView?

  init(_ webView: WKWebView) {
    self.webView = webView
  }
}

private enum WoxWebViewStore {
  private static var entries: [String: WoxCachedWebViewEntry] = [:]

  static func removeEntry(cacheKey: String?) {
    guard let normalizedKey = cacheKey?.trimmingCharacters(in: .whitespacesAndNewlines), !normalizedKey.isEmpty else {
      return
    }

    entries.removeValue(forKey: normalizedKey)
  }

  static func resolveWebView(for request: WoxWebViewPreviewRequest) -> (webView: WKWebView, shouldReload: Bool) {
    guard request.hasCache else {
      return (makeWebView(for: request), true)
    }

    let normalizedKey = request.cacheKey.trimmingCharacters(in: .whitespacesAndNewlines)
    if let cached = entries[normalizedKey], cached.signature == request.cacheSignature {
      let shouldReload = cached.currentURL != request.urlString
      if shouldReload {
        cached.currentURL = request.urlString
      }
      return (cached.webView, shouldReload)
    }

    let webView = makeWebView(for: request)
    entries[normalizedKey] = WoxCachedWebViewEntry(
      webView: webView,
      signature: request.cacheSignature,
      currentURL: request.urlString
    )
    return (webView, true)
  }

  private static func makeWebView(for request: WoxWebViewPreviewRequest) -> WKWebView {
    let configuration = WoxWebViewPreviewNativeView.makeConfiguration(
      sessionPolicy: .persistent,
      injectCss: request.injectCss
    )
    let webView = WKWebView(frame: .zero, configuration: configuration)
    if #available(macOS 13.3, *) {
      webView.isInspectable = true
    }
    // Preserve the plugin's mobile-preview behavior. Clearing site state is now a separate reset action, so existing sites
    // keep their mobile layout while users still have a way to recover from stale login/session storage.
    webView.customUserAgent = mobileUserAgent
    WoxWebViewPreviewPlugin.registerMessageSource(webView)
    return webView
  }
}

class WoxWebViewPreviewPlugin: NSObject {
  private static weak var activeWebView: WKWebView?
  private static var activeCacheKey: String?
  private static var methodChannel: FlutterMethodChannel?
  private static var messageSources: [ObjectIdentifier: WoxWeakWebViewBox] = [:]
  private static var cacheKeys: [ObjectIdentifier: String] = [:]

  static func register(with registrar: FlutterPluginRegistrar) {
    let factory = WoxWebViewPreviewFactory()
    registrar.register(factory, withId: "wox/webview_preview")
  }

  static func setMethodChannel(_ channel: FlutterMethodChannel) {
    methodChannel = channel
  }

  static func setActiveWebView(_ webView: WKWebView, cacheKey: String?) {
    activeWebView = webView
    activeCacheKey = cacheKey
    let webViewId = ObjectIdentifier(webView)
    if let cacheKey {
      cacheKeys[webViewId] = cacheKey
    } else {
      cacheKeys.removeValue(forKey: webViewId)
    }
  }

  static func registerMessageSource(_ webView: WKWebView) {
    messageSources[ObjectIdentifier(webView.configuration.userContentController)] = WoxWeakWebViewBox(webView)
  }

  private static func nativeWindowHandle(from value: Any?) -> UInt? {
    if let value = value as? UInt {
      return value
    }
    if let value = value as? UInt64 {
      return UInt(truncatingIfNeeded: value)
    }
    if let value = value as? Int {
      return value > 0 ? UInt(value) : nil
    }
    if let value = value as? NSNumber {
      return UInt(truncatingIfNeeded: value.uint64Value)
    }
    return nil
  }

  private static func targetWebView(from arguments: Any?) -> WKWebView? {
    guard let args = arguments as? [String: Any], let targetHandle = nativeWindowHandle(from: args["windowHandle"]) else {
      return activeWebView
    }

    for source in messageSources.values {
      guard let webView = source.webView, let window = webView.window else {
        continue
      }
      let windowHandle = UInt(bitPattern: Unmanaged.passUnretained(window).toOpaque())
      if windowHandle == targetHandle {
        return webView
      }
    }

    return nil
  }

  static func openInspector(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments) else {
      NSLog("WoxWebViewPreviewPlugin.openInspector skipped: no active WKWebView")
      return false
    }

    if #available(macOS 13.3, *) {
      // Newer WebKit defaults embedded WKWebView inspection to disabled. Re-applying this on the active
      // view makes cached views and views created before this action follow the same inspectable path.
      activeWebView.isInspectable = true
    }

    // The public isInspectable flag only exposes the view to Safari's Develop menu. Wox's action is
    // expected to open the inspector directly, so WebKit's private developer extras flag is still needed.
    activeWebView.configuration.preferences.setValue(true, forKey: "developerExtrasEnabled")

    if let inspector = activeWebView.value(forKey: "_inspector") as? NSObject {
      let showSelector = NSSelectorFromString("show")
      if inspector.responds(to: showSelector) {
        inspector.perform(showSelector)

        let detachSelector = NSSelectorFromString("detach")
        if inspector.responds(to: detachSelector) {
          // Detached inspector windows are more reliable for Wox's small, frequently resized preview
          // panel than the inline inspector that WebKit may otherwise try to embed in the WKWebView.
          inspector.perform(detachSelector)
        }
        NSLog("WoxWebViewPreviewPlugin.openInspector opened via _inspector")
        return true
      }
    }

    let showInspectorSelector = NSSelectorFromString("_showWebInspector")
    guard activeWebView.responds(to: showInspectorSelector) else {
      NSLog("WoxWebViewPreviewPlugin.openInspector failed: WKWebView has no supported inspector selector")
      return false
    }

    activeWebView.perform(showInspectorSelector)
    NSLog("WoxWebViewPreviewPlugin.openInspector opened via _showWebInspector")
    return true
  }

  static func refresh(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments) else {
      return false
    }

    activeWebView.reload()
    return true
  }

  static func goBack(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments), activeWebView.canGoBack else {
      return false
    }

    activeWebView.goBack()
    return true
  }

  static func goForward(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments), activeWebView.canGoForward else {
      return false
    }

    activeWebView.goForward()
    return true
  }

  static func getCurrentUrl(arguments: Any?) -> String? {
    // Flutter preview data only records the original URL. Reading WKWebView.url keeps the external-browser
    // toolbar action aligned with in-page navigation without adding another delegate state cache on macOS.
    return targetWebView(from: arguments)?.url?.absoluteString
  }

  static func focusActiveSession(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments), let window = activeWebView.window else {
      return false
    }

    return window.makeFirstResponder(activeWebView)
  }

  static func clearState(arguments: Any?) -> Bool {
    guard let activeWebView = targetWebView(from: arguments) else {
      return false
    }

    guard let targetURL = activeWebView.url, let targetHost = targetURL.host?.lowercased() else {
      return false
    }

    let dataStore = activeWebView.configuration.websiteDataStore
    let dataTypes = WKWebsiteDataStore.allWebsiteDataTypes()
    WoxWebViewStore.removeEntry(cacheKey: cacheKeys[ObjectIdentifier(activeWebView)] ?? activeCacheKey)

    // Clearing only cookies/cache is not enough for modern login flows. WKWebsiteDataStore records include IndexedDB,
    // local storage, service workers and cache storage, so clear the current host group before forcing a fresh bootstrap.
    dataStore.fetchDataRecords(ofTypes: dataTypes) { records in
      let matchingRecords = records.filter { record in
        let displayName = record.displayName.lowercased()
        return displayName == targetHost || displayName.hasSuffix(".\(targetHost)") || targetHost.hasSuffix(".\(displayName)")
      }

      dataStore.removeData(ofTypes: dataTypes, for: matchingRecords) {
        DispatchQueue.main.async {
          activeWebView.stopLoading()
          activeWebView.load(URLRequest(url: targetURL, cachePolicy: .reloadIgnoringLocalAndRemoteCacheData))
        }
      }
    }
    return true
  }

  private static func windowEventArguments(from userContentController: WKUserContentController) -> [String: Any]? {
    guard let webView = messageSources[ObjectIdentifier(userContentController)]?.webView, let window = webView.window else {
      return nil
    }

    return ["windowHandle": UInt(bitPattern: Unmanaged.passUnretained(window).toOpaque())]
  }

  private static func windowEventArguments(for webView: WKWebView) -> [String: Any]? {
    guard let window = webView.window else {
      return nil
    }

    return ["windowHandle": UInt(bitPattern: Unmanaged.passUnretained(window).toOpaque())]
  }

  static func notifyUnhandledEscape(from userContentController: WKUserContentController) {
    methodChannel?.invokeMethod("unhandledEscape", arguments: windowEventArguments(from: userContentController))
  }

  static func notifyStartDragging(from userContentController: WKUserContentController) {
    methodChannel?.invokeMethod("startDragging", arguments: windowEventArguments(from: userContentController))
  }

  static func notifyShowToolbar(for webView: WKWebView) {
    methodChannel?.invokeMethod("showToolbar", arguments: windowEventArguments(for: webView))
  }
}

private final class WoxWebViewScriptMessageHandler: NSObject, WKScriptMessageHandler {
  static let shared = WoxWebViewScriptMessageHandler()

  func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
    guard message.name == webViewPreviewMessageHandlerName else {
      return
    }

    guard let body = message.body as? [String: Any], let type = body["type"] as? String else {
      return
    }

    switch type {
    case unhandledEscapeMessageType:
      WoxWebViewPreviewPlugin.notifyUnhandledEscape(from: userContentController)
    case startDraggingMessageType:
      WoxWebViewPreviewPlugin.notifyStartDragging(from: userContentController)
    default:
      return
    }
  }
}

class WoxWebViewPreviewFactory: NSObject, FlutterPlatformViewFactory {
  func create(withViewIdentifier viewId: Int64, arguments args: Any?) -> NSView {
    return WoxWebViewPreviewNativeView(frame: .zero, args: args)
  }

  func createArgsCodec() -> (FlutterMessageCodec & NSObjectProtocol)? {
    return FlutterStandardMessageCodec.sharedInstance()
  }
}

final class WoxWebViewPreviewNativeView: NSView, WKNavigationDelegate, WKUIDelegate {
  private let webView: WKWebView
  private let toolbarTriggerWidth: Double
  private let toolbarTriggerHeight: Double
  private let toolbarTriggerBottom: Double
  private var toolbarTrackingArea: NSTrackingArea?
  private var lastNativeToolbarPostTime: TimeInterval = 0

  init(frame frameRect: NSRect, args: Any?) {
    let creationParams = args as? [String: Any] ?? [:]
    let request = WoxWebViewPreviewRequest(args: creationParams)
    let resolved = WoxWebViewStore.resolveWebView(for: request)

    webView = resolved.webView
    toolbarTriggerWidth = request.toolbarTriggerWidth
    toolbarTriggerHeight = request.toolbarTriggerHeight
    toolbarTriggerBottom = request.toolbarTriggerBottom
    super.init(frame: frameRect)

    WoxWebViewPreviewPlugin.setActiveWebView(webView, cacheKey: request.hasCache ? request.cacheKey : nil)
    webView.navigationDelegate = self
    webView.uiDelegate = self
    webView.autoresizingMask = [.width, .height]
    webView.frame = bounds
    webView.removeFromSuperview()
    addSubview(webView)
    installToolbarTrackingArea()

    wantsLayer = true
    layer?.backgroundColor = NSColor.clear.cgColor

    configure(with: request, shouldReload: resolved.shouldReload)
  }

  @available(*, unavailable)
  required init?(coder: NSCoder) {
    fatalError("init(coder:) has not been implemented")
  }

  deinit {
    if let toolbarTrackingArea {
      webView.removeTrackingArea(toolbarTrackingArea)
    }
  }

  override func viewDidMoveToWindow() {
    super.viewDidMoveToWindow()
    window?.acceptsMouseMovedEvents = true
  }

  override func updateTrackingAreas() {
    super.updateTrackingAreas()
    installToolbarTrackingArea()
  }

  override func mouseMoved(with event: NSEvent) {
    notifyToolbarIfNeeded(for: event)
  }

  private func installToolbarTrackingArea() {
    if let toolbarTrackingArea {
      webView.removeTrackingArea(toolbarTrackingArea)
    }

    let trackingArea = NSTrackingArea(
      rect: webView.bounds,
      options: [.activeAlways, .inVisibleRect, .mouseMoved],
      owner: self,
      userInfo: nil
    )
    webView.addTrackingArea(trackingArea)
    toolbarTrackingArea = trackingArea
  }

  private func notifyToolbarIfNeeded(for event: NSEvent) {
    guard event.window === webView.window else {
      return
    }

    let location = webView.convert(event.locationInWindow, from: nil)
    let bounds = webView.bounds
    // WKWebView can use a flipped coordinate space, so normalize to visual distance from the bottom edge.
    let distanceFromBottom = webView.isFlipped ? bounds.height - location.y : location.y
    let left = (bounds.width - toolbarTriggerWidth) / 2
    let right = left + toolbarTriggerWidth
    let bottom = toolbarTriggerBottom
    let top = toolbarTriggerBottom + toolbarTriggerHeight

    guard location.x >= left && location.x <= right && distanceFromBottom >= bottom && distanceFromBottom <= top else {
      return
    }

    let now = Date().timeIntervalSince1970
    guard now - lastNativeToolbarPostTime >= 0.3 else {
      return
    }

    lastNativeToolbarPostTime = now
    WoxWebViewPreviewPlugin.notifyShowToolbar(for: webView)
  }

  fileprivate static func makeConfiguration(sessionPolicy: WoxWebViewSessionPolicy, injectCss: String?) -> WKWebViewConfiguration {
    let configuration = WKWebViewConfiguration()
    let userContentController = WKUserContentController()

    switch sessionPolicy {
    case .persistent:
      // Keep cookies and storage across Wox restarts.
      configuration.websiteDataStore = WKWebsiteDataStore.default()
    }

    userContentController.add(WoxWebViewScriptMessageHandler.shared, name: webViewPreviewMessageHandlerName)
    userContentController.addUserScript(
      WKUserScript(
        source: makeUnhandledEscapeScript(),
        injectionTime: .atDocumentStart,
        forMainFrameOnly: true
      )
    )
    userContentController.addUserScript(
      WKUserScript(
        source: makeStartDraggingScript(),
        injectionTime: .atDocumentStart,
        forMainFrameOnly: true
      )
    )
    if let injectCss, !injectCss.isEmpty {
      userContentController.addUserScript(
        WKUserScript(
          source: makeInjectCssScript(css: injectCss),
          injectionTime: .atDocumentEnd,
          forMainFrameOnly: true
        )
      )
    }

    configuration.userContentController = userContentController
    return configuration
  }

  private static func makeUnhandledEscapeScript() -> String {
    return """
      (() => {
        if (window.__woxUnhandledEscapeInstalled__) {
          return;
        }

        window.__woxUnhandledEscapeInstalled__ = true;

        document.addEventListener('keydown', (event) => {
          if (event.key !== 'Escape' || event.repeat) {
            return;
          }

          setTimeout(() => {
            if (event.defaultPrevented || event.cancelBubble) {
              return;
            }

            window.webkit.messageHandlers.\(webViewPreviewMessageHandlerName).postMessage({ type: '\(unhandledEscapeMessageType)' });
          }, 0);
        }, true);
      })();
      """
  }

  // Mirrors the Windows WebView script so non-interactive page areas can start native window dragging.
  private static func makeStartDraggingScript() -> String {
    return """
      (() => {
        if (window.__woxStartDraggingInstalled__) {
          return;
        }

        window.__woxStartDraggingInstalled__ = true;

        const interactiveSelector = [
          'a[href]',
          'area[href]',
          'button',
          'input',
          'textarea',
          'select',
          'option',
          'summary',
          'label',
          '[contenteditable]',
          '[role="button"]',
          '[role="link"]',
          '[role="textbox"]',
          '[role="checkbox"]',
          '[role="radio"]',
          '[role="switch"]',
          '[role="slider"]',
          '[role="tab"]',
          '[role="menuitem"]',
          '[onclick]',
          '[data-wox-no-drag]',
          '[data-no-drag]',
          '[draggable="true"]',
        ].join(',');

        const isInteractiveElement = (element) => {
          if (!(element instanceof Element)) {
            return false;
          }

          if (element.isContentEditable) {
            return true;
          }

          return element.closest(interactiveSelector) !== null;
        };

        const isInteractiveTarget = (event) => {
          const path = typeof event.composedPath === 'function' ? event.composedPath() : [];
          for (const item of path) {
            if (item === window || item === document) {
              break;
            }
            if (isInteractiveElement(item)) {
              return true;
            }
          }

          return isInteractiveElement(event.target);
        };

        const isScrollbarClick = (event) => {
          const root = document.documentElement;
          if (!root) {
            return false;
          }

          return event.clientX >= root.clientWidth || event.clientY >= root.clientHeight;
        };

        const handlePointerStart = (event) => {
          if (event.defaultPrevented || event.button !== 0 || isScrollbarClick(event) || isInteractiveTarget(event)) {
            return;
          }

          window.webkit.messageHandlers.\(webViewPreviewMessageHandlerName).postMessage({ type: '\(startDraggingMessageType)' });
        };

        document.addEventListener(window.PointerEvent ? 'pointerdown' : 'mousedown', handlePointerStart, true);
      })();
      """
  }

  private static func makeInjectCssScript(css: String) -> String {
    guard
      let cssData = try? JSONSerialization.data(withJSONObject: [css]),
      let cssArrayLiteral = String(data: cssData, encoding: .utf8)
    else {
      return ""
    }

    return """
      (() => {
        const css = \(cssArrayLiteral)[0];
        if (!css) {
          return;
        }

        const styleId = "wox-webview-preview-style";
        let style = document.getElementById(styleId);
        if (!style) {
          style = document.createElement("style");
          style.id = styleId;
          (document.head || document.documentElement).appendChild(style);
        }
        style.textContent = css;
      })();
      """
  }

  private func configure(with request: WoxWebViewPreviewRequest, shouldReload: Bool) {
    guard shouldReload, let url = URL(string: request.urlString) else {
      return
    }

    webView.load(URLRequest(url: url))
  }

  func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures)
    -> WKWebView?
  {
    if navigationAction.targetFrame == nil, let url = navigationAction.request.url {
      webView.load(URLRequest(url: url))
    }

    return nil
  }
}
