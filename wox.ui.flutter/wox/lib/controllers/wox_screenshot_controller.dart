import 'dart:async';
import 'dart:io';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/screenshot/screenshot_platform_bridge.dart';
import 'package:wox/utils/windows/window_manager.dart';

enum _ScrollingCaptureDirection { append, prepend }

class WoxScreenshotController extends GetxController {
  static const Color defaultAnnotationColor = Color(0xFFFF5B36);
  static const Color mosaicAnnotationUiColor = Color(0xFF29FF72);
  static const double minTextFontSize = 12;
  static const double maxTextFontSize = 48;
  // A fixed 16-frame cap made narrow scrolling captures stop updating while the user was still
  // scrolling, even though those small frames used far less memory than a wide selection. The pixel
  // budget keeps large captures bounded while allowing small page regions to collect enough frames
  // for the live preview and final export to stay useful.
  static const int _scrollingCaptureMinimumFrameLimit = 16;
  static const int _scrollingCaptureMaximumFrameLimit = 96;
  static const int _scrollingCaptureStoredPixelBudget = 24 * 1024 * 1024;
  static const double _scrollingCaptureWheelSteps = 7;
  static const Duration _scrollingCaptureSettleDelay = Duration(milliseconds: 120);
  // Feature trial: scrolling capture now uses a single template-matching registration path. The
  // previous average-difference matcher was too strict for white pages with repeated text, so this
  // keeps the tuning local while manual testing compares the new score against real captures.
  static const int _scrollingCaptureTemplateMatchMaxHeight = 240;
  static const double _scrollingCaptureTemplateMatchHeightRatio = 0.28;
  static const double _scrollingCaptureTemplateMatchSearchRatio = 0.85;
  static const int _scrollingCaptureTemplateMatchMaxHorizontalShift = 2;
  // Optimization: real debug logs showed full per-row NCC taking hundreds of milliseconds on large
  // selections. A smaller sample grid plus coarse-to-fine search keeps the same matcher semantics
  // while scoring far fewer y candidates; the refine radius covers coarse-step misses around the
  // best coarse location.
  static const int _scrollingCaptureTemplateMatchTargetColumns = 64;
  static const int _scrollingCaptureTemplateMatchTargetRows = 48;
  static const int _scrollingCaptureTemplateMatchCoarseStep = 6;
  static const int _scrollingCaptureTemplateMatchRefineRadius = 12;
  static const double _scrollingCaptureTemplateMatchThreshold = 0.82;
  static const int _scrollingCaptureSeamFeatherRows = 10;
  static const double _scrollingCaptureToolbarMinWidth = 168;
  static const double _scrollingCapturePreviewMaxWidth = 320;

  final isSessionActive = false.obs;
  final stage = ScreenshotSessionStage.idle.obs;
  final currentTool = ScreenshotTool.select.obs;
  final displaySnapshots = <DisplaySnapshot>[].obs;
  final annotations = <ScreenshotAnnotation>[].obs;
  final scrollingCaptureFrames = <ScrollingCapturePreviewFrame>[].obs;
  final isScrollingCaptureUpdating = false.obs;
  final isNativeScrollingCaptureOverlay = false.obs;
  final selection = Rxn<ScreenshotRect>();
  final virtualBounds = Rxn<ScreenshotRect>();
  final workspaceScale = 1.0.obs;
  final selectedAnnotationId = RxnString();
  final editingTextAnnotationId = RxnString();
  final annotationCreationColor = defaultAnnotationColor.obs;
  final mosaicBrushRadius = screenshotMosaicBrushRadius.obs;
  final textDraftPosition = Rxn<Offset>();
  final textDraftFontSize = 20.0.obs;
  final textDraftColor = defaultAnnotationColor.obs;
  final textDraftController = TextEditingController();

  final Map<String, ui.Image> _decodedImages = <String, ui.Image>{};
  final Map<String, Future<void>> _displayDecodeTasks = <String, Future<void>>{};
  List<DisplaySnapshot> _pendingRawSnapshots = const <DisplaySnapshot>[];
  final Map<String, DisplaySnapshot> _hydratedRawSnapshots = <String, DisplaySnapshot>{};
  final Map<String, Future<DisplaySnapshot>> _rawSnapshotHydrationTasks = <String, Future<DisplaySnapshot>>{};
  Rect? _nativeWorkspaceBounds;
  Rect? _activeNativeWorkspaceBounds;
  String? _preparedDisplayId;
  Rect? _preparedDisplayBounds;
  ScreenshotWorkspacePresentation? _preparedPresentation;
  List<DisplaySnapshot>? _preparedSnapshots;
  Timer? _scrollingCaptureFrameDebounce;
  StreamSubscription<ScrollingCaptureWheelEvent>? _scrollingCaptureWheelSubscription;
  Rect? _scrollingCaptureControlsBounds;
  Rect? _pendingScrollingCaptureSelection;
  _ScrollingCaptureDirection? _pendingScrollingCaptureDirection;
  String _scrollingCaptureTraceId = "";
  StreamSubscription<ScreenshotSelectionDisplayHint>? _selectionDisplayHintSubscription;
  bool _acceptSelectionDisplayHints = false;
  int _preparedDisplayRevision = 0;
  int _captureSessionRevision = 0;
  Completer<CaptureScreenshotResult>? _sessionCompleter;
  _SavedScreenshotWindowState? _savedWindowState;
  CaptureScreenshotRequest? _activeRequest;

  String tr(String key) => Get.find<WoxSettingController>().tr(key);

  // The screenshot view needs read-only access to caller metadata such as the plugin icon. Keeping
  // mutation inside the controller preserves the existing session lifecycle while allowing the
  // toolbox to render request-scoped identity details.
  CaptureScreenshotRequest? get activeRequest => _activeRequest;

  String get scrollingCaptureTraceId => _scrollingCaptureTraceId;

  Rect get virtualBoundsRect => virtualBounds.value?.toRect() ?? Rect.zero;

  Rect? get selectionRect => selection.value?.toRect();

  ScreenshotAnnotation? get selectedAnnotation => annotationById(selectedAnnotationId.value);

  // Mosaic preview needs the same decoded display images that export uses. Exposing this map as
  // read-only view state keeps the pixelated preview aligned with the final PNG instead of forcing
  // the widget tree to decode MemoryImage bytes again on every brush movement.
  Map<String, ui.Image> get decodedDisplayImages => _decodedImages;

  // Scrolling capture spans Flutter scheduling, native screen capture, image decoding, overlap
  // matching, and repaint. Centralizing timing log formatting keeps every probe searchable with the
  // same prefix while avoiding heavy image/base64 data in the log stream.
  void _logScrollingCaptureTiming(String traceId, String event, Map<String, Object?> fields) {
    final details = fields.entries.map((entry) => '${entry.key}=${_formatScrollingTimingValue(entry.value)}').join(' ');
    Logger.instance.debug(traceId, 'scrolling_capture_timing event=$event${details.isEmpty ? '' : ' $details'}');
  }

  // Screenshot startup spans native capture, optional native selection, deferred PNG hydration, and
  // Flutter reveal. Keeping these probes small and searchable makes Windows startup regressions
  // diagnosable without logging image payloads or full monitor metadata.
  void _logScreenshotTiming(String traceId, String event, Map<String, Object?> fields) {
    final details = fields.entries.map((entry) => '${entry.key}=${_formatScrollingTimingValue(entry.value)}').join(' ');
    Logger.instance.debug(traceId, 'screenshot_timing event=$event${details.isEmpty ? '' : ' $details'}');
  }

  String _formatScrollingTimingValue(Object? value) {
    if (value == null) {
      return 'null';
    }
    if (value is double) {
      return _formatScrollingTimingDouble(value);
    }
    if (value is Rect) {
      return _formatScrollingTimingRect(value);
    }
    return value.toString();
  }

  String _formatScrollingTimingDouble(double value) {
    if (!value.isFinite) {
      return value.toString();
    }
    return value.toStringAsFixed(2);
  }

  String _formatScrollingTimingRect(Rect rect) {
    return '${rect.width.round()}x${rect.height.round()}@${rect.left.round()},${rect.top.round()}';
  }

  ScreenshotAnnotation? annotationById(String? annotationId) {
    if (annotationId == null) {
      return null;
    }

    for (final annotation in annotations) {
      if (annotation.id == annotationId) {
        return annotation;
      }
    }

    return null;
  }

  Future<CaptureScreenshotResult> startCaptureSession(String traceId, CaptureScreenshotRequest request) async {
    if (_sessionCompleter != null && !_sessionCompleter!.isCompleted) {
      return CaptureScreenshotResult.failed(errorCode: 'busy', errorMessage: 'Screenshot session is already running');
    }

    _activeRequest = request;
    _sessionCompleter = Completer<CaptureScreenshotResult>();
    await _prepareNewSession(traceId);

    try {
      final metadataWatch = Stopwatch()..start();
      final metadataSnapshots = await ScreenshotPlatformBridge.instance.captureDisplayMetadata();
      _logScreenshotTiming(traceId, 'capture_metadata', {'elapsedMs': metadataWatch.elapsedMilliseconds, 'displayCount': metadataSnapshots.length});
      if (metadataSnapshots.isEmpty) {
        throw StateError('No display snapshots returned');
      }

      final nativeWorkspaceBounds = _calculateUnionRect(metadataSnapshots.map((item) => item.logicalBounds.toRect()).toList());
      if (_shouldTryNativeSelection(metadataSnapshots)) {
        // Windows joins the macOS metadata-first handoff: show native selection before Flutter gets
        // PNG/base64 payloads. The previous Windows fallback prepared and hydrated the full virtual
        // desktop before reveal, which is exactly the slow path on large multi-monitor layouts.
        final nativeSelectionResult = await _tryStartNativeSelectionEditor(traceId, metadataSnapshots, nativeWorkspaceBounds);
        if (nativeSelectionResult != null) {
          return nativeSelectionResult;
        }
      }

      if (Platform.isMacOS || Platform.isWindows) {
        await _presentPreparedCaptureWorkspace(traceId, metadataSnapshots, nativeWorkspaceBounds);
        return _sessionFutureOrCancelled();
      }

      final rawSnapshots = await _hydrateRawSnapshots(metadataSnapshots);
      await _presentFlutterCaptureWorkspace(traceId, rawSnapshots, nativeWorkspaceBounds);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to start screenshot session: $e');
      final failed = CaptureScreenshotResult.failed(errorCode: 'capture_failed', errorMessage: e.toString());
      await _restoreWindowState(traceId);
      _resetSessionState();
      _sessionCompleter = null;
      return failed;
    }

    return _sessionFutureOrCancelled();
  }

  Future<CaptureScreenshotResult> _sessionFutureOrCancelled() {
    final completer = _sessionCompleter;
    if (completer == null || completer.isCompleted) {
      // Window show/focus recovery can cancel a screenshot session while the startup coroutine is
      // still unwinding. Returning a cancelled result here keeps that race user-visible but avoids
      // turning a legitimate cancellation into a null-completer crash.
      return Future<CaptureScreenshotResult>.value(CaptureScreenshotResult.cancelled());
    }

    return completer.future;
  }

  bool _shouldTryNativeSelection(List<DisplaySnapshot> metadataSnapshots) {
    if (Platform.isWindows) {
      return metadataSnapshots.isNotEmpty;
    }
    if (Platform.isMacOS) {
      return metadataSnapshots.length >= 2;
    }
    return false;
  }

  Future<void> _presentFlutterCaptureWorkspace(String traceId, List<DisplaySnapshot> rawSnapshots, Rect nativeWorkspaceBounds) async {
    _activeNativeWorkspaceBounds = nativeWorkspaceBounds;
    final presentation = await ScreenshotPlatformBridge.instance.presentCaptureWorkspace(ScreenshotRect.fromRect(nativeWorkspaceBounds));
    final normalizedSnapshots = _normalizeSnapshotsForWorkspace(
      rawSnapshots,
      nativeWorkspaceBounds: nativeWorkspaceBounds,
      workspaceBounds: presentation.workspaceBounds.toRect(),
      workspaceScale: presentation.workspaceScale,
    );

    await _decodeDisplayImages(normalizedSnapshots);
    displaySnapshots.assignAll(normalizedSnapshots);
    virtualBounds.value = ScreenshotRect.fromRect(presentation.workspaceBounds.toRect());
    workspaceScale.value = presentation.workspaceScale;

    if (!presentation.presentedByPlatform) {
      final bounds = virtualBoundsRect;
      // The fallback path still uses one Flutter window, but only platforms without screenshot-
      // specific native presentation should reach it. macOS and Windows install their own
      // capture overlay handling so multi-display selection does not inherit launcher assumptions.
      await windowManager.setBounds(bounds.topLeft, bounds.size);
      await windowManager.setAlwaysOnTop(true);
      await windowManager.show();
      await windowManager.focus();
    }

    await WoxApi.instance.onShow(traceId);
    stage.value = ScreenshotSessionStage.selecting;
  }

  Future<void> _presentPreparedCaptureWorkspace(String traceId, List<DisplaySnapshot> metadataSnapshots, Rect nativeWorkspaceBounds) async {
    _activeNativeWorkspaceBounds = nativeWorkspaceBounds;
    final presentation = await ScreenshotPlatformBridge.instance.prepareCaptureWorkspace(ScreenshotRect.fromRect(nativeWorkspaceBounds));
    final rawSnapshots = await _hydrateRawSnapshots(metadataSnapshots);
    final normalizedSnapshots = _normalizeSnapshotsForWorkspace(
      rawSnapshots,
      nativeWorkspaceBounds: nativeWorkspaceBounds,
      workspaceBounds: presentation.workspaceBounds.toRect(),
      workspaceScale: presentation.workspaceScale,
    );

    await _decodeDisplayImages(normalizedSnapshots);
    displaySnapshots.assignAll(normalizedSnapshots);
    virtualBounds.value = ScreenshotRect.fromRect(presentation.workspaceBounds.toRect());
    workspaceScale.value = presentation.workspaceScale;

    await WoxApi.instance.onShow(traceId);
    if (presentation.presentedByPlatform) {
      // macOS and Windows now share the same handoff: resize and prime the native screenshot shell
      // before Flutter decodes monitor PNGs, then reveal only after the first annotation frame is
      // ready. The previous all-in-one path made the user wait for capture, PNG encoding, layout,
      // and show on one visible transition.
      await ScreenshotPlatformBridge.instance.revealPreparedCaptureWorkspace();
    } else {
      final bounds = virtualBoundsRect;
      await windowManager.setBounds(bounds.topLeft, bounds.size);
      await windowManager.setAlwaysOnTop(true);
      await windowManager.show();
      await windowManager.focus();
    }

    stage.value = ScreenshotSessionStage.selecting;
  }

  Future<CaptureScreenshotResult?> _tryStartNativeSelectionEditor(String traceId, List<DisplaySnapshot> rawSnapshots, Rect nativeWorkspaceBounds) async {
    if (!_shouldTryNativeSelection(rawSnapshots)) {
      return null;
    }

    _pendingRawSnapshots = rawSnapshots;
    _nativeWorkspaceBounds = nativeWorkspaceBounds;
    // macOS can prewarm while its native overlay tracks the display. Windows sends only one hint
    // when the native overlay appears; drag-time hints were removed because they made PNG hydration
    // contend with mouse feedback and caused a visible mid-drag pause.
    _acceptSelectionDisplayHints = Platform.isMacOS || Platform.isWindows;
    final previousSelectionDisplayHintSubscription = _selectionDisplayHintSubscription;
    if (previousSelectionDisplayHintSubscription != null) {
      await previousSelectionDisplayHintSubscription.cancel();
    }
    _selectionDisplayHintSubscription =
        _acceptSelectionDisplayHints
            ? ScreenshotPlatformBridge.instance.selectionDisplayHints().listen((hint) {
              unawaited(_handleNativeSelectionDisplayHint(traceId, hint));
            })
            : null;

    final selectionWatch = Stopwatch()..start();
    final nativeSelection = await ScreenshotPlatformBridge.instance.selectCaptureRegion(ScreenshotRect.fromRect(nativeWorkspaceBounds));
    _logScreenshotTiming(traceId, 'native_selection_result', {
      'elapsedMs': selectionWatch.elapsedMilliseconds,
      'handled': nativeSelection.wasHandled,
      'selection': nativeSelection.selection?.toRect(),
    });
    _acceptSelectionDisplayHints = false;
    final activeSelectionDisplayHintSubscription = _selectionDisplayHintSubscription;
    if (activeSelectionDisplayHintSubscription != null) {
      await activeSelectionDisplayHintSubscription.cancel();
    }
    _selectionDisplayHintSubscription = null;
    if (!nativeSelection.wasHandled) {
      _clearNativePreparationState();
      return null;
    }

    if (nativeSelection.selection == null) {
      final cancelled = CaptureScreenshotResult.cancelled();
      await _restoreWindowState(traceId);
      _resetSessionState();
      _sessionCompleter = null;
      return cancelled;
    }

    // A single Flutter window cannot reliably render across mixed native displays, so the
    // annotation editor is confined to the monitor where the user drew the selection. This avoids
    // cross-display rendering artifacts while keeping the transition seamless on that monitor.
    final selectedDisplay = _findDisplaySnapshotForSelection(nativeSelection.selection!.toRect(), rawSnapshots);
    _activeNativeWorkspaceBounds = selectedDisplay.logicalBounds.toRect();
    await _prepareNativeDisplayForAnnotation(traceId, selectedDisplay);
    final presentation = _preparedPresentation;
    final normalizedSnapshots = _preparedSnapshots;
    if (presentation == null || normalizedSnapshots == null) {
      throw StateError('Native screenshot handoff did not prepare a Flutter workspace');
    }
    final normalizedSelection = _normalizeNativeRectForWorkspace(
      nativeSelection.selection!.toRect(),
      nativeWorkspaceBounds: selectedDisplay.logicalBounds.toRect(),
      workspaceBounds: presentation.workspaceBounds.toRect(),
      workspaceScale: presentation.workspaceScale,
    );

    displaySnapshots.assignAll(normalizedSnapshots);
    virtualBounds.value = ScreenshotRect.fromRect(presentation.workspaceBounds.toRect());
    selection.value = ScreenshotRect.fromRect(normalizedSelection);
    workspaceScale.value = presentation.workspaceScale;
    stage.value = ScreenshotSessionStage.annotating;
    // Native selection now hands Flutter one prepared display immediately, then hydrates any other
    // displays intersecting the chosen rect in the background. That keeps the reveal path fast
    // without regressing multi-display exports that still need the remaining pixels later.
    unawaited(_ensureSelectionSnapshotsReady(normalizedSelection));
    await WidgetsBinding.instance.endOfFrame;
    await WoxApi.instance.onShow(traceId);

    if (presentation.presentedByPlatform) {
      final revealWatch = Stopwatch()..start();
      await ScreenshotPlatformBridge.instance.revealPreparedCaptureWorkspace();
      _logScreenshotTiming(traceId, 'native_reveal_workspace', {'elapsedMs': revealWatch.elapsedMilliseconds, 'displayId': selectedDisplay.displayId});
    } else {
      final bounds = virtualBoundsRect;
      await windowManager.setBounds(bounds.topLeft, bounds.size);
      await windowManager.setAlwaysOnTop(true);
      await windowManager.show();
      await windowManager.focus();
    }

    // Native selection now stays on-screen until Flutter already holds the final annotation frame.
    // That removes the visible "loading / resize / repaint" gap that used to appear after mouse-up.
    await WidgetsBinding.instance.endOfFrame;
    final dismissWatch = Stopwatch()..start();
    await ScreenshotPlatformBridge.instance.dismissNativeSelectionOverlays();
    _logScreenshotTiming(traceId, 'native_dismiss_overlays', {'elapsedMs': dismissWatch.elapsedMilliseconds});
    if (_activeRequest?.autoConfirm == true) {
      // Auto-confirm still waits until Flutter owns the normalized selection and decoded workspace.
      // Exporting through confirmSelection keeps plugin API captures on the same file-output path as
      // a manual confirm while skipping the now-unnecessary annotation toolbar stopover.
      final sessionFuture = _sessionFutureOrCancelled();
      unawaited(confirmSelection(traceId));
      return sessionFuture;
    }
    return _sessionFutureOrCancelled();
  }

  Future<void> _prepareNewSession(String traceId) async {
    _clearNativePreparationState();
    _disposeDecodedImages();
    _captureSessionRevision += 1;
    displaySnapshots.clear();
    annotations.clear();
    selection.value = null;
    virtualBounds.value = null;
    currentTool.value = ScreenshotTool.select;
    textDraftController.clear();
    textDraftPosition.value = null;
    stage.value = ScreenshotSessionStage.loading;
    isSessionActive.value = true;

    final launcherController = Get.find<WoxLauncherController>();
    // The launcher query box can still hold primary focus from the action that started screenshot
    // capture. If that stale focus survives into the screenshot workspace, launcher-side IME and
    // focus listeners wake back up behind the overlay and can cancel the session before annotation
    // begins. Clear the launcher focus up front so the screenshot view becomes the only focus owner.
    FocusManager.instance.primaryFocus?.unfocus();
    launcherController.queryBoxFocusNode.unfocus();
    final isVisible = await windowManager.isVisible();
    final position = await windowManager.getPosition();
    final size = await windowManager.getSize();
    _savedWindowState = _SavedScreenshotWindowState(
      wasVisible: isVisible,
      wasInSettingView: launcherController.isInSettingView.value,
      position: position,
      size: size,
      forceHideOnBlur: launcherController.forceHideOnBlur,
    );

    launcherController.forceHideOnBlur = false;
    if (isVisible) {
      await WoxApi.instance.onHide(traceId);
    }

    // Hiding the current window before native capture prevents the launcher itself from ending up in
    // the captured background, which is a hard requirement for the single-window screenshot workflow.
    await windowManager.hide();
  }

  Future<void> cancelSession(String traceId, {String reason = 'unspecified'}) async {
    await _hideScreenshotWindowBeforeFinish(traceId);
    await _finishSession(traceId, CaptureScreenshotResult.cancelled(), ScreenshotSessionStage.cancelled, windowAlreadyHidden: true, reason: reason);
  }

  Future<void> failSession(String traceId, {required String errorCode, required String errorMessage}) async {
    await _hideScreenshotWindowBeforeFinish(traceId);
    await _finishSession(
      traceId,
      CaptureScreenshotResult.failed(errorCode: errorCode, errorMessage: errorMessage),
      ScreenshotSessionStage.failed,
      windowAlreadyHidden: true,
      reason: 'failure:$errorCode',
    );
  }

  Future<void> confirmSelection(String traceId) async {
    final currentSelection = selectionRect;
    if (currentSelection == null || currentSelection.width < 1 || currentSelection.height < 1) {
      return;
    }

    stage.value = ScreenshotSessionStage.exporting;
    try {
      final screenshotPath = await _exportCurrentSelectionPngFile(traceId: traceId, selection: currentSelection);

      var clipboardWriteSucceeded = true;
      String? clipboardWarningMessage;
      if (_activeRequest?.output == 'clipboard') {
        try {
          await ScreenshotPlatformBridge.instance.writeClipboardImageFile(filePath: screenshotPath);
        } catch (e) {
          // Clipboard rejection should not discard a screenshot file that was already exported.
          // Returning a completed session with warning fields lets Go notify the user about the
          // degraded clipboard path while keeping the saved PNG available.
          clipboardWriteSucceeded = false;
          clipboardWarningMessage = e.toString();
          Logger.instance.warn(traceId, 'Screenshot exported but clipboard write failed: $clipboardWarningMessage');
        }
      }

      final result = CaptureScreenshotResult.completed(
        selectionRect: currentSelection,
        screenshotPath: screenshotPath,
        clipboardWriteSucceeded: clipboardWriteSucceeded,
        clipboardWarningMessage: clipboardWarningMessage,
      );
      await _finishSession(traceId, result, ScreenshotSessionStage.done, restoreVisibility: false, windowAlreadyHidden: true);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to export screenshot: $e');
      await failSession(traceId, errorCode: 'export_failed', errorMessage: e.toString());
    }
  }

  Future<void> pinSelection(String traceId) async {
    final currentSelection = selectionRect;
    if (currentSelection == null || currentSelection.width < 1 || currentSelection.height < 1) {
      return;
    }

    stage.value = ScreenshotSessionStage.exporting;
    try {
      final screenshotPath = await _exportCurrentSelectionPngFile(traceId: traceId, selection: currentSelection);
      // Pin is a third completion mode for the same selected image. It deliberately skips clipboard
      // handoff and returns an explicit flag so the Go screenshot plugin can create the native,
      // draggable overlay after Flutter has finished composing the PNG.
      final result = CaptureScreenshotResult.completed(selectionRect: currentSelection, screenshotPath: screenshotPath, pinToScreen: true);
      await _finishSession(traceId, result, ScreenshotSessionStage.done, restoreVisibility: false, windowAlreadyHidden: true);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to pin screenshot: $e');
      await failSession(traceId, errorCode: 'pin_failed', errorMessage: e.toString());
    }
  }

  Future<String> _exportCurrentSelectionPngFile({required String traceId, required Rect selection}) async {
    await _hideScreenshotWindowBeforeFinish(traceId);

    await _ensureSelectionSnapshotsReady(selection);

    // Screenshot completion used to push full PNG/base64 payloads back through the websocket
    // bridge. The backend now preallocates the export path inside woxDataDirectory so both confirm
    // and pin actions write the same durable PNG before choosing clipboard or overlay behavior.
    final activeRequest = _activeRequest;
    if (activeRequest == null || activeRequest.exportFilePath.isEmpty) {
      throw StateError('Screenshot export file path is missing');
    }

    final screenshotPath = await _writeSelectionPngFile(
      exportFilePath: activeRequest.exportFilePath,
      selection: selection,
      snapshots: displaySnapshots.toList(),
      annotationsToPaint: annotations.toList(),
    );
    return screenshotPath;
  }

  Future<void> startScrollingCapture(String traceId) async {
    _scrollingCaptureTraceId = traceId;
    final sessionWatch = Stopwatch()..start();
    final currentSelection = selectionRect;
    if (currentSelection == null || currentSelection.width < 1 || currentSelection.height < 1) {
      _logScrollingCaptureTiming(traceId, 'start_ignored_invalid_selection', {'elapsedMs': sessionWatch.elapsedMilliseconds});
      return;
    }

    _logScrollingCaptureTiming(traceId, 'start', {'selection': currentSelection, 'frameCount': scrollingCaptureFrames.length});
    stage.value = ScreenshotSessionStage.scrolling;
    _disposeScrollingCaptureFrames();
    _scrollingCaptureTraceId = traceId;
    try {
      if (Platform.isWindows) {
        // Windows BitBlt/CAPTUREBLT includes this top-level Flutter screenshot window when it still
        // covers the selected rectangle. Move the window into the compact native overlay before the
        // first scrolling frame so the preview stores page pixels instead of our gray mask and green
        // selection handles.
        await _beginNativeScrollingCaptureOverlay(traceId, currentSelection);
        await WidgetsBinding.instance.endOfFrame;
        // DWM can present the old fullscreen window for one compositor tick after SetWindowPos. A
        // short Windows-only settle keeps the first BitBlt frame aligned with the compact overlay
        // geometry without slowing down the normal macOS capture path.
        await Future<void>.delayed(const Duration(milliseconds: 50));
      } else {
        await WidgetsBinding.instance.endOfFrame;
      }

      final firstFrameWatch = Stopwatch()..start();
      await _appendScrollingCaptureFrame(traceId, currentSelection, _ScrollingCaptureDirection.append);
      _logScrollingCaptureTiming(traceId, 'start_initial_frame_done', {'elapsedMs': firstFrameWatch.elapsedMilliseconds, 'frameCount': scrollingCaptureFrames.length});
      if (Platform.isMacOS && scrollingCaptureFrames.isNotEmpty) {
        await _beginNativeScrollingCaptureOverlay(traceId, currentSelection);
      }
      _logScrollingCaptureTiming(traceId, 'start_done', {'elapsedMs': sessionWatch.elapsedMilliseconds, 'frameCount': scrollingCaptureFrames.length});
    } catch (e) {
      _logScrollingCaptureTiming(traceId, 'start_failed', {'elapsedMs': sessionWatch.elapsedMilliseconds, 'error': e.runtimeType});
      Logger.instance.error(traceId, 'Failed to start scrolling screenshot: $e');
      await failSession(traceId, errorCode: 'scrolling_start_failed', errorMessage: e.toString());
    }
  }

  Future<void> _beginNativeScrollingCaptureOverlay(String traceId, Rect selection) async {
    final controlsBounds = _calculateScrollingControlsBounds(selection);
    _scrollingCaptureControlsBounds = controlsBounds;
    await ScreenshotPlatformBridge.instance.beginScrollingCaptureOverlay(
      workspaceBounds: ScreenshotRect.fromRect(virtualBoundsRect),
      selection: ScreenshotRect.fromRect(selection),
      controlsBounds: ScreenshotRect.fromRect(controlsBounds),
      traceId: traceId,
    );
    isNativeScrollingCaptureOverlay.value = true;
    _listenForNativeScrollingCaptureWheelEvents(traceId);
  }

  void _listenForNativeScrollingCaptureWheelEvents(String traceId) {
    _scrollingCaptureWheelSubscription?.cancel();
    _scrollingCaptureWheelSubscription = ScreenshotPlatformBridge.instance.scrollingCaptureWheelEvents().listen((event) {
      final selectionForRefresh = selectionRect;
      if (selectionForRefresh != null) {
        final direction = _scrollingCaptureDirectionFromDelta(event.deltaY);
        _logScrollingCaptureTiming(traceId, 'native_wheel_event', {
          'deltaY': event.deltaY,
          'rawDeltaY': event.rawDeltaY,
          'direction': _scrollingCaptureDirectionName(direction),
        });
        _scheduleScrollingCaptureFrame(traceId, selectionForRefresh, direction);
      }
    });
    // Native scrolling capture should append frames only after real wheel input. Polling while idle
    // made the preview grow without user movement, while this shared listener keeps the Windows
    // pre-capture overlay path and the macOS overlay path on the same event-driven update model.
  }

  Future<void> handleScrollingCaptureWheel(String traceId, double scrollDeltaY) async {
    if (stage.value != ScreenshotSessionStage.scrolling) {
      return;
    }

    final currentSelection = selectionRect;
    if (currentSelection == null || currentSelection.width < 1 || currentSelection.height < 1) {
      return;
    }

    try {
      final wheelSteps = _scrollDeltaToWheelSteps(scrollDeltaY);
      final scrollWatch = Stopwatch()..start();
      _logScrollingCaptureTiming(traceId, 'wheel_start', {
        'deltaY': scrollDeltaY,
        'wheelSteps': wheelSteps,
        'frameCount': scrollingCaptureFrames.length,
        'pending': _pendingScrollingCaptureSelection != null,
        'updating': isScrollingCaptureUpdating.value,
      });
      // Scrolling mode must not warp the cursor. Forward only the user's wheel delta at the current
      // pointer location, then refresh the stitched preview after a short settle window so rapid
      // wheel gestures do not trigger full-desktop capture on every native scroll tick.
      await ScreenshotPlatformBridge.instance.scrollMouse(deltaY: wheelSteps);
      _logScrollingCaptureTiming(traceId, 'wheel_synthetic_scroll_done', {
        'elapsedMs': scrollWatch.elapsedMilliseconds,
        'wheelSteps': wheelSteps,
        'frameCount': scrollingCaptureFrames.length,
      });
      _scheduleScrollingCaptureFrame(traceId, currentSelection, _scrollingCaptureDirectionFromDelta(scrollDeltaY));
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to update scrolling screenshot: $e');
    }
  }

  void _scheduleScrollingCaptureFrame(String traceId, Rect selection, _ScrollingCaptureDirection direction) {
    _pendingScrollingCaptureSelection = selection;
    _pendingScrollingCaptureDirection = direction;
    final hadDebounce = _scrollingCaptureFrameDebounce != null;
    _logScrollingCaptureTiming(traceId, 'schedule_request', {
      'selection': selection,
      'direction': _scrollingCaptureDirectionName(direction),
      'frameCount': scrollingCaptureFrames.length,
      'pending': _pendingScrollingCaptureSelection != null,
      'updating': isScrollingCaptureUpdating.value,
      'debounceActive': hadDebounce,
    });
    if (_scrollingCaptureFrameDebounce != null) {
      return;
    }

    final settleWatch = Stopwatch()..start();
    _scrollingCaptureFrameDebounce = Timer(_scrollingCaptureSettleDelay, () {
      _scrollingCaptureFrameDebounce = null;
      _logScrollingCaptureTiming(traceId, 'schedule_timer_fired', {
        'settleMs': settleWatch.elapsedMilliseconds,
        'frameCount': scrollingCaptureFrames.length,
        'pending': _pendingScrollingCaptureSelection != null,
        'updating': isScrollingCaptureUpdating.value,
      });
      if (stage.value != ScreenshotSessionStage.scrolling) {
        _pendingScrollingCaptureSelection = null;
        _pendingScrollingCaptureDirection = null;
        _logScrollingCaptureTiming(traceId, 'schedule_dropped_inactive', {'frameCount': scrollingCaptureFrames.length});
        return;
      }
      if (isScrollingCaptureUpdating.value) {
        final queuedSelection = _pendingScrollingCaptureSelection;
        final queuedDirection = _pendingScrollingCaptureDirection ?? _ScrollingCaptureDirection.append;
        if (queuedSelection != null) {
          _logScrollingCaptureTiming(traceId, 'schedule_merged_while_updating', {
            'selection': queuedSelection,
            'direction': _scrollingCaptureDirectionName(queuedDirection),
            'frameCount': scrollingCaptureFrames.length,
          });
          _scheduleScrollingCaptureFrame(traceId, queuedSelection, queuedDirection);
        }
        return;
      }

      final selectionForCapture = _pendingScrollingCaptureSelection;
      final directionForCapture = _pendingScrollingCaptureDirection ?? _ScrollingCaptureDirection.append;
      _pendingScrollingCaptureSelection = null;
      _pendingScrollingCaptureDirection = null;
      if (selectionForCapture == null) {
        _logScrollingCaptureTiming(traceId, 'schedule_dropped_no_selection', {'frameCount': scrollingCaptureFrames.length});
        return;
      }

      // Use throttling instead of debounce for wheel-driven capture. Debounce waited until scrolling
      // stopped, which allowed a single captured pair to be separated by more than one viewport and
      // caused poor overlap matches; throttling records intermediate frames while staying bounded.
      _logScrollingCaptureTiming(traceId, 'schedule_append_started', {
        'selection': selectionForCapture,
        'direction': _scrollingCaptureDirectionName(directionForCapture),
        'frameCount': scrollingCaptureFrames.length,
      });
      unawaited(
        _appendScrollingCaptureFrame(traceId, selectionForCapture, directionForCapture).whenComplete(() {
          final queuedSelection = _pendingScrollingCaptureSelection;
          final queuedDirection = _pendingScrollingCaptureDirection ?? _ScrollingCaptureDirection.append;
          if (queuedSelection != null && stage.value == ScreenshotSessionStage.scrolling) {
            _logScrollingCaptureTiming(traceId, 'schedule_append_done_with_pending', {
              'selection': queuedSelection,
              'direction': _scrollingCaptureDirectionName(queuedDirection),
              'frameCount': scrollingCaptureFrames.length,
            });
            _scheduleScrollingCaptureFrame(traceId, queuedSelection, queuedDirection);
          }
        }),
      );
    });
  }

  Rect _calculateScrollingControlsBounds(Rect selection) {
    final bounds = virtualBoundsRect;
    const verticalMargin = 24.0;
    const toolbarReserveHeight = 72.0;
    final rightAvailableWidth = math.max(0.0, bounds.right - selection.right - 44);
    final leftAvailableWidth = math.max(0.0, selection.left - bounds.left - 44);
    final maxAvailableWidth = math.min(math.max(rightAvailableWidth, leftAvailableWidth), _scrollingCapturePreviewMaxWidth);
    // The preview height used to be capped by the selected rectangle, which made long captures stay
    // tiny even when the dimmed workspace had plenty of vertical room. Reserving only the toolbar
    // and outer margins lets the side preview grow through the full masked area. Width still needs a
    // hard cap so a fresh long screenshot remains a side preview instead of expanding into a large
    // duplicate of the selected page.
    final maxPreviewHeight = math.max(1.0, bounds.height - verticalMargin * 2 - toolbarReserveHeight);
    final previewSize = _calculateScrollingPreviewRenderSize(selection: selection, maxWidth: maxAvailableWidth, maxHeight: maxPreviewHeight);
    final controlsWidth = math.max(previewSize.width, _scrollingCaptureToolbarMinWidth);
    final controlsHeight = previewSize.height + toolbarReserveHeight;
    final useRightSide = selection.right + 20 + controlsWidth <= bounds.right - 24 || rightAvailableWidth >= leftAvailableWidth;
    final left = useRightSide ? selection.right + 20 : math.max(bounds.left + verticalMargin, selection.left - controlsWidth - 20);
    final top = selection.top.clamp(bounds.top + verticalMargin, math.max(bounds.top + verticalMargin, bounds.bottom - controlsHeight - verticalMargin)).toDouble();

    // The macOS scrolling overlay moves Flutter into a compact preview/toolbox panel while a native
    // mouse-transparent overlay dims only the outside of the selected region. Keeping this geometry
    // in the controller lets the preview width follow the stitched image aspect ratio instead of a
    // fixed hard-coded side panel.
    return Rect.fromLTWH(left, top, controlsWidth, controlsHeight);
  }

  Size _calculateScrollingPreviewRenderSize({required Rect selection, required double maxWidth, required double maxHeight}) {
    final totalHeight = scrollingCaptureFrames.fold<int>(0, (total, frame) => total + frame.visibleHeight);
    final contentWidth = scrollingCaptureFrames.isEmpty ? selection.width : scrollingCaptureFrames.first.pixelWidth.toDouble();
    final contentHeight = scrollingCaptureFrames.isEmpty || totalHeight <= 0 ? selection.height : totalHeight.toDouble();
    final safeMaxWidth = math.max(1.0, maxWidth);
    final safeMaxHeight = math.max(1.0, maxHeight);
    final scale = math.min(safeMaxWidth / math.max(1.0, contentWidth), safeMaxHeight / math.max(1.0, contentHeight));
    return Size(math.max(1.0, contentWidth * scale), math.max(1.0, contentHeight * scale));
  }

  Future<void> _syncNativeScrollingControlsBounds(String traceId, Rect selection) async {
    if (!(Platform.isMacOS || Platform.isWindows) || !isNativeScrollingCaptureOverlay.value) {
      return;
    }

    final resizeWatch = Stopwatch()..start();
    final nextBounds = _calculateScrollingControlsBounds(selection);
    final previousBounds = _scrollingCaptureControlsBounds;
    if (previousBounds != null &&
        (previousBounds.left - nextBounds.left).abs() < 1 &&
        (previousBounds.top - nextBounds.top).abs() < 1 &&
        (previousBounds.width - nextBounds.width).abs() < 1 &&
        (previousBounds.height - nextBounds.height).abs() < 1) {
      _logScrollingCaptureTiming(traceId, 'native_preview_resize_skipped', {
        'elapsedMs': resizeWatch.elapsedMilliseconds,
        'frameCount': scrollingCaptureFrames.length,
        'bounds': nextBounds,
      });
      return;
    }

    _scrollingCaptureControlsBounds = nextBounds;
    try {
      // The stitched image becomes narrower as more vertical content is added. Resizing the compact
      // Flutter preview window after each accepted frame keeps the native side panel wrapped to the
      // image instead of leaving the old first-frame panel width visible as a gray gutter.
      await windowManager.setBounds(nextBounds.topLeft, nextBounds.size);
      _logScrollingCaptureTiming(traceId, 'native_preview_resize_done', {
        'elapsedMs': resizeWatch.elapsedMilliseconds,
        'frameCount': scrollingCaptureFrames.length,
        'bounds': nextBounds,
      });
    } catch (e) {
      _logScrollingCaptureTiming(traceId, 'native_preview_resize_failed', {'elapsedMs': resizeWatch.elapsedMilliseconds, 'error': e.runtimeType});
      Logger.instance.warn(traceId, 'Failed to resize scrolling screenshot preview window: $e');
    }
  }

  Future<void> confirmScrollingSelection(String traceId) async {
    final currentSelection = selectionRect;
    if (currentSelection == null || currentSelection.width < 1 || currentSelection.height < 1) {
      return;
    }

    if (scrollingCaptureFrames.isEmpty) {
      await _appendScrollingCaptureFrame(traceId, currentSelection, _ScrollingCaptureDirection.append);
    }

    stage.value = ScreenshotSessionStage.exporting;
    try {
      await _hideScreenshotWindowBeforeFinish(traceId);

      final activeRequest = _activeRequest;
      if (activeRequest == null || activeRequest.exportFilePath.isEmpty) {
        throw StateError('Screenshot export file path is missing');
      }

      final screenshotPath = await _writeScrollingSelectionPngFile(exportFilePath: activeRequest.exportFilePath, frames: scrollingCaptureFrames.toList());

      var clipboardWriteSucceeded = true;
      String? clipboardWarningMessage;
      if (activeRequest.output == 'clipboard') {
        try {
          await ScreenshotPlatformBridge.instance.writeClipboardImageFile(filePath: screenshotPath);
        } catch (e) {
          clipboardWriteSucceeded = false;
          clipboardWarningMessage = e.toString();
          Logger.instance.warn(traceId, 'Scrolling screenshot exported but clipboard write failed: $clipboardWarningMessage');
        }
      }

      final result = CaptureScreenshotResult.completed(
        selectionRect: currentSelection,
        screenshotPath: screenshotPath,
        clipboardWriteSucceeded: clipboardWriteSucceeded,
        clipboardWarningMessage: clipboardWarningMessage,
      );
      await _finishSession(traceId, result, ScreenshotSessionStage.done, restoreVisibility: false, windowAlreadyHidden: true);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to export scrolling screenshot: $e');
      await failSession(traceId, errorCode: 'scrolling_export_failed', errorMessage: e.toString());
    }
  }

  Future<String> _writeSelectionPngFile({
    required String exportFilePath,
    required Rect selection,
    required List<DisplaySnapshot> snapshots,
    required List<ScreenshotAnnotation> annotationsToPaint,
  }) async {
    final rendered = await _renderSelectionImage(selection: selection, snapshots: snapshots, annotationsToPaint: annotationsToPaint);

    final exportFile = File(exportFilePath);
    await exportFile.parent.create(recursive: true);
    await exportFile.writeAsBytes(rendered.pngBytes, flush: true);
    return exportFile.path;
  }

  Future<void> _appendScrollingCaptureFrame(String traceId, Rect selection, _ScrollingCaptureDirection direction) async {
    final appendWatch = Stopwatch()..start();
    final frameIndex = scrollingCaptureFrames.length;
    final frameLimit = _scrollingCaptureFrameLimit();
    if (scrollingCaptureFrames.length >= frameLimit) {
      _logScrollingCaptureTiming(traceId, 'append_dropped_max_frames', {
        'elapsedMs': appendWatch.elapsedMilliseconds,
        'frameIndex': frameIndex,
        'frameCount': scrollingCaptureFrames.length,
        'frameLimit': frameLimit,
      });
      return;
    }
    if (isScrollingCaptureUpdating.value) {
      _logScrollingCaptureTiming(traceId, 'append_dropped_busy', {
        'elapsedMs': appendWatch.elapsedMilliseconds,
        'frameIndex': frameIndex,
        'frameCount': scrollingCaptureFrames.length,
      });
      return;
    }

    isScrollingCaptureUpdating.value = true;
    try {
      _logScrollingCaptureTiming(traceId, 'append_start', {
        'frameIndex': frameIndex,
        'selection': selection,
        'direction': _scrollingCaptureDirectionName(direction),
        'frameCount': scrollingCaptureFrames.length,
      });
      final captureWatch = Stopwatch()..start();
      final nextFrame = await _captureScrollingSelectionFrame(selection);
      _logScrollingCaptureTiming(traceId, 'append_capture_done', {
        'elapsedMs': captureWatch.elapsedMilliseconds,
        'frameIndex': frameIndex,
        'direction': _scrollingCaptureDirectionName(direction),
        'pixel': '${nextFrame.pixelWidth}x${nextFrame.pixelHeight}',
      });
      if (scrollingCaptureFrames.isNotEmpty) {
        final anchorFrame = direction == _ScrollingCaptureDirection.append ? scrollingCaptureFrames.last : scrollingCaptureFrames.first;
        final overlap = _findScrollingOverlap(traceId, anchorFrame, nextFrame, direction);
        if (overlap.isDuplicate) {
          _logScrollingCaptureTiming(traceId, 'append_dropped_duplicate', {
            'elapsedMs': appendWatch.elapsedMilliseconds,
            'frameIndex': frameIndex,
            'direction': _scrollingCaptureDirectionName(direction),
            'overlapRows': overlap.overlapRows,
            'averageDifference': overlap.averageDifference,
          });
          nextFrame.dispose();
          return;
        }

        if (!overlap.isReliable) {
          // Feature trial: template matching is the only registration path for scrolling capture
          // right now. A low NCC score means the seam is not proven, so dropping the frame keeps the
          // stitched image monotonic while the debug log exposes the score for manual tuning.
          nextFrame.dispose();
          _logScrollingCaptureTiming(traceId, 'append_dropped_unreliable_overlap', {
            'elapsedMs': appendWatch.elapsedMilliseconds,
            'frameIndex': frameIndex,
            'direction': _scrollingCaptureDirectionName(direction),
            'overlapRows': overlap.overlapRows,
            'averageDifference': overlap.averageDifference,
          });
          Logger.instance.warn(traceId, 'Scrolling screenshot overlap was not reliable; dropped frame to avoid repeated stitched content');
          return;
        }

        if (direction == _ScrollingCaptureDirection.append) {
          nextFrame.cropTop = overlap.overlapRows;
          nextFrame.seamFeatherRows = math.min(_scrollingCaptureSeamFeatherRows, math.min(nextFrame.cropTop, nextFrame.visibleHeight)).toInt();
          if (nextFrame.visibleHeight <= 0) {
            nextFrame.dispose();
            return;
          }
        } else {
          final previousFirstFrame = scrollingCaptureFrames.first;
          // Bug fix: upward scrolling prepends the freshly captured viewport and removes the shared
          // rows from the old first frame. Reusing cropTop keeps preview/export drawing on the same
          // top-to-bottom frame model instead of introducing a second crop direction.
          previousFirstFrame.cropTop = math.min(previousFirstFrame.pixelHeight, previousFirstFrame.cropTop + overlap.overlapRows).toInt();
          previousFirstFrame.seamFeatherRows = math.min(_scrollingCaptureSeamFeatherRows, math.min(previousFirstFrame.cropTop, previousFirstFrame.visibleHeight)).toInt();
          if (previousFirstFrame.visibleHeight <= 0) {
            scrollingCaptureFrames.removeAt(0).dispose();
          }
        }
      }

      if (direction == _ScrollingCaptureDirection.append) {
        scrollingCaptureFrames.add(nextFrame);
      } else {
        scrollingCaptureFrames.insert(0, nextFrame);
      }
      _logScrollingCaptureTiming(traceId, 'append_accepted', {
        'elapsedMs': appendWatch.elapsedMilliseconds,
        'frameIndex': frameIndex,
        'direction': _scrollingCaptureDirectionName(direction),
        'frameCount': scrollingCaptureFrames.length,
        'pixel': '${nextFrame.pixelWidth}x${nextFrame.pixelHeight}',
        'visibleHeight': nextFrame.visibleHeight,
        'cropTop': nextFrame.cropTop,
      });
      await _syncNativeScrollingControlsBounds(traceId, selection);
    } finally {
      isScrollingCaptureUpdating.value = false;
      _logScrollingCaptureTiming(traceId, 'append_finished', {
        'elapsedMs': appendWatch.elapsedMilliseconds,
        'frameIndex': frameIndex,
        'direction': _scrollingCaptureDirectionName(direction),
        'frameCount': scrollingCaptureFrames.length,
      });
    }
  }

  int _scrollingCaptureFrameLimit() {
    if (scrollingCaptureFrames.isEmpty) {
      return _scrollingCaptureMaximumFrameLimit;
    }

    final firstFrame = scrollingCaptureFrames.first;
    final framePixels = firstFrame.pixelWidth * firstFrame.pixelHeight;
    if (framePixels <= 0) {
      return _scrollingCaptureMinimumFrameLimit;
    }

    // The old fixed frame cap was too small for narrow captures. Deriving the limit from the first
    // captured frame's pixel count preserves the previous safety boundary for large selections while
    // giving small selections more room before preview/export updates are intentionally stopped.
    final pixelBudgetLimit = (_scrollingCaptureStoredPixelBudget / framePixels).floor();
    return pixelBudgetLimit.clamp(_scrollingCaptureMinimumFrameLimit, _scrollingCaptureMaximumFrameLimit).toInt();
  }

  double _scrollDeltaToWheelSteps(double scrollDeltaY) {
    final direction = scrollDeltaY >= 0 ? 1.0 : -1.0;
    final magnitude = (scrollDeltaY.abs() / 60).clamp(1.0, _scrollingCaptureWheelSteps);
    return direction * magnitude;
  }

  _ScrollingCaptureDirection _scrollingCaptureDirectionFromDelta(double deltaY) {
    return deltaY < 0 ? _ScrollingCaptureDirection.prepend : _ScrollingCaptureDirection.append;
  }

  String _scrollingCaptureDirectionName(_ScrollingCaptureDirection direction) {
    return direction == _ScrollingCaptureDirection.prepend ? 'prepend' : 'append';
  }

  Future<String> _writeScrollingSelectionPngFile({required String exportFilePath, required List<ScrollingCapturePreviewFrame> frames}) async {
    final pngBytes = await _encodeScrollingFrames(frames);
    final exportFile = File(exportFilePath);
    await exportFile.parent.create(recursive: true);
    await exportFile.writeAsBytes(pngBytes, flush: true);
    return exportFile.path;
  }

  Future<ScrollingCapturePreviewFrame> _captureScrollingSelectionFrame(Rect selection) async {
    final traceId = _scrollingCaptureTraceId;
    final totalWatch = Stopwatch()..start();
    Map<String, ui.Image> decodedImages = <String, ui.Image>{};

    try {
      final captureWatch = Stopwatch()..start();
      final rawSnapshots = await ScreenshotPlatformBridge.instance.captureAllDisplays(traceId: traceId, logicalSelection: ScreenshotRect.fromRect(selection));
      _logScrollingCaptureTiming(traceId, 'capture_all_displays_done', {'elapsedMs': captureWatch.elapsedMilliseconds, 'snapshotCount': rawSnapshots.length});
      if (rawSnapshots.isEmpty) {
        throw StateError('No display snapshots returned while scrolling');
      }

      final normalizeWatch = Stopwatch()..start();
      final nativeWorkspaceBounds = _activeNativeWorkspaceBounds ?? _calculateUnionRect(rawSnapshots.map((snapshot) => snapshot.logicalBounds.toRect()).toList());
      final normalizedSnapshots = _normalizeSnapshotsForWorkspace(
        rawSnapshots,
        nativeWorkspaceBounds: nativeWorkspaceBounds,
        workspaceBounds: virtualBoundsRect,
        workspaceScale: workspaceScale.value,
      );
      final intersectingSnapshots = normalizedSnapshots.where((snapshot) => !snapshot.logicalBounds.toRect().intersect(selection).isEmpty).toList();
      _logScrollingCaptureTiming(traceId, 'capture_normalize_done', {
        'elapsedMs': normalizeWatch.elapsedMilliseconds,
        'snapshotCount': normalizedSnapshots.length,
        'intersectingCount': intersectingSnapshots.length,
        'selection': selection,
      });

      final decodeWatch = Stopwatch()..start();
      decodedImages = await _decodeSnapshotImages(intersectingSnapshots);
      _logScrollingCaptureTiming(traceId, 'capture_decode_done', {'elapsedMs': decodeWatch.elapsedMilliseconds, 'decodedCount': decodedImages.length});

      final composeWatch = Stopwatch()..start();
      final composed = await _composeSelectionImage(
        selection: selection,
        snapshots: normalizedSnapshots,
        annotationsToPaint: const <ScreenshotAnnotation>[],
        decodedImages: decodedImages,
      );
      _logScrollingCaptureTiming(traceId, 'capture_compose_done', {'elapsedMs': composeWatch.elapsedMilliseconds, 'pixel': '${composed.pixelWidth}x${composed.pixelHeight}'});

      final byteWatch = Stopwatch()..start();
      final byteData = await composed.image.toByteData(format: ui.ImageByteFormat.rawRgba);
      _logScrollingCaptureTiming(traceId, 'capture_to_byte_data_done', {'elapsedMs': byteWatch.elapsedMilliseconds, 'pixel': '${composed.pixelWidth}x${composed.pixelHeight}'});
      if (byteData == null) {
        composed.image.dispose();
        throw StateError('Failed to inspect scrolling screenshot frame');
      }

      _logScrollingCaptureTiming(traceId, 'capture_frame_done', {
        'elapsedMs': totalWatch.elapsedMilliseconds,
        'decodedCount': decodedImages.length,
        'pixel': '${composed.pixelWidth}x${composed.pixelHeight}',
      });
      return ScrollingCapturePreviewFrame(
        image: composed.image,
        rgbaBytes: byteData.buffer.asUint8List(byteData.offsetInBytes, byteData.lengthInBytes),
        pixelWidth: composed.pixelWidth,
        pixelHeight: composed.pixelHeight,
      );
    } catch (e) {
      _logScrollingCaptureTiming(traceId, 'capture_frame_failed', {'elapsedMs': totalWatch.elapsedMilliseconds, 'error': e.runtimeType});
      rethrow;
    } finally {
      for (final image in decodedImages.values) {
        image.dispose();
      }
    }
  }

  Future<Map<String, ui.Image>> _decodeSnapshotImages(List<DisplaySnapshot> snapshots) async {
    final decodedImages = <String, ui.Image>{};
    try {
      for (final snapshot in snapshots) {
        if (!snapshot.hasImageBytes) {
          continue;
        }

        final codec = await ui.instantiateImageCodec(snapshot.imageBytes);
        final frame = await codec.getNextFrame();
        decodedImages[snapshot.displayId] = frame.image;
      }
      return decodedImages;
    } catch (_) {
      for (final image in decodedImages.values) {
        image.dispose();
      }
      rethrow;
    }
  }

  _ScrollingCaptureOverlap _findScrollingOverlap(String traceId, ScrollingCapturePreviewFrame previous, ScrollingCapturePreviewFrame next, _ScrollingCaptureDirection direction) {
    final overlapWatch = Stopwatch()..start();
    if (previous.pixelWidth != next.pixelWidth || previous.pixelHeight != next.pixelHeight) {
      _logScrollingCaptureTiming(traceId, 'overlap_dimension_mismatch', {
        'elapsedMs': overlapWatch.elapsedMilliseconds,
        'direction': _scrollingCaptureDirectionName(direction),
        'previous': '${previous.pixelWidth}x${previous.pixelHeight}',
        'next': '${next.pixelWidth}x${next.pixelHeight}',
      });
      return const _ScrollingCaptureOverlap(overlapRows: 0, averageDifference: double.infinity, isReliable: false, isDuplicate: false);
    }

    final match = direction == _ScrollingCaptureDirection.append ? _findAppendTemplateMatchOverlap(previous, next) : _findPrependTemplateMatchOverlap(previous, next);
    final isReliable = match.score >= _scrollingCaptureTemplateMatchThreshold && match.overlapRows > 0 && match.overlapRows <= next.pixelHeight;
    final isDuplicate = isReliable && match.overlapRows >= next.pixelHeight * 0.94;
    _logScrollingCaptureTiming(traceId, 'overlap_template_match_done', {
      'elapsedMs': overlapWatch.elapsedMilliseconds,
      'direction': _scrollingCaptureDirectionName(direction),
      'templateHeight': match.templateHeight,
      'searchMaxY': match.searchMaxY,
      'candidateY': match.candidateY,
      'xShift': match.xShift,
      'overlapRows': match.overlapRows,
      'score': match.score,
      'sampleCount': match.sampleCount,
      'coarseStep': match.coarseStep,
      'refineRadius': match.refineRadius,
      'scoredCandidates': match.scoredCandidates,
      'reliable': isReliable,
      'duplicate': isDuplicate,
    });
    return _ScrollingCaptureOverlap(overlapRows: isReliable ? match.overlapRows : 0, averageDifference: 1 - match.score, isReliable: isReliable, isDuplicate: isDuplicate);
  }

  _ScrollingTemplateMatch _findAppendTemplateMatchOverlap(ScrollingCapturePreviewFrame previous, ScrollingCapturePreviewFrame next) {
    final templateHeight = _scrollingTemplateHeight(previous.visibleHeight);
    final templateTop = previous.pixelHeight - templateHeight;
    final searchMaxY = math.max(0, math.min(next.pixelHeight - templateHeight, (next.pixelHeight * _scrollingCaptureTemplateMatchSearchRatio).floor())).toInt();
    final sample = _buildTemplateMatchSample(previous, templateTop, templateHeight);
    final candidate = _findBestTemplateMatchCandidate(sample, next, searchMaxY);

    // Template matching searches for the previous visible bottom template's top edge inside the next
    // frame. The overlap is therefore the matched top plus the template height; using the inverse
    // would make an unchanged viewport look like only a small overlap and crop most of the image.
    final overlapRows = (candidate.candidateY + templateHeight).clamp(0, next.pixelHeight).toInt();
    return _ScrollingTemplateMatch(
      overlapRows: overlapRows,
      score: candidate.score,
      candidateY: candidate.candidateY,
      xShift: candidate.xShift,
      templateHeight: templateHeight,
      searchMaxY: searchMaxY,
      sampleCount: sample.values.length,
      coarseStep: candidate.coarseStep,
      refineRadius: candidate.refineRadius,
      scoredCandidates: candidate.scoredCandidates,
    );
  }

  _ScrollingTemplateMatch _findPrependTemplateMatchOverlap(ScrollingCapturePreviewFrame previous, ScrollingCapturePreviewFrame next) {
    final templateHeight = _scrollingTemplateHeight(previous.visibleHeight);
    final templateTop = previous.cropTop;
    final searchMaxY = math.max(0, math.min(next.pixelHeight - templateHeight, (next.pixelHeight * _scrollingCaptureTemplateMatchSearchRatio).floor())).toInt();
    final sample = _buildTemplateMatchSample(previous, templateTop, templateHeight);
    final candidate = _findBestTemplateMatchCandidate(sample, next, searchMaxY);

    // Bug fix: upward scrolling matches the old first frame's visible top inside the newly captured
    // viewport. Rows below that match are already present in the old first frame, so they become the
    // overlap removed from the old frame after the new frame is inserted above it.
    final overlapRows = (next.pixelHeight - candidate.candidateY).clamp(0, next.pixelHeight).toInt();
    return _ScrollingTemplateMatch(
      overlapRows: overlapRows,
      score: candidate.score,
      candidateY: candidate.candidateY,
      xShift: candidate.xShift,
      templateHeight: templateHeight,
      searchMaxY: searchMaxY,
      sampleCount: sample.values.length,
      coarseStep: candidate.coarseStep,
      refineRadius: candidate.refineRadius,
      scoredCandidates: candidate.scoredCandidates,
    );
  }

  int _scrollingTemplateHeight(int visibleHeight) {
    return math.max(1, math.min(_scrollingCaptureTemplateMatchMaxHeight, (visibleHeight * _scrollingCaptureTemplateMatchHeightRatio).round())).toInt();
  }

  _ScrollingTemplateCandidate _findBestTemplateMatchCandidate(_ScrollingTemplateSample sample, ScrollingCapturePreviewFrame next, int searchMaxY) {
    var bestScore = double.negativeInfinity;
    var bestCandidateY = 0;
    var bestXShift = 0;
    var scoredCandidates = 0;
    void scoreCandidateY(int candidateY) {
      for (var xShift = -_scrollingCaptureTemplateMatchMaxHorizontalShift; xShift <= _scrollingCaptureTemplateMatchMaxHorizontalShift; xShift++) {
        final score = _templateMatchScore(sample, next, candidateY, xShift);
        scoredCandidates += 1;
        if (score > bestScore) {
          bestScore = score;
          bestCandidateY = candidateY;
          bestXShift = xShift;
        }
      }
    }

    final coarseStep = math.max(1, math.min(_scrollingCaptureTemplateMatchCoarseStep, searchMaxY == 0 ? 1 : searchMaxY)).toInt();
    for (var candidateY = 0; candidateY <= searchMaxY; candidateY += coarseStep) {
      scoreCandidateY(candidateY);
    }
    if (searchMaxY % coarseStep != 0) {
      scoreCandidateY(searchMaxY);
    }

    final coarseBestY = bestCandidateY;
    final refineRadius = math.max(_scrollingCaptureTemplateMatchRefineRadius, coarseStep * 2);
    final refineMinY = math.max(0, coarseBestY - refineRadius).toInt();
    final refineMaxY = math.min(searchMaxY, coarseBestY + refineRadius).toInt();
    for (var candidateY = refineMinY; candidateY <= refineMaxY; candidateY++) {
      scoreCandidateY(candidateY);
    }

    return _ScrollingTemplateCandidate(
      score: bestScore,
      candidateY: bestCandidateY,
      xShift: bestXShift,
      coarseStep: coarseStep,
      refineRadius: refineRadius,
      scoredCandidates: scoredCandidates,
    );
  }

  _ScrollingTemplateSample _buildTemplateMatchSample(ScrollingCapturePreviewFrame frame, int templateTop, int templateHeight) {
    final horizontalPadding = math.min(_scrollingCaptureTemplateMatchMaxHorizontalShift, math.max(0, (frame.pixelWidth - 1) ~/ 2));
    final startX = horizontalPadding;
    final endX = math.max(startX + 1, frame.pixelWidth - horizontalPadding);
    final stepX = math.max(1, ((endX - startX) / _scrollingCaptureTemplateMatchTargetColumns).ceil());
    final stepY = math.max(1, (templateHeight / _scrollingCaptureTemplateMatchTargetRows).ceil());
    final xs = <int>[];
    final ys = <int>[];
    final values = <double>[];
    var sum = 0.0;
    var sumSquares = 0.0;

    for (var relativeY = 0; relativeY < templateHeight; relativeY += stepY) {
      final y = templateTop + relativeY;
      for (var x = startX; x < endX; x += stepX) {
        final luma = _lumaAt(frame, x, y);
        xs.add(x);
        ys.add(relativeY);
        values.add(luma);
        sum += luma;
        sumSquares += luma * luma;
      }
    }

    final count = values.length;
    final variance = count == 0 ? 0.0 : sumSquares - (sum * sum / count);
    return _ScrollingTemplateSample(xs: xs, ys: ys, values: values, sum: sum, variance: variance);
  }

  double _templateMatchScore(_ScrollingTemplateSample sample, ScrollingCapturePreviewFrame frame, int candidateY, int xShift) {
    final count = sample.values.length;
    if (count == 0 || sample.variance <= 0.0001) {
      return double.negativeInfinity;
    }

    var candidateSum = 0.0;
    var candidateSumSquares = 0.0;
    var crossSum = 0.0;
    for (var i = 0; i < count; i++) {
      final candidate = _lumaAt(frame, sample.xs[i] + xShift, candidateY + sample.ys[i]);
      candidateSum += candidate;
      candidateSumSquares += candidate * candidate;
      crossSum += sample.values[i] * candidate;
    }

    final candidateVariance = candidateSumSquares - (candidateSum * candidateSum / count);
    if (candidateVariance <= 0.0001) {
      return double.negativeInfinity;
    }

    // Zero-mean NCC compares texture instead of absolute brightness, which keeps matching stable on
    // mostly white pages where slight capture brightness changes would break raw pixel differences.
    final numerator = crossSum - (sample.sum * candidateSum / count);
    final denominator = math.sqrt(sample.variance * candidateVariance);
    return denominator <= 0 ? double.negativeInfinity : numerator / denominator;
  }

  double _lumaAt(ScrollingCapturePreviewFrame frame, int x, int y) {
    final safeX = x.clamp(0, frame.pixelWidth - 1).toInt();
    final safeY = y.clamp(0, frame.pixelHeight - 1).toInt();
    final offset = _rgbaOffset(frame.pixelWidth, safeX, safeY);
    return frame.rgbaBytes[offset] * 0.299 + frame.rgbaBytes[offset + 1] * 0.587 + frame.rgbaBytes[offset + 2] * 0.114;
  }

  int _rgbaOffset(int width, int x, int y) {
    return (y * width + x) * 4;
  }

  Future<Uint8List> _encodeScrollingFrames(List<ScrollingCapturePreviewFrame> frames) async {
    if (frames.isEmpty) {
      throw StateError('No scrolling screenshot frames were captured');
    }

    final width = frames.first.pixelWidth;
    final height = frames.fold<int>(0, (total, frame) => total + frame.visibleHeight);
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    final paint = Paint();
    var y = 0.0;

    for (final frame in frames) {
      final visibleHeight = frame.visibleHeight;
      if (visibleHeight <= 0) {
        continue;
      }

      _paintScrollingFrame(canvas: canvas, frame: frame, destinationY: y, destinationWidth: frame.pixelWidth.toDouble(), destinationHeight: visibleHeight.toDouble(), paint: paint);
      y += visibleHeight;
    }

    final picture = recorder.endRecording();
    final image = await picture.toImage(width, height);
    try {
      final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
      if (byteData == null) {
        throw StateError('Failed to encode scrolling screenshot');
      }
      return byteData.buffer.asUint8List(byteData.offsetInBytes, byteData.lengthInBytes);
    } finally {
      image.dispose();
    }
  }

  void _paintScrollingFrame({
    required Canvas canvas,
    required ScrollingCapturePreviewFrame frame,
    required double destinationY,
    required double destinationWidth,
    required double destinationHeight,
    required Paint paint,
  }) {
    canvas.drawImageRect(
      frame.image,
      Rect.fromLTWH(0, frame.cropTop.toDouble(), frame.pixelWidth.toDouble(), frame.visibleHeight.toDouble()),
      Rect.fromLTWH(0, destinationY, destinationWidth, destinationHeight),
      paint,
    );

    final featherRows = math.min(frame.seamFeatherRows, math.min(frame.cropTop, frame.visibleHeight)).toInt();
    if (destinationY <= 0 || featherRows <= 0) {
      return;
    }

    final featherHeight = destinationHeight * featherRows / frame.visibleHeight;
    if (featherHeight <= 0 || destinationY < featherHeight) {
      return;
    }

    // Hard cuts expose small overlap errors as horizontal lines. Paint the last overlapped rows of
    // the new frame back over the previous frame with a vertical alpha ramp so the seam transitions
    // through real shared pixels instead of a single hard boundary.
    final destinationRect = Rect.fromLTWH(0, destinationY - featherHeight, destinationWidth, featherHeight);
    canvas.saveLayer(destinationRect, Paint());
    canvas.drawImageRect(frame.image, Rect.fromLTWH(0, (frame.cropTop - featherRows).toDouble(), frame.pixelWidth.toDouble(), featherRows.toDouble()), destinationRect, paint);
    canvas.drawRect(
      destinationRect,
      Paint()
        ..blendMode = BlendMode.dstIn
        ..shader = ui.Gradient.linear(destinationRect.topLeft, destinationRect.bottomLeft, const [Color(0x00000000), Color(0xFF000000)]),
    );
    canvas.restore();
  }

  Future<void> _finishSession(
    String traceId,
    CaptureScreenshotResult result,
    ScreenshotSessionStage finalStage, {
    bool restoreVisibility = true,
    bool windowAlreadyHidden = false,
    String reason = 'unspecified',
  }) async {
    final completer = _sessionCompleter;
    if (completer == null || completer.isCompleted) {
      return;
    }

    stage.value = finalStage;
    await _restoreWindowState(traceId, restoreVisibility: restoreVisibility, windowAlreadyHidden: windowAlreadyHidden);
    _resetSessionState();
    completer.complete(result);
    _sessionCompleter = null;
  }

  Future<void> _restoreWindowState(String traceId, {bool restoreVisibility = true, bool windowAlreadyHidden = false}) async {
    final savedState = _savedWindowState;
    if (savedState == null) {
      Logger.instance.warn(traceId, 'Screenshot restore skipped because no saved window state is available');
      return;
    }

    final launcherController = Get.find<WoxLauncherController>();
    launcherController.forceHideOnBlur = savedState.forceHideOnBlur;

    // The native multi-display selector can stay alive until Flutter confirms its workspace is
    // visible. Closing it here as part of the generic restore path prevents a stuck topmost shade
    // when the screenshot session aborts before that handoff completes.
    await ScreenshotPlatformBridge.instance.dismissNativeSelectionOverlays();
    await ScreenshotPlatformBridge.instance.dismissCaptureWorkspacePresentation();
    await windowManager.setAlwaysOnTop(!savedState.wasInSettingView);
    await windowManager.setBounds(savedState.position, savedState.size);

    if (savedState.wasVisible && restoreVisibility) {
      await windowManager.show();
      await windowManager.focus();
      await WoxApi.instance.onShow(traceId);
      if (savedState.wasInSettingView) {
        Get.find<WoxSettingController>().settingFocusNode.requestFocus();
      } else {
        launcherController.focusQueryBox(selectAll: true);
      }
    } else {
      if (!windowAlreadyHidden) {
        // Screenshot completion should leave Wox hidden. The previous restore path always tried to
        // show the launcher again before the session reset, which made the finished capture linger
        // on-screen and briefly re-opened Wox after the user had already confirmed the export.
        await windowManager.hide();
        await WoxApi.instance.onHide(traceId);
      }
    }
  }

  Future<void> _hideScreenshotWindowBeforeFinish(String traceId) async {
    // Finishing a screenshot changes the reused Wox window back from fullscreen capture bounds to
    // the saved launcher bounds. Hide first so cancel/failure/confirm do not visibly shrink the
    // capture surface before the normal restore path decides whether to show the launcher again.
    final isVisible = await windowManager.isVisible();
    if (!isVisible) {
      return;
    }

    await windowManager.hide();
    await WoxApi.instance.onHide(traceId);
  }

  void _resetSessionState() {
    _savedWindowState = null;
    _activeRequest = null;
    for (final snapshot in displaySnapshots) {
      snapshot.releaseImageCache();
    }
    _clearNativePreparationState();
    _disposeScrollingCaptureFrames();
    isScrollingCaptureUpdating.value = false;
    selectedAnnotationId.value = null;
    editingTextAnnotationId.value = null;
    textDraftPosition.value = null;
    textDraftFontSize.value = 20;
    textDraftColor.value = annotationCreationColor.value;
    textDraftController.clear();
    currentTool.value = ScreenshotTool.select;
    selection.value = null;
    displaySnapshots.clear();
    annotations.clear();
    virtualBounds.value = null;
    workspaceScale.value = 1;
    stage.value = ScreenshotSessionStage.idle;
    isSessionActive.value = false;
    _disposeDecodedImages();
  }

  void _disposeScrollingCaptureFrames() {
    _scrollingCaptureFrameDebounce?.cancel();
    _scrollingCaptureFrameDebounce = null;
    _pendingScrollingCaptureSelection = null;
    _pendingScrollingCaptureDirection = null;
    _scrollingCaptureWheelSubscription?.cancel();
    _scrollingCaptureWheelSubscription = null;
    _scrollingCaptureControlsBounds = null;
    isNativeScrollingCaptureOverlay.value = false;
    _scrollingCaptureTraceId = "";
    for (final frame in scrollingCaptureFrames) {
      frame.dispose();
    }
    scrollingCaptureFrames.clear();
  }

  List<DisplaySnapshot> _normalizeSnapshotsForWorkspace(
    List<DisplaySnapshot> snapshots, {
    required Rect nativeWorkspaceBounds,
    required Rect workspaceBounds,
    required double workspaceScale,
  }) {
    final safeWorkspaceScale = workspaceScale <= 0 ? 1.0 : workspaceScale;

    // Windows capture now reports native virtual-desktop coordinates for every monitor snapshot so
    // one screenshot overlay can span mixed-DPI displays. Normalizing those native coordinates here
    // keeps the widget tree and export logic on one stable workspace contract regardless of the
    // platform-specific capture source.
    return snapshots.map((snapshot) {
      final nativeBounds = snapshot.logicalBounds.toRect();
      final normalizedBounds = Rect.fromLTWH(
        workspaceBounds.left + (nativeBounds.left - nativeWorkspaceBounds.left) / safeWorkspaceScale,
        workspaceBounds.top + (nativeBounds.top - nativeWorkspaceBounds.top) / safeWorkspaceScale,
        nativeBounds.width / safeWorkspaceScale,
        nativeBounds.height / safeWorkspaceScale,
      );

      return snapshot.copyWith(logicalBounds: ScreenshotRect.fromRect(normalizedBounds));
    }).toList();
  }

  List<DisplaySnapshot> _mergeHydratedSnapshotBytes(List<DisplaySnapshot> snapshots) {
    return snapshots.map((snapshot) {
      final hydrated = _hydratedRawSnapshots[snapshot.displayId];
      if (hydrated == null || !hydrated.hasImageBytes || snapshot.imageBytesBase64 == hydrated.imageBytesBase64) {
        return snapshot;
      }

      // Native selection now prewarms only the displays that are likely to be shown next.
      // Merge hydrated bytes back into the normalized snapshot list by display id so the visible
      // workspace and later export both reuse the deferred payloads without rebuilding geometry.
      return snapshot.copyWith(imageBytesBase64: hydrated.imageBytesBase64);
    }).toList();
  }

  DisplaySnapshot _rawSnapshotForDisplayId(String displayId) {
    for (final snapshot in _pendingRawSnapshots) {
      if (snapshot.displayId == displayId) {
        return snapshot;
      }
    }

    throw StateError('Display snapshot $displayId is not available');
  }

  Future<DisplaySnapshot> _ensureRawSnapshotHydrated(String displayId) {
    final hydrated = _hydratedRawSnapshots[displayId];
    if (hydrated != null && hydrated.hasImageBytes) {
      return Future<DisplaySnapshot>.value(hydrated);
    }

    final existingTask = _rawSnapshotHydrationTasks[displayId];
    if (existingTask != null) {
      return existingTask;
    }

    final sessionRevision = _captureSessionRevision;
    final rawSnapshot = _rawSnapshotForDisplayId(displayId);
    late final Future<DisplaySnapshot> hydrationTask;
    hydrationTask = () async {
      final loadedSnapshots = await ScreenshotPlatformBridge.instance.loadDisplaySnapshots([displayId]);
      if (loadedSnapshots.isEmpty) {
        throw StateError('Display snapshot $displayId could not be hydrated');
      }

      final loadedSnapshot = loadedSnapshots.first;
      final hydratedSnapshot = rawSnapshot.copyWith(
        logicalBounds: loadedSnapshot.logicalBounds,
        pixelBounds: loadedSnapshot.pixelBounds,
        scale: loadedSnapshot.scale,
        rotation: loadedSnapshot.rotation,
        imageBytesBase64: loadedSnapshot.imageBytesBase64,
      );

      if (sessionRevision == _captureSessionRevision && _sessionCompleter != null && !_sessionCompleter!.isCompleted) {
        _hydratedRawSnapshots[displayId] = hydratedSnapshot;
      }
      return hydratedSnapshot;
    }().whenComplete(() {
      if (_rawSnapshotHydrationTasks[displayId] == hydrationTask) {
        _rawSnapshotHydrationTasks.remove(displayId);
      }
    });

    _rawSnapshotHydrationTasks[displayId] = hydrationTask;
    return hydrationTask;
  }

  Future<List<DisplaySnapshot>> _hydrateRawSnapshotBatch(List<String> displayIds) async {
    if (displayIds.isEmpty) {
      return const <DisplaySnapshot>[];
    }

    final requestedDisplayIds = displayIds.toSet().toList();
    final pendingDisplayIds = <String>[];
    final resolvedSnapshots = <String, DisplaySnapshot>{};
    for (final displayId in requestedDisplayIds) {
      final hydratedSnapshot = _hydratedRawSnapshots[displayId];
      if (hydratedSnapshot != null && hydratedSnapshot.hasImageBytes) {
        resolvedSnapshots[displayId] = hydratedSnapshot;
        continue;
      }

      pendingDisplayIds.add(displayId);
    }

    if (pendingDisplayIds.isNotEmpty) {
      final sessionRevision = _captureSessionRevision;
      final loadedSnapshots = await ScreenshotPlatformBridge.instance.loadDisplaySnapshots(pendingDisplayIds);
      final loadedSnapshotMap = <String, DisplaySnapshot>{};
      for (final loadedSnapshot in loadedSnapshots) {
        loadedSnapshotMap[loadedSnapshot.displayId] = loadedSnapshot;
      }

      // The original hydration path called the native bridge once per monitor. Batch-loading keeps
      // the metadata-first startup useful on Windows and Linux by collapsing those repeated method
      // channel round-trips into one payload fetch while still updating the per-display cache.
      for (final displayId in pendingDisplayIds) {
        final loadedSnapshot = loadedSnapshotMap[displayId];
        if (loadedSnapshot == null) {
          throw StateError('Display snapshot $displayId could not be hydrated');
        }

        final rawSnapshot = _rawSnapshotForDisplayId(displayId);
        final hydratedSnapshot = rawSnapshot.copyWith(
          logicalBounds: loadedSnapshot.logicalBounds,
          pixelBounds: loadedSnapshot.pixelBounds,
          scale: loadedSnapshot.scale,
          rotation: loadedSnapshot.rotation,
          imageBytesBase64: loadedSnapshot.imageBytesBase64,
        );

        resolvedSnapshots[displayId] = hydratedSnapshot;
        if (sessionRevision == _captureSessionRevision && _sessionCompleter != null && !_sessionCompleter!.isCompleted) {
          _hydratedRawSnapshots[displayId] = hydratedSnapshot;
        }
      }
    }

    return displayIds.map((displayId) {
      final hydratedSnapshot = _hydratedRawSnapshots[displayId] ?? resolvedSnapshots[displayId];
      if (hydratedSnapshot == null) {
        throw StateError('Display snapshot $displayId could not be resolved');
      }
      return hydratedSnapshot;
    }).toList();
  }

  Future<List<DisplaySnapshot>> _hydrateRawSnapshots(List<DisplaySnapshot> rawSnapshots) async {
    if (rawSnapshots.isEmpty) {
      return rawSnapshots;
    }

    _pendingRawSnapshots = rawSnapshots;
    return _hydrateRawSnapshotBatch(rawSnapshots.map((snapshot) => snapshot.displayId).toList());
  }

  Future<void> _ensureSelectionSnapshotsReady(Rect selection) async {
    final snapshotsNeedingHydration =
        displaySnapshots.where((snapshot) {
          return !snapshot.hasImageBytes && !snapshot.logicalBounds.toRect().intersect(selection).isEmpty;
        }).toList();
    if (snapshotsNeedingHydration.isEmpty) {
      return;
    }

    final hydratedSnapshots = await _hydrateRawSnapshotBatch(snapshotsNeedingHydration.map((snapshot) => snapshot.displayId).toList());
    for (final hydratedSnapshot in hydratedSnapshots) {
      DisplaySnapshot? currentSnapshot;
      for (final snapshot in displaySnapshots) {
        if (snapshot.displayId == hydratedSnapshot.displayId) {
          currentSnapshot = snapshot;
          break;
        }
      }
      if (currentSnapshot == null) {
        continue;
      }

      await _ensureDisplayDecoded(currentSnapshot.copyWith(imageBytesBase64: hydratedSnapshot.imageBytesBase64));
    }

    if (_sessionCompleter == null || _sessionCompleter!.isCompleted) {
      return;
    }

    final mergedSnapshots = _mergeHydratedSnapshotBytes(displaySnapshots.toList());
    displaySnapshots.assignAll(mergedSnapshots);
    if (_preparedSnapshots != null) {
      _preparedSnapshots = _mergeHydratedSnapshotBytes(_preparedSnapshots!);
    }
  }

  Future<void> _handleNativeSelectionDisplayHint(String traceId, ScreenshotSelectionDisplayHint hint) async {
    if (!_acceptSelectionDisplayHints || _pendingRawSnapshots.isEmpty) {
      return;
    }

    final nativeWorkspaceBounds = _nativeWorkspaceBounds;
    final displayBounds = hint.displayBounds.toRect();
    if (nativeWorkspaceBounds != null && nativeWorkspaceBounds.intersect(displayBounds).isEmpty) {
      return;
    }

    DisplaySnapshot? targetDisplay;
    for (final snapshot in _pendingRawSnapshots) {
      if (snapshot.displayId == hint.displayId) {
        targetDisplay = snapshot;
        break;
      }
    }
    if (targetDisplay == null) {
      return;
    }

    if (_preparedDisplayId == targetDisplay.displayId && _preparedPresentation != null && _preparedSnapshots != null) {
      return;
    }

    try {
      _logScreenshotTiming(traceId, 'native_selection_hint', {'displayId': targetDisplay.displayId, 'bounds': displayBounds});
      await _prepareNativeDisplayForAnnotation(traceId, targetDisplay);
    } catch (error) {
      Logger.instance.error(traceId, 'Failed to prewarm native screenshot workspace for ${targetDisplay.displayId}: $error');
    }
  }

  Future<void> _prepareNativeDisplayForAnnotation(String traceId, DisplaySnapshot targetDisplay) async {
    final targetBounds = targetDisplay.logicalBounds.toRect();
    if (_preparedDisplayId == targetDisplay.displayId && _preparedDisplayBounds == targetBounds && _preparedPresentation != null && _preparedSnapshots != null) {
      return;
    }

    final revision = ++_preparedDisplayRevision;
    final prepareWatch = Stopwatch()..start();
    final presentation = await ScreenshotPlatformBridge.instance.prepareCaptureWorkspace(ScreenshotRect.fromRect(targetBounds));
    _logScreenshotTiming(traceId, 'native_prepare_display', {'elapsedMs': prepareWatch.elapsedMilliseconds, 'displayId': targetDisplay.displayId, 'bounds': targetBounds});
    final hydrateWatch = Stopwatch()..start();
    await _ensureRawSnapshotHydrated(targetDisplay.displayId);
    _logScreenshotTiming(traceId, 'native_hydrate_display', {'elapsedMs': hydrateWatch.elapsedMilliseconds, 'displayId': targetDisplay.displayId});
    var normalizedSnapshots = _normalizeSnapshotsForWorkspace(
      _pendingRawSnapshots,
      nativeWorkspaceBounds: targetBounds,
      workspaceBounds: presentation.workspaceBounds.toRect(),
      workspaceScale: presentation.workspaceScale,
    );
    normalizedSnapshots = _mergeHydratedSnapshotBytes(normalizedSnapshots);

    DisplaySnapshot? preparedTargetSnapshot;
    for (final snapshot in normalizedSnapshots) {
      if (snapshot.displayId == targetDisplay.displayId) {
        preparedTargetSnapshot = snapshot;
        break;
      }
    }
    if (preparedTargetSnapshot == null) {
      throw StateError('Prepared native display snapshot is missing for ${targetDisplay.displayId}');
    }

    final decodeWatch = Stopwatch()..start();
    await _ensureDisplayDecoded(preparedTargetSnapshot);
    _logScreenshotTiming(traceId, 'native_decode_display', {'elapsedMs': decodeWatch.elapsedMilliseconds, 'displayId': targetDisplay.displayId});

    if (revision != _preparedDisplayRevision || _sessionCompleter == null || _sessionCompleter!.isCompleted) {
      return;
    }

    // Mouse-up used to be the point where Flutter first learned which display would host the
    // annotation editor, so the first visible frame still had to decode and lay out the new
    // backdrop. Warming the hidden workspace here makes the reveal path effectively frame-only.
    _preparedDisplayId = targetDisplay.displayId;
    _preparedDisplayBounds = targetBounds;
    _preparedPresentation = presentation;
    _preparedSnapshots = normalizedSnapshots;
    displaySnapshots.assignAll(normalizedSnapshots);
    virtualBounds.value = ScreenshotRect.fromRect(presentation.workspaceBounds.toRect());
    workspaceScale.value = presentation.workspaceScale;
    stage.value = ScreenshotSessionStage.selecting;
  }

  void _clearNativePreparationState() {
    _acceptSelectionDisplayHints = false;
    _selectionDisplayHintSubscription?.cancel();
    _selectionDisplayHintSubscription = null;
    for (final snapshot in _pendingRawSnapshots) {
      snapshot.releaseImageCache();
    }
    for (final snapshot in _hydratedRawSnapshots.values) {
      snapshot.releaseImageCache();
    }
    for (final snapshot in _preparedSnapshots ?? const <DisplaySnapshot>[]) {
      snapshot.releaseImageCache();
    }
    _pendingRawSnapshots = const <DisplaySnapshot>[];
    _hydratedRawSnapshots.clear();
    _rawSnapshotHydrationTasks.clear();
    _nativeWorkspaceBounds = null;
    _activeNativeWorkspaceBounds = null;
    _preparedDisplayId = null;
    _preparedDisplayBounds = null;
    _preparedPresentation = null;
    _preparedSnapshots = null;
    _preparedDisplayRevision = 0;
  }

  void updateSelection(Rect rect) {
    final clampedRect = _clampRectToBounds(rect, virtualBoundsRect);
    selection.value = ScreenshotRect.fromRect(clampedRect);
    if (stage.value == ScreenshotSessionStage.selecting || stage.value == ScreenshotSessionStage.annotating) {
      stage.value = ScreenshotSessionStage.annotating;
    }
  }

  void setTool(ScreenshotTool tool) {
    currentTool.value = tool;
    if (tool != ScreenshotTool.text) {
      cancelTextDraft();
    }
    if (tool != ScreenshotTool.select) {
      // Switching to a creation tool should immediately show that tool's side settings instead of
      // leaving a previous annotation selected. It also avoids reusing the edit-first drag path when
      // the user's next gesture is meant to draw a new mark.
      selectAnnotation(null);
    }
  }

  void selectAnnotation(String? annotationId) {
    if (annotationId != null && annotationById(annotationId) == null) {
      return;
    }

    selectedAnnotationId.value = annotationId;
    if (annotationId == null || editingTextAnnotationId.value != annotationId) {
      editingTextAnnotationId.value = null;
    }
  }

  void startTextDraft(Offset position, {String? annotationId, String initialText = '', double fontSize = 20, Color? color}) {
    textDraftPosition.value = position;
    editingTextAnnotationId.value = annotationId;
    textDraftFontSize.value = fontSize.clamp(minTextFontSize, maxTextFontSize).toDouble();
    textDraftColor.value = color ?? annotationCreationColor.value;
    textDraftController.text = initialText;
    // Existing text annotations now enter inline editing on a single click. Selecting all text made
    // the visual jump obvious and did not feel like editing the same rendered label, so the caret is
    // placed at the end to keep the editing state visually continuous with the painted text.
    textDraftController.selection = TextSelection.collapsed(offset: initialText.length);
  }

  void cancelTextDraft() {
    textDraftPosition.value = null;
    textDraftController.clear();
    editingTextAnnotationId.value = null;
    textDraftFontSize.value = 20;
    textDraftColor.value = annotationCreationColor.value;
  }

  void commitTextDraft() {
    final position = textDraftPosition.value;
    final text = textDraftController.text.trim();
    if (position == null || text.isEmpty) {
      cancelTextDraft();
      return;
    }

    final editingAnnotationId = editingTextAnnotationId.value;
    if (editingAnnotationId != null) {
      _replaceAnnotationById(editingAnnotationId, (annotation) => annotation.copyWith(text: text, start: position, fontSize: textDraftFontSize.value, color: textDraftColor.value));
    } else {
      annotations.add(
        ScreenshotAnnotation(
          id: const UuidV4().generate(),
          type: ScreenshotAnnotationType.text,
          start: position,
          text: text,
          color: textDraftColor.value,
          fontSize: textDraftFontSize.value,
        ),
      );
    }
    cancelTextDraft();
  }

  void addShapeAnnotation(ScreenshotAnnotationType type, Rect rect) {
    if (rect.width < 2 || rect.height < 2) {
      return;
    }

    annotations.add(ScreenshotAnnotation(id: const UuidV4().generate(), type: type, rect: rect, color: annotationCreationColor.value));
  }

  void addArrowAnnotation(Offset start, Offset end) {
    if ((start - end).distance < 2) {
      return;
    }

    annotations.add(ScreenshotAnnotation(id: const UuidV4().generate(), type: ScreenshotAnnotationType.arrow, start: start, end: end, color: annotationCreationColor.value));
  }

  String addMosaicAnnotation(Offset point) {
    final annotationId = const UuidV4().generate();
    // Mosaic is stored as one brush annotation instead of many tiny rectangles so undo, selection,
    // and export all treat a continuous smear as the single user action that created it.
    annotations.add(
      ScreenshotAnnotation(
        id: annotationId,
        type: ScreenshotAnnotationType.mosaic,
        points: <Offset>[point],
        // Mosaic does not expose user color configuration because it edits the underlying pixels
        // instead of drawing a colored mark. A fixed UI color keeps selection outlines and brush
        // hints consistent without coupling the privacy mask to the annotation palette.
        color: mosaicAnnotationUiColor,
        mosaicRadius: mosaicBrushRadius.value,
      ),
    );
    return annotationId;
  }

  void appendMosaicPoint(String annotationId, Offset point) {
    _replaceAnnotationById(annotationId, (annotation) {
      if (annotation.type != ScreenshotAnnotationType.mosaic) {
        return annotation;
      }

      final nextPoints = _extendMosaicPoints(annotation.points, point);
      if (nextPoints.length == annotation.points.length) {
        return annotation;
      }
      return annotation.copyWith(points: nextPoints);
    });
  }

  void updateMosaicPoints(String annotationId, List<Offset> points) {
    _replaceAnnotationById(annotationId, (annotation) => annotation.type == ScreenshotAnnotationType.mosaic ? annotation.copyWith(points: points) : annotation);
  }

  // Existing annotations now support editing in place, so controller-level update helpers keep
  // geometry and color mutations out of the widget tree and make selection-aware edits reusable.
  void updateSelectedAnnotationColor(Color color) {
    final annotationId = selectedAnnotationId.value;
    if (annotationId == null) {
      annotationCreationColor.value = color;
      return;
    }

    _replaceAnnotationById(annotationId, (annotation) {
      // Mosaic annotations intentionally ignore color edits. Their only editable user setting is
      // brush size, while the stored color remains a fixed UI affordance for outlines and handles.
      if (annotation.type == ScreenshotAnnotationType.mosaic) {
        return annotation;
      }
      return annotation.copyWith(color: color);
    });
    if (editingTextAnnotationId.value == annotationId) {
      textDraftColor.value = color;
    }
  }

  void updateSelectedTextFontSize(double delta) {
    final annotation = selectedAnnotation;
    if (annotation == null || annotation.type != ScreenshotAnnotationType.text) {
      return;
    }

    final nextSize = (annotation.fontSize + delta).clamp(minTextFontSize, maxTextFontSize).toDouble();
    _replaceAnnotationById(annotation.id, (current) => current.copyWith(fontSize: nextSize));
    if (editingTextAnnotationId.value == annotation.id) {
      textDraftFontSize.value = nextSize;
    }
  }

  void setAnnotationCreationColor(Color color) {
    annotationCreationColor.value = color;
  }

  void setMosaicBrushRadius(double radius) {
    // The mosaic radius is session UI state, not part of the global annotation color palette.
    // Clamp it to supported toolbar choices so preview size, hit testing, and exported masks stay
    // aligned even if a future caller tries to set an arbitrary value.
    mosaicBrushRadius.value = _nearestMosaicBrushRadius(radius);
  }

  void updateSelectedMosaicBrushRadius(double radius) {
    final annotation = selectedAnnotation;
    if (annotation == null || annotation.type != ScreenshotAnnotationType.mosaic) {
      setMosaicBrushRadius(radius);
      return;
    }

    final nearestRadius = _nearestMosaicBrushRadius(radius);
    // Existing mosaic masks now expose their size in the annotation edit bar. Updating the stored
    // radius keeps hit testing, selection bounds, live preview, and export all using the same brush.
    _replaceAnnotationById(annotation.id, (current) => current.copyWith(mosaicRadius: nearestRadius));
    mosaicBrushRadius.value = nearestRadius;
  }

  void updateAnnotationRect(String annotationId, Rect rect) {
    _replaceAnnotationById(annotationId, (annotation) => annotation.copyWith(rect: rect));
  }

  void updateArrowPoints(String annotationId, {Offset? start, Offset? end}) {
    _replaceAnnotationById(annotationId, (annotation) => annotation.copyWith(start: start ?? annotation.start, end: end ?? annotation.end));
  }

  void updateTextPosition(String annotationId, Offset position) {
    _replaceAnnotationById(annotationId, (annotation) => annotation.copyWith(start: position));
    if (editingTextAnnotationId.value == annotationId) {
      textDraftPosition.value = position;
    }
  }

  void deleteSelectedAnnotation() {
    final annotationId = selectedAnnotationId.value;
    if (annotationId == null) {
      return;
    }

    final removedEditingAnnotation = editingTextAnnotationId.value == annotationId;
    annotations.removeWhere((annotation) => annotation.id == annotationId);
    selectedAnnotationId.value = null;
    if (removedEditingAnnotation) {
      cancelTextDraft();
    }
  }

  void undoAnnotation() {
    if (annotations.isEmpty) {
      return;
    }
    final removed = annotations.removeLast();
    if (removed.id == selectedAnnotationId.value) {
      selectedAnnotationId.value = null;
    }
    if (removed.id == editingTextAnnotationId.value) {
      cancelTextDraft();
    }
  }

  Rect _normalizeNativeRectForWorkspace(Rect nativeRect, {required Rect nativeWorkspaceBounds, required Rect workspaceBounds, required double workspaceScale}) {
    final safeWorkspaceScale = workspaceScale <= 0 ? 1.0 : workspaceScale;
    return Rect.fromLTWH(
      workspaceBounds.left + (nativeRect.left - nativeWorkspaceBounds.left) / safeWorkspaceScale,
      workspaceBounds.top + (nativeRect.top - nativeWorkspaceBounds.top) / safeWorkspaceScale,
      nativeRect.width / safeWorkspaceScale,
      nativeRect.height / safeWorkspaceScale,
    );
  }

  /// Finds the display whose bounds best contain the given selection rect. Native drag overlays can
  /// span multiple monitors, but the Flutter annotation editor still has to collapse to one target
  /// display so the handoff can prepare and reveal a single stable workspace.
  DisplaySnapshot _findDisplaySnapshotForSelection(Rect selection, List<DisplaySnapshot> snapshots) {
    final center = selection.center;
    for (final snapshot in snapshots) {
      if (snapshot.logicalBounds.toRect().contains(center)) {
        return snapshot;
      }
    }

    DisplaySnapshot? best;
    double bestArea = 0;
    for (final snapshot in snapshots) {
      final intersection = snapshot.logicalBounds.toRect().intersect(selection);
      if (!intersection.isEmpty) {
        final area = intersection.width * intersection.height;
        if (area > bestArea) {
          bestArea = area;
          best = snapshot;
        }
      }
    }
    return best ?? snapshots.first;
  }

  Future<_RenderedSelectionImage> _renderSelectionImage({
    required Rect selection,
    required List<DisplaySnapshot> snapshots,
    required List<ScreenshotAnnotation> annotationsToPaint,
  }) async {
    final composed = await _composeSelectionImage(selection: selection, snapshots: snapshots, annotationsToPaint: annotationsToPaint);
    try {
      final byteData = await composed.image.toByteData(format: ui.ImageByteFormat.png);
      if (byteData == null) {
        throw StateError('Failed to encode exported screenshot');
      }

      final pngBytes = byteData.buffer.asUint8List();
      return _RenderedSelectionImage(pngBytes: pngBytes);
    } finally {
      composed.image.dispose();
    }
  }

  Future<_ComposedSelectionImage> _composeSelectionImage({
    required Rect selection,
    required List<DisplaySnapshot> snapshots,
    required List<ScreenshotAnnotation> annotationsToPaint,
    Map<String, ui.Image>? decodedImages,
  }) async {
    final imageLookup = decodedImages ?? _decodedImages;
    final exportSlices = <_DisplayExportSlice>[];
    for (final snapshot in snapshots) {
      final logicalRect = snapshot.logicalBounds.toRect();
      final intersection = logicalRect.intersect(selection);
      if (intersection.isEmpty) {
        continue;
      }

      final decodedImage = imageLookup[snapshot.displayId];
      if (decodedImage == null) {
        continue;
      }

      final sourceScaleX = decodedImage.width / logicalRect.width;
      final sourceScaleY = decodedImage.height / logicalRect.height;
      final pixelScaleX = snapshot.pixelBounds.width / logicalRect.width;
      final pixelScaleY = snapshot.pixelBounds.height / logicalRect.height;
      final sourceRect = Rect.fromLTWH(
        (intersection.left - logicalRect.left) * sourceScaleX,
        (intersection.top - logicalRect.top) * sourceScaleY,
        intersection.width * sourceScaleX,
        intersection.height * sourceScaleY,
      );
      final destRect = Rect.fromLTWH(
        snapshot.pixelBounds.x + (intersection.left - logicalRect.left) * pixelScaleX,
        snapshot.pixelBounds.y + (intersection.top - logicalRect.top) * pixelScaleY,
        intersection.width * pixelScaleX,
        intersection.height * pixelScaleY,
      );

      exportSlices.add(
        _DisplayExportSlice(
          image: decodedImage,
          logicalRect: logicalRect,
          intersectionRect: intersection,
          sourceRect: sourceRect,
          destRect: destRect,
          pixelScaleX: pixelScaleX,
          pixelScaleY: pixelScaleY,
        ),
      );
    }

    if (exportSlices.isEmpty) {
      throw StateError('Selection does not intersect any captured display');
    }

    final pixelUnion = _calculateUnionRect(exportSlices.map((item) => item.destRect).toList());
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    final paint = Paint();

    for (final slice in exportSlices) {
      canvas.drawImageRect(slice.image, slice.sourceRect, slice.destRect.shift(-pixelUnion.topLeft), paint);
    }

    for (final slice in exportSlices) {
      canvas.save();
      final localDestRect = slice.destRect.shift(-pixelUnion.topLeft);
      canvas.clipRect(localDestRect);
      // Exported annotations are painted in selection-local logical coordinates. The previous
      // translation anchored them to the full display origin, which pushed shapes/text outside the
      // exported crop whenever the selection started away from the monitor's top-left corner.
      // Align each slice to the slice/selection intersection instead so mixed-DPI exports keep the
      // same annotation positions the user saw in the editor and clipboard output.
      canvas.translate(
        localDestRect.left - (slice.intersectionRect.left - selection.left) * slice.pixelScaleX,
        localDestRect.top - (slice.intersectionRect.top - selection.top) * slice.pixelScaleY,
      );
      canvas.scale(slice.pixelScaleX, slice.pixelScaleY);
      // Mosaic annotations need the original display pixels while normal marks only need geometry.
      // Passing the active export slice into the shared annotation painter keeps preview and PNG
      // output on the same brush model without re-sampling unrelated monitor regions.
      paintScreenshotAnnotations(
        canvas,
        annotationsToPaint,
        selection.topLeft,
        mosaicSources: <ScreenshotMosaicSource>[
          ScreenshotMosaicSource(image: slice.image, sourceRect: slice.sourceRect, destinationRect: slice.intersectionRect.shift(-selection.topLeft)),
        ],
      );
      canvas.restore();
    }

    final picture = recorder.endRecording();
    final image = await picture.toImage(pixelUnion.width.ceil(), pixelUnion.height.ceil());
    return _ComposedSelectionImage(image: image, pixelWidth: pixelUnion.width.ceil(), pixelHeight: pixelUnion.height.ceil());
  }

  Future<void> _decodeDisplayImages(List<DisplaySnapshot> snapshots) async {
    _disposeDecodedImages();
    for (final snapshot in snapshots) {
      if (!snapshot.hasImageBytes) {
        continue;
      }
      await _ensureDisplayDecoded(snapshot);
    }
  }

  Future<void> _ensureDisplayDecoded(DisplaySnapshot snapshot) {
    if (!snapshot.hasImageBytes) {
      return Future<void>.value();
    }

    final decodedImage = _decodedImages[snapshot.displayId];
    if (decodedImage != null) {
      return Future<void>.value();
    }

    final existingTask = _displayDecodeTasks[snapshot.displayId];
    if (existingTask != null) {
      return existingTask;
    }

    final decodeTask = _decodeDisplayImage(snapshot).whenComplete(() {
      _displayDecodeTasks.remove(snapshot.displayId);
    });
    _displayDecodeTasks[snapshot.displayId] = decodeTask;
    return decodeTask;
  }

  Future<void> _decodeDisplayImage(DisplaySnapshot snapshot) async {
    final codec = await ui.instantiateImageCodec(snapshot.imageBytes);
    final frame = await codec.getNextFrame();
    final previousImage = _decodedImages[snapshot.displayId];
    if (previousImage != null) {
      previousImage.dispose();
    }
    _decodedImages[snapshot.displayId] = frame.image;
  }

  void _disposeDecodedImages() {
    for (final image in _decodedImages.values) {
      image.dispose();
    }
    _decodedImages.clear();
    _displayDecodeTasks.clear();
  }

  Rect _calculateUnionRect(List<Rect> rects) {
    var left = rects.first.left;
    var top = rects.first.top;
    var right = rects.first.right;
    var bottom = rects.first.bottom;

    for (final rect in rects.skip(1)) {
      left = left < rect.left ? left : rect.left;
      top = top < rect.top ? top : rect.top;
      right = right > rect.right ? right : rect.right;
      bottom = bottom > rect.bottom ? bottom : rect.bottom;
    }

    return Rect.fromLTRB(left, top, right, bottom);
  }

  Rect _clampRectToBounds(Rect rect, Rect bounds) {
    final normalized = Rect.fromPoints(rect.topLeft, rect.bottomRight);
    final left = normalized.left.clamp(bounds.left, bounds.right);
    final top = normalized.top.clamp(bounds.top, bounds.bottom);
    final right = normalized.right.clamp(bounds.left, bounds.right);
    final bottom = normalized.bottom.clamp(bounds.top, bounds.bottom);
    return Rect.fromLTRB(left, top, right, bottom);
  }

  List<Offset> _extendMosaicPoints(List<Offset> points, Offset point) {
    if (points.isEmpty) {
      return <Offset>[point];
    }

    final lastPoint = points.last;
    final distance = (point - lastPoint).distance;
    if (distance < screenshotMosaicPointSpacing) {
      return points;
    }

    final stepCount = math.max(1, (distance / screenshotMosaicPointSpacing).ceil());
    final nextPoints = List<Offset>.of(points);
    // Pointer updates can be sparse when the user moves quickly. Interpolating brush centers keeps
    // the mosaic mask continuous instead of leaving unpixelated gaps between drag events.
    for (var step = 1; step <= stepCount; step++) {
      nextPoints.add(Offset.lerp(lastPoint, point, step / stepCount)!);
    }
    return nextPoints;
  }

  double _nearestMosaicBrushRadius(double radius) {
    var nearestRadius = screenshotMosaicBrushRadii.first;
    var nearestDistance = (nearestRadius - radius).abs();
    for (final option in screenshotMosaicBrushRadii.skip(1)) {
      final distance = (option - radius).abs();
      if (distance < nearestDistance) {
        nearestRadius = option;
        nearestDistance = distance;
      }
    }
    return nearestRadius;
  }

  void _replaceAnnotationById(String annotationId, ScreenshotAnnotation Function(ScreenshotAnnotation annotation) replace) {
    final index = annotations.indexWhere((annotation) => annotation.id == annotationId);
    if (index < 0) {
      return;
    }

    annotations[index] = replace(annotations[index]);
    annotations.refresh();
  }

  @override
  void onClose() {
    _clearNativePreparationState();
    _disposeScrollingCaptureFrames();
    _disposeDecodedImages();
    textDraftController.dispose();
    super.onClose();
  }

  Future<void> resetForIntegrationTest() async {
    if (_sessionCompleter != null && !_sessionCompleter!.isCompleted) {
      _sessionCompleter!.complete(CaptureScreenshotResult.cancelled());
    }
    _sessionCompleter = null;
    _savedWindowState = null;
    _resetSessionState();
    ScreenshotPlatformBridge.resetInstance();
  }
}

class _SavedScreenshotWindowState {
  const _SavedScreenshotWindowState({required this.wasVisible, required this.wasInSettingView, required this.position, required this.size, required this.forceHideOnBlur});

  final bool wasVisible;
  final bool wasInSettingView;
  final Offset position;
  final Size size;
  final bool forceHideOnBlur;
}

class _DisplayExportSlice {
  const _DisplayExportSlice({
    required this.image,
    required this.logicalRect,
    required this.intersectionRect,
    required this.sourceRect,
    required this.destRect,
    required this.pixelScaleX,
    required this.pixelScaleY,
  });

  final ui.Image image;
  final Rect logicalRect;
  final Rect intersectionRect;
  final Rect sourceRect;
  final Rect destRect;
  final double pixelScaleX;
  final double pixelScaleY;
}

class _RenderedSelectionImage {
  const _RenderedSelectionImage({required this.pngBytes});

  final Uint8List pngBytes;
}

class _ComposedSelectionImage {
  const _ComposedSelectionImage({required this.image, required this.pixelWidth, required this.pixelHeight});

  final ui.Image image;
  final int pixelWidth;
  final int pixelHeight;
}

class ScrollingCapturePreviewFrame {
  ScrollingCapturePreviewFrame({required this.image, required this.rgbaBytes, required this.pixelWidth, required this.pixelHeight});

  final ui.Image image;
  final Uint8List rgbaBytes;
  final int pixelWidth;
  final int pixelHeight;
  int cropTop = 0;
  int seamFeatherRows = 0;

  int get visibleHeight => math.max(0, pixelHeight - cropTop);

  void dispose() {
    image.dispose();
  }
}

class _ScrollingCaptureOverlap {
  const _ScrollingCaptureOverlap({required this.overlapRows, required this.averageDifference, required this.isReliable, required this.isDuplicate});

  final int overlapRows;
  final double averageDifference;
  final bool isReliable;
  final bool isDuplicate;
}

class _ScrollingTemplateMatch {
  const _ScrollingTemplateMatch({
    required this.overlapRows,
    required this.score,
    required this.candidateY,
    required this.xShift,
    required this.templateHeight,
    required this.searchMaxY,
    required this.sampleCount,
    required this.coarseStep,
    required this.refineRadius,
    required this.scoredCandidates,
  });

  final int overlapRows;
  final double score;
  final int candidateY;
  final int xShift;
  final int templateHeight;
  final int searchMaxY;
  final int sampleCount;
  final int coarseStep;
  final int refineRadius;
  final int scoredCandidates;
}

class _ScrollingTemplateCandidate {
  const _ScrollingTemplateCandidate({
    required this.score,
    required this.candidateY,
    required this.xShift,
    required this.coarseStep,
    required this.refineRadius,
    required this.scoredCandidates,
  });

  final double score;
  final int candidateY;
  final int xShift;
  final int coarseStep;
  final int refineRadius;
  final int scoredCandidates;
}

class _ScrollingTemplateSample {
  const _ScrollingTemplateSample({required this.xs, required this.ys, required this.values, required this.sum, required this.variance});

  final List<int> xs;
  final List<int> ys;
  final List<double> values;
  final double sum;
  final double variance;
}

class ScreenshotMosaicSource {
  const ScreenshotMosaicSource({required this.image, required this.sourceRect, required this.destinationRect});

  // A mosaic source maps real screenshot pixels to the current canvas coordinate space. The painter
  // samples from sourceRect and stretches tiny samples into destination blocks to create pixelation.
  final ui.Image image;
  final Rect sourceRect;
  final Rect destinationRect;
}

void paintScreenshotAnnotations(
  Canvas canvas,
  List<ScreenshotAnnotation> annotations,
  Offset selectionOrigin, {
  List<ScreenshotMosaicSource> mosaicSources = const <ScreenshotMosaicSource>[],
}) {
  for (final annotation in annotations) {
    final paint =
        Paint()
          ..color = annotation.color
          ..strokeWidth = annotation.strokeWidth
          ..style = annotation.type == ScreenshotAnnotationType.text ? PaintingStyle.fill : PaintingStyle.stroke
          ..strokeCap = StrokeCap.round
          ..strokeJoin = StrokeJoin.round;

    switch (annotation.type) {
      case ScreenshotAnnotationType.rect:
        if (annotation.rect != null) {
          canvas.drawRect(annotation.rect!.shift(-selectionOrigin), paint);
        }
        break;
      case ScreenshotAnnotationType.ellipse:
        if (annotation.rect != null) {
          canvas.drawOval(annotation.rect!.shift(-selectionOrigin), paint);
        }
        break;
      case ScreenshotAnnotationType.arrow:
        final start = annotation.start;
        final end = annotation.end;
        if (start == null || end == null) {
          break;
        }

        final localStart = start - selectionOrigin;
        final localEnd = end - selectionOrigin;
        canvas.drawLine(localStart, localEnd, paint);
        final angle = (localEnd - localStart).direction;
        const arrowLength = 16.0;
        final arrowLeft = localEnd - Offset.fromDirection(angle - 0.5, arrowLength);
        final arrowRight = localEnd - Offset.fromDirection(angle + 0.5, arrowLength);
        canvas.drawLine(localEnd, arrowLeft, paint);
        canvas.drawLine(localEnd, arrowRight, paint);
        break;
      case ScreenshotAnnotationType.text:
        final start = annotation.start;
        final textPainter = buildScreenshotTextPainter(annotation);
        if (start == null || textPainter == null) {
          break;
        }

        textPainter.paint(canvas, start - selectionOrigin);
        break;
      case ScreenshotAnnotationType.mosaic:
        _paintMosaicAnnotation(canvas, annotation, selectionOrigin, mosaicSources);
        break;
    }
  }
}

void _paintMosaicAnnotation(Canvas canvas, ScreenshotAnnotation annotation, Offset selectionOrigin, List<ScreenshotMosaicSource> mosaicSources) {
  final bounds = _mosaicAnnotationBounds(annotation)?.shift(-selectionOrigin);
  if (bounds == null || bounds.isEmpty) {
    return;
  }

  final brushPath = _mosaicAnnotationPath(annotation, selectionOrigin);
  if (mosaicSources.isEmpty) {
    _paintMosaicFallback(canvas, brushPath, annotation.color);
    return;
  }

  canvas.save();
  canvas.clipPath(brushPath);
  final paint = Paint()..filterQuality = FilterQuality.none;
  for (final source in mosaicSources) {
    final paintBounds = bounds.intersect(source.destinationRect);
    if (paintBounds.isEmpty) {
      continue;
    }
    _paintMosaicSource(canvas, source, paintBounds, paint);
  }
  canvas.restore();
}

Path _mosaicAnnotationPath(ScreenshotAnnotation annotation, Offset selectionOrigin) {
  final radius = math.max(1.0, annotation.mosaicRadius);
  final path = Path();
  // The brush mask is stored as dense circles rather than a stroked Path because both Flutter
  // preview and export need the same filled clip region for pixel replacement.
  for (final point in annotation.points) {
    path.addOval(Rect.fromCircle(center: point - selectionOrigin, radius: radius));
  }
  return path;
}

void _paintMosaicSource(Canvas canvas, ScreenshotMosaicSource source, Rect bounds, Paint paint) {
  final blockSize = math.max(1.0, screenshotMosaicBlockSize);
  final startX = (bounds.left / blockSize).floorToDouble() * blockSize;
  final startY = (bounds.top / blockSize).floorToDouble() * blockSize;

  for (var y = startY; y < bounds.bottom; y += blockSize) {
    for (var x = startX; x < bounds.right; x += blockSize) {
      final blockRect = Rect.fromLTWH(x, y, blockSize, blockSize).intersect(bounds).intersect(source.destinationRect);
      if (blockRect.isEmpty) {
        continue;
      }
      canvas.drawImageRect(source.image, _mosaicSampleRect(source, blockRect.center), blockRect, paint);
    }
  }
}

Rect _mosaicSampleRect(ScreenshotMosaicSource source, Offset destinationPoint) {
  final destinationRect = source.destinationRect;
  final sourceRect = source.sourceRect;
  final xRatio = ((destinationPoint.dx - destinationRect.left) / destinationRect.width).clamp(0.0, 1.0).toDouble();
  final yRatio = ((destinationPoint.dy - destinationRect.top) / destinationRect.height).clamp(0.0, 1.0).toDouble();
  final sampleWidth = math.min(sourceRect.width, math.max(1.0, sourceRect.width / destinationRect.width));
  final sampleHeight = math.min(sourceRect.height, math.max(1.0, sourceRect.height / destinationRect.height));
  final centerX = sourceRect.left + sourceRect.width * xRatio;
  final centerY = sourceRect.top + sourceRect.height * yRatio;
  final left = (centerX - sampleWidth / 2).clamp(sourceRect.left, sourceRect.right - sampleWidth).toDouble();
  final top = (centerY - sampleHeight / 2).clamp(sourceRect.top, sourceRect.bottom - sampleHeight).toDouble();
  return Rect.fromLTWH(left, top, sampleWidth, sampleHeight);
}

void _paintMosaicFallback(Canvas canvas, Path brushPath, Color color) {
  // If a test or delayed hydration path paints before source pixels are available, show the exact
  // brush footprint in the annotation color instead of pretending to pixelate unknown content.
  canvas.drawPath(brushPath, Paint()..color = color.withValues(alpha: 0.24));
  canvas.drawPath(
    brushPath,
    Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5,
  );
}

// Text annotations now support selection, drag, and inline editing. Sharing the exact same text
// style between painter and editor keeps the caret overlay visually merged with the rendered label
// instead of swapping between two slightly different text appearances.
TextStyle buildScreenshotTextStyle({required Color color, required double fontSize}) {
  return TextStyle(color: color, fontSize: fontSize, fontWeight: FontWeight.w600, shadows: const [Shadow(color: Color(0xAA000000), blurRadius: 4)]);
}

TextPainter? buildScreenshotTextPainter(ScreenshotAnnotation annotation) {
  final start = annotation.start;
  final text = annotation.text;
  if (start == null || text == null || text.isEmpty) {
    return null;
  }

  return TextPainter(text: TextSpan(text: text, style: buildScreenshotTextStyle(color: annotation.color, fontSize: annotation.fontSize)), textDirection: TextDirection.ltr)
    ..layout(maxWidth: 480);
}

Rect? screenshotAnnotationBounds(ScreenshotAnnotation annotation) {
  switch (annotation.type) {
    case ScreenshotAnnotationType.rect:
    case ScreenshotAnnotationType.ellipse:
      return annotation.rect;
    case ScreenshotAnnotationType.arrow:
      final start = annotation.start;
      final end = annotation.end;
      if (start == null || end == null) {
        return null;
      }
      return Rect.fromPoints(start, end);
    case ScreenshotAnnotationType.text:
      final start = annotation.start;
      final textPainter = buildScreenshotTextPainter(annotation);
      if (start == null || textPainter == null) {
        return null;
      }
      return start & textPainter.size;
    case ScreenshotAnnotationType.mosaic:
      return _mosaicAnnotationBounds(annotation);
  }
}

Rect? _mosaicAnnotationBounds(ScreenshotAnnotation annotation) {
  if (annotation.points.isEmpty) {
    return null;
  }

  var left = annotation.points.first.dx;
  var top = annotation.points.first.dy;
  var right = annotation.points.first.dx;
  var bottom = annotation.points.first.dy;
  for (final point in annotation.points.skip(1)) {
    left = math.min(left, point.dx);
    top = math.min(top, point.dy);
    right = math.max(right, point.dx);
    bottom = math.max(bottom, point.dy);
  }

  final radius = math.max(1.0, annotation.mosaicRadius);
  return Rect.fromLTRB(left - radius, top - radius, right + radius, bottom + radius);
}
