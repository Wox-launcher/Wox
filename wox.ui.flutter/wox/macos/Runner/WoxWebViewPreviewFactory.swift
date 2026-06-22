import Cocoa
import FlutterMacOS
import WebKit

private let mobileUserAgent =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1"
private let webViewPreviewMessageHandlerName = "woxWebViewPreview"
private let unhandledEscapeMessageType = "woxUnhandledEscape"

private enum WoxWebViewSessionPolicy {
  case persistent
}

private struct WoxWebViewPreviewRequest {
  let urlString: String
  let htmlString: String
  let injectCss: String
  let cacheDisabled: Bool
  let cacheKey: String

  init(args: [String: Any]) {
    urlString = args["url"] as? String ?? ""
    htmlString = args["html"] as? String ?? ""
    injectCss = args["injectCss"] as? String ?? ""
    cacheDisabled = args["cacheDisabled"] as? Bool ?? false
    cacheKey = args["cacheKey"] as? String ?? ""
  }

  var hasCache: Bool {
    !cacheDisabled && !cacheKey.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
  }

  var cacheSignature: String {
    "\(injectCss)|\(mobileUserAgent)"
  }

  var contentKey: String {
    if !htmlString.isEmpty {
      return "html|\(htmlString)"
    }

    return "url|\(urlString)"
  }
}

private final class WoxCachedWebViewEntry {
  let webView: WKWebView
  let signature: String
  var currentContentKey: String

  init(webView: WKWebView, signature: String, currentContentKey: String) {
    self.webView = webView
    self.signature = signature
    self.currentContentKey = currentContentKey
  }
}

private final class WoxWebViewPreviewWebView: WKWebView {
  private static let browserBackButtonNumber = 3
  private static let browserForwardButtonNumber = 4

  override func otherMouseDown(with event: NSEvent) {
    // Flutter's AppKitView-hosted WKWebView does not always translate auxiliary mouse buttons into browser navigation.
    // Handle standard back/forward side buttons here while leaving middle-click and unsupported buttons to WebKit.
    switch event.buttonNumber {
    case Self.browserBackButtonNumber:
      if canGoBack {
        goBack()
        return
      }
    case Self.browserForwardButtonNumber:
      if canGoForward {
        goForward()
        return
      }
    default:
      break
    }

    super.otherMouseDown(with: event)
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
      let shouldReload = cached.currentContentKey != request.contentKey
      if shouldReload {
        cached.currentContentKey = request.contentKey
      }
      return (cached.webView, shouldReload)
    }

    let webView = makeWebView(for: request)
    entries[normalizedKey] = WoxCachedWebViewEntry(
      webView: webView,
      signature: request.cacheSignature,
      currentContentKey: request.contentKey
    )
    return (webView, true)
  }

  private static func makeWebView(for request: WoxWebViewPreviewRequest) -> WKWebView {
    let configuration = WoxWebViewPreviewNativeView.makeConfiguration(
      sessionPolicy: .persistent,
      injectCss: request.injectCss
    )
    let webView = WoxWebViewPreviewWebView(frame: .zero, configuration: configuration)
    if #available(macOS 13.3, *) {
      webView.isInspectable = true
    }
    // Preserve the plugin's mobile-preview behavior. Clearing site state is now a separate reset action, so existing sites
    // keep their mobile layout while users still have a way to recover from stale login/session storage.
    webView.customUserAgent = mobileUserAgent
    return webView
  }
}

class WoxWebViewPreviewPlugin: NSObject {
  private static weak var activeWebView: WKWebView?
  private static var activeCacheKey: String?
  private static var methodChannel: FlutterMethodChannel?

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
  }

  static func openInspector() -> Bool {
    guard let activeWebView else {
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

  static func refresh() -> Bool {
    guard let activeWebView else {
      return false
    }

    activeWebView.reload()
    return true
  }

  static func goBack() -> Bool {
    guard let activeWebView, activeWebView.canGoBack else {
      return false
    }

    activeWebView.goBack()
    return true
  }

  static func goForward() -> Bool {
    guard let activeWebView, activeWebView.canGoForward else {
      return false
    }

    activeWebView.goForward()
    return true
  }

  static func getCurrentUrl() -> String? {
    // Flutter preview data only records the original URL. Reading WKWebView.url keeps the external-browser
    // toolbar action aligned with in-page navigation without adding another delegate state cache on macOS.
    return activeWebView?.url?.absoluteString
  }

  static func focusActiveSession() -> Bool {
    guard let activeWebView, let window = activeWebView.window else {
      return false
    }

    return window.makeFirstResponder(activeWebView)
  }

  static func clearState() -> Bool {
    guard let activeWebView else {
      return false
    }

    guard let targetURL = activeWebView.url, let targetHost = targetURL.host?.lowercased() else {
      return false
    }

    let dataStore = activeWebView.configuration.websiteDataStore
    let dataTypes = WKWebsiteDataStore.allWebsiteDataTypes()
    WoxWebViewStore.removeEntry(cacheKey: activeCacheKey)

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

  static func notifyUnhandledEscape() {
    methodChannel?.invokeMethod("unhandledEscape", arguments: nil)
  }
}

private final class WoxWebViewScriptMessageHandler: NSObject, WKScriptMessageHandler {
  static let shared = WoxWebViewScriptMessageHandler()

  func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
    guard message.name == webViewPreviewMessageHandlerName else {
      return
    }

    guard
      let body = message.body as? [String: Any],
      let type = body["type"] as? String,
      type == unhandledEscapeMessageType
    else {
      return
    }

    WoxWebViewPreviewPlugin.notifyUnhandledEscape()
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

  init(frame frameRect: NSRect, args: Any?) {
    let creationParams = args as? [String: Any] ?? [:]
    let request = WoxWebViewPreviewRequest(args: creationParams)
    let resolved = WoxWebViewStore.resolveWebView(for: request)

    webView = resolved.webView
    super.init(frame: frameRect)

    WoxWebViewPreviewPlugin.setActiveWebView(webView, cacheKey: request.hasCache ? request.cacheKey : nil)
    webView.navigationDelegate = self
    webView.uiDelegate = self
    webView.autoresizingMask = [.width, .height]
    webView.frame = bounds
    webView.removeFromSuperview()
    addSubview(webView)

    wantsLayer = true
    layer?.backgroundColor = NSColor.clear.cgColor

    configure(with: request, shouldReload: resolved.shouldReload)
  }

  @available(*, unavailable)
  required init?(coder: NSCoder) {
    fatalError("init(coder:) has not been implemented")
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
    if shouldReload, !request.htmlString.isEmpty {
      webView.loadHTMLString(request.htmlString, baseURL: nil)
      return
    }

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
