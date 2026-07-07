import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_preview_dictation_history.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxDictationHistoryPreviewView extends StatefulWidget {
  final WoxDictationHistoryPreviewData data;
  final WoxTheme woxTheme;

  const WoxDictationHistoryPreviewView({super.key, required this.data, required this.woxTheme});

  @override
  State<WoxDictationHistoryPreviewView> createState() => _WoxDictationHistoryPreviewViewState();
}

class _WoxDictationHistoryPreviewViewState extends State<WoxDictationHistoryPreviewView> {
  late WoxDictationHistoryPreviewData _data;
  final WoxLauncherController _launcherController = Get.find<WoxLauncherController>();
  final GlobalKey _textKey = GlobalKey();
  OverlayEntry? _correctionOverlay;
  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  @override
  void initState() {
    super.initState();
    _data = widget.data;
  }

  @override
  void didUpdateWidget(covariant WoxDictationHistoryPreviewView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.data.recordId != widget.data.recordId || oldWidget.data.content != widget.data.content) {
      _hideCorrectionOverlay();
      _data = widget.data;
    }
  }

  @override
  void dispose() {
    _hideCorrectionOverlay();
    super.dispose();
  }

  String _tr(String key) => _launcherController.tr(key);

  TextStyle _currentTextStyle(BuildContext context) {
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor, defaultColor: Theme.of(context).colorScheme.onSurface);
    return TextStyle(color: textColor.withValues(alpha: 0.88), fontSize: _metrics.previewTextFontSize, height: 1.55, fontWeight: FontWeight.w400, letterSpacing: 0);
  }

  void _handleSelectionChanged(TextSelection selection, SelectionChangedCause? cause) {
    if (!selection.isValid || selection.isCollapsed) {
      _hideCorrectionOverlay();
      return;
    }

    final display = _data.buildCorrectionDisplay();
    final start = math.min(selection.start, selection.end);
    final end = math.max(selection.start, selection.end);
    if (start < 0 || end > display.displayText.length || start == end) {
      _hideCorrectionOverlay();
      return;
    }

    final contentRange = display.contentRangeForDisplayRange(start, end);
    if (contentRange.start < 0 ||
        contentRange.end > _data.content.length ||
        contentRange.start == contentRange.end ||
        _data.content.substring(contentRange.start, contentRange.end).trim().isEmpty) {
      _hideCorrectionOverlay();
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _showCorrectionOverlay(selection, TextSelection(baseOffset: contentRange.start, extentOffset: contentRange.end));
      }
    });
  }

  void _showCorrectionOverlay(TextSelection displaySelection, TextSelection contentSelection) {
    final anchors = _anchorsForSelection(displaySelection);
    if (anchors == null) {
      _hideCorrectionOverlay();
      return;
    }

    final start = math.min(contentSelection.start, contentSelection.end);
    final end = math.max(contentSelection.start, contentSelection.end);
    final selectedText = _data.content.substring(start, end);
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor, defaultColor: Theme.of(context).colorScheme.onSurface);
    final colors = _DictationCorrectionColors.fromTheme(context, widget.woxTheme, textColor);

    _hideCorrectionOverlay();
    _correctionOverlay = OverlayEntry(
      builder: (overlayContext) {
        return _DictationCorrectionToolbar(
          anchors: anchors,
          selectedText: selectedText,
          actionLabel: _tr("plugin_dictation_correction_action"),
          inputHint: _tr("plugin_dictation_correction_hint"),
          savingLabel: _tr("plugin_dictation_correction_saving"),
          colors: colors,
          onCancel: _hideCorrectionOverlay,
          onSubmit: (replacementText) => _saveCorrection(contentSelection, selectedText, replacementText),
        );
      },
    );
    Overlay.of(context).insert(_correctionOverlay!);
  }

  TextSelectionToolbarAnchors? _anchorsForSelection(TextSelection selection) {
    final textContext = _textKey.currentContext;
    final renderObject = textContext?.findRenderObject();
    if (textContext == null || renderObject is! RenderBox || !renderObject.hasSize) {
      return null;
    }

    final direction = Directionality.of(context);
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor, defaultColor: Theme.of(context).colorScheme.onSurface);
    final display = _data.buildCorrectionDisplay();
    final textPainter = TextPainter(text: _buildContentTextSpan(display, _currentTextStyle(context), textColor), textAlign: TextAlign.left, textDirection: direction)
      ..layout(maxWidth: renderObject.size.width);
    final boxes = textPainter.getBoxesForSelection(selection);
    if (boxes.isEmpty) {
      return null;
    }

    var selectionRect = boxes.first.toRect();
    for (final box in boxes.skip(1)) {
      selectionRect = selectionRect.expandToInclude(box.toRect());
    }

    final topAnchor = renderObject.localToGlobal(Offset(selectionRect.left + selectionRect.width / 2, selectionRect.top));
    final bottomAnchor = renderObject.localToGlobal(Offset(selectionRect.left + selectionRect.width / 2, selectionRect.bottom));
    return TextSelectionToolbarAnchors(primaryAnchor: topAnchor, secondaryAnchor: bottomAnchor);
  }

  void _hideCorrectionOverlay() {
    _correctionOverlay?.remove();
    _correctionOverlay = null;
  }

  Future<String?> _saveCorrection(TextSelection selection, String selectedText, String replacementText) async {
    if (replacementText.trim().isEmpty) {
      return _tr("plugin_dictation_correction_empty");
    }

    final start = math.min(selection.start, selection.end);
    final end = math.max(selection.start, selection.end);
    if (start < 0 || end > _data.content.length || start == end) {
      return _tr("plugin_dictation_correction_failed");
    }

    final previousContent = _data.content;
    final updatedContent = previousContent.replaceRange(start, end, replacementText);
    if (updatedContent == previousContent) {
      return _tr("plugin_dictation_correction_no_change");
    }

    final traceId = const UuidV4().generate();
    try {
      final response = await WoxApi.instance.correctDictationHistory(
        traceId,
        recordId: _data.recordId,
        previousContent: previousContent,
        selectedText: selectedText,
        replacementText: replacementText,
        updatedContent: updatedContent,
      );
      if (!mounted) {
        return null;
      }
      setState(() {
        _data = response.toPreviewData();
      });
      _hideCorrectionOverlay();
      return null;
    } catch (e) {
      Logger.instance.error(traceId, "Failed to correct dictation history: $e");
      if (e.toString().contains("content changed")) {
        return _tr("plugin_dictation_correction_stale");
      }
      return _tr("plugin_dictation_correction_failed");
    }
  }

  @override
  Widget build(BuildContext context) {
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor, defaultColor: Theme.of(context).colorScheme.onSurface);
    final currentTextStyle = _currentTextStyle(context);
    final display = _data.buildCorrectionDisplay();

    return Padding(
      padding: EdgeInsets.all(_metrics.previewTextPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          KeyedSubtree(
            key: _textKey,
            child: WoxSelectableText.rich(
              _buildContentTextSpan(display, currentTextStyle, textColor),
              textAlign: TextAlign.left,
              style: currentTextStyle,
              onSelectionChanged: _handleSelectionChanged,
              contextMenuBuilder: (_, _) => const SizedBox.shrink(),
            ),
          ),
        ],
      ),
    );
  }

  TextSpan _buildContentTextSpan(WoxDictationHistoryCorrectionDisplay display, TextStyle currentTextStyle, Color textColor) {
    final correctionBackground = safeFromCssColor(widget.woxTheme.actionItemActiveBackgroundColor, defaultColor: textColor.withValues(alpha: 0.12)).withValues(alpha: 0.22);
    final oldStyle = currentTextStyle.copyWith(
      color: textColor.withValues(alpha: 0.42),
      decoration: TextDecoration.lineThrough,
      decorationColor: textColor.withValues(alpha: 0.44),
      backgroundColor: correctionBackground,
      fontWeight: FontWeight.w500,
    );
    final newStyle = currentTextStyle.copyWith(color: textColor.withValues(alpha: 0.94), backgroundColor: correctionBackground, fontWeight: FontWeight.w700);

    return TextSpan(
      style: currentTextStyle,
      children: [
        for (final segment in display.segments)
          if (segment.isCorrection) ...[
            TextSpan(text: segment.oldText, style: oldStyle),
            TextSpan(text: segment.newText, style: newStyle),
          ] else
            TextSpan(text: segment.text, style: currentTextStyle),
      ],
    );
  }
}

class _DictationCorrectionToolbar extends StatefulWidget {
  final TextSelectionToolbarAnchors anchors;
  final String selectedText;
  final String actionLabel;
  final String inputHint;
  final String savingLabel;
  final _DictationCorrectionColors colors;
  final VoidCallback onCancel;
  final Future<String?> Function(String replacementText) onSubmit;

  const _DictationCorrectionToolbar({
    required this.anchors,
    required this.selectedText,
    required this.actionLabel,
    required this.inputHint,
    required this.savingLabel,
    required this.colors,
    required this.onCancel,
    required this.onSubmit,
  });

  @override
  State<_DictationCorrectionToolbar> createState() => _DictationCorrectionToolbarState();
}

class _DictationCorrectionToolbarState extends State<_DictationCorrectionToolbar> {
  late final TextEditingController _controller;
  late final FocusNode _focusNode;
  bool _isEditing = false;
  bool _isSaving = false;
  String _error = "";

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.selectedText);
    _focusNode = FocusNode();
    _focusNode.addListener(_handleFocusChange);
  }

  @override
  void dispose() {
    _focusNode.removeListener(_handleFocusChange);
    _focusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  void _handleFocusChange() {
    if (_isEditing && !_isSaving && !_focusNode.hasFocus) {
      scheduleMicrotask(widget.onCancel);
    }
  }

  void _startEditing() {
    if (_isEditing) {
      return;
    }
    setState(() {
      _isEditing = true;
      _error = "";
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      _focusNode.requestFocus();
      _controller.selection = TextSelection(baseOffset: 0, extentOffset: _controller.text.length);
    });
  }

  Future<void> _submit() async {
    if (_isSaving) {
      return;
    }
    setState(() {
      _isSaving = true;
      _error = "";
    });

    final error = await widget.onSubmit(_controller.text);
    if (!mounted) {
      return;
    }
    if (error == null) {
      return;
    }
    setState(() {
      _isSaving = false;
      _error = error;
    });
    _focusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    return CustomSingleChildLayout(
      delegate: _CorrectionBubbleLayoutDelegate(anchorAbove: widget.anchors.primaryAnchor, anchorBelow: widget.anchors.secondaryAnchor ?? widget.anchors.primaryAnchor),
      child: _isEditing ? _buildInputBubble(context) : _buildActionBubble(context),
    );
  }

  Widget _buildActionBubble(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTapDown: (_) => _startEditing(),
        child: Material(
          color: Colors.transparent,
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: widget.colors.actionBackground,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: widget.colors.border.withValues(alpha: 0.7)),
              boxShadow: [BoxShadow(color: widget.colors.shadow, blurRadius: 14, offset: const Offset(0, 6))],
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
              child: Text(widget.actionLabel, style: TextStyle(color: widget.colors.actionForeground, fontSize: 13, fontWeight: FontWeight.w600, letterSpacing: 0)),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInputBubble(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: Container(
        constraints: const BoxConstraints(minWidth: 220, maxWidth: 320),
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: widget.colors.inputBackground,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: widget.colors.border),
          boxShadow: [BoxShadow(color: widget.colors.shadow, blurRadius: 18, offset: const Offset(0, 8))],
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Focus(
              onKeyEvent: (node, event) {
                if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
                  widget.onCancel();
                  return KeyEventResult.handled;
                }
                return KeyEventResult.ignored;
              },
              child: TextField(
                controller: _controller,
                focusNode: _focusNode,
                enabled: !_isSaving,
                autofocus: true,
                minLines: 1,
                maxLines: 1,
                textInputAction: TextInputAction.done,
                onSubmitted: (_) => unawaited(_submit()),
                style: TextStyle(color: widget.colors.inputForeground, fontSize: 13, letterSpacing: 0),
                cursorColor: widget.colors.accent,
                decoration: InputDecoration(
                  isDense: true,
                  hintText: widget.inputHint,
                  hintStyle: TextStyle(color: widget.colors.inputForeground.withValues(alpha: 0.48), fontSize: 13, letterSpacing: 0),
                  contentPadding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
                  filled: true,
                  fillColor: widget.colors.inputFieldBackground,
                  enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: widget.colors.border.withValues(alpha: 0.66))),
                  focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: widget.colors.accent)),
                ),
              ),
            ),
            if (_error.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(_error, style: TextStyle(color: Theme.of(context).colorScheme.error, fontSize: 11, height: 1.2, letterSpacing: 0)),
            ],
            if (_isSaving) ...[
              const SizedBox(height: 6),
              Text(widget.savingLabel, style: TextStyle(color: widget.colors.inputForeground.withValues(alpha: 0.72), fontSize: 11, height: 1.2, letterSpacing: 0)),
            ],
          ],
        ),
      ),
    );
  }
}

class _CorrectionBubbleLayoutDelegate extends SingleChildLayoutDelegate {
  final Offset anchorAbove;
  final Offset anchorBelow;

  const _CorrectionBubbleLayoutDelegate({required this.anchorAbove, required this.anchorBelow});

  static const double _screenPadding = 8;
  static const double _anchorGap = 8;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints constraints) {
    return BoxConstraints.loose(Size(math.max(0, constraints.maxWidth - _screenPadding * 2), math.max(0, constraints.maxHeight - _screenPadding * 2)));
  }

  @override
  Offset getPositionForChild(Size size, Size childSize) {
    final fitsAbove = anchorAbove.dy - childSize.height - _anchorGap >= _screenPadding;
    final rawTop = fitsAbove ? anchorAbove.dy - childSize.height - _anchorGap : anchorBelow.dy + _anchorGap;
    final left = (anchorAbove.dx - childSize.width / 2).clamp(_screenPadding, math.max(_screenPadding, size.width - childSize.width - _screenPadding));
    final top = rawTop.clamp(_screenPadding, math.max(_screenPadding, size.height - childSize.height - _screenPadding));
    return Offset(left.toDouble(), top.toDouble());
  }

  @override
  bool shouldRelayout(covariant _CorrectionBubbleLayoutDelegate oldDelegate) {
    return oldDelegate.anchorAbove != anchorAbove || oldDelegate.anchorBelow != anchorBelow;
  }
}

class _DictationCorrectionColors {
  final Color actionBackground;
  final Color actionForeground;
  final Color inputBackground;
  final Color inputFieldBackground;
  final Color inputForeground;
  final Color border;
  final Color accent;
  final Color shadow;

  const _DictationCorrectionColors({
    required this.actionBackground,
    required this.actionForeground,
    required this.inputBackground,
    required this.inputFieldBackground,
    required this.inputForeground,
    required this.border,
    required this.accent,
    required this.shadow,
  });

  factory _DictationCorrectionColors.fromTheme(BuildContext context, WoxTheme theme, Color fallbackTextColor) {
    final actionBackground = safeFromCssColor(theme.actionItemActiveBackgroundColor, defaultColor: WoxSelectionTheme.selectionColorOf(context));
    final actionForeground = safeFromCssColor(theme.actionItemActiveFontColor, defaultColor: _foregroundFor(actionBackground));
    final inputBackground = safeFromCssColor(
      theme.actionQueryBoxBackgroundColor,
      defaultColor: safeFromCssColor(theme.toolbarBackgroundColor, defaultColor: Theme.of(context).colorScheme.surface),
    );
    final inputForeground = safeFromCssColor(theme.actionQueryBoxFontColor, defaultColor: fallbackTextColor);
    final inputFieldBackground = safeFromCssColor(theme.queryBoxBackgroundColor, defaultColor: inputBackground);
    final border = safeFromCssColor(theme.previewSplitLineColor, defaultColor: actionBackground.withValues(alpha: 0.68));
    final accent = safeFromCssColor(theme.queryBoxCursorColor, defaultColor: actionBackground);
    return _DictationCorrectionColors(
      actionBackground: actionBackground,
      actionForeground: actionForeground,
      inputBackground: inputBackground,
      inputFieldBackground: inputFieldBackground,
      inputForeground: inputForeground,
      border: border,
      accent: accent,
      shadow: Colors.black.withValues(alpha: inputBackground.computeLuminance() > 0.5 ? 0.18 : 0.32),
    );
  }

  static Color _foregroundFor(Color backgroundColor) {
    return backgroundColor.computeLuminance() > 0.5 ? Colors.black : Colors.white;
  }
}
