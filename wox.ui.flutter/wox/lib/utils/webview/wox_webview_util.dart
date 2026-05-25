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

  static Future<bool> openInspector({int? windowHandle}) async {
    return (await _platform?.openInspector(windowHandle: windowHandle)) ?? false;
  }

  static Future<bool> refresh({int? windowHandle}) async {
    return (await _platform?.refresh(windowHandle: windowHandle)) ?? false;
  }

  static Future<bool> goBack({int? windowHandle}) async {
    return (await _platform?.goBack(windowHandle: windowHandle)) ?? false;
  }

  static Future<bool> goForward({int? windowHandle}) async {
    return (await _platform?.goForward(windowHandle: windowHandle)) ?? false;
  }

  static Future<String?> getCurrentUrl({int? windowHandle}) async {
    return _platform?.getCurrentUrl(windowHandle: windowHandle);
  }

  static Future<bool> clearState({int? windowHandle}) async {
    return (await _platform?.clearState(windowHandle: windowHandle)) ?? false;
  }

  static Future<bool> focusActiveSession({int? windowHandle}) async {
    return (await _platform?.focusActiveSession(windowHandle: windowHandle)) ?? false;
  }

  static Stream<int?> get unhandledEscape {
    if (Platform.isMacOS) {
      return _macosPlatform.unhandledEscape;
    }

    return const Stream<int?>.empty();
  }

  static Stream<int?> get startDragging {
    if (Platform.isMacOS) {
      return _macosPlatform.startDragging;
    }

    return const Stream<int?>.empty();
  }

  static Stream<int?> get showToolbar {
    if (Platform.isMacOS) {
      return _macosPlatform.showToolbar;
    }

    return const Stream<int?>.empty();
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
