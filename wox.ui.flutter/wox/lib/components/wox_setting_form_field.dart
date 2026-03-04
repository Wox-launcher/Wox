import 'package:flutter/material.dart';
import 'package:wox/components/wox_label.dart';

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
    required this.labelWidth,
    this.tips,
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
              Padding(padding: EdgeInsets.only(right: labelGap), child: WoxLabel(label: label, width: labelWidth)),
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
