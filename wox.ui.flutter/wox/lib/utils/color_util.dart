import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';

/// Safely parse CSS color strings with a fallback to a bright red that highlights invalid values.
Color safeFromCssColor(String? cssColor, {Color defaultColor = Colors.redAccent}) {
  if (cssColor == null || cssColor.isEmpty) {
    return defaultColor;
  }

  try {
    return fromCssColor(cssColor);
  } catch (_) {
    return defaultColor;
  }
}

Color getHoverColorFromActiveColor(Color activeColor) {
  // Hover used to replace the active color alpha with a fixed value. That made
  // translucent glass active rows darker on hover, so derive hover opacity by
  // halving the active token's own alpha and preserving its original color.
  final hoverAlpha = (activeColor.a * 0.25).clamp(0.0, 1.0).toDouble();
  return activeColor.withValues(alpha: hoverAlpha);
}
