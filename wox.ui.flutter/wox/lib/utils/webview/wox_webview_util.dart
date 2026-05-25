import 'dart:async';
import 'dart:io';

import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/macos/wox_macos_webview_platform.dart';
import 'package:wox/utils/webview/windows/wox_windows_webview_platform.dart';
import 'package:wox/utils/webview/wox_webview_platform.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

class WoxWebViewUtil {
  static final WoxWindowsWebViewPlatform _windowsPlatform = WoxWindowsWebViewPlatform();
  static final WoxMacosWebViewPlatform _macosPlatform = WoxMacosWebViewPlatform();

  static WoxWebViewPlatform? get _platform {
    if (Platform.isWindows) {
      return _windowsPlatform;
    }

    if (Platform.isMacOS) {
      return _macosPlatform;
    }

    return null;
  }

  static Future<bool> openInspector() async {
    return (await _platform?.openInspector()) ?? false;
  }

  static Future<bool> refresh() async {
    return (await _platform?.refresh()) ?? false;
  }

  static Future<bool> goBack() async {
    return (await _platform?.goBack()) ?? false;
  }

  static Future<bool> goForward() async {
    return (await _platform?.goForward()) ?? false;
  }

  static Future<String?> getCurrentUrl() async {
    return _platform?.getCurrentUrl();
  }

  static Future<bool> clearState() async {
    return (await _platform?.clearState()) ?? false;
  }

  static Future<bool> focusActiveSession() async {
    return (await _platform?.focusActiveSession()) ?? false;
  }

  static Stream<void> get unhandledEscape {
    if (Platform.isMacOS) {
      return _macosPlatform.unhandledEscape;
    }

    return const Stream<void>.empty();
  }

  static Stream<void> get startDragging {
    if (Platform.isMacOS) {
      return _macosPlatform.startDragging;
    }

    return const Stream<void>.empty();
  }

  /// Acquires a webview session for the given preview data. The caller should call [releaseSession] when the session is no longer needed.
  static Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData) async {
    return _platform?.acquireSession(previewData);
  }

  static Future<void> releaseSession(WoxWebViewSession? session) async {
    await _platform?.releaseSession(session);
  }

  static void setActiveSession(WoxWebViewSession? session) {
    _platform?.setActiveSession(session);
  }

  static void clearActiveSession(WoxWebViewSession session) {
    _platform?.clearActiveSession(session);
  }
}
