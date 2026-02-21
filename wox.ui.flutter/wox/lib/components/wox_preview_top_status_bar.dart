import 'package:flutter/material.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';

class WoxPreviewTopStatusBarAction {
  final Widget icon;
  final VoidCallback? onPressed;
  final String? tooltip;
  final Color? color;

  const WoxPreviewTopStatusBarAction({required this.icon, this.onPressed, this.tooltip, this.color});
}

class WoxPreviewTopStatusBar extends StatelessWidget {
  final WoxTheme woxTheme;
  final Widget? leading;
  final Widget title;
  final Widget? trailing;
  final List<WoxPreviewTopStatusBarAction> actions;

  const WoxPreviewTopStatusBar({super.key, required this.woxTheme, required this.title, this.leading, this.trailing, this.actions = const []});

  @override
  Widget build(BuildContext context) {
    final borderColor = safeFromCssColor(woxTheme.previewSplitLineColor).withValues(alpha: 0.75);
    final backgroundColor = safeFromCssColor(woxTheme.queryBoxBackgroundColor).withValues(alpha: 0.35);
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);

    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor, width: 1)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          if (leading != null) ...[leading!, const SizedBox(width: 8)],
          Expanded(child: title),
          if (trailing != null) ...[trailing!, const SizedBox(width: 6)],
          ...actions.map((action) {
            return Padding(
              padding: const EdgeInsets.only(left: 2),
              child: IconButton(
                tooltip: action.tooltip,
                onPressed: action.onPressed,
                icon: action.icon,
                iconSize: 18,
                color: action.color ?? fontColor,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints.tightFor(width: 28, height: 28),
                splashRadius: 14,
                visualDensity: VisualDensity.compact,
              ),
            );
          }),
        ],
      ),
    );
  }
}
