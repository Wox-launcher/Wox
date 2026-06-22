import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:wox/entity/wox_image.dart';

enum ScreenshotSessionStage { idle, loading, selecting, annotating, scrolling, exporting, done, cancelled, failed }

enum ScreenshotTool { select, rect, ellipse, arrow, text, mosaic }

enum ScreenshotAnnotationType { rect, ellipse, arrow, text, mosaic }

// Mosaic uses fixed brush geometry so preview, editing hit tests, and export all agree on the
// privacy mask. Keeping these values in the session model avoids duplicated magic numbers across
// the controller and screenshot view.
const List<double> screenshotMosaicBrushRadii = <double>[10, 18, 28];
const double screenshotMosaicBrushRadius = 18;
const double screenshotMosaicBlockSize = 12;
const double screenshotMosaicPointSpacing = 7;

Map<String, dynamic>? _normalizeJsonMap(dynamic value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is! Map) {
    return null;
  }

  // MethodChannel payloads arrive as JSON-like maps whose nested values are usually typed as
  // Map<Object?, Object?>. Normalizing the structure at the entity boundary keeps screenshot
  // parsing tolerant to native bridge responses instead of crashing before the session starts.
  return value.map<String, dynamic>((key, entryValue) => MapEntry(key.toString(), _normalizeJsonValue(entryValue)));
}

dynamic _normalizeJsonValue(dynamic value) {
  if (value is Map) {
    return _normalizeJsonMap(value);
  }
  if (value is List) {
    return value.map(_normalizeJsonValue).toList();
  }
  return value;
}

WoxImage? _parseOptionalWoxImage(dynamic value) {
  if (value == null) {
    return null;
  }
  if (value is String) {
    return WoxImage.parse(value);
  }

  final iconJson = _normalizeJsonMap(value);
  if (iconJson == null) {
    return null;
  }

  final imageType = iconJson['ImageType'] as String? ?? iconJson['imageType'] as String? ?? '';
  final imageData = iconJson['ImageData'] as String? ?? iconJson['imageData'] as String? ?? '';
  if (imageType.isEmpty || imageData.isEmpty) {
    return null;
  }

  // Screenshot requests can arrive from Go JSON, Dart tests, or platform-like maps. Constructing
  // the WoxImage at the model boundary keeps toolbar rendering typed instead of spreading raw map
  // parsing through the screenshot view.
  return WoxImage(imageType: imageType, imageData: imageData);
}

class ScreenshotPoint {
  const ScreenshotPoint({required this.x, required this.y});

  final double x;
  final double y;

  factory ScreenshotPoint.fromOffset(Offset offset) => ScreenshotPoint(x: offset.dx, y: offset.dy);

  Offset toOffset() => Offset(x, y);
}

class ScreenshotRect {
  const ScreenshotRect({required this.x, required this.y, required this.width, required this.height});

  final double x;
  final double y;
  final double width;
  final double height;

  factory ScreenshotRect.fromJson(Map<String, dynamic> json) {
    return ScreenshotRect(
      x: (json['x'] ?? json['X'] ?? 0).toDouble(),
      y: (json['y'] ?? json['Y'] ?? 0).toDouble(),
      width: (json['width'] ?? json['Width'] ?? 0).toDouble(),
      height: (json['height'] ?? json['Height'] ?? 0).toDouble(),
    );
  }

  factory ScreenshotRect.fromRect(Rect rect) {
    return ScreenshotRect(x: rect.left, y: rect.top, width: rect.width, height: rect.height);
  }

  Rect toRect() => Rect.fromLTWH(x, y, width, height);

  Map<String, dynamic> toJson() {
    return {'x': x, 'y': y, 'width': width, 'height': height};
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ScreenshotRect && other.x == x && other.y == y && other.width == width && other.height == height;
  }

  @override
  int get hashCode => Object.hash(x, y, width, height);
}

class ScreenshotWorkspacePresentation {
  const ScreenshotWorkspacePresentation({required this.workspaceBounds, required this.workspaceScale, required this.presentedByPlatform});

  final ScreenshotRect workspaceBounds;
  final double workspaceScale;
  final bool presentedByPlatform;

  factory ScreenshotWorkspacePresentation.fromJson(Map<String, dynamic> json) {
    final workspaceBounds = _normalizeJsonMap(json['workspaceBounds'] ?? json['WorkspaceBounds']);

    return ScreenshotWorkspacePresentation(
      workspaceBounds: ScreenshotRect.fromJson(workspaceBounds ?? const <String, dynamic>{}),
      workspaceScale: (json['workspaceScale'] ?? json['WorkspaceScale'] ?? 1).toDouble(),
      presentedByPlatform: json['presentedByPlatform'] as bool? ?? json['PresentedByPlatform'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {'workspaceBounds': workspaceBounds.toJson(), 'workspaceScale': workspaceScale, 'presentedByPlatform': presentedByPlatform};
  }
}

class ScreenshotNativeSelectionResult {
  const ScreenshotNativeSelectionResult({required this.wasHandled, this.selection, this.editorVisibleBounds});

  final bool wasHandled;
  final ScreenshotRect? selection;
  final ScreenshotRect? editorVisibleBounds;

  factory ScreenshotNativeSelectionResult.fromJson(Map<String, dynamic> json) {
    final selection = _normalizeJsonMap(json['selection'] ?? json['Selection']);
    final editorVisibleBounds = _normalizeJsonMap(json['editorVisibleBounds'] ?? json['EditorVisibleBounds']);

    return ScreenshotNativeSelectionResult(
      wasHandled: json['wasHandled'] as bool? ?? json['WasHandled'] as bool? ?? false,
      selection: selection == null ? null : ScreenshotRect.fromJson(selection),
      editorVisibleBounds: editorVisibleBounds == null ? null : ScreenshotRect.fromJson(editorVisibleBounds),
    );
  }

  Map<String, dynamic> toJson() {
    return {'wasHandled': wasHandled, 'selection': selection?.toJson(), 'editorVisibleBounds': editorVisibleBounds?.toJson()};
  }
}

class ScreenshotSelectionDisplayHint {
  const ScreenshotSelectionDisplayHint({required this.displayId, required this.displayBounds});

  final String displayId;
  final ScreenshotRect displayBounds;

  factory ScreenshotSelectionDisplayHint.fromJson(Map<String, dynamic> json) {
    final displayBounds = _normalizeJsonMap(json['displayBounds'] ?? json['DisplayBounds']);

    return ScreenshotSelectionDisplayHint(
      displayId: json['displayId'] as String? ?? json['DisplayId'] as String? ?? '',
      displayBounds: ScreenshotRect.fromJson(displayBounds ?? const <String, dynamic>{}),
    );
  }

  Map<String, dynamic> toJson() {
    return {'displayId': displayId, 'displayBounds': displayBounds.toJson()};
  }
}

class CaptureScreenshotRequest {
  const CaptureScreenshotRequest({
    required this.sessionId,
    required this.trigger,
    required this.scope,
    required this.output,
    required this.tools,
    required this.exportFilePath,
    required this.hideAnnotationToolbar,
    required this.autoConfirm,
    this.callerIcon,
  });

  final String sessionId;
  final String trigger;
  final String scope;
  final String output;
  final List<String> tools;
  final String exportFilePath;
  final bool hideAnnotationToolbar;
  final bool autoConfirm;
  final WoxImage? callerIcon;

  factory CaptureScreenshotRequest.fromJson(Map<String, dynamic> json) {
    return CaptureScreenshotRequest(
      sessionId: json['SessionId'] as String? ?? json['sessionId'] as String? ?? '',
      trigger: json['Trigger'] as String? ?? json['trigger'] as String? ?? 'plugin',
      scope: json['Scope'] as String? ?? json['scope'] as String? ?? 'all_displays',
      output: json['Output'] as String? ?? json['output'] as String? ?? 'clipboard',
      tools: ((json['Tools'] ?? json['tools']) as List<dynamic>? ?? const []).map((tool) => tool.toString()).toList(),
      exportFilePath: json['ExportFilePath'] as String? ?? json['exportFilePath'] as String? ?? '',
      hideAnnotationToolbar: json['HideAnnotationToolbar'] as bool? ?? json['hideAnnotationToolbar'] as bool? ?? false,
      autoConfirm: json['AutoConfirm'] as bool? ?? json['autoConfirm'] as bool? ?? false,
      callerIcon: _parseOptionalWoxImage(json['CallerIcon'] ?? json['callerIcon']),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'sessionId': sessionId,
      'trigger': trigger,
      'scope': scope,
      'output': output,
      'tools': tools,
      'exportFilePath': exportFilePath,
      'hideAnnotationToolbar': hideAnnotationToolbar,
      'autoConfirm': autoConfirm,
      if (callerIcon != null) 'callerIcon': callerIcon!.toJson(),
    };
  }
}

class CaptureScreenshotResult {
  const CaptureScreenshotResult({
    required this.status,
    this.screenshotPath,
    this.logicalSelectionRect,
    this.pinToScreen = false,
    this.clipboardWriteSucceeded,
    this.clipboardWarningMessage,
    this.errorCode,
    this.errorMessage,
  });

  final String status;
  final String? screenshotPath;
  final ScreenshotRect? logicalSelectionRect;
  // Pin completion uses the normal screenshot export result plus this explicit intent flag. The old
  // result shape could only mean "saved/copied", which left Go unable to decide when to create a
  // desktop overlay for the image the user just selected.
  final bool pinToScreen;
  // Clipboard export is intentionally modeled as a warning channel so screenshot file export can
  // still complete successfully even when the platform clipboard rejects the image handoff.
  final bool? clipboardWriteSucceeded;
  final String? clipboardWarningMessage;
  final String? errorCode;
  final String? errorMessage;

  factory CaptureScreenshotResult.completed({
    required Rect selectionRect,
    required String screenshotPath,
    bool pinToScreen = false,
    bool clipboardWriteSucceeded = true,
    String? clipboardWarningMessage,
  }) {
    return CaptureScreenshotResult(
      status: 'completed',
      screenshotPath: screenshotPath,
      logicalSelectionRect: ScreenshotRect.fromRect(selectionRect),
      pinToScreen: pinToScreen,
      clipboardWriteSucceeded: clipboardWriteSucceeded,
      clipboardWarningMessage: clipboardWarningMessage,
    );
  }

  factory CaptureScreenshotResult.cancelled() => const CaptureScreenshotResult(status: 'cancelled');

  factory CaptureScreenshotResult.failed({String? errorCode, String? errorMessage}) {
    return CaptureScreenshotResult(status: 'failed', errorCode: errorCode, errorMessage: errorMessage);
  }

  factory CaptureScreenshotResult.fromJson(Map<String, dynamic> json) {
    final logicalSelectionRect = _normalizeJsonMap(json['logicalSelectionRect'] ?? json['LogicalSelectionRect']);

    return CaptureScreenshotResult(
      status: json['status'] as String? ?? json['Status'] as String? ?? 'failed',
      screenshotPath: json['screenshotPath'] as String? ?? json['ScreenshotPath'] as String?,
      logicalSelectionRect: logicalSelectionRect != null ? ScreenshotRect.fromJson(logicalSelectionRect) : null,
      pinToScreen: json['pinToScreen'] as bool? ?? json['PinToScreen'] as bool? ?? false,
      clipboardWriteSucceeded: json['clipboardWriteSucceeded'] as bool? ?? json['ClipboardWriteSucceeded'] as bool?,
      clipboardWarningMessage: json['clipboardWarningMessage'] as String? ?? json['ClipboardWarningMessage'] as String?,
      errorCode: json['errorCode'] as String? ?? json['ErrorCode'] as String?,
      errorMessage: json['errorMessage'] as String? ?? json['ErrorMessage'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'status': status,
      'screenshotPath': screenshotPath,
      'logicalSelectionRect': logicalSelectionRect?.toJson(),
      'pinToScreen': pinToScreen,
      'clipboardWriteSucceeded': clipboardWriteSucceeded,
      'clipboardWarningMessage': clipboardWarningMessage,
      'errorCode': errorCode,
      'errorMessage': errorMessage,
    };
  }
}

class DisplaySnapshot {
  DisplaySnapshot({
    required this.displayId,
    required this.logicalBounds,
    required this.pixelBounds,
    required this.scale,
    required this.rotation,
    this.imageBytesBase64 = '',
    this.imageFilePath = '',
  });

  final String displayId;
  final ScreenshotRect logicalBounds;
  final ScreenshotRect pixelBounds;
  final double scale;
  final int rotation;
  final String imageBytesBase64;
  final String imageFilePath;
  // Native screenshot flows start from metadata plus platform overlays, then hydrate PNG payloads
  // only for displays Flutter actually renders or exports. Windows can provide that payload as a
  // temp file path to avoid pushing large base64 strings through MethodChannel during prewarm.
  bool get hasImageBytes => imageBytesBase64.isNotEmpty || imageFilePath.isNotEmpty;

  Uint8List? _cachedImageBytes;
  MemoryImage? _cachedImageProvider;

  // Screenshot annotation drags rebuild the workspace frequently. Decoding base64 on every build
  // created a fresh MemoryImage each frame, which made the captured backdrop briefly restart and
  // visually flicker while drawing. Cache both the bytes and image provider per snapshot so the
  // background stays stable across rebuilds and export still reuses the same decoded payload.
  Uint8List get imageBytes {
    if (!hasImageBytes) {
      throw StateError('Display snapshot $displayId does not have image bytes yet');
    }

    final cachedBytes = _cachedImageBytes;
    if (cachedBytes != null) {
      return cachedBytes;
    }
    if (imageBytesBase64.isNotEmpty) {
      return _cachedImageBytes = base64Decode(imageBytesBase64);
    }
    return _cachedImageBytes = File(imageFilePath).readAsBytesSync();
  }

  // Load deferred snapshot pixels without forcing file-backed Windows payloads through sync IO.
  Future<Uint8List> loadImageBytes() async {
    if (!hasImageBytes) {
      throw StateError('Display snapshot $displayId does not have image bytes yet');
    }

    final cachedBytes = _cachedImageBytes;
    if (cachedBytes != null) {
      return cachedBytes;
    }
    if (imageBytesBase64.isNotEmpty) {
      return _cachedImageBytes = base64Decode(imageBytesBase64);
    }
    return _cachedImageBytes = await File(imageFilePath).readAsBytes();
  }

  MemoryImage get imageProvider {
    if (!hasImageBytes) {
      throw StateError('Display snapshot $displayId does not have image bytes yet');
    }

    return _cachedImageProvider ??= MemoryImage(imageBytes);
  }

  void releaseImageCache() {
    final cachedProvider = _cachedImageProvider;
    if (cachedProvider != null) {
      unawaited(cachedProvider.evict());
    }
    _cachedImageProvider = null;
    _cachedImageBytes = null;
    if (imageFilePath.isNotEmpty) {
      unawaited(() async {
        try {
          await File(imageFilePath).delete();
        } catch (_) {
          // Copied snapshots can point at the same temp file; duplicate cleanup is harmless.
        }
      }());
    }
  }

  factory DisplaySnapshot.fromJson(Map<String, dynamic> json) {
    final logicalBounds = _normalizeJsonMap(json['logicalBounds'] ?? json['LogicalBounds']);
    final pixelBounds = _normalizeJsonMap(json['pixelBounds'] ?? json['PixelBounds']);

    return DisplaySnapshot(
      displayId: json['displayId'] as String? ?? json['DisplayId'] as String? ?? '',
      logicalBounds: ScreenshotRect.fromJson(logicalBounds ?? const <String, dynamic>{}),
      pixelBounds: ScreenshotRect.fromJson(pixelBounds ?? const <String, dynamic>{}),
      scale: (json['scale'] ?? json['Scale'] ?? 1).toDouble(),
      rotation: (json['rotation'] ?? json['Rotation'] ?? 0) as int,
      imageBytesBase64: json['imageBytesBase64'] as String? ?? json['ImageBytesBase64'] as String? ?? '',
      imageFilePath: json['imageFilePath'] as String? ?? json['ImageFilePath'] as String? ?? '',
    );
  }

  DisplaySnapshot copyWith({ScreenshotRect? logicalBounds, ScreenshotRect? pixelBounds, double? scale, int? rotation, String? imageBytesBase64, String? imageFilePath}) {
    final next = DisplaySnapshot(
      displayId: displayId,
      logicalBounds: logicalBounds ?? this.logicalBounds,
      pixelBounds: pixelBounds ?? this.pixelBounds,
      scale: scale ?? this.scale,
      rotation: rotation ?? this.rotation,
      imageBytesBase64: imageBytesBase64 ?? this.imageBytesBase64,
      imageFilePath: imageFilePath ?? this.imageFilePath,
    );
    if (next.imageBytesBase64 == this.imageBytesBase64 && next.imageFilePath == this.imageFilePath) {
      next._cachedImageBytes = _cachedImageBytes;
      next._cachedImageProvider = _cachedImageProvider;
    }
    return next;
  }
}

class ScreenshotAnnotation {
  const ScreenshotAnnotation({
    required this.id,
    required this.type,
    this.rect,
    this.start,
    this.end,
    this.text,
    this.color = const Color(0xFFFF5B36),
    this.strokeWidth = 3,
    this.fontSize = 20,
    this.points = const <Offset>[],
    this.mosaicRadius = screenshotMosaicBrushRadius,
  });

  final String id;
  final ScreenshotAnnotationType type;
  final Rect? rect;
  final Offset? start;
  final Offset? end;
  final String? text;
  final Color color;
  final double strokeWidth;
  final double fontSize;
  // Mosaic annotations need path geometry rather than a bounding rect because the user paints an
  // irregular privacy mask. Other annotation types leave this empty.
  final List<Offset> points;
  final double mosaicRadius;

  static const Object _unset = Object();

  // Screenshot editing now mutates existing annotations in place, so the model needs a local
  // copy helper instead of rebuilding ad-hoc objects in the view layer. Keeping the update logic
  // next to the annotation fields avoids duplicated constructor branches for every edit action.
  ScreenshotAnnotation copyWith({
    Object? rect = _unset,
    Object? start = _unset,
    Object? end = _unset,
    Object? text = _unset,
    Color? color,
    double? strokeWidth,
    double? fontSize,
    List<Offset>? points,
    double? mosaicRadius,
  }) {
    return ScreenshotAnnotation(
      id: id,
      type: type,
      rect: identical(rect, _unset) ? this.rect : rect as Rect?,
      start: identical(start, _unset) ? this.start : start as Offset?,
      end: identical(end, _unset) ? this.end : end as Offset?,
      text: identical(text, _unset) ? this.text : text as String?,
      color: color ?? this.color,
      strokeWidth: strokeWidth ?? this.strokeWidth,
      fontSize: fontSize ?? this.fontSize,
      points: points ?? this.points,
      mosaicRadius: mosaicRadius ?? this.mosaicRadius,
    );
  }
}
