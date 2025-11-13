import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// A reusable panel component with consistent styling across the app
/// Uses the same styling as runtime settings and other panels
class WoxPanel extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final double? borderRadius;
  final bool showBorder;

  const WoxPanel({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.borderRadius,
    this.showBorder = true,
  });

  @override
  Widget build(BuildContext context) {
    // Calculate panel color similar to runtime settings
    final Color baseBackground = getThemeBackgroundColor();
    final bool isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final Color panelColor = getThemePanelBackgroundColor();
    Color cardColor = panelColor.a < 1 ? Color.alphaBlend(panelColor, baseBackground) : panelColor;
    cardColor = isDarkTheme ? cardColor.lighter(6) : cardColor.darker(4);
    final Color outlineColor = getThemeDividerColor().withValues(alpha: isDarkTheme ? 0.45 : 0.25);

    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(borderRadius ?? 12),
        border: showBorder ? Border.all(color: outlineColor) : null,
      ),
      child: child,
    );
  }
}
