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
