import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// A reusable settings panel with the same neutral card treatment used by dense settings dashboards.
class WoxPanel extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final double? borderRadius;
  final bool showBorder;
  final bool showShadow;
  final double? width;
  final double? height;

  const WoxPanel({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.borderRadius,
    this.showBorder = true,
    this.showShadow = true,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final bool darkTheme = isThemeDark();
    final Color cardColor = darkTheme ? getThemePanelBackgroundColor().lighter(4) : Colors.white;
    final Color outlineColor = darkTheme ? Colors.white.withValues(alpha: 0.08) : const Color(0xFFE6EAF0);
    final List<BoxShadow> shadows =
        !showShadow || darkTheme ? const [] : [BoxShadow(color: const Color(0xFF1F2937).withValues(alpha: 0.04), blurRadius: 18, offset: const Offset(0, 8))];

    // Usage and runtime cards used to duplicate slightly different panel colors. Centralizing the
    // treatment here keeps settings dashboards visually consistent while preserving theme-aware
    // contrast for both dark and light appearances.
    return Container(
      width: width,
      height: height,
      padding: padding,
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(borderRadius ?? 8),
        border: showBorder ? Border.all(color: outlineColor) : null,
        boxShadow: shadows,
      ),
      child: child,
    );
  }
}
