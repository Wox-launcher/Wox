import 'package:flutter/material.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

/// A [TextEditingController] that renders `{skill:name}` tags as inline pill
/// widgets inside the editable text. Backspace right after a pill deletes the
/// entire tag in one keystroke.
class SkillTagTextEditingController extends TextEditingController {
  SkillTagTextEditingController({super.text});

  static final RegExp _skillTagPattern = RegExp(r'\{skill:([^}]+)\}');

  @override
  TextSpan buildTextSpan({required BuildContext context, TextStyle? style, required bool withComposing}) {
    final text = this.text;
    final matches = _skillTagPattern.allMatches(text);

    if (matches.isEmpty) {
      return super.buildTextSpan(context: context, style: style, withComposing: withComposing);
    }

    final children = <InlineSpan>[];
    int lastEnd = 0;
    for (final match in matches) {
      if (match.start > lastEnd) {
        children.add(TextSpan(text: text.substring(lastEnd, match.start)));
      }
      children.add(WidgetSpan(alignment: PlaceholderAlignment.middle, child: _SkillPill(name: match.group(1)!, fontColor: style?.color)));
      lastEnd = match.end;
    }
    if (lastEnd < text.length) {
      children.add(TextSpan(text: text.substring(lastEnd)));
    }

    return TextSpan(children: children, style: style);
  }

  /// Delete the entire `{skill:...}` tag adjacent to or containing the cursor.
  ///
  /// For backspace: if the cursor is right after a tag OR anywhere inside a
  /// tag, the whole tag is removed.
  /// For forward-delete: if the cursor is right before a tag OR anywhere
  /// inside a tag, the whole tag is removed.
  bool deleteAdjacentSkillTag({bool forward = false}) {
    final text = this.text;
    final cursor = selection.extentOffset;

    for (final match in _skillTagPattern.allMatches(text)) {
      if (forward) {
        // Forward delete: cursor at or inside the tag.
        if (match.start == cursor || (cursor > match.start && cursor < match.end)) {
          _deleteRange(match.start, match.end);
          return true;
        }
      } else {
        // Backspace: cursor right after the tag, or anywhere inside it.
        if (match.end == cursor || (cursor > match.start && cursor <= match.end)) {
          _deleteRange(match.start, match.end);
          return true;
        }
      }
    }

    return false;
  }

  void _deleteRange(int start, int end) {
    final text = this.text;
    final newText = text.replaceRange(start, end, '');
    final newCursor = start.clamp(0, newText.length).toInt();
    value = TextEditingValue(text: newText, selection: TextSelection.collapsed(offset: newCursor));
  }
}

/// The pill widget rendered inline for each `{skill:name}` tag.
class _SkillPill extends StatelessWidget {
  final String name;
  final Color? fontColor;

  const _SkillPill({required this.name, this.fontColor});

  @override
  Widget build(BuildContext context) {
    final theme = WoxThemeUtil.instance.currentTheme.value;
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final color = fontColor ?? safeFromCssColor(theme.queryBoxFontColor);
    final backgroundColor = safeFromCssColor(theme.actionItemActiveBackgroundColor).withAlpha(50);
    final borderColor = safeFromCssColor(theme.actionItemActiveBackgroundColor).withAlpha(100);

    return Container(
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(5), vertical: 1),
      margin: const EdgeInsets.symmetric(horizontal: 4),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(4), border: Border.all(color: borderColor)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.extension_rounded, size: metrics.scaledSpacing(12), color: color.withAlpha(180)),
          SizedBox(width: metrics.scaledSpacing(3)),
          Text(name, style: TextStyle(color: color, fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }
}
