import 'dart:io';

import 'package:flutter/services.dart';
import 'package:path_provider/path_provider.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/windows/webview.dart';
import 'package:wox/utils/webview/windows/wox_windows_webview_session.dart';
import 'package:wox/utils/webview/wox_webview_platform.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';

class WoxWindowsWebViewPlatform implements WoxWebViewPlatform {
  final Map<String, WoxWindowsWebViewSession> _cachedSessions = {};

  WebviewController? _activeController;
  WoxWindowsWebViewSession? _activeSession;
  Future<void>? _environmentInitialization;
  bool _runtimeChecked = false;
  bool _runtimeAvailable = false;

  @override
  Future<WoxWebViewSession?> acquireSession(WoxPreviewWebviewData previewData) async {
    final runtimeReady = await ensureReady();
    if (!runtimeReady) {
      return null;
    }

    final cacheKey = previewData.resolvedCacheKey;
    final shouldCache = cacheKey.isNotEmpty;
    final session = shouldCache ? (_cachedSessions[cacheKey] ??= WoxWindowsWebViewSession.cached(cacheKey: cacheKey)) : WoxWindowsWebViewSession.transient();
    await session.ensureInitialized();
    await session.resume();
    await session.apply(previewData);
    return session;
  }

  @override
  void clearActiveSession(WoxWebViewSession session) {
    if (session is! WoxWindowsWebViewSession) {
      return;
    }

    if (identical(_activeController, session.controller)) {
      _activeController = null;
      _activeSession = null;
    }
  }

  @override
  Future<bool> goBack({int? windowHandle}) async {
    final controller = _activeController;
    if (controller == null) {
      return false;
    }

    await controller.goBack();
    return true;
  }

  @override
  Future<bool> goForward({int? windowHandle}) async {
    final controller = _activeController;
    if (controller == null) {
      return false;
    }

    await controller.goForward();
    return true;
  }

  @override
  Future<String?> getCurrentUrl({int? windowHandle}) async {
    return _activeSession?.currentUrl;
  }

  @override
  Future<bool> clearState({int? windowHandle}) async {
    final session = _activeSession;
    if (session == null) {
      return false;
    }

    // Drop the reusable session entry after clearing so the next preview open gets a fresh WebView controller instead of
    // inheriting navigation history or renderer-side memory that is not part of persistent browser storage.
    final cacheKey = session.cacheKey;
    if (cacheKey != null && identical(_cachedSessions[cacheKey], session)) {
      _cachedSessions.remove(cacheKey);
    }
    return session.clearState();
  }

  @override
  Future<bool> focusActiveSession({int? windowHandle}) async {
    final controller = _activeController;
    if (controller == null) {
      return false;
    }

    await controller.focus();
    return true;
  }

  @override
  Future<bool> openInspector({int? windowHandle}) async {
    final controller = _activeController;
    if (controller == null) {
      return false;
    }

    await controller.openDevTools();
    return true;
  }

  @override
  Future<bool> refresh({int? windowHandle}) async {
    final controller = _activeController;
    if (controller == null) {
      return false;
    }

    await controller.reload();
    return true;
  }

  @override
  Future<void> releaseSession(WoxWebViewSession? session) async {
    if (session is! WoxWindowsWebViewSession) {
      return;
    }

    if (session.isCached) {
      // Cached WebViews keep their browser state, but they should not keep rendering or owning focus while Flutter no longer mounts their texture.
      if (!identical(_activeSession, session)) {
        await session.suspend();
      }
      return;
    }

    await session.dispose();
  }

  @override
  void setActiveSession(WoxWebViewSession? session) {
    if (session is WoxWindowsWebViewSession) {
      _activeController = session.controller;
      _activeSession = session;
    } else {
      _activeController = null;
      _activeSession = null;
    }
  }

  Future<bool> ensureReady() async {
    if (_runtimeChecked) {
      return _runtimeAvailable;
    }

    final version = await WebviewController.getWebViewVersion();
    _runtimeChecked = true;
    _runtimeAvailable = version != null;
    if (!_runtimeAvailable) {
      return false;
    }

    _environmentInitialization ??= _initializeEnvironment();
    await _environmentInitialization;
    return true;
  }

  Future<void> _initializeEnvironment() async {
    final supportDirectory = await getApplicationSupportDirectory();
    final userDataPath = "${supportDirectory.path}${Platform.pathSeparator}webview_windows";

    try {
      await WebviewController.initializeEnvironment(userDataPath: userDataPath);
    } on PlatformException catch (error) {
      final message = error.message?.toLowerCase() ?? "";
      if (!message.contains("initialized")) {
        rethrow;
      }
    }
  }
}
