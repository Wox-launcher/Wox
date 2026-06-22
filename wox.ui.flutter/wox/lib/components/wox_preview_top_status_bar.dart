import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

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
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final borderColor = safeFromCssColor(woxTheme.previewSplitLineColor).withValues(alpha: 0.75);
    final backgroundColor = fontColor.withValues(alpha: 0.035);

    // The preview status bar is part of the launcher preview surface, so its
    // controls follow density while colors, borders, and radii stay theme-owned.
    return Container(
      margin: EdgeInsets.only(bottom: WoxInterfaceSizeUtil.instance.current.scaledSpacing(6)),
      padding: EdgeInsets.symmetric(horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(10), vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(4)),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor, width: 1)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          if (leading != null) ...[leading!, SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(8))],
          Expanded(child: title),
          if (trailing != null) ...[trailing!, SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(6))],
          ...actions.map((action) {
            final button = IconButton(
              onPressed: action.onPressed,
              icon: action.icon,
              iconSize: WoxInterfaceSizeUtil.instance.current.scaledSpacing(18),
              color: action.color ?? fontColor,
              padding: EdgeInsets.zero,
              constraints: BoxConstraints.tightFor(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(28), height: WoxInterfaceSizeUtil.instance.current.scaledSpacing(28)),
              splashRadius: WoxInterfaceSizeUtil.instance.current.scaledSpacing(14),
              visualDensity: VisualDensity.compact,
            );

            return Padding(
              padding: EdgeInsets.only(left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(2)),
              // Top status-bar actions are reused by AI chat and terminal preview,
              // so this shared wrapper removes IconButton's Material tooltip path
              // for every consumer at once.
              child: WoxTooltip(message: action.tooltip ?? "", child: button),
            );
          }),
        ],
      ),
    );
  }
}
