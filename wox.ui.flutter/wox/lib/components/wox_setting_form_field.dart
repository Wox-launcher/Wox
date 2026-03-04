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

  bool _isTextOverflow({required BuildContext context, required String text, required TextStyle style, required double maxWidth}) {
    // Merge with DefaultTextStyle to match how the Text widget actually renders
    // (Text widget inherits letterSpacing, fontFamily, etc. from the theme)
    final resolvedStyle = DefaultTextStyle.of(context).style.merge(style);
    final textPainter = TextPainter(
      text: TextSpan(text: text, style: resolvedStyle),
      textDirection: Directionality.of(context),
      textScaler: MediaQuery.textScalerOf(context),
      locale: Localizations.maybeLocaleOf(context),
      maxLines: 1,
    )..layout(minWidth: 0, maxWidth: double.infinity);

    return textPainter.width > maxWidth;
  }

  @override
  Widget build(BuildContext context) {
    final textStyle = TextStyle(color: getThemeTextColor(), fontSize: 13);
    final textWidget = Text(label, textAlign: TextAlign.right, style: textStyle, maxLines: 1, overflow: TextOverflow.ellipsis);

    return LayoutBuilder(
      builder: (context, constraints) {
        final maxWidth = constraints.maxWidth;
        final hasLabel = label.trim().isNotEmpty;
        final shouldShowTooltip = hasLabel && maxWidth > 0 && _isTextOverflow(context: context, text: label, style: textStyle, maxWidth: maxWidth);
        final child = SizedBox(width: double.infinity, child: textWidget);

        if (!shouldShowTooltip) {
          return child;
        }

        return WoxTooltip(message: label, child: child);
      },
    );
  }
}
