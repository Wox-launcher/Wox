import 'dart:async';

import 'package:flutter/services.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/webview/wox_webview_platform.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

class WoxMacosWebViewPlatform implements WoxWebViewPlatform {
  static const MethodChannel _channel = MethodChannel('com.wox.webview_preview');
  final StreamController<int?> _unhandledEscapeController = StreamController<int?>.broadcast();
  final StreamController<int?> _startDraggingController = StreamController<int?>.broadcast();
  final StreamController<int?> _showToolbarController = StreamController<int?>.broadcast();

  WoxMacosWebViewPlatform() {
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'unhandledEscape') {
        final windowHandle = _windowHandleFromArguments(call.arguments);
        Logger.instance.info("webview-toolbar-debug", "webview toolbar debug macOS methodChannel unhandledEscape: sourceWindowHandle=$windowHandle, arguments=${call.arguments}");
        _unhandledEscapeController.add(windowHandle);
        return;
      }

      if (call.method == 'startDragging') {
        final windowHandle = _windowHandleFromArguments(call.arguments);
        Logger.instance.info("webview-toolbar-debug", "webview toolbar debug macOS methodChannel startDragging: sourceWindowHandle=$windowHandle, arguments=${call.arguments}");
        _startDraggingController.add(windowHandle);
        return;
      }

      if (call.method == 'showToolbar') {
        final windowHandle = _windowHandleFromArguments(call.arguments);
        Logger.instance.info("webview-toolbar-debug", "webview toolbar debug macOS methodChannel showToolbar: sourceWindowHandle=$windowHandle, arguments=${call.arguments}");
        _showToolbarController.add(windowHandle);
        return;
      }

      throw MissingPluginException('Unknown method ${call.method}');
    });
  }

  Stream<int?> get unhandledEscape => _unhandledEscapeController.stream;

  Stream<int?> get startDragging => _startDraggingController.stream;

  Stream<int?> get showToolbar => _showToolbarController.stream;

  int? _windowHandleFromArguments(Object? arguments) {
    if (arguments is! Map) {
      return null;
    }

    final windowHandle = arguments['windowHandle'];
    if (windowHandle is num) {
      return windowHandle.toInt();
    }
    return null;
  }

  @override
  Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData) async {
    return null;
  }

  @override
  void clearActiveSession(WoxWebViewSession session) {}

  @override
  Future<bool> goBack({int? windowHandle}) async {
    return _invoke('goBack', windowHandle: windowHandle);
  }

  @override
  Future<bool> goForward({int? windowHandle}) async {
    return _invoke('goForward', windowHandle: windowHandle);
  }

  @override
  Future<String?> getCurrentUrl({int? windowHandle}) async {
    final result = await _channel.invokeMethod<String?>('getCurrentUrl', _argumentsForWindowHandle(windowHandle));
    return result?.trim().isEmpty == true ? null : result;
  }

  @override
  Future<bool> clearState({int? windowHandle}) async {
    return _invoke('clearState', windowHandle: windowHandle);
  }

  @override
  Future<bool> focusActiveSession({int? windowHandle}) async {
    return _invoke('focusActiveSession', windowHandle: windowHandle);
  }

  @override
  Future<bool> openInspector({int? windowHandle}) async {
    return _invoke('openInspector', windowHandle: windowHandle);
  }

  @override
  Future<bool> refresh({int? windowHandle}) async {
    return _invoke('refresh', windowHandle: windowHandle);
  }

  @override
  Future<void> releaseSession(WoxWebViewSession? session) async {}

  @override
  void setActiveSession(WoxWebViewSession? session) {}

  Map<String, Object?>? _argumentsForWindowHandle(int? windowHandle) {
    if (windowHandle == null) {
      return null;
    }

    return {"windowHandle": windowHandle};
  }

  Future<bool> _invoke(String method, {int? windowHandle}) async {
    final result = await _channel.invokeMethod<bool>(method, _argumentsForWindowHandle(windowHandle));
    return result ?? false;
  }
}
