import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingFormField extends StatelessWidget {
  final String label;
  final Widget child;
  final Widget? tips;
  final double labelWidth;
  final double labelGap;
  final double bottomSpacing;
  final double tipsTopSpacing;
  final CrossAxisAlignment rowCrossAxisAlignment;

  const WoxSettingFormField({
    super.key,
    required this.label,
    required this.child,
    this.tips,
    this.labelWidth = 160,
    this.labelGap = 20,
    this.bottomSpacing = 20,
    this.tipsTopSpacing = 2,
    this.rowCrossAxisAlignment = CrossAxisAlignment.center,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: bottomSpacing),
      child: Column(
        children: [
          Row(
            crossAxisAlignment: rowCrossAxisAlignment,
            children: [
              Padding(padding: EdgeInsets.only(right: labelGap), child: SizedBox(width: labelWidth, child: _SettingLabelWithTooltip(label: label))),
              Flexible(child: Align(alignment: Alignment.centerLeft, child: child)),
            ],
          ),
          if (tips != null)
            Padding(
              padding: EdgeInsets.only(top: tipsTopSpacing),
              child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [SizedBox(width: labelWidth + labelGap), Flexible(child: tips!)]),
            ),
        ],
      ),
    );
  }
}

class _SettingLabelWithTooltip extends StatelessWidget {
  final String label;

  const _SettingLabelWithTooltip({required this.label});

  @override
  Widget build(BuildContext context) {
    final textStyle = TextStyle(color: getThemeTextColor(), fontSize: 13);
    final textWidget = Text(label, textAlign: TextAlign.right, style: textStyle, maxLines: 1, overflow: TextOverflow.ellipsis);

    return LayoutBuilder(
      builder: (context, constraints) {
        final maxWidth = constraints.maxWidth;
        if (!maxWidth.isFinite || maxWidth <= 0) {
          return textWidget;
        }

        final textPainter = TextPainter(text: TextSpan(text: label, style: textStyle), maxLines: 1, textDirection: Directionality.of(context))..layout(maxWidth: maxWidth);

        if (!textPainter.didExceedMaxLines) {
          return textWidget;
        }

        return WoxTooltip(message: label, child: textWidget);
      },
    );
  }
}
