import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;

class WoxTooltipView extends StatelessWidget {
  final String tooltip;

  const WoxTooltipView({super.key, required this.tooltip});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 4.0, right: 4),
      child: material.Tooltip(
        message: tooltip,
        child: const Icon(FluentIcons.info, size: 14),
      ),
    );
  }
}
