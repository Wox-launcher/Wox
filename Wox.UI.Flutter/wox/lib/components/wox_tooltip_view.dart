import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;

class WoxTooltipView extends StatefulWidget {
  final String tooltip;
  final double paddingLeft;
  final double paddingRight;

  const WoxTooltipView({super.key, required this.tooltip, this.paddingLeft = 4.0, this.paddingRight = 4.0});

  @override
  State<WoxTooltipView> createState() => _WoxTooltipViewState();
}

class _WoxTooltipViewState extends State<WoxTooltipView> {
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(left: widget.paddingLeft, right: widget.paddingRight),
      child: material.Tooltip(
        message: widget.tooltip,
        child: const Icon(FluentIcons.info, size: 14),
      ),
    );
  }
}
