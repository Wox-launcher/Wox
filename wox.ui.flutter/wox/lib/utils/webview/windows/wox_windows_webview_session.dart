import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/windows/webview.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';
import 'package:wox/utils/webview/wox_webview_support.dart';

class WoxWindowsWebViewSession implements WoxWebViewSession {
  @override
  final bool isCached;

  @override
  final String? cacheKey;

  final WebviewController controller = WebviewController();
  final StreamController<WoxWebViewSessionAction> _actions = StreamController<WoxWebViewSessionAction>.broadcast();
  final ValueNotifier<WoxWebViewNavigationState> _navigationState = ValueNotifier(const WoxWebViewNavigationState());

  Future<void>? _initialization;
  StreamSubscription<AcceleratorKeyPressedEvent>? _acceleratorKeySubscription;
  StreamSubscription<HistoryChanged>? _historyChangedSubscription;
  StreamSubscription<String>? _urlSubscription;
  StreamSubscription<dynamic>? _webMessageSubscription;
  String _currentUrl = "";
  String _currentHtml = "";
  String _currentCss = "";
  String? _currentScriptId;
  bool _disposed = false;

  WoxWindowsWebViewSession.cached({required this.cacheKey}) : isCached = true;

  WoxWindowsWebViewSession.transient() : isCached = false, cacheKey = null;

  @override
  Stream<WoxWebViewSessionAction> get actions => _actions.stream;

  @override
  ValueListenable<WoxWebViewNavigationState> get navigationState => _navigationState;

  String? get currentUrl => _currentUrl.trim().isEmpty ? null : _currentUrl;

  @override
  Widget buildWidget() => SizedBox.expand(child: Webview(controller));

  /// Resume the cached WebView before it is mounted into the preview tree again.
  Future<void> resume() async {
    if (_disposed) {
      return;
    }

    await ensureInitialized();
    await controller.resume();
  }

  /// Suspend a cached WebView after the preview tree releases it so it cannot keep painting or holding focus off-screen.
  Future<void> suspend() async {
    if (_disposed) {
      return;
    }

    await ensureInitialized();
    await controller.suspend();
  }

  Future<void> ensureInitialized() {
    _initialization ??= _initialize();
    return _initialization!;
  }

  Future<void> apply(WoxPreviewWebviewData previewData) async {
    if (_disposed) {
      return;
    }

    await ensureInitialized();

    final injectCssChanged = _currentCss != previewData.injectCss;
    if (injectCssChanged && _currentScriptId != null) {
      await controller.removeScriptToExecuteOnDocumentCreated(_currentScriptId!);
      _currentScriptId = null;
    }

    if (injectCssChanged && previewData.injectCss.isNotEmpty) {
      _currentScriptId = await controller.addScriptToExecuteOnDocumentCreated(WoxWebViewSupport.buildInjectCssScript(previewData.injectCss));
    }

    final shouldReload = _currentUrl != previewData.url || _currentHtml != previewData.html || injectCssChanged;
    _currentCss = previewData.injectCss;

    if (shouldReload && previewData.html.isNotEmpty) {
      await controller.loadStringContent(previewData.html);
      _currentHtml = previewData.html;
      _currentUrl = previewData.url;
      return;
    }

    if (shouldReload && previewData.url.isNotEmpty) {
      await controller.loadUrl(previewData.url);
      _currentHtml = "";
      _currentUrl = previewData.url;
    }
  }

  Future<bool> clearState() async {
    if (_disposed) {
      return false;
    }

    await ensureInitialized();
    final targetUrl = _currentUrl;
    if (targetUrl.isEmpty) {
      return false;
    }

    final origin = _resolveHttpOrigin(targetUrl);
    if (origin == null || origin == "null") {
      return false;
    }

    // Clear both browser-wide network state and origin-scoped storage. Cookies/cache alone were not enough for login flows
    // such as X because their onboarding state also lives in IndexedDB, Cache Storage and service worker registrations.
    await controller.clearCookies();
    await controller.clearCache();
    await controller.clearStorageForOrigin(origin);

    // Reload through a blank page so the current document's in-memory JavaScript state is discarded before the site starts
    // a new login/session bootstrap from the cleared persistent storage.
    await controller.loadUrl("about:blank");
    _currentUrl = "";
    await controller.loadUrl(targetUrl);
    _currentUrl = targetUrl;
    return true;
  }

  String? _resolveHttpOrigin(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null || (uri.scheme != "http" && uri.scheme != "https") || uri.host.isEmpty) {
      return null;
    }

    return uri.origin;
  }

  @override
  Future<void> dispose() async {
    if (_disposed) {
      return;
    }

    _disposed = true;
    await _acceleratorKeySubscription?.cancel();
    await _historyChangedSubscription?.cancel();
    await _urlSubscription?.cancel();
    await _webMessageSubscription?.cancel();
    await _actions.close();
    _navigationState.dispose();
    await controller.dispose();
  }

  Future<void> _initialize() async {
    await controller.initialize();
    await controller.addScriptToExecuteOnDocumentCreated(WoxWebViewSupport.buildUnhandledEscapeScript(postMessageExpression: "window.chrome.webview.postMessage"));
    _acceleratorKeySubscription = controller.acceleratorKeyPressed.listen((event) {
      final isAltJ = event.keyEventKind == 2 && event.virtualKey == 0x4A;

      if (isAltJ) {
        _actions.add(WoxWebViewSessionAction.toggleActionPanel);
      }
    });
    _historyChangedSubscription = controller.historyChanged.listen((event) {
      _navigationState.value = WoxWebViewNavigationState(canGoBack: event.canGoBack, canGoForward: event.canGoForward);
    });
    _urlSubscription = controller.url.listen((url) {
      // The floating toolbar opens the current page in the system browser. Preview data only contains the
      // initial URL, so track WebView navigation events here and expose that simple state through the platform wrapper.
      _currentUrl = url;
    });
    _webMessageSubscription = controller.webMessage.listen((message) {
      if (message is! Map) {
        return;
      }

      if (message["type"] == WoxWebViewSupport.unhandledEscapeMessageType) {
        _actions.add(WoxWebViewSessionAction.fallbackEscape);
      }
    });
    await controller.setBackgroundColor(Colors.transparent);
    await controller.setPopupWindowPolicy(WebviewPopupWindowPolicy.sameWindow);
    // Keep the WebView plugin in its mobile-preview mode. The clear-state action handles stuck login/session data without
    // changing the user-facing mobile layout that existing configured sites were built around.
    await controller.setUserAgent(WoxWebViewSupport.mobileUserAgent);
    await controller.setCacheDisabled(!isCached);
  }
}
