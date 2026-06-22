import 'dart:async';

import 'package:flutter/services.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/wox_webview_platform.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

class WoxMacosWebViewPlatform implements WoxWebViewPlatform {
  static const MethodChannel _channel = MethodChannel('com.wox.webview_preview');
  final StreamController<void> _unhandledEscapeController = StreamController<void>.broadcast();

  WoxMacosWebViewPlatform() {
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'unhandledEscape') {
        _unhandledEscapeController.add(null);
        return;
      }

      throw MissingPluginException('Unknown method ${call.method}');
    });
  }

  Stream<void> get unhandledEscape => _unhandledEscapeController.stream;

  @override
  Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData) async {
    return null;
  }

  @override
  void clearActiveSession(WoxWebViewSession session) {}

  @override
  Future<bool> goBack() async {
    return _invoke('goBack');
  }

  @override
  Future<bool> goForward() async {
    return _invoke('goForward');
  }

  @override
  Future<String?> getCurrentUrl() async {
    final result = await _channel.invokeMethod<String?>('getCurrentUrl');
    return result?.trim().isEmpty == true ? null : result;
  }

  @override
  Future<bool> clearState() async {
    return _invoke('clearState');
  }

  @override
  Future<bool> focusActiveSession() async {
    return _invoke('focusActiveSession');
  }

  @override
  Future<bool> openInspector() async {
    return _invoke('openInspector');
  }

  @override
  Future<bool> refresh() async {
    return _invoke('refresh');
  }

  @override
  Future<void> releaseSession(WoxWebViewSession? session) async {}

  @override
  void setActiveSession(WoxWebViewSession? session) {}

  Future<bool> _invoke(String method) async {
    final result = await _channel.invokeMethod<bool>(method);
    return result ?? false;
  }
}
