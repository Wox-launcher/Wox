import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

class WoxQueryVariableTextField extends StatefulWidget {
  final TextEditingController? controller;
  final FocusNode? focusNode;
  final int maxLines;
  final ValueChanged<String>? onChanged;

  const WoxQueryVariableTextField({super.key, required this.controller, required this.focusNode, required this.maxLines, required this.onChanged});

  @override
  State<WoxQueryVariableTextField> createState() => _WoxQueryVariableTextFieldState();
}

class _WoxQueryVariableTextFieldState extends State<WoxQueryVariableTextField> {
  final LayerLink _layerLink = LayerLink();
  static final RegExp _queryVariablePattern = RegExp(r'\{wox:[a-zA-Z0-9_]+\}');
  final List<_QueryVariableOption> _options = const [
    _QueryVariableOption(
      id: 'selected_text',
      value: '{wox:selected_text}',
      labelKey: 'ui_query_variable_selected_text',
      descriptionKey: 'ui_query_variable_selected_text_tooltip',
      icon: Icons.text_fields,
    ),
    _QueryVariableOption(
      id: 'selected_file',
      value: '{wox:selected_file}',
      labelKey: 'ui_query_variable_selected_file',
      descriptionKey: 'ui_query_variable_selected_file_tooltip',
      icon: Icons.insert_drive_file_outlined,
    ),
    _QueryVariableOption(
      id: 'active_browser_url',
      value: '{wox:active_browser_url}',
      labelKey: 'ui_query_variable_active_browser_url',
      descriptionKey: 'ui_query_variable_active_browser_url_tooltip',
      icon: Icons.public,
    ),
    _QueryVariableOption(
      id: 'file_explorer_path',
      value: '{wox:file_explorer_path}',
      labelKey: 'ui_query_variable_file_explorer_path',
      descriptionKey: 'ui_query_variable_file_explorer_path_tooltip',
      icon: Icons.folder_open,
    ),
  ];

  OverlayEntry? _overlayEntry;
  int _selectedIndex = 0;
  int? _replaceStart;
  bool _openedFromButton = false;
  TextSelection? _buttonOpenSelection;
  late TextEditingController _fallbackController;
  late QueryVariableTextEditingController _highlightController;
  late FocusNode _fallbackFocusNode;
  bool _isSyncingController = false;

  TextEditingController get _controller => _highlightController;

  FocusNode get _focusNode => widget.focusNode ?? _fallbackFocusNode;

  @override
  void initState() {
    super.initState();
    _fallbackController = TextEditingController();
    _highlightController = QueryVariableTextEditingController(text: widget.controller?.text ?? _fallbackController.text);
    _highlightController.addListener(_syncHighlightControllerToExternal);
    widget.controller?.addListener(_syncExternalControllerToHighlight);
    _fallbackFocusNode = FocusNode();
  }

  @override
  void dispose() {
    _removeOverlay();
    widget.controller?.removeListener(_syncExternalControllerToHighlight);
    _highlightController.removeListener(_syncHighlightControllerToExternal);
    _highlightController.dispose();
    _fallbackController.dispose();
    _fallbackFocusNode.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant WoxQueryVariableTextField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller == widget.controller) {
      return;
    }

    oldWidget.controller?.removeListener(_syncExternalControllerToHighlight);
    widget.controller?.addListener(_syncExternalControllerToHighlight);
    _syncExternalControllerToHighlight();
  }

  void _syncExternalControllerToHighlight() {
    final externalController = widget.controller;
    if (_isSyncingController || externalController == null || externalController.value == _highlightController.value) {
      return;
    }

    _isSyncingController = true;
    _highlightController.value = externalController.value;
    _isSyncingController = false;
  }

  void _syncHighlightControllerToExternal() {
    final externalController = widget.controller ?? _fallbackController;
    if (_isSyncingController || externalController.value == _highlightController.value) {
      return;
    }

    _isSyncingController = true;
    externalController.value = _highlightController.value;
    _isSyncingController = false;
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  List<_QueryVariableOption> get _filteredOptions {
    final triggerStart = _replaceStart;
    if (triggerStart == null || _openedFromButton) {
      return _options;
    }

    final selection = _controller.selection;
    if (!selection.isValid || selection.baseOffset != selection.extentOffset || selection.extentOffset <= triggerStart) {
      return _options;
    }

    final query = _controller.text.substring(triggerStart + 1, selection.extentOffset).toLowerCase();
    if (query.isEmpty) {
      return _options;
    }

    final filtered =
        _options.where((option) {
          final label = tr(option.labelKey).toLowerCase();
          return option.value.toLowerCase().contains(query) || label.contains(query) || option.id.contains(query);
        }).toList();
    return filtered.isEmpty ? _options : filtered;
  }

  void _handleChanged(String value) {
    widget.onChanged?.call(value);
    final triggerStart = _findActiveTriggerStart();
    if (triggerStart == null) {
      if (!_openedFromButton) {
        _removeOverlay();
      }
      return;
    }

    _openedFromButton = false;
    _replaceStart = triggerStart;
    _selectedIndex = 0;
    _showOverlay();
  }

  int? _findActiveTriggerStart() {
    final selection = _controller.selection;
    if (!selection.isValid || selection.baseOffset != selection.extentOffset) {
      return null;
    }

    final cursor = selection.extentOffset;
    if (cursor <= 0 || cursor > _controller.text.length) {
      return null;
    }

    final beforeCursor = _controller.text.substring(0, cursor);
    final start = beforeCursor.lastIndexOf('{');
    if (start < 0) {
      return null;
    }

    final token = beforeCursor.substring(start + 1);
    if (token.contains('}') || token.contains(RegExp(r'\s'))) {
      return null;
    }

    return start;
  }

  void _openFromButton() {
    _openedFromButton = true;
    _buttonOpenSelection = _controller.selection;
    _replaceStart = null;
    _selectedIndex = 0;
    _showOverlay();
    _focusNode.requestFocus();
  }

  void _showOverlay() {
    if (_overlayEntry != null) {
      _overlayEntry!.markNeedsBuild();
      return;
    }

    _overlayEntry = OverlayEntry(
      builder:
          (context) => GestureDetector(
            behavior: HitTestBehavior.translucent,
            onTap: _removeOverlay,
            child: Stack(
              children: [
                CompositedTransformFollower(
                  link: _layerLink,
                  showWhenUnlinked: false,
                  offset: const Offset(0, 42),
                  child: Material(color: Colors.transparent, child: _buildPicker()),
                ),
              ],
            ),
          ),
    );

    Overlay.of(context).insert(_overlayEntry!);
  }

  void _removeOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    _replaceStart = null;
    _openedFromButton = false;
    _buttonOpenSelection = null;
  }

  bool _deleteWholeVariable({required bool forward}) {
    final selection = _controller.selection;
    if (!selection.isValid || !selection.isCollapsed) {
      return false;
    }

    final caret = selection.extentOffset;
    for (final match in _queryVariablePattern.allMatches(_controller.text)) {
      final shouldDelete = forward ? caret >= match.start && caret < match.end : caret > match.start && caret <= match.end;
      if (!shouldDelete) {
        continue;
      }

      // Placeholder tokens represent runtime values, so partial deletion would
      // leave invalid `{wox:...}` fragments. Delete the whole token whenever the
      // caret is inside or next to one.
      final updatedText = _controller.text.replaceRange(match.start, match.end, '');
      _controller.value = TextEditingValue(text: updatedText, selection: TextSelection.collapsed(offset: match.start));
      widget.onChanged?.call(updatedText);
      return true;
    }

    return false;
  }

  bool _moveCaretAcrossVariable({required bool forward}) {
    final selection = _controller.selection;
    if (!selection.isValid || !selection.isCollapsed) {
      return false;
    }

    final caret = selection.extentOffset;
    for (final match in _queryVariablePattern.allMatches(_controller.text)) {
      final shouldMove = forward ? caret >= match.start && caret < match.end : caret > match.start && caret <= match.end;
      if (!shouldMove) {
        continue;
      }

      _controller.selection = TextSelection.collapsed(offset: forward ? match.end : match.start);
      return true;
    }

    return false;
  }

  void _normalizeCaretOutsideVariable() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }

      final selection = _controller.selection;
      if (!selection.isValid || !selection.isCollapsed) {
        return;
      }

      final caret = selection.extentOffset;
      for (final match in _queryVariablePattern.allMatches(_controller.text)) {
        if (caret <= match.start || caret >= match.end) {
          continue;
        }

        // Mouse clicks can land inside a rendered variable token. Snap the caret
        // to the nearest token edge so subsequent typing, arrow movement, and
        // deletion treat the runtime placeholder as one atomic value.
        final distanceToStart = caret - match.start;
        final distanceToEnd = match.end - caret;
        _controller.selection = TextSelection.collapsed(offset: distanceToStart <= distanceToEnd ? match.start : match.end);
        return;
      }
    });
  }

  void _insertOption(_QueryVariableOption option) {
    final selection = _openedFromButton ? (_buttonOpenSelection ?? _controller.selection) : _controller.selection;
    final text = _controller.text;
    int start;
    int end;

    // The picker replaces the active `{...` token when typing triggered it, but
    // uses the current selection/caret when the explicit button opened it. This
    // keeps fast placeholder completion from damaging normal manual edits.
    if (!_openedFromButton && _replaceStart != null && selection.isValid && selection.baseOffset == selection.extentOffset && _replaceStart! <= selection.extentOffset) {
      start = _replaceStart!;
      end = selection.extentOffset;
    } else if (selection.isValid) {
      start = selection.start;
      end = selection.end;
    } else {
      start = text.length;
      end = text.length;
    }

    final normalizedStart = start.clamp(0, text.length).toInt();
    final normalizedEnd = end.clamp(normalizedStart, text.length).toInt();
    final updatedText = text.replaceRange(normalizedStart, normalizedEnd, option.value);
    final cursorOffset = normalizedStart + option.value.length;
    _controller.value = TextEditingValue(text: updatedText, selection: TextSelection.collapsed(offset: cursorOffset));
    widget.onChanged?.call(updatedText);
    _removeOverlay();
    _focusNode.requestFocus();
  }

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) {
      return KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.escape && _overlayEntry != null) {
      _removeOverlay();
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.backspace && _deleteWholeVariable(forward: false)) {
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.delete && _deleteWholeVariable(forward: true)) {
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowLeft && _moveCaretAcrossVariable(forward: false)) {
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowRight && _moveCaretAcrossVariable(forward: true)) {
      return KeyEventResult.handled;
    }

    if (_overlayEntry == null) {
      return KeyEventResult.ignored;
    }

    final options = _filteredOptions;
    if (options.isEmpty) {
      return KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
      _selectedIndex = (_selectedIndex + 1) % options.length;
      _overlayEntry?.markNeedsBuild();
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
      _selectedIndex = (_selectedIndex - 1 + options.length) % options.length;
      _overlayEntry?.markNeedsBuild();
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.enter || event.logicalKey == LogicalKeyboardKey.tab) {
      _insertOption(options[_selectedIndex.clamp(0, options.length - 1).toInt()]);
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  Widget _buildPicker() {
    final options = _filteredOptions;
    final background = getThemePopupSurfaceColor();
    final textColor = getThemeTextColor();
    final borderColor = getThemeSubTextColor().withValues(alpha: 0.55);
    final activeColor = getThemeActiveBackgroundColor();

    return Container(
      width: 360,
      constraints: const BoxConstraints(maxHeight: 260),
      decoration: BoxDecoration(
        color: background,
        border: Border.all(color: borderColor),
        borderRadius: BorderRadius.circular(4),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.18), blurRadius: 18, offset: const Offset(0, 10))],
      ),
      child: ListView.builder(
        shrinkWrap: true,
        padding: const EdgeInsets.symmetric(vertical: 6),
        itemCount: options.length,
        itemBuilder: (context, index) {
          final option = options[index];
          final isSelected = index == _selectedIndex;
          return InkWell(
            key: ValueKey('query-variable-picker-option-${option.id}'),
            onTap: () => _insertOption(option),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
              color: isSelected ? activeColor.withValues(alpha: 0.18) : Colors.transparent,
              child: Row(
                children: [
                  Icon(option.icon, color: textColor.withValues(alpha: 0.82), size: 18),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(tr(option.labelKey), maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 13, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 2),
                        Text(tr(option.descriptionKey), maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor.withValues(alpha: 0.62), fontSize: 12)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    // Query hotkeys used to require manually typing long `{wox:...}` placeholders.
    // Keeping the picker in this text-field wrapper preserves the generic table
    // editor while giving placeholder-aware columns a fast insertion path.
    return Focus(
      onKeyEvent: _handleKeyEvent,
      child: CompositedTransformTarget(
        link: _layerLink,
        child: WoxTextField(
          textFieldKey: ValueKey('query-variable-text-field-${widget.key is ValueKey ? (widget.key as ValueKey).value : 'field'}'),
          controller: _controller,
          focusNode: _focusNode,
          maxLines: widget.maxLines,
          onChanged: _handleChanged,
          onTap: _normalizeCaretOutsideVariable,
          suffixIcon: WoxTooltip(
            message: tr('ui_query_variable_picker_insert'),
            child: IconButton(
              key: ValueKey('query-variable-picker-button-${widget.key is ValueKey ? (widget.key as ValueKey).value : 'field'}'),
              icon: Icon(Icons.data_object, size: 18, color: getThemeTextColor().withValues(alpha: 0.72)),
              splashRadius: 16,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 34, minHeight: 34),
              onPressed: _openFromButton,
            ),
          ),
        ),
      ),
    );
  }
}

class QueryVariableTextEditingController extends TextEditingController {
  QueryVariableTextEditingController({super.text});

  static final RegExp _queryVariablePattern = RegExp(r'\{wox:[a-zA-Z0-9_]+\}');

  @override
  TextSpan buildTextSpan({required BuildContext context, TextStyle? style, required bool withComposing}) {
    final baseStyle = style ?? DefaultTextStyle.of(context).style;
    const variableBraceStyle = TextStyle(color: Color(0xFF6D28D9), fontWeight: FontWeight.w700);
    final spans = <InlineSpan>[];
    var cursor = 0;

    for (final match in _queryVariablePattern.allMatches(text)) {
      if (match.start > cursor) {
        spans.add(TextSpan(text: text.substring(cursor, match.start), style: baseStyle));
      }

      // Only the braces are colored because a full-token pill is too visually
      // heavy inside a normal text input. The keyboard handlers still keep the
      // whole `{wox:...}` token atomic for caret movement and deletion.
      final variableText = match.group(0) ?? "";
      spans.add(
        TextSpan(
          children: [
            TextSpan(text: "{", style: baseStyle.merge(variableBraceStyle)),
            TextSpan(text: variableText.length > 2 ? variableText.substring(1, variableText.length - 1) : "", style: baseStyle),
            TextSpan(text: "}", style: baseStyle.merge(variableBraceStyle)),
          ],
        ),
      );
      cursor = match.end;
    }

    if (cursor < text.length) {
      spans.add(TextSpan(text: text.substring(cursor), style: baseStyle));
    }

    return TextSpan(style: baseStyle, children: spans);
  }
}

class _QueryVariableOption {
  final String id;
  final String value;
  final String labelKey;
  final String descriptionKey;
  final IconData icon;

  const _QueryVariableOption({required this.id, required this.value, required this.labelKey, required this.descriptionKey, required this.icon});
}
