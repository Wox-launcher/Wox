import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_text_measure_util.dart';

class WoxLabel extends StatelessWidget {
  final String label;
  final double width;
  final TextStyle? style;
  final TextAlign textAlign;
  final bool enableTooltipOnOverflow;

  const WoxLabel({super.key, required this.label, required this.width, this.style, this.textAlign = TextAlign.right, this.enableTooltipOnOverflow = true});

  @override
  Widget build(BuildContext context) {
    final resolvedStyle = TextStyle(color: getThemeTextColor(), fontSize: 13).merge(style);
    final textWidget = Text(label, textAlign: textAlign, style: resolvedStyle, maxLines: 1, overflow: TextOverflow.ellipsis);

    return SizedBox(
      width: width,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final maxWidth = constraints.maxWidth;
          final hasLabel = label.trim().isNotEmpty;
          final shouldShowTooltip =
              enableTooltipOnOverflow && hasLabel && maxWidth > 0 && WoxTextMeasureUtil.isTextOverflow(context: context, text: label, style: resolvedStyle, maxWidth: maxWidth);
          final child = SizedBox(width: double.infinity, child: textWidget);

          if (!shouldShowTooltip) {
            return child;
          }

          return WoxTooltip(message: label, child: child);
        },
      ),
    );
  }
}
