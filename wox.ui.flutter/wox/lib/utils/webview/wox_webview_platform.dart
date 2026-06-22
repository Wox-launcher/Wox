import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

abstract class WoxWebViewPlatform {
  Future<bool> openInspector();

  Future<bool> refresh();

  Future<bool> goBack();

  Future<bool> goForward();

  Future<String?> getCurrentUrl();

  Future<bool> clearState();

  Future<bool> focusActiveSession();

  Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData);

  Future<void> releaseSession(WoxWebViewSession? session);

  void setActiveSession(WoxWebViewSession? session);

  void clearActiveSession(WoxWebViewSession session);
}
