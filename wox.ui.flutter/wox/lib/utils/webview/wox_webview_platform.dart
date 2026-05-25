import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

abstract class WoxWebViewPlatform {
  Future<bool> openInspector({int? windowHandle});

  Future<bool> refresh({int? windowHandle});

  Future<bool> goBack({int? windowHandle});

  Future<bool> goForward({int? windowHandle});

  Future<String?> getCurrentUrl({int? windowHandle});

  Future<bool> clearState({int? windowHandle});

  Future<bool> focusActiveSession({int? windowHandle});

  Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData);

  Future<void> releaseSession(WoxWebViewSession? session);

  void setActiveSession(WoxWebViewSession? session);

  void clearActiveSession(WoxWebViewSession session);
}
