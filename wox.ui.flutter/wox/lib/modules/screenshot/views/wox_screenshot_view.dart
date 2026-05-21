import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/utils/log.dart';

const Key screenshotCanvasKey = Key('screenshot-canvas');
const Key screenshotToolbarKey = Key('screenshot-toolbar');
const Key screenshotEditBarKey = Key('screenshot-edit-bar');
const Key screenshotConfirmKey = Key('screenshot-confirm');
const Key screenshotScrollingCaptureKey = Key('screenshot-scrolling-capture');
const Key screenshotPinKey = Key('screenshot-pin');
const Key screenshotCancelKey = Key('screenshot-cancel');
const Key screenshotUndoKey = Key('screenshot-undo');
const Key screenshotToolSelectKey = Key('screenshot-tool-select');
const Key screenshotToolRectKey = Key('screenshot-tool-rect');
const Key screenshotToolEllipseKey = Key('screenshot-tool-ellipse');
const Key screenshotToolArrowKey = Key('screenshot-tool-arrow');
const Key screenshotToolTextKey = Key('screenshot-tool-text');
const Key screenshotToolMosaicKey = Key('screenshot-tool-mosaic');

const List<Color> _annotationPalette = <Color>[Color(0xFFFF5B36), Color(0xFFF9C74F), Color(0xFF29FF72), Color(0xFF4DA3FF), Color(0xFFC77DFF), Color(0xFFFFFFFF)];
const double _selectionHandleSize = 12;
const double _annotationHandleSize = 12;
const double _selectionEdgeTolerance = 7;
const double _textDraftMaxWidth = 480;
const double _scrollingPreviewMaxWidth = 320;
const double _scrollingNativeToolbarSlotHeight = 72;
const double _screenshotToolbarIconSize = 24;
const double _screenshotMosaicToolIconPaintSize = 18;
const MethodChannel _macOSWindowManagerChannel = MethodChannel('com.wox.macos_window_manager');
const MouseCursor _macOSResizeUpLeftDownRightCursor = _MacOSDiagonalResizeCursor('resizeUpLeftDownRight');
const MouseCursor _macOSResizeUpRightDownLeftCursor = _MacOSDiagonalResizeCursor('resizeUpRightDownLeft');

class _MacOSDiagonalResizeCursor extends MouseCursor {
  const _MacOSDiagonalResizeCursor(this.kind);

  final String kind;

  @override
  MouseCursorSession createSession(int device) => _MacOSDiagonalResizeCursorSession(this, device);

  @override
  String get debugDescription => 'MacOSDiagonalResizeCursor($kind)';

  @override
  bool operator ==(Object other) => other is _MacOSDiagonalResizeCursor && other.kind == kind;

  @override
  int get hashCode => kind.hashCode;
}

class _MacOSDiagonalResizeCursorSession extends MouseCursorSession {
  _MacOSDiagonalResizeCursorSession(_MacOSDiagonalResizeCursor super.cursor, super.device);

  @override
  _MacOSDiagonalResizeCursor get cursor => super.cursor as _MacOSDiagonalResizeCursor;

  @override
  Future<void> activate() async {
    try {
      // Flutter's macOS cursor table does not map the diagonal resize cursors, so selection corner
      // handles used to activate a plain arrow even though the hit test was correct. Route only
      // those screenshot-specific diagonal handles through a native cursor image while keeping the
      // normal system cursor path for supported edge resize cursors.
      await _macOSWindowManagerChannel.invokeMethod<void>('activateScreenshotDiagonalResizeCursor', {'kind': cursor.kind});
    } on MissingPluginException {
      // Widget tests and older runners do not install the native cursor method. Leaving activation
      // as a no-op keeps the interaction code testable and lets the next supported cursor update
      // restore the platform cursor without crashing the screenshot session.
    }
  }

  @override
  void dispose() {}
}

class WoxScreenshotView extends StatefulWidget {
  const WoxScreenshotView({super.key});

  @override
  State<WoxScreenshotView> createState() => _WoxScreenshotViewState();
}

enum _InteractionMode {
  createSelection,
  moveSelection,
  resizeSelection,
  createAnnotation,
  paintMosaic,
  moveAnnotation,
  resizeShapeAnnotation,
  moveArrowStart,
  moveArrowEnd,
  moveText,
}

enum _ResizeHandle { topLeft, top, topRight, right, bottomRight, bottom, bottomLeft, left }

enum _AnnotationHandle { topLeft, top, topRight, right, bottomRight, bottom, bottomLeft, left, arrowStart, arrowEnd }

class _AnnotationHitTarget {
  const _AnnotationHitTarget({required this.annotation, this.handle, required this.cursor});

  final ScreenshotAnnotation annotation;
  final _AnnotationHandle? handle;
  final MouseCursor cursor;
}

class _WoxScreenshotViewState extends State<WoxScreenshotView> {
  final controller = Get.find<WoxScreenshotController>();
  final focusNode = FocusNode(debugLabel: 'screenshot-workspace');
  final _mosaicBrushCenter = ValueNotifier<Offset?>(null);
  bool _isCancellingSession = false;
  bool _isConfirmingSession = false;
  bool _isScrollingCaptureSession = false;
  bool _isPinningSession = false;

  _InteractionMode? _interactionMode;
  _ResizeHandle? _resizeHandle;
  _AnnotationHandle? _annotationHandle;
  Offset? _dragStartGlobal;
  Rect? _selectionAtDragStart;
  Rect? _annotationDraftRect;
  Offset? _annotationStart;
  Offset? _annotationEnd;
  String? _dragAnnotationId;
  String? _mosaicAnnotationId;
  ScreenshotAnnotation? _annotationAtDragStart;
  MouseCursor _hoverCursor = SystemMouseCursors.basic;

  @override
  void initState() {
    super.initState();
    HardwareKeyboard.instance.addHandler(_handleGlobalScreenshotKeyEvent);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        focusNode.requestFocus();
      }
    });
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_handleGlobalScreenshotKeyEvent);
    _mosaicBrushCenter.dispose();
    focusNode.dispose();
    super.dispose();
  }

  bool _handleGlobalScreenshotKeyEvent(KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return false;
    }

    if (event.logicalKey == LogicalKeyboardKey.escape) {
      // The screenshot workflow used to rely on the workspace Focus node receiving Escape. That was
      // not reliable once annotation text fields took focus or a toolbar click left the page without a
      // stable primary focus, so Escape stopped dismissing the active screenshot session. A dedicated
      // screenshot-level HardwareKeyboard handler keeps the cancel shortcut working anywhere inside the
      // annotation UI without changing the rest of the keyboard flow.
      _cancelSessionFromKeyboard();
      return true;
    }

    if (event.logicalKey == LogicalKeyboardKey.enter) {
      // Confirming a screenshot is a session-level action just like Escape. Keeping Enter in the
      // global screenshot handler avoids the focus-dependent path where toolbar clicks or stale
      // launcher focus made the workspace Focus node miss the key press entirely.
      return _confirmSelectionFromKeyboard();
    }

    return false;
  }

  void _cancelSessionFromKeyboard() {
    if (_isCancellingSession || !controller.isSessionActive.value) {
      return;
    }

    _isCancellingSession = true;
    controller.cancelSession(const UuidV4().generate(), reason: 'keyboard_escape').whenComplete(() {
      if (mounted) {
        _isCancellingSession = false;
      }
    });
  }

  bool _confirmSelectionFromKeyboard() {
    if (_isConfirmingSession || !controller.isSessionActive.value || controller.selectionRect == null || controller.textDraftPosition.value != null) {
      return false;
    }

    _isConfirmingSession = true;
    final traceId = const UuidV4().generate();
    // Bug fix: Enter used to always run the normal selection export, so a scrolling screenshot
    // preview could show the full stitched image while the clipboard received only the current
    // viewport. Route keyboard confirmation through the same scrolling export path as the toolbar
    // button so both confirmation surfaces copy the stitched frame list.
    final confirmFuture = controller.stage.value == ScreenshotSessionStage.scrolling ? controller.confirmScrollingSelection(traceId) : controller.confirmSelection(traceId);
    confirmFuture.whenComplete(() {
      if (mounted) {
        _isConfirmingSession = false;
      }
    });
    return true;
  }

  void _confirmSelectionFromAutoConfirm() {
    final selectionRect = controller.selectionRect;
    if (_isConfirmingSession || !controller.isSessionActive.value || selectionRect == null || selectionRect.width < 1 || selectionRect.height < 1) {
      return;
    }

    // AutoConfirm is intentionally wired to the same controller method as the confirm button. The
    // old plugin flow forced a second click even when no annotation was desired; this keeps export
    // cleanup identical while letting capture-only API callers finish on mouse-up.
    _isConfirmingSession = true;
    controller.confirmSelection(const UuidV4().generate()).whenComplete(() {
      if (mounted) {
        _isConfirmingSession = false;
      }
    });
  }

  void _startScrollingSelectionFromToolbar() {
    if (_isScrollingCaptureSession || !controller.isSessionActive.value || controller.selectionRect == null) {
      return;
    }

    if (controller.textDraftPosition.value != null) {
      controller.commitTextDraft();
    }

    setState(() {
      _isScrollingCaptureSession = true;
    });
    controller.startScrollingCapture(const UuidV4().generate()).whenComplete(() {
      if (mounted) {
        setState(() {
          _isScrollingCaptureSession = false;
        });
      }
    });
  }

  void _pinSelectionFromToolbar() {
    if (_isPinningSession || !controller.isSessionActive.value || controller.selectionRect == null || controller.stage.value == ScreenshotSessionStage.scrolling) {
      return;
    }

    if (controller.textDraftPosition.value != null) {
      controller.commitTextDraft();
    }

    setState(() {
      _isPinningSession = true;
    });
    controller.pinSelection(const UuidV4().generate()).whenComplete(() {
      if (mounted) {
        setState(() {
          _isPinningSession = false;
        });
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final stage = controller.stage.value;
      final virtualBounds = controller.virtualBoundsRect;

      return Focus(
        focusNode: focusNode,
        autofocus: true,
        onKeyEvent: _handleWorkspaceKeyEvent,
        child: Material(
          color: Colors.transparent,
          child: stage == ScreenshotSessionStage.loading ? _LoadingView(label: controller.tr('plugin_screenshot_capture_title')) : _buildWorkspace(context, virtualBounds),
        ),
      );
    });
  }

  KeyEventResult _handleWorkspaceKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) {
      return KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.escape) {
      _cancelSessionFromKeyboard();
      return KeyEventResult.handled;
    }

    if (controller.textDraftPosition.value != null) {
      return KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.enter) {
      return _confirmSelectionFromKeyboard() ? KeyEventResult.handled : KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.delete || event.logicalKey == LogicalKeyboardKey.backspace) {
      controller.deleteSelectedAnnotation();
      return KeyEventResult.handled;
    }

    if ((HardwareKeyboard.instance.isControlPressed || HardwareKeyboard.instance.isMetaPressed) && event.logicalKey == LogicalKeyboardKey.keyZ) {
      controller.undoAnnotation();
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  Widget _buildWorkspace(BuildContext context, Rect virtualBounds) {
    if (controller.isNativeScrollingCaptureOverlay.value) {
      return _buildNativeScrollingControls();
    }

    final isScrollingCapture = controller.stage.value == ScreenshotSessionStage.scrolling;
    final scrollingSelectionLocalRect = isScrollingCapture ? controller.selectionRect?.shift(-virtualBounds.topLeft) : null;
    final hideSessionChrome = controller.activeRequest?.autoConfirm ?? false;

    return Stack(
      children: [
        MouseRegion(
          cursor: _hoverCursor,
          onHover: (event) => _handleHover(event.localPosition),
          onExit: (_) {
            _setHoverCursor(SystemMouseCursors.basic);
            _setMosaicBrushCenter(null);
          },
          child: Listener(
            onPointerSignal: _handlePointerSignal,
            child: GestureDetector(
              key: screenshotCanvasKey,
              behavior: HitTestBehavior.translucent,
              onPanStart: (details) => _handlePanStart(details.localPosition),
              onPanUpdate: (details) => _handlePanUpdate(details.localPosition),
              onPanEnd: (_) => _handlePanEnd(),
              onTapDown: (details) => _handleTap(details.localPosition),
              onDoubleTapDown: (details) => _handleDoubleTap(details.localPosition),
              child: Stack(
                children: [
                  RepaintBoundary(
                    // Dragging annotations used to visually disturb the captured background because the
                    // entire workspace repainted on every pointer update. Keeping the snapshots isolated in
                    // their own repaint boundary limits redraws to overlays that actually changed.
                    child: _WorkspaceBackground(snapshots: controller.displaySnapshots.toList(), virtualBounds: virtualBounds, clearLocalRect: scrollingSelectionLocalRect),
                  ),
                  Obx(() {
                    final selectionRect = controller.selectionRect;
                    final selectionLocalRect = selectionRect?.shift(-virtualBounds.topLeft);
                    final textDraftPosition = controller.textDraftPosition.value;
                    final selectedAnnotationId = controller.selectedAnnotationId.value;
                    final isScrollingCapture = controller.stage.value == ScreenshotSessionStage.scrolling;
                    final currentTool = controller.currentTool.value;
                    final mosaicBrushRadius = controller.mosaicBrushRadius.value;

                    return Stack(
                      children: [
                        Positioned.fill(
                          child: CustomPaint(
                            painter: _WorkspaceShadePainter(
                              selectionRect: selectionLocalRect,
                              selectionSizeLabel: selectionRect == null ? null : '${selectionRect.width.round()} x ${selectionRect.height.round()}',
                            ),
                          ),
                        ),
                        if (selectionRect != null && !isScrollingCapture)
                          Positioned.fill(
                            child: RepaintBoundary(
                              child: CustomPaint(
                                painter: _AnnotationPainter(
                                  annotations: controller.annotations.toList(),
                                  snapshots: controller.displaySnapshots.toList(),
                                  decodedImages: controller.decodedDisplayImages,
                                  canvasOrigin: virtualBounds.topLeft,
                                  selectionClipRect: selectionLocalRect,
                                  draftRect: _annotationDraftRect,
                                  draftStart: _annotationStart,
                                  draftEnd: _annotationEnd,
                                  draftType: _currentDraftType(),
                                  previewColor: controller.annotationCreationColor.value,
                                  selectedAnnotationId: selectedAnnotationId,
                                  editingTextAnnotationId: controller.editingTextAnnotationId.value,
                                ),
                              ),
                            ),
                          ),
                        if (selectionRect != null && !isScrollingCapture && currentTool == ScreenshotTool.mosaic)
                          Positioned.fill(
                            child: IgnorePointer(
                              // The brush-size marker is isolated from the annotation painter. Hover
                              // updates now repaint only this tiny overlay instead of rebuilding the
                              // screenshot workspace, so the circle can stay visible without making
                              // the mosaic tool lag behind the mouse.
                              child: RepaintBoundary(
                                child: ValueListenableBuilder<Offset?>(
                                  valueListenable: _mosaicBrushCenter,
                                  builder: (context, center, child) {
                                    return CustomPaint(
                                      painter: _MosaicBrushMarkerPainter(
                                        center: center == null ? null : center - virtualBounds.topLeft,
                                        radius: mosaicBrushRadius,
                                        color: WoxScreenshotController.mosaicAnnotationUiColor,
                                      ),
                                    );
                                  },
                                ),
                              ),
                            ),
                          ),
                        if (isScrollingCapture && selectionLocalRect != null) _buildScrollingSelectionFrame(selectionLocalRect),
                        if (selectionLocalRect != null) _buildSelectionFrame(selectionLocalRect),
                        if (isScrollingCapture && selectionLocalRect != null) _buildScrollingPreview(selectionLocalRect),
                        if (textDraftPosition != null && selectionRect != null && !isScrollingCapture) _buildTextDraftField(textDraftPosition - virtualBounds.topLeft),
                      ],
                    );
                  }),
                ],
              ),
            ),
          ),
        ),
        // The toolbar and edit bar used to live inside the canvas GestureDetector, so clicking a
        // swatch or action button also triggered the workspace tap handler and cleared the current
        // annotation selection first. Lifting those overlays above the gesture layer keeps their
        // controls interactive without letting canvas hit-testing cancel the active edit target.
        if (!hideSessionChrome) ...[
          // AutoConfirm sessions should complete as soon as the rectangle is chosen. The previous
          // implementation still built the overlay controls for one frame before confirmSelection
          // hid the window, which made the toolbar flash even though the user never needed it.
          Obx(() => _buildToolbar(context, controller.selectionRect, virtualBounds)),
          Obx(() => _buildEditBar(virtualBounds)),
        ],
      ],
    );
  }

  Widget _buildNativeScrollingControls() {
    final frames = controller.scrollingCaptureFrames.toList();
    final totalHeight = frames.fold<int>(0, (total, frame) => total + frame.visibleHeight);

    return Material(
      color: Colors.transparent,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final previewSize = _calculateScrollingPreviewRenderSize(
            frames: frames,
            totalHeight: totalHeight,
            maxWidth: constraints.maxWidth,
            // Bug fix: Windows clips the compact scrolling panel to the preview plus toolbar areas
            // to match macOS' transparent controls window. Keep the Flutter preview height aligned
            // with the controller/native reserved toolbar slot so the clipped region does not expose
            // an unpainted backing area below the image.
            maxHeight: math.max(1.0, constraints.maxHeight - _scrollingNativeToolbarSlotHeight),
          );

          return Stack(
            children: [
              Positioned(
                left: (constraints.maxWidth - previewSize.width) / 2,
                top: 0,
                width: previewSize.width,
                height: previewSize.height,
                child: DecoratedBox(
                  decoration: BoxDecoration(color: Colors.transparent, border: Border.all(color: const Color(0xCCFFFFFF), width: 2)),
                  child:
                      frames.isEmpty
                          ? const Center(child: SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2.4, color: Color(0xFF4DA3FF))))
                          : CustomPaint(painter: _ScrollingPreviewPainter(frames: frames, totalHeight: totalHeight, traceId: controller.scrollingCaptureTraceId)),
                ),
              ),
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                height: 56,
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                    decoration: BoxDecoration(color: const Color(0xE61E1A18), borderRadius: BorderRadius.circular(18)),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        _ToolButton(
                          key: screenshotCancelKey,
                          icon: Icons.close,
                          color: const Color(0xFFFF6B6B),
                          tooltip: controller.tr('ui_screenshot_tool_cancel'),
                          onPressed: () => controller.cancelSession(const UuidV4().generate(), reason: 'scrolling_native_toolbar_cancel'),
                        ),
                        _ToolButton(
                          key: screenshotConfirmKey,
                          icon: Icons.check,
                          color: const Color(0xFF30E37A),
                          enabled: frames.isNotEmpty,
                          tooltip: controller.tr('ui_screenshot_tool_confirm'),
                          onPressed: frames.isNotEmpty ? () => controller.confirmScrollingSelection(const UuidV4().generate()) : null,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  Size _calculateScrollingPreviewRenderSize({required List<ScrollingCapturePreviewFrame> frames, required int totalHeight, required double maxWidth, required double maxHeight}) {
    if (frames.isEmpty || totalHeight <= 0) {
      final fallbackSize = math.min(120.0, math.min(math.max(1.0, maxWidth), math.max(1.0, maxHeight)));
      return Size(fallbackSize, fallbackSize);
    }

    final contentWidth = frames.first.pixelWidth.toDouble();
    final contentHeight = totalHeight.toDouble();
    final safeMaxWidth = math.max(1.0, maxWidth);
    final safeMaxHeight = math.max(1.0, maxHeight);
    final scale = math.min(safeMaxWidth / math.max(1.0, contentWidth), safeMaxHeight / math.max(1.0, contentHeight));

    // The native preview panel is transparent outside this child. Returning the exact rendered image
    // size prevents the old first-frame panel width from showing as gray gutters after the stitched
    // screenshot grows taller and the scaled image becomes narrower.
    return Size(math.max(1.0, contentWidth * scale), math.max(1.0, contentHeight * scale));
  }

  Widget _buildToolbar(BuildContext context, Rect? selectionRect, Rect virtualBounds) {
    final currentTool = controller.currentTool.value;
    final isScrollingCapture = controller.stage.value == ScreenshotSessionStage.scrolling;
    final canConfirm = selectionRect != null && selectionRect.width >= 1 && selectionRect.height >= 1 && (!isScrollingCapture || controller.scrollingCaptureFrames.isNotEmpty);
    final selectionLocalRect = selectionRect?.shift(-virtualBounds.topLeft);
    final hideAnnotationToolbar = controller.activeRequest?.hideAnnotationToolbar ?? false;
    final showBuiltInPinAction = controller.activeRequest?.callerIcon == null && !hideAnnotationToolbar;
    final canPin = showBuiltInPinAction && selectionRect != null && !isScrollingCapture && !_isPinningSession;

    return Positioned.fill(
      child: CustomSingleChildLayout(
        delegate: _SelectionToolbarLayoutDelegate(selectionRect: selectionLocalRect),
        // The creation toolbar stays attached to the active capture rect so tool switching and new
        // annotation placement happen near the selected region instead of forcing long pointer
        // travel to the edge of the screen.
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Container(
            key: screenshotToolbarKey,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: const Color(0xCC1E1A18),
              borderRadius: BorderRadius.circular(18),
              boxShadow: const [BoxShadow(color: Color(0x55000000), blurRadius: 24, offset: Offset(0, 12))],
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (controller.activeRequest?.callerIcon != null) ...[
                  // Plugin API screenshot sessions carry the resolved caller icon so users can tell
                  // this toolbox belongs to a third-party request. Wox-owned captures omit the field,
                  // which keeps the built-in screenshot toolbar unchanged.
                  _CallerIcon(icon: controller.activeRequest!.callerIcon!),
                  const SizedBox(width: 10),
                ],
                if (!hideAnnotationToolbar) ...[
                  // Plugin callers can hide markup controls for raw capture workflows. Confirm and
                  // cancel stay outside this branch because even simplified API sessions still need
                  // an explicit user-controlled finish path when AutoConfirm is disabled.
                  _ToolButton(
                    key: screenshotToolSelectKey,
                    icon: Icons.select_all,
                    selected: currentTool == ScreenshotTool.select,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_select'),
                    onPressed: () => controller.setTool(ScreenshotTool.select),
                  ),
                  _ToolButton(
                    key: screenshotToolRectKey,
                    icon: Icons.crop_square,
                    selected: currentTool == ScreenshotTool.rect,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_rectangle'),
                    onPressed: () => controller.setTool(ScreenshotTool.rect),
                  ),
                  _ToolButton(
                    key: screenshotToolEllipseKey,
                    icon: Icons.circle_outlined,
                    selected: currentTool == ScreenshotTool.ellipse,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_ellipse'),
                    onPressed: () => controller.setTool(ScreenshotTool.ellipse),
                  ),
                  _ToolButton(
                    key: screenshotToolTextKey,
                    icon: Icons.text_fields,
                    selected: currentTool == ScreenshotTool.text,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_text'),
                    onPressed: () => controller.setTool(ScreenshotTool.text),
                  ),
                  _ToolButton(
                    key: screenshotToolArrowKey,
                    icon: Icons.north_east,
                    selected: currentTool == ScreenshotTool.arrow,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_arrow'),
                    onPressed: () => controller.setTool(ScreenshotTool.arrow),
                  ),
                  _ToolButton(
                    key: screenshotToolMosaicKey,
                    iconBuilder: (foreground) => _MosaicToolIcon(color: foreground),
                    selected: currentTool == ScreenshotTool.mosaic,
                    activateOnTapDown: true,
                    tooltip: controller.tr('ui_screenshot_tool_mosaic'),
                    onPressed: () {
                      // Tool changes usually happen over the toolbar, not the canvas. Clear any
                      // stale marker so re-entering mosaic mode waits for the next real canvas hover.
                      _setMosaicBrushCenter(null);
                      controller.setTool(ScreenshotTool.mosaic);
                    },
                  ),
                  const SizedBox(width: 6),
                  _ToolButton(
                    key: screenshotUndoKey,
                    icon: Icons.undo,
                    enabled: controller.annotations.isNotEmpty,
                    tooltip: controller.tr('ui_screenshot_tool_undo'),
                    onPressed: controller.undoAnnotation,
                  ),
                  const SizedBox(width: 6),
                  // Scrolling capture is exposed as a selection action instead of a drawing tool
                  // because it exports a stitched live page after Wox hides, so keeping it beside
                  // confirm/cancel matches the point where the selected region becomes final. The
                  // double-ended arrow tile reads as vertical expansion; down-arrow glyphs looked
                  // like download/export actions and conflicted with this workflow.
                  _ToolButton(
                    key: screenshotScrollingCaptureKey,
                    iconBuilder: (foreground) => _ScrollingCaptureToolIcon(color: foreground),
                    selected: isScrollingCapture,
                    enabled: selectionRect != null && !_isScrollingCaptureSession && !isScrollingCapture,
                    tooltip: controller.tr('ui_screenshot_tool_scrolling_capture'),
                    onPressed: selectionRect != null ? _startScrollingSelectionFromToolbar : null,
                  ),
                ],
                if (showBuiltInPinAction) ...[
                  const SizedBox(width: 6),
                  // Pin is intentionally a completion action, not an annotation tool. Keeping it
                  // beside confirm/cancel makes the toolbar flow explicit: draw a region, then
                  // either copy it, pin it as a desktop overlay, or cancel the session. It still
                  // uses the default white toolbar foreground so only destructive/confirm actions
                  // carry special colors.
                  _ToolButton(
                    key: screenshotPinKey,
                    icon: Icons.push_pin_outlined,
                    enabled: canPin,
                    tooltip: controller.tr('ui_screenshot_tool_pin'),
                    onPressed: canPin ? _pinSelectionFromToolbar : null,
                  ),
                ],
                _ToolButton(
                  key: screenshotCancelKey,
                  icon: Icons.close,
                  color: const Color(0xFFFF6B6B),
                  tooltip: controller.tr('ui_screenshot_tool_cancel'),
                  onPressed: () => controller.cancelSession(const UuidV4().generate(), reason: 'toolbar_cancel_button'),
                ),
                _ToolButton(
                  key: screenshotConfirmKey,
                  icon: Icons.check,
                  color: const Color(0xFF30E37A),
                  enabled: canConfirm,
                  tooltip: controller.tr('ui_screenshot_tool_confirm'),
                  onPressed:
                      canConfirm
                          ? () {
                            if (controller.textDraftPosition.value != null) {
                              controller.commitTextDraft();
                            }
                            if (controller.stage.value == ScreenshotSessionStage.scrolling) {
                              controller.confirmScrollingSelection(const UuidV4().generate());
                            } else {
                              controller.confirmSelection(const UuidV4().generate());
                            }
                          }
                          : null,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildEditBar(Rect virtualBounds) {
    final selectionRect = controller.selectionRect;
    final selectedAnnotation = controller.selectedAnnotation;
    final currentTool = controller.currentTool.value;
    final showCreationConfig = selectedAnnotation == null && _isAnnotationCreationTool(currentTool);
    if (selectionRect == null || (selectedAnnotation == null && !showCreationConfig)) {
      return const SizedBox.shrink();
    }

    final selectionLocalRect = selectionRect.shift(-virtualBounds.topLeft);
    final annotationLocalRect = selectedAnnotation == null ? null : screenshotAnnotationBounds(selectedAnnotation)?.shift(-virtualBounds.topLeft);
    final isMosaicConfig = selectedAnnotation?.type == ScreenshotAnnotationType.mosaic || (selectedAnnotation == null && currentTool == ScreenshotTool.mosaic);
    final editBarWidth = isMosaicConfig ? 116.0 : 92.0;

    return Positioned.fill(
      child: CustomSingleChildLayout(
        delegate: _SelectionEditBarLayoutDelegate(selectionRect: selectionLocalRect, anchorRect: annotationLocalRect),
        // The side bar now serves both creation tools and selected annotations. Clicking a bottom
        // tool should expose its creation settings before drawing, while clicking an existing mark
        // still reuses the same space for edit/delete actions anchored near that annotation.
        child: Container(
          key: screenshotEditBarKey,
          width: editBarWidth,
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 12),
          decoration: BoxDecoration(
            color: const Color(0xD91B1715),
            borderRadius: BorderRadius.circular(18),
            boxShadow: const [BoxShadow(color: Color(0x55000000), blurRadius: 20, offset: Offset(0, 10))],
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: selectedAnnotation != null ? _buildSelectedAnnotationEditActions(selectedAnnotation) : _buildCreationToolEditActions(currentTool),
          ),
        ),
      ),
    );
  }

  List<Widget> _buildSelectedAnnotationEditActions(ScreenshotAnnotation selectedAnnotation) {
    final actions = <Widget>[];

    if (selectedAnnotation.type != ScreenshotAnnotationType.mosaic) {
      // Annotation color editing belongs to the selected annotation's side bar. Mosaic is excluded
      // because its visible result is pixelation rather than a colored stroke, so size is the only
      // user-facing setting that changes the privacy mask.
      actions.add(_buildColorPalette(selectedColor: selectedAnnotation.color, onColorSelected: controller.updateSelectedAnnotationColor));
    }

    if (selectedAnnotation.type == ScreenshotAnnotationType.mosaic) {
      actions.addAll([_buildMosaicBrushSizePicker(selectedRadius: selectedAnnotation.mosaicRadius, onRadiusSelected: controller.updateSelectedMosaicBrushRadius)]);
    }

    if (selectedAnnotation.type == ScreenshotAnnotationType.text) {
      actions.addAll([
        const SizedBox(height: 10),
        _EditActionButton(icon: Icons.remove, tooltip: controller.tr('ui_screenshot_tool_decrease_text'), onPressed: () => controller.updateSelectedTextFontSize(-2)),
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Text('${selectedAnnotation.fontSize.round()}', style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
        ),
        _EditActionButton(icon: Icons.add, tooltip: controller.tr('ui_screenshot_tool_increase_text'), onPressed: () => controller.updateSelectedTextFontSize(2)),
      ]);
    }

    actions.addAll([
      const SizedBox(height: 10),
      _EditActionButton(
        icon: Icons.delete_outline,
        color: const Color(0xFFFF6B6B),
        tooltip: controller.tr('ui_screenshot_tool_delete_annotation'),
        onPressed: controller.deleteSelectedAnnotation,
      ),
    ]);
    return actions;
  }

  List<Widget> _buildCreationToolEditActions(ScreenshotTool currentTool) {
    if (currentTool == ScreenshotTool.mosaic) {
      // Mosaic creation exposes brush size only. Keeping color out of this path avoids implying
      // that pixelation can be tinted, while the fixed green marker still communicates the radius.
      return <Widget>[_buildMosaicBrushSizePicker(selectedRadius: controller.mosaicBrushRadius.value, onRadiusSelected: controller.setMosaicBrushRadius)];
    }

    return <Widget>[
      // Creation settings use the same side bar so users can choose color first, then draw. These
      // controls update the defaults used by the next annotation instead of editing an existing one.
      _buildColorPalette(selectedColor: controller.annotationCreationColor.value, onColorSelected: controller.setAnnotationCreationColor),
    ];
  }

  Widget _buildScrollingPreview(Rect selectionLocalRect) {
    final frames = controller.scrollingCaptureFrames.toList();
    final totalHeight = frames.fold<int>(0, (total, frame) => total + frame.visibleHeight);
    final screenSize = MediaQuery.sizeOf(context);
    final rightAvailableWidth = math.max(0.0, screenSize.width - selectionLocalRect.right - 44);
    final leftAvailableWidth = math.max(0.0, selectionLocalRect.left - 44);
    final maxAvailableWidth = math.min(math.max(rightAvailableWidth, leftAvailableWidth), _scrollingPreviewMaxWidth);
    // Match the native scrolling preview layout: the old selection-height cap made the preview stop
    // growing at the selected rectangle even though the dimmed workspace above and below was empty.
    // Using the full masked viewport keeps the fallback Flutter preview consistent with macOS, while
    // capping width prevents the preview from becoming a second full-size page copy.
    final maxPreviewHeight = math.max(1.0, screenSize.height - 48);
    final previewSize = _calculateScrollingPreviewRenderSize(frames: frames, totalHeight: totalHeight, maxWidth: maxAvailableWidth, maxHeight: maxPreviewHeight);

    final hasRightSpace = selectionLocalRect.right + 20 + previewSize.width <= screenSize.width - 24;
    final left = hasRightSpace ? selectionLocalRect.right + 20 : math.max(24.0, selectionLocalRect.left - previewSize.width - 20);
    final top = selectionLocalRect.top.clamp(24.0, math.max(24.0, screenSize.height - previewSize.height - 24)).toDouble();

    return Positioned(
      left: left,
      top: top,
      width: previewSize.width,
      height: previewSize.height,
      child: IgnorePointer(
        // The preview is a live rendering aid, not an editing surface. Keeping it pointer-transparent
        // prevents it from stealing wheel gestures that should continue driving the selected page.
        child: DecoratedBox(
          decoration: BoxDecoration(color: Colors.transparent, border: Border.all(color: const Color(0xCCFFFFFF), width: 2)),
          child:
              frames.isEmpty
                  ? const Center(child: SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2.4, color: Color(0xFF4DA3FF))))
                  : CustomPaint(painter: _ScrollingPreviewPainter(frames: frames, totalHeight: totalHeight, traceId: controller.scrollingCaptureTraceId)),
        ),
      ),
    );
  }

  Widget _buildScrollingSelectionFrame(Rect selectionLocalRect) {
    final frames = controller.scrollingCaptureFrames;
    if (frames.isEmpty) {
      return Positioned.fromRect(
        rect: selectionLocalRect,
        child: const ColoredBox(
          color: Color(0x11FFFFFF),
          child: Center(child: SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2.4, color: Color(0xFF4DA3FF)))),
        ),
      );
    }

    final latestFrame = frames.last;
    return Positioned.fromRect(
      rect: selectionLocalRect,
      child: IgnorePointer(
        // The selected region is a live viewport. Paint the latest captured crop back into it so
        // users see page movement immediately even when the native overlay cannot expose a truly
        // transparent hole through the Flutter window.
        child: RawImage(image: latestFrame.image, fit: BoxFit.fill, filterQuality: FilterQuality.medium),
      ),
    );
  }

  Widget _buildColorPalette({required Color selectedColor, required ValueChanged<Color> onColorSelected, bool compact = false}) {
    final paletteChildren =
        _annotationPalette
            .map((color) => _ColorSwatchButton(color: color, selected: selectedColor.toARGB32() == color.toARGB32(), onPressed: () => onColorSelected(color), compact: compact))
            .toList();

    return SizedBox(width: compact ? 56 : 72, child: Wrap(spacing: compact ? 4 : 8, runSpacing: compact ? 4 : 8, alignment: WrapAlignment.center, children: paletteChildren));
  }

  Widget _buildMosaicBrushSizePicker({required double selectedRadius, required ValueChanged<double> onRadiusSelected}) {
    final options = <({double radius, String tooltip})>[
      (radius: screenshotMosaicBrushRadii[0], tooltip: controller.tr('ui_screenshot_tool_mosaic_brush_small')),
      (radius: screenshotMosaicBrushRadii[1], tooltip: controller.tr('ui_screenshot_tool_mosaic_brush_medium')),
      (radius: screenshotMosaicBrushRadii[2], tooltip: controller.tr('ui_screenshot_tool_mosaic_brush_large')),
    ];

    // The picker is shared by mosaic creation and selected mosaic editing. Compact circle buttons
    // keep the toolbox icon-driven while still making the current privacy brush radius explicit.
    return SizedBox(
      width: 96,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children:
            options.map((option) {
              return _MosaicBrushSizeButton(
                radius: option.radius,
                selected: (selectedRadius - option.radius).abs() < 0.1,
                tooltip: option.tooltip,
                onPressed: () => onRadiusSelected(option.radius),
              );
            }).toList(),
      ),
    );
  }

  Widget _buildSelectionFrame(Rect selectionLocalRect) {
    const borderColor = Color(0xFF29FF72);

    final handles = _ResizeHandle.values.map((handle) {
      final position = _handleOffsetForRect(selectionLocalRect, handle);
      return Positioned(
        left: position.dx - _selectionHandleSize / 2,
        top: position.dy - _selectionHandleSize / 2,
        child: Container(
          width: _selectionHandleSize,
          height: _selectionHandleSize,
          decoration: BoxDecoration(color: borderColor, border: Border.all(color: Colors.black.withValues(alpha: 0.45), width: 1), borderRadius: BorderRadius.circular(4)),
        ),
      );
    });

    return Stack(
      children: [
        Positioned.fromRect(
          rect: selectionLocalRect,
          child: IgnorePointer(
            // The previous frame decoration used a full-rect box shadow, which visually bled into
            // the selected area and made the "transparent" capture region look slightly grey. Keep
            // the frame to a pure stroke so the user sees the raw screenshot pixels inside the crop.
            child: Container(decoration: BoxDecoration(border: Border.all(color: borderColor, width: 2))),
          ),
        ),
        ...handles,
      ],
    );
  }

  Widget _buildTextDraftField(Offset localPosition) {
    return Positioned(
      left: localPosition.dx,
      top: localPosition.dy,
      child: MouseRegion(
        cursor: SystemMouseCursors.text,
        child: Material(
          type: MaterialType.transparency,
          child: ConstrainedBox(
            // The draft editor now sits directly on top of the rendered text instead of inside a
            // decorated popup. Matching the painter width cap keeps wrapping identical while the
            // transparent field makes the edit state look like the same annotation gaining a caret.
            constraints: const BoxConstraints(minWidth: 24, maxWidth: _textDraftMaxWidth),
            child: TextField(
              controller: controller.textDraftController,
              autofocus: true,
              maxLines: null,
              minLines: 1,
              cursorColor: controller.textDraftColor.value,
              style: buildScreenshotTextStyle(color: controller.textDraftColor.value, fontSize: controller.textDraftFontSize.value),
              decoration: const InputDecoration.collapsed(hintText: ''),
              onSubmitted: (_) => controller.commitTextDraft(),
              onTapOutside: (_) => controller.commitTextDraft(),
            ),
          ),
        ),
      ),
    );
  }

  void _handleTap(Offset localPosition) {
    if (controller.stage.value == ScreenshotSessionStage.scrolling) {
      return;
    }

    if (controller.textDraftPosition.value != null) {
      return;
    }

    final globalPosition = _toGlobalPosition(localPosition);
    final selectedHandleHit = _hitTestSelectedAnnotationHandle(globalPosition);
    if (selectedHandleHit != null) {
      // GestureDetector fires tap-down before pan-start. Ellipse corner handles sit outside the
      // ellipse body hit region, so tap-down used to clear the selected annotation before dragging
      // could resize it. Preserve the selected handle here and let pan-start choose the resize path.
      return;
    }

    final annotation = _hitTestAnnotationBody(globalPosition);
    if (annotation != null) {
      // Text annotations should feel editable in place. A single click now both selects the label
      // and opens the inline editor so the cursor changes to text mode immediately instead of
      // forcing the user through a separate double-click gesture.
      if (annotation.type == ScreenshotAnnotationType.text && annotation.start != null) {
        _selectAnnotationForEdit(annotation);
        controller.startTextDraft(annotation.start!, annotationId: annotation.id, initialText: annotation.text ?? '', fontSize: annotation.fontSize, color: annotation.color);
        return;
      }

      _selectAnnotationForEdit(annotation);
      return;
    }

    switch (controller.currentTool.value) {
      case ScreenshotTool.select:
        controller.selectAnnotation(null);
        return;
      case ScreenshotTool.rect:
      case ScreenshotTool.ellipse:
      case ScreenshotTool.arrow:
      case ScreenshotTool.mosaic:
        controller.selectAnnotation(null);
        return;
      case ScreenshotTool.text:
        final selectionRect = controller.selectionRect;
        if (selectionRect == null || !selectionRect.contains(globalPosition)) {
          controller.selectAnnotation(null);
          return;
        }

        controller.selectAnnotation(null);
        controller.startTextDraft(globalPosition, fontSize: controller.textDraftFontSize.value, color: controller.annotationCreationColor.value);
        return;
    }
  }

  void _handleDoubleTap(Offset localPosition) {
    if (controller.stage.value == ScreenshotSessionStage.scrolling) {
      return;
    }

    final globalPosition = _toGlobalPosition(localPosition);
    final annotation = _hitTestAnnotationBody(globalPosition, allowOnlyText: true);
    if (annotation == null || annotation.type != ScreenshotAnnotationType.text || annotation.start == null) {
      return;
    }

    controller.selectAnnotation(annotation.id);
    controller.startTextDraft(annotation.start!, annotationId: annotation.id, initialText: annotation.text ?? '', fontSize: annotation.fontSize, color: annotation.color);
  }

  void _handlePanStart(Offset localPosition) {
    if (controller.stage.value == ScreenshotSessionStage.scrolling) {
      return;
    }

    if (controller.textDraftPosition.value != null) {
      controller.commitTextDraft();
    }

    final selectionRect = controller.selectionRect;
    final globalPosition = _toGlobalPosition(localPosition);
    _dragStartGlobal = globalPosition;

    // Annotation editing now depends on an explicit tap-based selection instead of whichever tool
    // drew the mark. Once something is selected, keep its handles interactive even if the toolbar
    // still shows a creation tool so follow-up edits do not require a separate mode switch.
    final selectedHandleHit = _hitTestSelectedAnnotationHandle(globalPosition);
    if (selectedHandleHit != null) {
      _dragAnnotationId = selectedHandleHit.annotation.id;
      _annotationAtDragStart = selectedHandleHit.annotation;
      _annotationHandle = selectedHandleHit.handle;
      _interactionMode =
          selectedHandleHit.handle == _AnnotationHandle.arrowStart
              ? _InteractionMode.moveArrowStart
              : selectedHandleHit.handle == _AnnotationHandle.arrowEnd
              ? _InteractionMode.moveArrowEnd
              : _InteractionMode.resizeShapeAnnotation;
      return;
    }

    final selectedAnnotation = controller.selectedAnnotation;
    if (selectedAnnotation != null && _annotationContainsPoint(selectedAnnotation, globalPosition)) {
      _dragAnnotationId = selectedAnnotation.id;
      _annotationAtDragStart = selectedAnnotation;
      _interactionMode = selectedAnnotation.type == ScreenshotAnnotationType.text ? _InteractionMode.moveText : _InteractionMode.moveAnnotation;
      return;
    }

    switch (controller.currentTool.value) {
      case ScreenshotTool.select:
        final annotationBodyHit = _hitTestAnnotationBody(globalPosition);
        if (annotationBodyHit != null) {
          _selectAnnotationForEdit(annotationBodyHit);
          _dragAnnotationId = annotationBodyHit.id;
          _annotationAtDragStart = annotationBodyHit;
          _interactionMode = annotationBodyHit.type == ScreenshotAnnotationType.text ? _InteractionMode.moveText : _InteractionMode.moveAnnotation;
          return;
        }

        controller.selectAnnotation(null);
        if (selectionRect != null) {
          final handle = _hitTestSelectionHandle(selectionRect, globalPosition);
          if (handle != null) {
            _interactionMode = _InteractionMode.resizeSelection;
            _resizeHandle = handle;
            _selectionAtDragStart = selectionRect;
            return;
          }
          if (selectionRect.contains(globalPosition)) {
            _interactionMode = _InteractionMode.moveSelection;
            _selectionAtDragStart = selectionRect;
            return;
          }
        }

        _interactionMode = _InteractionMode.createSelection;
        controller.updateSelection(Rect.fromPoints(globalPosition, globalPosition));
        break;
      case ScreenshotTool.rect:
      case ScreenshotTool.ellipse:
      case ScreenshotTool.arrow:
        controller.selectAnnotation(null);
        if (selectionRect == null || !selectionRect.contains(globalPosition)) {
          return;
        }
        _interactionMode = _InteractionMode.createAnnotation;
        _annotationStart = globalPosition;
        _annotationEnd = globalPosition;
        _annotationDraftRect = Rect.fromPoints(globalPosition, globalPosition);
        break;
      case ScreenshotTool.mosaic:
        controller.selectAnnotation(null);
        if (selectionRect == null || !selectionRect.contains(globalPosition)) {
          return;
        }
        // Mosaic paints immediately on drag start so a short press still creates a privacy mask.
        // The controller owns the stroke id so later drag updates append to one undoable mark.
        _interactionMode = _InteractionMode.paintMosaic;
        _mosaicAnnotationId = controller.addMosaicAnnotation(globalPosition);
        _setMosaicBrushCenter(globalPosition);
        break;
      case ScreenshotTool.text:
        break;
    }
  }

  void _handlePanUpdate(Offset localPosition) {
    final interactionMode = _interactionMode;
    final dragStart = _dragStartGlobal;
    if (interactionMode == null || dragStart == null) {
      return;
    }

    final globalPosition = _toGlobalPosition(localPosition);
    switch (interactionMode) {
      case _InteractionMode.createSelection:
        controller.updateSelection(Rect.fromPoints(dragStart, globalPosition));
        break;
      case _InteractionMode.moveSelection:
        final original = _selectionAtDragStart;
        if (original == null) {
          break;
        }
        controller.updateSelection(_shiftRectWithinBounds(original, globalPosition - dragStart, controller.virtualBoundsRect));
        break;
      case _InteractionMode.resizeSelection:
        final original = _selectionAtDragStart;
        final handle = _resizeHandle;
        if (original == null || handle == null) {
          break;
        }
        controller.updateSelection(_resizeRect(original, handle, globalPosition));
        break;
      case _InteractionMode.createAnnotation:
        final currentSelection = controller.selectionRect;
        if (currentSelection == null) {
          break;
        }
        final clamped = _clampOffsetToRect(globalPosition, currentSelection);
        _annotationEnd = clamped;
        // Holding Shift while drawing rectangle/ellipse annotations should create a true square or
        // circle. Keep the constraint in the geometry layer so the preview and final annotation use
        // the same rect, while arrow drawing remains free-form.
        _annotationDraftRect =
            _isCurrentShapeCreationTool() && HardwareKeyboard.instance.isShiftPressed
                ? _squareRectFromAnchorAndPoint(dragStart, clamped, currentSelection)
                : Rect.fromPoints(dragStart, clamped);
        setState(() {});
        break;
      case _InteractionMode.paintMosaic:
        final currentSelection = controller.selectionRect;
        final mosaicAnnotationId = _mosaicAnnotationId;
        if (currentSelection == null || mosaicAnnotationId == null) {
          break;
        }
        final clamped = _clampOffsetToRect(globalPosition, currentSelection);
        _setMosaicBrushCenter(clamped);
        controller.appendMosaicPoint(mosaicAnnotationId, clamped);
        break;
      case _InteractionMode.moveAnnotation:
        _updateDraggedAnnotation(globalPosition, dragStart);
        break;
      case _InteractionMode.resizeShapeAnnotation:
        _resizeSelectedShape(globalPosition);
        break;
      case _InteractionMode.moveArrowStart:
        _moveArrowEndpoint(globalPosition, updateStart: true);
        break;
      case _InteractionMode.moveArrowEnd:
        _moveArrowEndpoint(globalPosition, updateStart: false);
        break;
      case _InteractionMode.moveText:
        _updateDraggedAnnotation(globalPosition, dragStart);
        break;
    }
  }

  void _handlePanEnd() {
    final interactionMode = _interactionMode;
    final needsOverlayRefresh = interactionMode == _InteractionMode.createAnnotation || interactionMode == _InteractionMode.paintMosaic;
    final shouldAutoConfirmSelection = interactionMode == _InteractionMode.createSelection && (controller.activeRequest?.autoConfirm ?? false);

    if (interactionMode == _InteractionMode.createAnnotation && _annotationStart != null && _annotationEnd != null) {
      // Freshly drawn annotations now stay unselected by default. Auto-selecting every new mark
      // made the next pointer gesture look like an unwanted edit, so selection is left to explicit
      // taps on existing annotations and blank taps already clear the current selection.
      switch (controller.currentTool.value) {
        case ScreenshotTool.rect:
          controller.addShapeAnnotation(ScreenshotAnnotationType.rect, _annotationDraftRect!);
          break;
        case ScreenshotTool.ellipse:
          controller.addShapeAnnotation(ScreenshotAnnotationType.ellipse, _annotationDraftRect!);
          break;
        case ScreenshotTool.arrow:
          controller.addArrowAnnotation(_annotationStart!, _annotationEnd!);
          break;
        case ScreenshotTool.select:
        case ScreenshotTool.text:
        case ScreenshotTool.mosaic:
          break;
      }
    }

    _interactionMode = null;
    _resizeHandle = null;
    _annotationHandle = null;
    _dragStartGlobal = null;
    _selectionAtDragStart = null;
    _annotationDraftRect = null;
    _annotationStart = null;
    _annotationEnd = null;
    _dragAnnotationId = null;
    _mosaicAnnotationId = null;
    _annotationAtDragStart = null;
    if (needsOverlayRefresh) {
      setState(() {});
    }
    if (shouldAutoConfirmSelection) {
      _confirmSelectionFromAutoConfirm();
    }
  }

  void _handlePointerSignal(PointerSignalEvent event) {
    if (event is! PointerScrollEvent || controller.stage.value != ScreenshotSessionStage.scrolling) {
      return;
    }

    final selectionRect = controller.selectionRect;
    if (selectionRect == null) {
      return;
    }

    final globalPosition = _toGlobalPosition(event.localPosition);
    if (!selectionRect.contains(globalPosition)) {
      return;
    }

    // In scrolling-capture mode the rectangle behaves like a mouse-through viewport: Flutter uses
    // the wheel delta to drive the native app underneath, then refreshes the stitched preview from
    // the live desktop pixels. Drag/annotation gestures stay disabled until the mode is confirmed.
    controller.handleScrollingCaptureWheel(const UuidV4().generate(), event.scrollDelta.dy);
  }

  void _handleHover(Offset localPosition) {
    if (controller.stage.value == ScreenshotSessionStage.scrolling) {
      _setHoverCursor(SystemMouseCursors.basic);
      return;
    }

    final currentTool = controller.currentTool.value;
    if (currentTool == ScreenshotTool.mosaic) {
      final selectionRect = controller.selectionRect;
      final globalPosition = _toGlobalPosition(localPosition);
      final brushCenter = selectionRect != null && selectionRect.contains(globalPosition) ? globalPosition : null;
      // The mosaic brush marker is updated outside setState so it can show the active radius while
      // avoiding the full-workspace hover rebuild that previously made the tool feel delayed.
      _setMosaicBrushCenter(brushCenter);
      _setHoverCursor(brushCenter == null ? SystemMouseCursors.basic : SystemMouseCursors.precise);
      return;
    }

    if (_interactionMode != null || currentTool != ScreenshotTool.select) {
      _setHoverCursor(SystemMouseCursors.basic);
      return;
    }

    final globalPosition = _toGlobalPosition(localPosition);
    final selectionRect = controller.selectionRect;
    final selectedHandleHit = _hitTestSelectedAnnotationHandle(globalPosition);
    if (selectedHandleHit != null) {
      _setHoverCursor(selectedHandleHit.cursor);
      return;
    }

    final annotationBodyHit = _hitTestAnnotationBody(globalPosition);
    if (annotationBodyHit != null) {
      if (annotationBodyHit.type == ScreenshotAnnotationType.text) {
        _setHoverCursor(SystemMouseCursors.text);
        return;
      }
      _setHoverCursor(SystemMouseCursors.move);
      return;
    }

    if (selectionRect != null) {
      final handle = _hitTestSelectionHandle(selectionRect, globalPosition);
      if (handle != null) {
        _setHoverCursor(_cursorForResizeHandle(handle));
        return;
      }
      if (selectionRect.contains(globalPosition)) {
        _setHoverCursor(SystemMouseCursors.move);
        return;
      }
    }

    _setHoverCursor(SystemMouseCursors.basic);
  }

  void _setHoverCursor(MouseCursor cursor) {
    if (_hoverCursor == cursor) {
      return;
    }
    setState(() {
      _hoverCursor = cursor;
    });
  }

  void _setMosaicBrushCenter(Offset? center) {
    if (_mosaicBrushCenter.value == center) {
      return;
    }
    _mosaicBrushCenter.value = center;
  }

  void _selectAnnotationForEdit(ScreenshotAnnotation annotation) {
    // Clicking an existing annotation is an edit intent, even when a drawing tool is still active.
    // Switch back to select and hide transient mosaic brush UI so the annotation's side toolbox is
    // the immediate focus for color, size, and delete changes.
    _setMosaicBrushCenter(null);
    if (controller.currentTool.value != ScreenshotTool.select) {
      controller.setTool(ScreenshotTool.select);
    }
    controller.selectAnnotation(annotation.id);
  }

  void _updateDraggedAnnotation(Offset globalPosition, Offset dragStart) {
    final annotation = _annotationAtDragStart;
    final annotationId = _dragAnnotationId;
    final currentSelection = controller.selectionRect;
    if (annotation == null || annotationId == null || currentSelection == null) {
      return;
    }

    final delta = globalPosition - dragStart;
    switch (annotation.type) {
      case ScreenshotAnnotationType.rect:
      case ScreenshotAnnotationType.ellipse:
        final rect = annotation.rect;
        if (rect == null) {
          return;
        }
        controller.updateAnnotationRect(annotationId, _shiftRectWithinBounds(rect, delta, currentSelection));
        break;
      case ScreenshotAnnotationType.arrow:
        final start = annotation.start;
        final end = annotation.end;
        if (start == null || end == null) {
          return;
        }
        final originalBounds = Rect.fromPoints(start, end).inflate(1);
        final shiftedBounds = _shiftRectWithinBounds(originalBounds, delta, currentSelection);
        final clampedDelta = shiftedBounds.topLeft - originalBounds.topLeft;
        controller.updateArrowPoints(annotationId, start: start + clampedDelta, end: end + clampedDelta);
        break;
      case ScreenshotAnnotationType.text:
        final start = annotation.start;
        final textBounds = screenshotAnnotationBounds(annotation);
        if (start == null || textBounds == null) {
          return;
        }
        final shiftedBounds = _shiftRectWithinBounds(textBounds, delta, currentSelection);
        controller.updateTextPosition(annotationId, start + (shiftedBounds.topLeft - textBounds.topLeft));
        break;
      case ScreenshotAnnotationType.mosaic:
        final mosaicBounds = screenshotAnnotationBounds(annotation);
        if (mosaicBounds == null) {
          return;
        }
        final shiftedBounds = _shiftRectWithinBounds(mosaicBounds, delta, currentSelection);
        final clampedDelta = shiftedBounds.topLeft - mosaicBounds.topLeft;
        controller.updateMosaicPoints(annotationId, annotation.points.map((point) => point + clampedDelta).toList(growable: false));
        break;
    }
  }

  void _resizeSelectedShape(Offset globalPosition) {
    final annotation = _annotationAtDragStart;
    final annotationId = _dragAnnotationId;
    final handle = _annotationHandle;
    final currentSelection = controller.selectionRect;
    if (annotation == null || annotation.rect == null || annotationId == null || handle == null || currentSelection == null) {
      return;
    }

    // Shift-resizing rectangle/ellipse annotations is an aspect-ratio constraint, not a different
    // tool mode. Reading the current keyboard state here lets users press or release Shift mid-drag
    // and immediately switch between free resize and square/circle scaling.
    final constrainToSquare = HardwareKeyboard.instance.isShiftPressed;
    controller.updateAnnotationRect(
      annotationId,
      _resizeShapeRect(annotation.rect!, handle, _clampOffsetToRect(globalPosition, currentSelection), currentSelection, constrainToSquare: constrainToSquare),
    );
  }

  void _moveArrowEndpoint(Offset globalPosition, {required bool updateStart}) {
    final annotation = _annotationAtDragStart;
    final annotationId = _dragAnnotationId;
    final currentSelection = controller.selectionRect;
    if (annotation == null || annotationId == null || currentSelection == null) {
      return;
    }

    final nextPoint = _clampOffsetToRect(globalPosition, currentSelection);
    controller.updateArrowPoints(annotationId, start: updateStart ? nextPoint : annotation.start, end: updateStart ? annotation.end : nextPoint);
  }

  Offset _toGlobalPosition(Offset localPosition) {
    return localPosition + controller.virtualBoundsRect.topLeft;
  }

  _ResizeHandle? _hitTestSelectionHandle(Rect rect, Offset point) {
    for (final handle in _ResizeHandle.values) {
      final handleOffset = _handleOffsetForRect(rect, handle);
      if ((handleOffset - point).distance <= 12) {
        return handle;
      }
    }

    if (point.dx >= rect.left && point.dx <= rect.right) {
      if ((point.dy - rect.top).abs() <= _selectionEdgeTolerance) {
        return _ResizeHandle.top;
      }
      if ((point.dy - rect.bottom).abs() <= _selectionEdgeTolerance) {
        return _ResizeHandle.bottom;
      }
    }

    if (point.dy >= rect.top && point.dy <= rect.bottom) {
      if ((point.dx - rect.left).abs() <= _selectionEdgeTolerance) {
        return _ResizeHandle.left;
      }
      if ((point.dx - rect.right).abs() <= _selectionEdgeTolerance) {
        return _ResizeHandle.right;
      }
    }

    return null;
  }

  _AnnotationHitTarget? _hitTestSelectedAnnotationHandle(Offset point) {
    final annotation = controller.selectedAnnotation;
    if (annotation == null) {
      return null;
    }

    switch (annotation.type) {
      case ScreenshotAnnotationType.rect:
      case ScreenshotAnnotationType.ellipse:
        final rect = annotation.rect;
        if (rect == null) {
          return null;
        }
        for (final handle in _shapeAnnotationHandles) {
          if ((_handleOffsetForAnnotationRect(rect, handle) - point).distance <= 12) {
            return _AnnotationHitTarget(annotation: annotation, handle: handle, cursor: _cursorForAnnotationHandle(handle));
          }
        }
        return null;
      case ScreenshotAnnotationType.arrow:
        final start = annotation.start;
        final end = annotation.end;
        if (start == null || end == null) {
          return null;
        }
        if ((start - point).distance <= 12) {
          return _AnnotationHitTarget(annotation: annotation, handle: _AnnotationHandle.arrowStart, cursor: SystemMouseCursors.precise);
        }
        if ((end - point).distance <= 12) {
          return _AnnotationHitTarget(annotation: annotation, handle: _AnnotationHandle.arrowEnd, cursor: SystemMouseCursors.precise);
        }
        return null;
      case ScreenshotAnnotationType.text:
        return null;
      case ScreenshotAnnotationType.mosaic:
        return null;
    }
  }

  ScreenshotAnnotation? _hitTestAnnotationBody(Offset point, {bool allowOnlyText = false}) {
    for (final annotation in controller.annotations.reversed) {
      if (allowOnlyText && annotation.type != ScreenshotAnnotationType.text) {
        continue;
      }
      if (_annotationContainsPoint(annotation, point)) {
        return annotation;
      }
    }
    return null;
  }

  bool _annotationContainsPoint(ScreenshotAnnotation annotation, Offset point) {
    switch (annotation.type) {
      case ScreenshotAnnotationType.rect:
        final rect = annotation.rect;
        return rect != null && rect.inflate(8).contains(point);
      case ScreenshotAnnotationType.ellipse:
        final rect = annotation.rect;
        if (rect == null) {
          return false;
        }
        final inflated = rect.inflate(8);
        final radiusX = inflated.width / 2;
        final radiusY = inflated.height / 2;
        final center = inflated.center;
        final dx = (point.dx - center.dx) / radiusX;
        final dy = (point.dy - center.dy) / radiusY;
        return dx * dx + dy * dy <= 1;
      case ScreenshotAnnotationType.arrow:
        final start = annotation.start;
        final end = annotation.end;
        if (start == null || end == null) {
          return false;
        }
        return _distanceToSegment(point, start, end) <= 10;
      case ScreenshotAnnotationType.text:
        final bounds = screenshotAnnotationBounds(annotation);
        return bounds != null && bounds.inflate(8).contains(point);
      case ScreenshotAnnotationType.mosaic:
        return annotation.points.any((brushPoint) => (brushPoint - point).distance <= annotation.mosaicRadius + 4);
    }
  }

  Offset _handleOffsetForRect(Rect rect, _ResizeHandle handle) {
    switch (handle) {
      case _ResizeHandle.topLeft:
        return rect.topLeft;
      case _ResizeHandle.top:
        return Offset(rect.center.dx, rect.top);
      case _ResizeHandle.topRight:
        return rect.topRight;
      case _ResizeHandle.right:
        return Offset(rect.right, rect.center.dy);
      case _ResizeHandle.bottomRight:
        return rect.bottomRight;
      case _ResizeHandle.bottom:
        return Offset(rect.center.dx, rect.bottom);
      case _ResizeHandle.bottomLeft:
        return rect.bottomLeft;
      case _ResizeHandle.left:
        return Offset(rect.left, rect.center.dy);
    }
  }

  Offset _handleOffsetForAnnotationRect(Rect rect, _AnnotationHandle handle) {
    switch (handle) {
      case _AnnotationHandle.topLeft:
        return rect.topLeft;
      case _AnnotationHandle.top:
        return Offset(rect.center.dx, rect.top);
      case _AnnotationHandle.topRight:
        return rect.topRight;
      case _AnnotationHandle.right:
        return Offset(rect.right, rect.center.dy);
      case _AnnotationHandle.bottomRight:
        return rect.bottomRight;
      case _AnnotationHandle.bottom:
        return Offset(rect.center.dx, rect.bottom);
      case _AnnotationHandle.bottomLeft:
        return rect.bottomLeft;
      case _AnnotationHandle.left:
        return Offset(rect.left, rect.center.dy);
      case _AnnotationHandle.arrowStart:
      case _AnnotationHandle.arrowEnd:
        return rect.center;
    }
  }

  Rect _resizeRect(Rect original, _ResizeHandle handle, Offset point) {
    final rect = _rectForResizeHandle(original, handle, point);
    return rect;
  }

  Rect _resizeShapeRect(Rect original, _AnnotationHandle handle, Offset point, Rect bounds, {required bool constrainToSquare}) {
    final rect = constrainToSquare ? _squareRectForAnnotationHandle(original, handle, point, bounds) : _rectForAnnotationHandle(original, handle, point);
    return _clampRectToBounds(rect, bounds);
  }

  Rect _rectForResizeHandle(Rect original, _ResizeHandle handle, Offset point) {
    switch (handle) {
      case _ResizeHandle.topLeft:
        return Rect.fromPoints(point, original.bottomRight);
      case _ResizeHandle.top:
        return _normalizedRectFromLTRB(original.left, point.dy, original.right, original.bottom);
      case _ResizeHandle.topRight:
        return _normalizedRectFromLTRB(original.left, point.dy, point.dx, original.bottom);
      case _ResizeHandle.right:
        return _normalizedRectFromLTRB(original.left, original.top, point.dx, original.bottom);
      case _ResizeHandle.bottomRight:
        return Rect.fromPoints(original.topLeft, point);
      case _ResizeHandle.bottom:
        return _normalizedRectFromLTRB(original.left, original.top, original.right, point.dy);
      case _ResizeHandle.bottomLeft:
        return _normalizedRectFromLTRB(point.dx, original.top, original.right, point.dy);
      case _ResizeHandle.left:
        return _normalizedRectFromLTRB(point.dx, original.top, original.right, original.bottom);
    }
  }

  Rect _rectForAnnotationHandle(Rect original, _AnnotationHandle handle, Offset point) {
    switch (handle) {
      case _AnnotationHandle.topLeft:
        return Rect.fromPoints(point, original.bottomRight);
      case _AnnotationHandle.top:
        return _normalizedRectFromLTRB(original.left, point.dy, original.right, original.bottom);
      case _AnnotationHandle.topRight:
        return _normalizedRectFromLTRB(original.left, point.dy, point.dx, original.bottom);
      case _AnnotationHandle.right:
        return _normalizedRectFromLTRB(original.left, original.top, point.dx, original.bottom);
      case _AnnotationHandle.bottomRight:
        return Rect.fromPoints(original.topLeft, point);
      case _AnnotationHandle.bottom:
        return _normalizedRectFromLTRB(original.left, original.top, original.right, point.dy);
      case _AnnotationHandle.bottomLeft:
        return _normalizedRectFromLTRB(point.dx, original.top, original.right, point.dy);
      case _AnnotationHandle.left:
        return _normalizedRectFromLTRB(point.dx, original.top, original.right, original.bottom);
      case _AnnotationHandle.arrowStart:
      case _AnnotationHandle.arrowEnd:
        return original;
    }
  }

  Rect _squareRectForAnnotationHandle(Rect original, _AnnotationHandle handle, Offset point, Rect bounds) {
    switch (handle) {
      case _AnnotationHandle.topLeft:
        return _squareRectFromAnchorAndPoint(original.bottomRight, point, bounds);
      case _AnnotationHandle.topRight:
        return _squareRectFromAnchorAndPoint(original.bottomLeft, point, bounds);
      case _AnnotationHandle.bottomRight:
        return _squareRectFromAnchorAndPoint(original.topLeft, point, bounds);
      case _AnnotationHandle.bottomLeft:
        return _squareRectFromAnchorAndPoint(original.topRight, point, bounds);
      case _AnnotationHandle.top:
        return _squareRectFromVerticalResize(original: original, point: point, bounds: bounds, resizeTop: true);
      case _AnnotationHandle.bottom:
        return _squareRectFromVerticalResize(original: original, point: point, bounds: bounds, resizeTop: false);
      case _AnnotationHandle.left:
        return _squareRectFromHorizontalResize(original: original, point: point, bounds: bounds, resizeLeft: true);
      case _AnnotationHandle.right:
        return _squareRectFromHorizontalResize(original: original, point: point, bounds: bounds, resizeLeft: false);
      case _AnnotationHandle.arrowStart:
      case _AnnotationHandle.arrowEnd:
        return original;
    }
  }

  Rect _squareRectFromAnchorAndPoint(Offset anchor, Offset point, Rect bounds) {
    final dx = point.dx - anchor.dx;
    final dy = point.dy - anchor.dy;
    final directionX = dx < 0 ? -1.0 : 1.0;
    final directionY = dy < 0 ? -1.0 : 1.0;
    final maxSideX = directionX > 0 ? bounds.right - anchor.dx : anchor.dx - bounds.left;
    final maxSideY = directionY > 0 ? bounds.bottom - anchor.dy : anchor.dy - bounds.top;
    final side = math.min(math.max(dx.abs(), dy.abs()), math.min(maxSideX, maxSideY)).clamp(0.0, double.infinity).toDouble();
    return Rect.fromPoints(anchor, Offset(anchor.dx + directionX * side, anchor.dy + directionY * side));
  }

  Rect _squareRectFromVerticalResize({required Rect original, required Offset point, required Rect bounds, required bool resizeTop}) {
    final fixedY = resizeTop ? original.bottom : original.top;
    final desiredSide = (fixedY - point.dy).abs();
    final maxSideByHeight = resizeTop ? fixedY - bounds.top : bounds.bottom - fixedY;
    final maxSideByWidth = 2 * math.min(original.center.dx - bounds.left, bounds.right - original.center.dx);
    final side = math.min(desiredSide, math.min(maxSideByHeight, maxSideByWidth)).clamp(0.0, double.infinity).toDouble();
    final top = resizeTop ? fixedY - side : fixedY;
    final bottom = resizeTop ? fixedY : fixedY + side;
    return Rect.fromLTRB(original.center.dx - side / 2, top, original.center.dx + side / 2, bottom);
  }

  Rect _squareRectFromHorizontalResize({required Rect original, required Offset point, required Rect bounds, required bool resizeLeft}) {
    final fixedX = resizeLeft ? original.right : original.left;
    final desiredSide = (fixedX - point.dx).abs();
    final maxSideByWidth = resizeLeft ? fixedX - bounds.left : bounds.right - fixedX;
    final maxSideByHeight = 2 * math.min(original.center.dy - bounds.top, bounds.bottom - original.center.dy);
    final side = math.min(desiredSide, math.min(maxSideByWidth, maxSideByHeight)).clamp(0.0, double.infinity).toDouble();
    final left = resizeLeft ? fixedX - side : fixedX;
    final right = resizeLeft ? fixedX : fixedX + side;
    return Rect.fromLTRB(left, original.center.dy - side / 2, right, original.center.dy + side / 2);
  }

  Rect _shiftRectWithinBounds(Rect rect, Offset delta, Rect bounds) {
    var shifted = rect.shift(delta);
    if (shifted.left < bounds.left) {
      shifted = shifted.shift(Offset(bounds.left - shifted.left, 0));
    }
    if (shifted.top < bounds.top) {
      shifted = shifted.shift(Offset(0, bounds.top - shifted.top));
    }
    if (shifted.right > bounds.right) {
      shifted = shifted.shift(Offset(bounds.right - shifted.right, 0));
    }
    if (shifted.bottom > bounds.bottom) {
      shifted = shifted.shift(Offset(0, bounds.bottom - shifted.bottom));
    }
    return shifted;
  }

  Offset _clampOffsetToRect(Offset point, Rect bounds) {
    return Offset(point.dx.clamp(bounds.left, bounds.right).toDouble(), point.dy.clamp(bounds.top, bounds.bottom).toDouble());
  }

  Rect _clampRectToBounds(Rect rect, Rect bounds) {
    final normalized = _normalizedRectFromLTRB(rect.left, rect.top, rect.right, rect.bottom);
    final left = normalized.left.clamp(bounds.left, bounds.right).toDouble();
    final top = normalized.top.clamp(bounds.top, bounds.bottom).toDouble();
    final right = normalized.right.clamp(bounds.left, bounds.right).toDouble();
    final bottom = normalized.bottom.clamp(bounds.top, bounds.bottom).toDouble();
    return _normalizedRectFromLTRB(left, top, right, bottom);
  }

  Rect _normalizedRectFromLTRB(double left, double top, double right, double bottom) {
    return Rect.fromLTRB(math.min(left, right), math.min(top, bottom), math.max(left, right), math.max(top, bottom));
  }

  MouseCursor _cursorForResizeHandle(_ResizeHandle handle) {
    switch (handle) {
      case _ResizeHandle.topLeft:
      case _ResizeHandle.bottomRight:
        if (Platform.isMacOS) {
          // macOS Flutter does not provide this diagonal system cursor, but corner handles still
          // resize diagonally. Use Wox's native screenshot cursor so the visible affordance matches
          // the drag behavior instead of falling back to the default arrow.
          return _macOSResizeUpLeftDownRightCursor;
        }
        return SystemMouseCursors.resizeUpLeftDownRight;
      case _ResizeHandle.topRight:
      case _ResizeHandle.bottomLeft:
        if (Platform.isMacOS) {
          // Keep the macOS workaround local to diagonal corner handles; side handles already map to
          // supported AppKit resize cursors and should keep the standard Flutter system path.
          return _macOSResizeUpRightDownLeftCursor;
        }
        return SystemMouseCursors.resizeUpRightDownLeft;
      case _ResizeHandle.top:
      case _ResizeHandle.bottom:
        return SystemMouseCursors.resizeUpDown;
      case _ResizeHandle.left:
      case _ResizeHandle.right:
        return SystemMouseCursors.resizeLeftRight;
    }
  }

  MouseCursor _cursorForAnnotationHandle(_AnnotationHandle handle) {
    switch (handle) {
      case _AnnotationHandle.topLeft:
      case _AnnotationHandle.bottomRight:
        if (Platform.isMacOS) {
          // Selected shape annotations share the same corner-resize affordance as the capture
          // selection. Reusing the native diagonal cursor keeps annotation editing consistent with
          // the fixed selection frame behavior.
          return _macOSResizeUpLeftDownRightCursor;
        }
        return SystemMouseCursors.resizeUpLeftDownRight;
      case _AnnotationHandle.topRight:
      case _AnnotationHandle.bottomLeft:
        if (Platform.isMacOS) {
          // Flutter's macOS cursor fallback is a plain arrow for this diagonal direction too, so
          // shape annotation corners need the same screenshot-owned native cursor path.
          return _macOSResizeUpRightDownLeftCursor;
        }
        return SystemMouseCursors.resizeUpRightDownLeft;
      case _AnnotationHandle.top:
      case _AnnotationHandle.bottom:
        return SystemMouseCursors.resizeUpDown;
      case _AnnotationHandle.left:
      case _AnnotationHandle.right:
        return SystemMouseCursors.resizeLeftRight;
      case _AnnotationHandle.arrowStart:
      case _AnnotationHandle.arrowEnd:
        return SystemMouseCursors.precise;
    }
  }

  double _distanceToSegment(Offset point, Offset start, Offset end) {
    final dx = end.dx - start.dx;
    final dy = end.dy - start.dy;
    if (dx == 0 && dy == 0) {
      return (point - start).distance;
    }

    final projection = ((point.dx - start.dx) * dx + (point.dy - start.dy) * dy) / (dx * dx + dy * dy);
    final clampedProjection = projection.clamp(0.0, 1.0).toDouble();
    final closest = Offset(start.dx + dx * clampedProjection, start.dy + dy * clampedProjection);
    return (point - closest).distance;
  }

  ScreenshotAnnotationType? _currentDraftType() {
    switch (controller.currentTool.value) {
      case ScreenshotTool.rect:
        return ScreenshotAnnotationType.rect;
      case ScreenshotTool.ellipse:
        return ScreenshotAnnotationType.ellipse;
      case ScreenshotTool.arrow:
        return ScreenshotAnnotationType.arrow;
      case ScreenshotTool.select:
      case ScreenshotTool.text:
      case ScreenshotTool.mosaic:
        return null;
    }
  }

  bool _isCurrentShapeCreationTool() {
    return controller.currentTool.value == ScreenshotTool.rect || controller.currentTool.value == ScreenshotTool.ellipse;
  }

  bool _isAnnotationCreationTool(ScreenshotTool tool) {
    switch (tool) {
      case ScreenshotTool.rect:
      case ScreenshotTool.ellipse:
      case ScreenshotTool.arrow:
      case ScreenshotTool.text:
      case ScreenshotTool.mosaic:
        return true;
      case ScreenshotTool.select:
        return false;
    }
  }
}

class _WorkspaceBackground extends StatelessWidget {
  const _WorkspaceBackground({required this.snapshots, required this.virtualBounds, this.clearLocalRect});

  final List<DisplaySnapshot> snapshots;
  final Rect virtualBounds;
  final Rect? clearLocalRect;

  @override
  Widget build(BuildContext context) {
    final background = Stack(
      children: [
        for (final snapshot in snapshots)
          if (snapshot.hasImageBytes)
            Positioned.fromRect(
              rect: snapshot.logicalBounds.toRect().shift(-virtualBounds.topLeft),
              // macOS now reveals the native selection overlay before every display payload is
              // serialized for Flutter. Skip metadata-only snapshots here so the first annotation
              // frame can show the displays that are ready instead of crashing on deferred bytes.
              child: Image(image: snapshot.imageProvider, fit: BoxFit.fill, gaplessPlayback: true),
            ),
      ],
    );

    if (clearLocalRect == null) {
      return background;
    }

    // Scrolling capture needs the selected rectangle to reveal the live app behind Wox, while the
    // rest of the workspace still uses the frozen screenshot backdrop for a stable capture shell.
    return ClipPath(clipper: _SelectionHoleClipper(clearLocalRect!), child: background);
  }
}

class _SelectionHoleClipper extends CustomClipper<Path> {
  const _SelectionHoleClipper(this.clearRect);

  final Rect clearRect;

  @override
  Path getClip(Size size) {
    return Path()
      ..fillType = PathFillType.evenOdd
      ..addRect(Offset.zero & size)
      ..addRect(clearRect);
  }

  @override
  bool shouldReclip(covariant _SelectionHoleClipper oldClipper) {
    return oldClipper.clearRect != clearRect;
  }
}

class _SelectionToolbarLayoutDelegate extends SingleChildLayoutDelegate {
  const _SelectionToolbarLayoutDelegate({required this.selectionRect});

  final Rect? selectionRect;
  static const EdgeInsets _viewportPadding = EdgeInsets.all(24);
  static const double _selectionGap = 16;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    return BoxConstraints(
      maxWidth: (constraints.maxWidth - _viewportPadding.horizontal).clamp(0, double.infinity),
      maxHeight: (constraints.maxHeight - _viewportPadding.vertical).clamp(0, double.infinity),
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) {
    if (selectionRect == null) {
      return Offset(
        ((size.width - childSize.width) / 2).clamp(_viewportPadding.left, size.width - childSize.width - _viewportPadding.right),
        (size.height - childSize.height - _viewportPadding.bottom).clamp(_viewportPadding.top, size.height - childSize.height - _viewportPadding.bottom),
      );
    }

    final rightAlignedLeft = selectionRect!.right - childSize.width;
    final left = rightAlignedLeft.clamp(_viewportPadding.left, size.width - childSize.width - _viewportPadding.right);

    final preferredBelowTop = selectionRect!.bottom + _selectionGap;
    final belowFits = preferredBelowTop + childSize.height <= size.height - _viewportPadding.bottom;
    final top =
        belowFits
            ? preferredBelowTop
            : (selectionRect!.top - childSize.height - _selectionGap).clamp(_viewportPadding.top, size.height - childSize.height - _viewportPadding.bottom);

    return Offset(left, top);
  }

  @override
  bool shouldRelayout(covariant _SelectionToolbarLayoutDelegate oldDelegate) {
    return oldDelegate.selectionRect != selectionRect;
  }
}

class _SelectionEditBarLayoutDelegate extends SingleChildLayoutDelegate {
  const _SelectionEditBarLayoutDelegate({required this.selectionRect, required this.anchorRect});

  final Rect selectionRect;
  final Rect? anchorRect;
  static const EdgeInsets _viewportPadding = EdgeInsets.all(24);
  static const double _selectionGap = 16;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    return BoxConstraints(
      maxWidth: (constraints.maxWidth - _viewportPadding.horizontal).clamp(0, double.infinity),
      maxHeight: (constraints.maxHeight - _viewportPadding.vertical).clamp(0, double.infinity),
    );
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) {
    final hasRightSpace = selectionRect.right + _selectionGap + childSize.width <= size.width - _viewportPadding.right;
    final left =
        hasRightSpace
            ? selectionRect.right + _selectionGap
            : (selectionRect.left - childSize.width - _selectionGap).clamp(_viewportPadding.left, size.width - childSize.width - _viewportPadding.right);

    final targetCenterY = (anchorRect ?? selectionRect).center.dy;
    final top = (targetCenterY - childSize.height / 2).clamp(_viewportPadding.top, size.height - childSize.height - _viewportPadding.bottom);
    return Offset(left, top);
  }

  @override
  bool shouldRelayout(covariant _SelectionEditBarLayoutDelegate oldDelegate) {
    return oldDelegate.selectionRect != selectionRect || oldDelegate.anchorRect != anchorRect;
  }
}

class _LoadingView extends StatelessWidget {
  const _LoadingView({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: const Color(0xFF090909),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(width: 28, height: 28, child: CircularProgressIndicator(strokeWidth: 2.4, color: Color(0xFF29FF72))),
            const SizedBox(height: 14),
            Text(label, style: const TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
          ],
        ),
      ),
    );
  }
}

class _CallerIcon extends StatelessWidget {
  const _CallerIcon({required this.icon});

  final WoxImage icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(color: const Color(0x2EFFFFFF), borderRadius: BorderRadius.circular(8), border: Border.all(color: const Color(0x33FFFFFF))),
      child: WoxImageView(woxImage: icon, width: 20, height: 20),
    );
  }
}

class _ToolButton extends StatelessWidget {
  const _ToolButton({
    super.key,
    required this.onPressed,
    this.icon,
    this.iconBuilder,
    this.selected = false,
    this.enabled = true,
    this.color = Colors.white,
    this.activateOnTapDown = false,
    this.tooltip,
  }) : assert(icon != null || iconBuilder != null);

  final IconData? icon;
  final Widget Function(Color foreground)? iconBuilder;
  final VoidCallback? onPressed;
  final bool selected;
  final bool enabled;
  final Color color;
  final bool activateOnTapDown;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    // Tool buttons share one selected color so switching between annotation tools feels consistent;
    // individual drawing previews still use the active annotation color where that color matters.
    const activeColor = Color(0xFF29FF72);
    final foreground = enabled ? (selected ? activeColor : color) : Colors.white38;
    final enabledAction = enabled ? onPressed : null;
    final iconWidget = iconBuilder?.call(foreground) ?? Icon(icon, color: foreground, size: _screenshotToolbarIconSize);

    final button = Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4),
      child: InkWell(
        // Tool switching on desktop felt delayed because InkWell.onTap waits for pointer-up and also
        // competes with the toolbar scroll view's gesture arena. Triggering the low-risk tool-change
        // actions on tap-down makes the active icon respond immediately while leaving confirm/cancel
        // style actions on the safer pointer-up path.
        onTapDown: activateOnTapDown && enabledAction != null ? (_) => enabledAction() : null,
        onTap: activateOnTapDown ? null : enabledAction,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(color: selected ? activeColor.withValues(alpha: 0.2) : Colors.transparent, borderRadius: BorderRadius.circular(10)),
          child: iconWidget,
        ),
      ),
    );

    if (tooltip == null || tooltip!.isEmpty) {
      return button;
    }

    // The screenshot toolbox is icon-only. Adding WoxTooltip at the shared button wrapper keeps hover
    // help consistent for drawing tools, session actions, and disabled actions without duplicating
    // overlay wiring at every call site. The delay preserves the old dense-toolbar hover timing.
    return WoxTooltip(message: tooltip!, waitDuration: const Duration(milliseconds: 350), child: button);
  }
}

class _ScrollingCaptureToolIcon extends StatelessWidget {
  const _ScrollingCaptureToolIcon({required this.color});

  final Color color;

  @override
  Widget build(BuildContext context) {
    // Long screenshot needs a distinct vertical-expansion symbol instead of a down arrow, which
    // users read as download/export. Keep it as a plain foreground-colored glyph so pin and
    // scrolling capture stay visually grouped with the white annotation tools.
    return Center(child: Icon(Icons.height, color: color, size: _screenshotToolbarIconSize));
  }
}

class _MosaicToolIcon extends StatelessWidget {
  const _MosaicToolIcon({required this.color});

  final Color color;

  @override
  Widget build(BuildContext context) {
    // The built-in grid icons read as layout controls. A bordered checkerboard matches common
    // screenshot mosaic tools, while the smaller paint size keeps the solid square optically aligned
    // with Material icons that reserve padding inside the shared toolbar icon slot.
    return Center(child: CustomPaint(size: const Size.square(_screenshotMosaicToolIconPaintSize), painter: _MosaicToolIconPainter(color)));
  }
}

class _MosaicToolIconPainter extends CustomPainter {
  const _MosaicToolIconPainter(this.color);

  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final cellSize = size.width / 5;
    final fillPaint = Paint()..color = color;
    for (var row = 0; row < 5; row++) {
      for (var column = 0; column < 5; column++) {
        if ((row + column).isEven) {
          canvas.drawRect(Rect.fromLTWH(column * cellSize, row * cellSize, cellSize, cellSize), fillPaint);
        }
      }
    }

    final borderPaint =
        Paint()
          ..color = color
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2;
    canvas.drawRect((Offset.zero & size).deflate(1), borderPaint);
  }

  @override
  bool shouldRepaint(covariant _MosaicToolIconPainter oldDelegate) {
    return oldDelegate.color != color;
  }
}

class _MosaicBrushSizeButton extends StatelessWidget {
  const _MosaicBrushSizeButton({required this.radius, required this.selected, required this.tooltip, required this.onPressed});

  final double radius;
  final bool selected;
  final String tooltip;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final maxRadius = screenshotMosaicBrushRadii.last;
    final visualRadius = (4 + radius / maxRadius * 6).clamp(4.0, 10.0).toDouble();
    final color = selected ? const Color(0xFF29FF72) : Colors.white70;

    return WoxTooltip(
      message: tooltip,
      waitDuration: const Duration(milliseconds: 350),
      child: InkWell(
        onTap: onPressed,
        borderRadius: BorderRadius.circular(10),
        child: SizedBox(
          width: 32,
          height: 32,
          child: Center(
            child: Container(
              width: visualRadius * 2,
              height: visualRadius * 2,
              decoration: BoxDecoration(shape: BoxShape.circle, color: color.withValues(alpha: selected ? 0.3 : 0.16), border: Border.all(color: color, width: selected ? 2 : 1.5)),
            ),
          ),
        ),
      ),
    );
  }
}

class _ColorSwatchButton extends StatelessWidget {
  const _ColorSwatchButton({required this.color, required this.selected, required this.onPressed, required this.compact});

  final Color color;
  final bool selected;
  final VoidCallback onPressed;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final size = compact ? 16.0 : 20.0;
    return GestureDetector(
      onTap: onPressed,
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(color: color, shape: BoxShape.circle, border: Border.all(color: selected ? const Color(0xFF29FF72) : Colors.white24, width: selected ? 2 : 1)),
      ),
    );
  }
}

class _EditActionButton extends StatelessWidget {
  const _EditActionButton({required this.icon, required this.onPressed, required this.tooltip, this.color = Colors.white});

  final IconData icon;
  final VoidCallback onPressed;
  final String tooltip;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return WoxTooltip(
      message: tooltip,
      waitDuration: const Duration(milliseconds: 350),
      child: InkWell(
        onTap: onPressed,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          width: 42,
          height: 42,
          decoration: BoxDecoration(color: const Color(0x22FFFFFF), borderRadius: BorderRadius.circular(10)),
          child: Icon(icon, color: color, size: 22),
        ),
      ),
    );
  }
}

class _WorkspaceShadePainter extends CustomPainter {
  _WorkspaceShadePainter({required this.selectionRect, required this.selectionSizeLabel});

  final Rect? selectionRect;
  final String? selectionSizeLabel;

  @override
  void paint(Canvas canvas, Size size) {
    final overlayPaint = Paint()..color = const Color(0x77000000);
    if (selectionRect != null) {
      final clampedSelection = selectionRect!.intersect(Offset.zero & size);
      // The even-odd path version was logically correct, but anti-aliased path filling plus the
      // selection-frame shadow still made the crop interior look slightly tinted. Painting the four
      // outside bands directly guarantees the selected pixels remain fully untouched.
      if (clampedSelection.top > 0) {
        canvas.drawRect(Rect.fromLTWH(0, 0, size.width, clampedSelection.top), overlayPaint);
      }
      if (clampedSelection.bottom < size.height) {
        canvas.drawRect(Rect.fromLTWH(0, clampedSelection.bottom, size.width, size.height - clampedSelection.bottom), overlayPaint);
      }
      if (clampedSelection.left > 0) {
        canvas.drawRect(Rect.fromLTWH(0, clampedSelection.top, clampedSelection.left, clampedSelection.height), overlayPaint);
      }
      if (clampedSelection.right < size.width) {
        canvas.drawRect(Rect.fromLTWH(clampedSelection.right, clampedSelection.top, size.width - clampedSelection.right, clampedSelection.height), overlayPaint);
      }
    } else {
      canvas.drawRect(Offset.zero & size, overlayPaint);
    }

    if (selectionRect != null && selectionSizeLabel != null) {
      final backgroundPaint = Paint()..color = const Color(0xE6171717);
      final labelPainter = TextPainter(
        text: TextSpan(text: selectionSizeLabel, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w700)),
        textDirection: TextDirection.ltr,
      )..layout();

      final labelOffset = Offset(selectionRect!.left + 12, selectionRect!.top - 28);
      final labelRect = RRect.fromRectAndRadius(Rect.fromLTWH(labelOffset.dx - 8, labelOffset.dy - 4, labelPainter.width + 16, labelPainter.height + 8), const Radius.circular(10));
      canvas.drawRRect(labelRect, backgroundPaint);
      labelPainter.paint(canvas, labelOffset);
    }
  }

  @override
  bool shouldRepaint(covariant _WorkspaceShadePainter oldDelegate) {
    return oldDelegate.selectionRect != selectionRect || oldDelegate.selectionSizeLabel != selectionSizeLabel;
  }
}

class _ScrollingPreviewPainter extends CustomPainter {
  _ScrollingPreviewPainter({required this.frames, required this.totalHeight, required this.traceId});

  final List<ScrollingCapturePreviewFrame> frames;
  final int totalHeight;
  final String traceId;

  @override
  void paint(Canvas canvas, Size size) {
    final paintWatch = Stopwatch()..start();
    if (frames.isEmpty || totalHeight <= 0) {
      return;
    }

    final contentWidth = frames.first.pixelWidth.toDouble();
    final scale = math.min(size.width / contentWidth, size.height / totalHeight);
    final renderedWidth = contentWidth * scale;
    final dx = (size.width - renderedWidth) / 2;
    var y = 0.0;
    final paint = Paint()..filterQuality = FilterQuality.medium;

    // The preview is rendered from the same stitched frame list used for export. Painting from the
    // accumulated frame crops avoids a separate preview-only bitmap and keeps export/preview drift
    // out of the scrolling capture flow.
    for (final frame in frames) {
      final visibleHeight = frame.visibleHeight;
      if (visibleHeight <= 0) {
        continue;
      }

      final scaledHeight = visibleHeight * scale;
      _paintScrollingFrame(canvas: canvas, frame: frame, dx: dx, destinationY: y, destinationWidth: renderedWidth, destinationHeight: scaledHeight, paint: paint);
      y += scaledHeight;
    }

    // Preview paint is the final visible boundary of the scrolling-capture pipeline. Logging it
    // separately lets timing analysis distinguish slow capture/stitching from slow Flutter repaint.
    Logger.instance.debug(
      traceId,
      'scrolling_capture_timing event=preview_paint frameCount=${frames.length} totalHeight=$totalHeight canvas=${size.width.round()}x${size.height.round()} elapsedMs=${paintWatch.elapsedMilliseconds}',
    );
  }

  void _paintScrollingFrame({
    required Canvas canvas,
    required ScrollingCapturePreviewFrame frame,
    required double dx,
    required double destinationY,
    required double destinationWidth,
    required double destinationHeight,
    required Paint paint,
  }) {
    canvas.drawImageRect(
      frame.image,
      Rect.fromLTWH(0, frame.cropTop.toDouble(), frame.pixelWidth.toDouble(), frame.visibleHeight.toDouble()),
      Rect.fromLTWH(dx, destinationY, destinationWidth, destinationHeight),
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

    // Match the export renderer by feathering each frame's top seam in preview too. This prevents
    // the live side preview from showing horizontal hard cuts that will not represent the final PNG.
    final destinationRect = Rect.fromLTWH(dx, destinationY - featherHeight, destinationWidth, featherHeight);
    canvas.saveLayer(destinationRect, Paint());
    canvas.drawImageRect(frame.image, Rect.fromLTWH(0, (frame.cropTop - featherRows).toDouble(), frame.pixelWidth.toDouble(), featherRows.toDouble()), destinationRect, paint);
    canvas.drawRect(
      destinationRect,
      Paint()
        ..blendMode = BlendMode.dstIn
        ..shader = const LinearGradient(colors: [Color(0x00000000), Color(0xFF000000)], begin: Alignment.topCenter, end: Alignment.bottomCenter).createShader(destinationRect),
    );
    canvas.restore();
  }

  @override
  bool shouldRepaint(covariant _ScrollingPreviewPainter oldDelegate) {
    return oldDelegate.frames != frames || oldDelegate.totalHeight != totalHeight || oldDelegate.traceId != traceId;
  }
}

class _MosaicBrushMarkerPainter extends CustomPainter {
  const _MosaicBrushMarkerPainter({required this.center, required this.radius, required this.color});

  final Offset? center;
  final double radius;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final markerCenter = center;
    if (markerCenter == null) {
      return;
    }

    final fill =
        Paint()
          ..color = color.withValues(alpha: 0.12)
          ..style = PaintingStyle.fill;
    final darkStroke =
        Paint()
          ..color = const Color(0xAA000000)
          ..strokeWidth = 3
          ..style = PaintingStyle.stroke;
    final colorStroke =
        Paint()
          ..color = color
          ..strokeWidth = 2
          ..style = PaintingStyle.stroke;

    canvas.drawCircle(markerCenter, radius, fill);
    canvas.drawCircle(markerCenter, radius, darkStroke);
    canvas.drawCircle(markerCenter, radius, colorStroke);
  }

  @override
  bool shouldRepaint(covariant _MosaicBrushMarkerPainter oldDelegate) {
    return oldDelegate.center != center || oldDelegate.radius != radius || oldDelegate.color != color;
  }
}

class _AnnotationPainter extends CustomPainter {
  _AnnotationPainter({
    required this.annotations,
    required this.snapshots,
    required this.decodedImages,
    required this.canvasOrigin,
    required this.selectionClipRect,
    required this.draftRect,
    required this.draftStart,
    required this.draftEnd,
    required this.draftType,
    required this.previewColor,
    required this.selectedAnnotationId,
    required this.editingTextAnnotationId,
  });

  final List<ScreenshotAnnotation> annotations;
  final List<DisplaySnapshot> snapshots;
  final Map<String, ui.Image> decodedImages;
  final Offset canvasOrigin;
  final Rect? selectionClipRect;
  final Rect? draftRect;
  final Offset? draftStart;
  final Offset? draftEnd;
  final ScreenshotAnnotationType? draftType;
  final Color previewColor;
  final String? selectedAnnotationId;
  final String? editingTextAnnotationId;

  @override
  void paint(Canvas canvas, Size size) {
    final visibleAnnotations = editingTextAnnotationId == null ? annotations : annotations.where((annotation) => annotation.id != editingTextAnnotationId).toList(growable: false);
    final mosaicSources = _buildMosaicSources();

    // Inline text editing paints the caret field directly above the annotation. Hiding the source
    // text while that editor is active prevents the stale rendered label from peeking through and
    // keeps editing visually identical to the non-editing state apart from the caret itself.
    paintWorkspaceAnnotations(canvas, annotations: visibleAnnotations, canvasOrigin: canvasOrigin, selectionClipRect: selectionClipRect, mosaicSources: mosaicSources);
    if (draftType != null) {
      final previewAnnotations = <ScreenshotAnnotation>[];
      if (draftType == ScreenshotAnnotationType.arrow && draftStart != null && draftEnd != null) {
        previewAnnotations.add(ScreenshotAnnotation(id: 'draft-arrow', type: ScreenshotAnnotationType.arrow, start: draftStart, end: draftEnd, color: previewColor));
      } else if (draftRect != null) {
        previewAnnotations.add(ScreenshotAnnotation(id: 'draft-shape', type: draftType!, rect: draftRect, color: previewColor));
      }
      paintWorkspaceAnnotations(canvas, annotations: previewAnnotations, canvasOrigin: canvasOrigin, selectionClipRect: selectionClipRect, mosaicSources: mosaicSources);
    }

    ScreenshotAnnotation? selectedAnnotation;
    if (selectedAnnotationId != null) {
      for (final annotation in annotations) {
        if (annotation.id == selectedAnnotationId) {
          selectedAnnotation = annotation;
          break;
        }
      }
    }
    if (selectedAnnotation != null) {
      _paintSelectedAnnotationControls(canvas, selectedAnnotation, canvasOrigin);
    }
  }

  List<ScreenshotMosaicSource> _buildMosaicSources() {
    final sources = <ScreenshotMosaicSource>[];
    for (final snapshot in snapshots) {
      final image = decodedImages[snapshot.displayId];
      if (image == null) {
        continue;
      }

      // The preview painter reuses the controller's decoded images so the mosaic mask
      // pixelates the same display pixels that the export path later writes to the PNG.
      sources.add(
        ScreenshotMosaicSource(
          image: image,
          sourceRect: Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
          destinationRect: snapshot.logicalBounds.toRect().shift(-canvasOrigin),
        ),
      );
    }
    return sources;
  }

  void _paintSelectedAnnotationControls(Canvas canvas, ScreenshotAnnotation annotation, Offset origin) {
    final handleFill = Paint()..color = Colors.white;
    final handleStroke =
        Paint()
          ..color = const Color(0xCC111111)
          ..strokeWidth = 1
          ..style = PaintingStyle.stroke;

    switch (annotation.type) {
      case ScreenshotAnnotationType.rect:
      case ScreenshotAnnotationType.ellipse:
        final rect = annotation.rect?.shift(-origin);
        if (rect == null) {
          return;
        }
        // The previous selected-state outline wrapped the whole annotation in a bright white frame.
        // That made edits feel noisy and partially obscured the mark itself even though the drag
        // handles already communicate editability. Keep only the handles so selection stays clear
        // without repainting an extra border over the user's annotation.
        for (final handle in _shapeAnnotationHandles) {
          final handleCenter = _annotationHandleOffsetForSelection(rect, handle);
          final handleRect = Rect.fromCenter(center: handleCenter, width: _annotationHandleSize, height: _annotationHandleSize);
          canvas.drawRRect(RRect.fromRectAndRadius(handleRect, const Radius.circular(4)), handleFill);
          canvas.drawRRect(RRect.fromRectAndRadius(handleRect, const Radius.circular(4)), handleStroke);
        }
        break;
      case ScreenshotAnnotationType.arrow:
        final start = annotation.start;
        final end = annotation.end;
        if (start == null || end == null) {
          return;
        }
        for (final handleCenter in <Offset>[start - origin, end - origin]) {
          canvas.drawCircle(handleCenter, _annotationHandleSize / 2, handleFill);
          canvas.drawCircle(handleCenter, _annotationHandleSize / 2, handleStroke);
        }
        break;
      case ScreenshotAnnotationType.text:
        break;
      case ScreenshotAnnotationType.mosaic:
        final bounds = screenshotAnnotationBounds(annotation)?.shift(-origin);
        if (bounds == null) {
          return;
        }
        // Mosaic annotations do not have resize handles because changing the brush footprint after
        // drawing would make the privacy mask unpredictable; show a subtle color-matched outline instead.
        canvas.drawRRect(
          RRect.fromRectAndRadius(bounds.inflate(2), const Radius.circular(8)),
          Paint()
            ..color = annotation.color
            ..strokeWidth = 1.5
            ..style = PaintingStyle.stroke,
        );
        break;
    }
  }

  Offset _annotationHandleOffsetForSelection(Rect rect, _AnnotationHandle handle) {
    switch (handle) {
      case _AnnotationHandle.topLeft:
        return rect.topLeft;
      case _AnnotationHandle.top:
        return Offset(rect.center.dx, rect.top);
      case _AnnotationHandle.topRight:
        return rect.topRight;
      case _AnnotationHandle.right:
        return Offset(rect.right, rect.center.dy);
      case _AnnotationHandle.bottomRight:
        return rect.bottomRight;
      case _AnnotationHandle.bottom:
        return Offset(rect.center.dx, rect.bottom);
      case _AnnotationHandle.bottomLeft:
        return rect.bottomLeft;
      case _AnnotationHandle.left:
        return Offset(rect.left, rect.center.dy);
      case _AnnotationHandle.arrowStart:
      case _AnnotationHandle.arrowEnd:
        return rect.center;
    }
  }

  @override
  bool shouldRepaint(covariant _AnnotationPainter oldDelegate) {
    return oldDelegate.annotations != annotations ||
        oldDelegate.canvasOrigin != canvasOrigin ||
        oldDelegate.selectionClipRect != selectionClipRect ||
        oldDelegate.draftRect != draftRect ||
        oldDelegate.draftStart != draftStart ||
        oldDelegate.draftEnd != draftEnd ||
        oldDelegate.draftType != draftType ||
        oldDelegate.previewColor != previewColor ||
        oldDelegate.snapshots != snapshots ||
        oldDelegate.decodedImages != decodedImages ||
        oldDelegate.selectedAnnotationId != selectedAnnotationId ||
        oldDelegate.editingTextAnnotationId != editingTextAnnotationId;
  }
}

void paintWorkspaceAnnotations(
  Canvas canvas, {
  required List<ScreenshotAnnotation> annotations,
  required Offset canvasOrigin,
  required Rect? selectionClipRect,
  List<ScreenshotMosaicSource> mosaicSources = const <ScreenshotMosaicSource>[],
}) {
  if (selectionClipRect != null) {
    // Annotation tools must stay visually inside the captured selection. Clipping the workspace
    // paint to the selection rect keeps shapes aligned with the user's drag origin and prevents
    // any stroke from leaking outside the active capture region.
    canvas.save();
    canvas.clipRect(selectionClipRect);
  }

  paintScreenshotAnnotations(canvas, annotations, canvasOrigin, mosaicSources: mosaicSources);

  if (selectionClipRect != null) {
    canvas.restore();
  }
}

const List<_AnnotationHandle> _shapeAnnotationHandles = <_AnnotationHandle>[
  _AnnotationHandle.topLeft,
  _AnnotationHandle.top,
  _AnnotationHandle.topRight,
  _AnnotationHandle.right,
  _AnnotationHandle.bottomRight,
  _AnnotationHandle.bottom,
  _AnnotationHandle.bottomLeft,
  _AnnotationHandle.left,
];
