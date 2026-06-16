import 'package:flutter/material.dart';
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
  final bool fullWidth;
  final double? controlMaxWidth;

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
    this.fullWidth = false,
    this.controlMaxWidth,
  });

  @override
  Widget build(BuildContext context) {
    final labelText = Text(label, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500), maxLines: 2, overflow: TextOverflow.ellipsis);
    final description = tips == null ? null : Padding(padding: EdgeInsets.only(top: tipsTopSpacing), child: tips!);

    if (fullWidth) {
      // Table-sized settings need their title above the control so the table can use the available width; the old left-label layout made dense tables feel cramped.
      return Padding(
        padding: EdgeInsets.only(bottom: bottomSpacing),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [labelText, if (description != null) description, const SizedBox(height: 10), child]),
      );
    }

    return Padding(
      padding: EdgeInsets.only(bottom: bottomSpacing),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final shouldStack = constraints.maxWidth < labelWidth + labelGap + 260;
          final control = controlMaxWidth == null ? child : ConstrainedBox(constraints: BoxConstraints(maxWidth: controlMaxWidth!), child: child);

          if (shouldStack) {
            // Narrow settings panes fall back to a vertical row to prevent controls and descriptions from colliding.
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [labelText, if (description != null) description, const SizedBox(height: 8), Align(alignment: Alignment.centerLeft, child: control)],
            );
          }

          return Row(
            crossAxisAlignment: rowCrossAxisAlignment,
            children: [
              SizedBox(width: labelWidth, child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [labelText, if (description != null) description])),
              SizedBox(width: labelGap),
              Expanded(child: Align(alignment: Alignment.centerRight, child: control)),
            ],
          );
        },
      ),
    );
  }
}
