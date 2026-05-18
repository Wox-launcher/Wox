import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Shared glass-dark visual tokens for onboarding chrome.
///
/// Onboarding previously mixed feature accents into the frame, active rows, and
/// card surfaces. The glass-dark redesign keeps the frame neutral and derives
/// translucent white overlays from the active theme, so feature colors remain
/// inside the demos while the surrounding tour matches the launcher theme.
class WoxOnboardingGlassStyle {
  const WoxOnboardingGlassStyle._();

  static Color get textColor => getThemeTextColor();

  static Color get subTextColor => getThemeSubTextColor();

  static Color get backgroundColor => getThemeBackgroundColor();

  static Color surface([double alpha = 0.055]) {
    return textColor.withValues(alpha: alpha);
  }

  static Color activeSurface([double alpha = 0.14]) {
    return textColor.withValues(alpha: alpha);
  }

  static Color outline([double alpha = 0.14]) {
    return textColor.withValues(alpha: alpha);
  }

  static Color mutedOutline([double alpha = 0.16]) {
    return subTextColor.withValues(alpha: alpha);
  }

  static Color chromeSurface([double alpha = 0.46]) {
    return backgroundColor.withValues(alpha: alpha);
  }

  static List<BoxShadow> panelShadow([double alpha = 0.14]) {
    return [BoxShadow(color: Colors.black.withValues(alpha: alpha), blurRadius: 28, offset: const Offset(0, 18))];
  }
}
