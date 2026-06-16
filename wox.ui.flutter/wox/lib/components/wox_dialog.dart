import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Shared themed dialog chrome for Wox settings and popup workflows.
class WoxDialog extends StatelessWidget {
  final Widget? title;
  final Widget content;
  final List<Widget>? actions;
  final EdgeInsets insetPadding;
  final EdgeInsetsGeometry contentPadding;
  final EdgeInsetsGeometry actionsPadding;
  final MainAxisAlignment actionsAlignment;
  final TextStyle? titleTextStyle;
  final TextStyle? contentTextStyle;
  final double elevation;

  const WoxDialog({
    super.key,
    this.title,
    required this.content,
    this.actions,
    this.insetPadding = const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
    this.contentPadding = const EdgeInsets.fromLTRB(24, 24, 24, 0),
    this.actionsPadding = const EdgeInsets.fromLTRB(24, 12, 24, 24),
    this.actionsAlignment = MainAxisAlignment.end,
    this.titleTextStyle,
    this.contentTextStyle,
    this.elevation = 18,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = isThemeDark();
    final accentColor = getThemeActiveBackgroundColor();
    final surfaceColor = getThemePopupSurfaceColor();
    final textColor = getThemeTextColor();
    final baseTheme = Theme.of(context);
    final dialogTheme = baseTheme.copyWith(
      colorScheme: ColorScheme.fromSeed(seedColor: accentColor, brightness: isDark ? Brightness.dark : Brightness.light),
      scaffoldBackgroundColor: Colors.transparent,
      cardColor: surfaceColor,
      shadowColor: textColor.withAlpha(50),
    );

    return Theme(
      data: dialogTheme,
      child: AlertDialog(
        backgroundColor: surfaceColor,
        surfaceTintColor: Colors.transparent,
        elevation: elevation,
        insetPadding: insetPadding,
        contentPadding: contentPadding,
        actionsPadding: actionsPadding,
        actionsAlignment: actionsAlignment,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: getThemePopupOutlineColor())),
        title: title,
        titleTextStyle: titleTextStyle ?? TextStyle(color: textColor, fontSize: 16, fontWeight: FontWeight.w700),
        contentTextStyle: contentTextStyle ?? TextStyle(color: textColor, fontSize: 13),
        content: content,
        actions: actions,
      ),
    );
  }
}
