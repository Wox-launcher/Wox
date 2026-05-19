import 'dart:async';
import 'dart:io';

import 'package:flutter/services.dart';
import 'package:wox/entity/screenshot_session.dart';

abstract class ScreenshotPlatformBridge {
  static ScreenshotPlatformBridge _instance = MethodChannelScreenshotPlatformBridge();

  static ScreenshotPlatformBridge get instance => _instance;

  static void emitScrollingCaptureWheelEventForPlatform() {
    final bridge = _instance;
    if (bridge is MethodChannelScreenshotPlatformBridge) {
      bridge._emitScrollingCaptureWheelEvent();
    }
  }

  static void emitSelectionDisplayHintForPlatform(Map<dynamic, dynamic> arguments) {
    final bridge = _instance;
    if (bridge is MethodChannelScreenshotPlatformBridge) {
      bridge._emitSelectionDisplayHint(arguments);
    }
  }

  static void setInstanceForTest(ScreenshotPlatformBridge bridge) {
    _instance = bridge;
  }

  static void resetInstance() {
    _instance = MethodChannelScreenshotPlatformBridge();
  }

  Future<List<DisplaySnapshot>> captureAllDisplays({String? traceId, ScreenshotRect? logicalSelection});

  Future<List<DisplaySnapshot>> captureDisplayMetadata() {
    return captureAllDisplays();
  }

  Future<List<DisplaySnapshot>> loadDisplaySnapshots(List<String> displayIds) async {
    final snapshots = await captureAllDisplays();
    if (displayIds.isEmpty) {
      return snapshots;
    }

    final displayIdSet = displayIds.toSet();
    return snapshots.where((snapshot) => displayIdSet.contains(snapshot.displayId)).toList();
  }

  Future<ScreenshotNativeSelectionResult> selectCaptureRegion(ScreenshotRect nativeWorkspaceBounds);

  Future<ScreenshotWorkspacePresentation> presentCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds);

  Future<ScreenshotWorkspacePresentation> prepareCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds) {
    return presentCaptureWorkspace(nativeWorkspaceBounds);
  }

  Future<void> revealPreparedCaptureWorkspace() async {}

  Stream<ScreenshotSelectionDisplayHint> selectionDisplayHints() => const Stream<ScreenshotSelectionDisplayHint>.empty();

  Future<void> dismissCaptureWorkspacePresentation();

  Future<void> dismissNativeSelectionOverlays();

  Future<void> writeClipboardImageFile({required String filePath}) async {}

  Future<void> moveMouseTo(Offset position) async {}

  Future<void> scrollMouse({required double deltaY}) async {}

  Future<void> beginScrollingCaptureOverlay({
    required ScreenshotRect workspaceBounds,
    required ScreenshotRect selection,
    required ScreenshotRect controlsBounds,
    String? traceId,
  }) async {}

  Stream<void> scrollingCaptureWheelEvents() => const Stream<void>.empty();

  Future<Map<String, dynamic>> debugCaptureWorkspaceState();
}

class MethodChannelScreenshotPlatformBridge implements ScreenshotPlatformBridge {
  static const String _windowsChannelName = 'com.wox.windows_window_manager';
  static const String _macosChannelName = 'com.wox.macos_window_manager';
  static const String _macosScreenshotEventChannelName = 'com.wox.macos_screenshot_events';
  static const String _linuxChannelName = 'com.wox.linux_window_manager';

  late final MethodChannel _channel = MethodChannel(_resolveChannelName());
  late final StreamController<ScreenshotSelectionDisplayHint> _selectionDisplayHintController = StreamController<ScreenshotSelectionDisplayHint>.broadcast();
  late final StreamController<void> _scrollingCaptureWheelController = StreamController<void>.broadcast();

  MethodChannelScreenshotPlatformBridge() {
    if (Platform.isMacOS) {
      const MethodChannel(_macosScreenshotEventChannelName).setMethodCallHandler(_handleMacOSScreenshotEvent);
    }
  }

  String _resolveChannelName() {
    if (Platform.isWindows) {
      return _windowsChannelName;
    }
    if (Platform.isMacOS) {
      return _macosChannelName;
    }
    if (Platform.isLinux) {
      return _linuxChannelName;
    }
    throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
  }

  @override
  Future<List<DisplaySnapshot>> captureAllDisplays({String? traceId, ScreenshotRect? logicalSelection}) async {
    // Scrolling capture must pass the selected region to every desktop runner. The earlier bridge
    // only sent it to macOS, which left Windows/Linux stuck on full-display captures even though
    // the final export only needed a narrow region.
    final methodArguments = <String, dynamic>{
      if (traceId != null && traceId.isNotEmpty) 'traceId': traceId,
      if (logicalSelection != null) 'logicalSelection': logicalSelection.toJson(),
    };
    final Object? arguments = methodArguments.isNotEmpty ? methodArguments : null;
    final response = await _channel.invokeMethod<List<dynamic>>('captureAllDisplays', arguments);
    return _decodeSnapshotResponse(response);
  }

  @override
  Future<List<DisplaySnapshot>> captureDisplayMetadata() async {
    try {
      final response = await _channel.invokeMethod<List<dynamic>>('captureDisplayMetadata');
      return _decodeSnapshotResponse(response);
    } on MissingPluginException {
      return captureAllDisplays();
    }
  }

  @override
  Future<List<DisplaySnapshot>> loadDisplaySnapshots(List<String> displayIds) async {
    try {
      final response = await _channel.invokeMethod<List<dynamic>>('loadDisplaySnapshots', {'displayIds': displayIds});
      return _decodeSnapshotResponse(response);
    } on MissingPluginException {
      final snapshots = await captureAllDisplays();
      if (displayIds.isEmpty) {
        return snapshots;
      }

      final displayIdSet = displayIds.toSet();
      return snapshots.where((snapshot) => displayIdSet.contains(snapshot.displayId)).toList();
    }
  }

  @override
  Future<ScreenshotNativeSelectionResult> selectCaptureRegion(ScreenshotRect nativeWorkspaceBounds) async {
    try {
      final response = await _channel.invokeMethod<Map<dynamic, dynamic>>('selectCaptureRegion', nativeWorkspaceBounds.toJson());
      if (response == null) {
        return const ScreenshotNativeSelectionResult(wasHandled: false);
      }

      return ScreenshotNativeSelectionResult.fromJson(response.map((key, value) => MapEntry(key.toString(), value)));
    } on MissingPluginException {
      // Native region selection is capability-based. Returning an unhandled response keeps the
      // existing Flutter workspace path active on Linux and older runners.
      return const ScreenshotNativeSelectionResult(wasHandled: false);
    }
  }

  @override
  Future<ScreenshotWorkspacePresentation> presentCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds) async {
    try {
      final response = await _channel.invokeMethod<Map<dynamic, dynamic>>('presentCaptureWorkspace', nativeWorkspaceBounds.toJson());
      if (response == null) {
        return ScreenshotWorkspacePresentation(workspaceBounds: nativeWorkspaceBounds, workspaceScale: 1, presentedByPlatform: false);
      }

      return ScreenshotWorkspacePresentation.fromJson(response.map((key, value) => MapEntry(key.toString(), value)));
    } on MissingPluginException {
      // Linux and older runners do not implement screenshot-only presentation. Falling back to the
      // generic window-manager path keeps the existing single-window workflow available there.
      return ScreenshotWorkspacePresentation(workspaceBounds: nativeWorkspaceBounds, workspaceScale: 1, presentedByPlatform: false);
    }
  }

  @override
  Future<ScreenshotWorkspacePresentation> prepareCaptureWorkspace(ScreenshotRect nativeWorkspaceBounds) async {
    try {
      final response = await _channel.invokeMethod<Map<dynamic, dynamic>>('prepareCaptureWorkspace', nativeWorkspaceBounds.toJson());
      if (response == null) {
        return ScreenshotWorkspacePresentation(workspaceBounds: nativeWorkspaceBounds, workspaceScale: 1, presentedByPlatform: false);
      }

      return ScreenshotWorkspacePresentation.fromJson(response.map((key, value) => MapEntry(key.toString(), value)));
    } on MissingPluginException {
      // Native preparation is optional. Older/native-simple runners can still satisfy the contract
      // through the original present call.
      return presentCaptureWorkspace(nativeWorkspaceBounds);
    }
  }

  @override
  Future<void> revealPreparedCaptureWorkspace() async {
    try {
      await _channel.invokeMethod<void>('revealPreparedCaptureWorkspace');
    } on MissingPluginException {
      return;
    }
  }

  @override
  Stream<ScreenshotSelectionDisplayHint> selectionDisplayHints() => _selectionDisplayHintController.stream;

  @override
  Stream<void> scrollingCaptureWheelEvents() => _scrollingCaptureWheelController.stream;

  void _emitScrollingCaptureWheelEvent() {
    _scrollingCaptureWheelController.add(null);
  }

  void _emitSelectionDisplayHint(Map<dynamic, dynamic> arguments) {
    // Native selection hints arrive as loosely typed method-channel maps. Parsing them here keeps
    // controller code focused on prewarm policy rather than transport-specific null checks.
    final normalized = arguments.map<String, dynamic>((key, value) => MapEntry(key.toString(), value));
    _selectionDisplayHintController.add(ScreenshotSelectionDisplayHint.fromJson(normalized));
  }

  @override
  Future<void> dismissCaptureWorkspacePresentation() async {
    try {
      await _channel.invokeMethod<void>('dismissCaptureWorkspacePresentation');
    } on MissingPluginException {
      return;
    }
  }

  @override
  Future<void> dismissNativeSelectionOverlays() async {
    try {
      await _channel.invokeMethod<void>('dismissNativeSelectionOverlays');
    } on MissingPluginException {
      return;
    }
  }

  @override
  Future<void> writeClipboardImageFile({required String filePath}) async {
    try {
      await _channel.invokeMethod<void>('writeClipboardImageFile', {'filePath': filePath});
    } on MissingPluginException {
      throw UnsupportedError('Clipboard screenshot export is not available on ${Platform.operatingSystem}');
    }
  }

  @override
  Future<void> moveMouseTo(Offset position) async {
    try {
      await _channel.invokeMethod<void>('inputMouseMove', {'x': position.dx, 'y': position.dy});
    } on MissingPluginException {
      throw UnsupportedError('Mouse movement input is not available on ${Platform.operatingSystem}');
    }
  }

  @override
  Future<void> scrollMouse({required double deltaY}) async {
    try {
      await _channel.invokeMethod<void>('inputMouseScroll', {'deltaY': deltaY});
    } on MissingPluginException {
      throw UnsupportedError('Mouse scroll input is not available on ${Platform.operatingSystem}');
    }
  }

  @override
  Future<void> beginScrollingCaptureOverlay({
    required ScreenshotRect workspaceBounds,
    required ScreenshotRect selection,
    required ScreenshotRect controlsBounds,
    String? traceId,
  }) async {
    try {
      await _channel.invokeMethod<void>('beginScrollingCaptureOverlay', {
        'workspaceBounds': workspaceBounds.toJson(),
        'selection': selection.toJson(),
        'controlsBounds': controlsBounds.toJson(),
        if (traceId != null && traceId.isNotEmpty) 'traceId': traceId,
      });
    } on MissingPluginException {
      return;
    }
  }

  @override
  Future<Map<String, dynamic>> debugCaptureWorkspaceState() async {
    try {
      final response = await _channel.invokeMethod<Map<dynamic, dynamic>>('debugCaptureWorkspaceState');
      if (response == null) {
        return const <String, dynamic>{};
      }
      return response.map((key, value) => MapEntry(key.toString(), value));
    } on MissingPluginException {
      return const <String, dynamic>{};
    }
  }

  Future<void> _handleMacOSScreenshotEvent(MethodCall call) async {
    if (call.method == 'onScrollingCaptureWheel') {
      _emitScrollingCaptureWheelEvent();
      return;
    }

    if (call.method != 'onSelectionDisplayHint') {
      return;
    }

    final arguments = call.arguments;
    if (arguments is! Map) {
      return;
    }

    _emitSelectionDisplayHint(arguments);
  }

  List<DisplaySnapshot> _decodeSnapshotResponse(List<dynamic>? response) {
    final snapshots = response ?? const <dynamic>[];

    // The native bridge returns JSON-like maps so Flutter can keep the screenshot session platform-agnostic.
    return snapshots.whereType<Map<dynamic, dynamic>>().map((item) {
      return DisplaySnapshot.fromJson(item.map((key, value) => MapEntry(key.toString(), value)));
    }).toList();
  }
}
