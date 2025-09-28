import 'package:flutter/material.dart';

/// A [TextEditingController] that applies a custom style to the currently selected text.
class QueryBoxTextEditingController extends TextEditingController {
  QueryBoxTextEditingController({required TextStyle selectedTextStyle}) : _selectedTextStyle = selectedTextStyle;

  TextStyle _selectedTextStyle;

  TextStyle get selectedTextStyle => _selectedTextStyle;

  /// Updates the style used for rendering selected text and notifies listeners when it changes.
  void updateSelectedTextStyle(TextStyle style) {
    if (_selectedTextStyle == style) {
      return;
    }

    _selectedTextStyle = style;
    notifyListeners();
  }

  @override
  TextSpan buildTextSpan({required BuildContext context, TextStyle? style, bool withComposing = false}) {
    final selection = value.selection;
    final hasStyledSelection = selection.isValid && !selection.isCollapsed;

    if (!hasStyledSelection) {
      return super.buildTextSpan(
        context: context,
        style: style,
        withComposing: withComposing,
      );
    }

    final composing = withComposing && value.composing.isValid ? value.composing : null;
    final text = value.text;

    final boundaries = <int>{0, text.length, selection.start, selection.end};
    if (composing != null) {
      boundaries
        ..add(composing.start)
        ..add(composing.end);
    }

    final sortedBoundaries = boundaries.toList()..sort();
    final children = <InlineSpan>[];

    for (var i = 0; i < sortedBoundaries.length - 1; i++) {
      final start = sortedBoundaries[i];
      final end = sortedBoundaries[i + 1];
      if (start >= end) {
        continue;
      }

      final segmentText = text.substring(start, end);
      TextStyle? segmentStyle;

      final inSelection = start >= selection.start && start < selection.end;
      final inComposing = composing != null && start >= composing.start && start < composing.end;

      if (inSelection) {
        segmentStyle = _selectedTextStyle;
      }

      if (inComposing) {
        const composingStyle = TextStyle(decoration: TextDecoration.underline);
        segmentStyle = segmentStyle == null ? composingStyle : segmentStyle.merge(composingStyle);
      }

      children.add(TextSpan(text: segmentText, style: segmentStyle));
    }

    return TextSpan(style: style, children: children);
  }
}
